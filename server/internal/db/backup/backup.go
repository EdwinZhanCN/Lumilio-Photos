// Package backup implements the app-driven PostgreSQL logical-backup engine
// shared by the Docker server and the desktop supervisor (see
// exec-plans/active/db-backup-upgrade.md). It shells out to a pg_dump whose
// major version matches the connected server, writes gzip dumps atomically
// (.tmp + rename) with provenance-carrying filenames, and prunes by count.
package backup

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Conn is the connection target for the dump tools. Host may be a Unix socket
// directory (desktop) or a hostname (Docker); Password is the resolved
// plaintext password (config loading already applies password_file).
type Conn struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// Logf matches the supervisor-style logging callback used across the app.
type Logf func(format string, args ...any)

// ErrUnsupportedTools means no pg_dump matching the server's major version
// could be located; dumping with a mismatched client is never attempted.
type ErrUnsupportedTools struct {
	Major int
	Tried []string
}

func (e *ErrUnsupportedTools) Error() string {
	return fmt.Sprintf("no PostgreSQL %d client tools found (tried: %s)", e.Major, strings.Join(e.Tried, ", "))
}

// RowQuerier is the slice of pgxpool.Pool the version probe needs.
type RowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ServerVersion asks the connected server for its version ("17.5") and major
// (17). Always ask the server, never the binaries: the tools are chosen to
// match the server, not the other way around.
func ServerVersion(ctx context.Context, q RowQuerier) (string, int, error) {
	var v string
	if err := q.QueryRow(ctx, "SHOW server_version").Scan(&v); err != nil {
		return "", 0, fmt.Errorf("query server_version: %w", err)
	}
	major, err := majorOf(v)
	if err != nil {
		return "", 0, err
	}
	return v, major, nil
}

var majorRe = regexp.MustCompile(`^(\d+)`)

func majorOf(version string) (int, error) {
	m := majorRe.FindStringSubmatch(strings.TrimSpace(version))
	if m == nil {
		return 0, fmt.Errorf("unparseable PostgreSQL version %q", version)
	}
	return strconv.Atoi(m[1])
}

// LocateTools resolves the directory holding pg_dump/psql for the given server
// major version. Resolution order:
//
//  1. binDirOverride (desktop: the bundled bin dir) — trusted, only checked
//     for the binary's existence;
//  2. the Debian/PGDG layout /usr/lib/postgresql/<major>/bin (Docker);
//  3. pg_dump on PATH, accepted only when `pg_dump --version` reports the
//     same major (dev machines).
func LocateTools(binDirOverride string, major int) (string, error) {
	tried := make([]string, 0, 3)

	if dir := strings.TrimSpace(binDirOverride); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, "pg_dump")); err == nil {
			return dir, nil
		}
		tried = append(tried, dir)
	}

	debianDir := fmt.Sprintf("/usr/lib/postgresql/%d/bin", major)
	if _, err := os.Stat(filepath.Join(debianDir, "pg_dump")); err == nil {
		return debianDir, nil
	}
	tried = append(tried, debianDir)

	if pathBin, err := exec.LookPath("pg_dump"); err == nil {
		out, verr := exec.Command(pathBin, "--version").Output()
		if verr == nil {
			// "pg_dump (PostgreSQL) 17.5"
			fields := strings.Fields(strings.TrimSpace(string(out)))
			if len(fields) > 0 {
				if clientMajor, merr := majorOf(fields[len(fields)-1]); merr == nil && clientMajor == major {
					return filepath.Dir(pathBin), nil
				}
			}
		}
		tried = append(tried, pathBin)
	} else {
		tried = append(tried, "$PATH")
	}

	return "", &ErrUnsupportedTools{Major: major, Tried: tried}
}

// Dump runs pg_dump --clean --if-exists against conn and writes a gzip dump
// into destDir, returning the final path. The dump is written to a .tmp file
// and renamed only after pg_dump exits cleanly, so a completed filename always
// means a complete dump.
func Dump(ctx context.Context, conn Conn, toolsBinDir, destDir, appVersion, pgVersion string, logf Logf) (string, error) {
	return DumpWithPrefix(ctx, conn, toolsBinDir, destDir, "", appVersion, pgVersion, logf)
}

// DumpWithPrefix is Dump with a filename prefix; RestorePointPrefix keeps
// pre-restore safety dumps out of routine retention.
func DumpWithPrefix(ctx context.Context, conn Conn, toolsBinDir, destDir, prefix, appVersion, pgVersion string, logf Logf) (string, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("create backup dir %s: %w", destDir, err)
	}

	finalPath := filepath.Join(destDir, prefix+FileName(time.Now(), appVersion, pgVersion))
	tmpPath := finalPath + TmpSuffix

	cmd := exec.CommandContext(ctx, filepath.Join(toolsBinDir, "pg_dump"),
		"--clean",
		"--if-exists",
		"--host", conn.Host,
		"--port", conn.Port,
		"--username", conn.User,
		"--dbname", conn.DBName,
	)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+conn.Password)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("pg_dump stdout pipe: %w", err)
	}

	file, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", tmpPath, err)
	}

	fail := func(cause error) (string, error) {
		file.Close()
		os.Remove(tmpPath)
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 4096 {
			msg = msg[len(msg)-4096:]
		}
		if msg != "" {
			return "", fmt.Errorf("pg_dump: %w\n%s", cause, msg)
		}
		return "", fmt.Errorf("pg_dump: %w", cause)
	}

	gz := gzip.NewWriter(file)
	logf("backup: dumping %s to %s", conn.DBName, filepath.Base(finalPath))

	if err := cmd.Start(); err != nil {
		return fail(err)
	}
	if _, err := io.Copy(gz, stdout); err != nil {
		_ = cmd.Wait()
		return fail(err)
	}
	if err := cmd.Wait(); err != nil {
		return fail(err)
	}
	if err := gz.Close(); err != nil {
		return fail(err)
	}
	if err := file.Sync(); err != nil {
		return fail(err)
	}
	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("close %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("finalize backup: %w", err)
	}

	logf("backup: wrote %s", finalPath)
	return finalPath, nil
}

// Prune enforces count-based retention on routine backups in dir (newest kept
// first, ordered by the filename timestamp) and removes stale .tmp leftovers
// from failed runs. Restore points are never pruned here.
func Prune(dir string, keep int, logf Logf) ([]string, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if keep < 1 {
		keep = 1
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read backup dir %s: %w", dir, err)
	}

	var routine []string
	var removed []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		switch {
		case IsRoutineName(name):
			routine = append(routine, name)
		case strings.HasSuffix(name, suffix+TmpSuffix):
			// A .tmp is a failed run — unless it is the one currently being
			// written. The db_backup queue runs one worker, so anything older
			// than a generous dump window is safely dead.
			if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > time.Hour {
				if err := os.Remove(filepath.Join(dir, name)); err == nil {
					removed = append(removed, name)
				}
			}
		}
	}

	// Filename timestamps sort lexicographically; newest first.
	sort.Sort(sort.Reverse(sort.StringSlice(routine)))
	for _, name := range routine[min(keep, len(routine)):] {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return removed, fmt.Errorf("prune %s: %w", name, err)
		}
		removed = append(removed, name)
	}

	if len(removed) > 0 {
		logf("backup: pruned %d file(s): %s", len(removed), strings.Join(removed, ", "))
	}
	return removed, nil
}

// LatestRoutine returns the creation time of the newest routine backup in dir,
// or zero when none exists. It trusts filename timestamps, so due-ness survives
// restarts without any database state.
func LatestRoutine(dir string) (time.Time, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return time.Time{}, false
	}
	var latest time.Time
	found := false
	for _, e := range entries {
		if info, ok := ParseName(e.Name()); ok && info.CreatedAt.After(latest) {
			latest = info.CreatedAt
			found = true
		}
	}
	return latest, found
}
