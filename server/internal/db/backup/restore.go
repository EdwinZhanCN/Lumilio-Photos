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
	"strings"
)

// restorePreamble runs at the head of the psql stream, inside the same single
// transaction as the dump itself: kick every other connection off the database
// (their transactions would block the schema drop), then reset the schema so
// --clean dumps restore into a predictable empty state regardless of drift.
// The owning role is a superuser on both shapes, so no OWNER rewriting is
// needed (unlike Immich's streaming transform).
const restorePreamble = `
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE datname = current_database() AND pid <> pg_backend_pid();

DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
GRANT ALL ON SCHEMA public TO public;
`

// RestoreDump streams the dump at dumpPath (plain or gzip SQL) into psql as
// one transaction with ON_ERROR_STOP, preceded by restorePreamble. Either the
// whole dump applies or the database is left untouched.
func RestoreDump(ctx context.Context, conn Conn, toolsBinDir, dumpPath string, logf Logf) error {
	if logf == nil {
		logf = func(string, ...any) {}
	}

	file, err := os.Open(dumpPath)
	if err != nil {
		return fmt.Errorf("open dump: %w", err)
	}
	defer file.Close()

	var dumpReader io.Reader = file
	if strings.HasSuffix(dumpPath, ".gz") {
		gz, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("dump is not valid gzip: %w", err)
		}
		defer gz.Close()
		dumpReader = gz
	}

	cmd := exec.CommandContext(ctx, filepath.Join(toolsBinDir, "psql"),
		"--host", conn.Host,
		"--port", conn.Port,
		"--username", conn.User,
		"--dbname", conn.DBName,
		"--single-transaction",
		"--set", "ON_ERROR_STOP=on",
		"--quiet",
	)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+conn.Password)
	cmd.Stdin = io.MultiReader(strings.NewReader(restorePreamble), dumpReader)
	cmd.Stdout = io.Discard

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	logf("restore: applying %s", filepath.Base(dumpPath))
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 4096 {
			msg = msg[len(msg)-4096:]
		}
		if msg != "" {
			return fmt.Errorf("psql restore: %w\n%s", err, msg)
		}
		return fmt.Errorf("psql restore: %w", err)
	}
	logf("restore: %s applied", filepath.Base(dumpPath))
	return nil
}

// RestoreHooks are the app-level steps around the raw dump application. All
// hooks are optional; nil hooks are skipped.
type RestoreHooks struct {
	// Quiesce runs before the restore point is taken: pause queues so job
	// churn stops mutating the database mid-dump.
	Quiesce func(ctx context.Context) error
	// Resume undoes Quiesce. Always attempted, even on failure.
	Resume func(ctx context.Context) error
	// Migrate brings the restored schema up to the running binary's version
	// (the dump may come from an older app release).
	Migrate func(ctx context.Context) error
	// Verify is the post-restore health check (e.g. settings row readable,
	// users present). A Verify failure triggers rollback.
	Verify func(ctx context.Context) error
}

// RestoreWithRollback restores dumpPath with a safety net: a restore-point
// dump of the current database is taken first, and if the restore, migration,
// or verification fails, the restore point is applied back so the database
// never stays in a broken state. The returned error is the original failure;
// rollback problems are appended to it.
func RestoreWithRollback(ctx context.Context, conn Conn, toolsBinDir, backupsDir, dumpPath, appVersion, pgVersion string, hooks RestoreHooks, logf Logf) (err error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}

	if hooks.Quiesce != nil {
		if qerr := hooks.Quiesce(ctx); qerr != nil {
			return fmt.Errorf("quiesce before restore: %w", qerr)
		}
	}
	if hooks.Resume != nil {
		defer func() {
			if rerr := hooks.Resume(context.WithoutCancel(ctx)); rerr != nil {
				logf("restore: resume after restore failed: %v", rerr)
				if err == nil {
					err = fmt.Errorf("resume after restore: %w", rerr)
				}
			}
		}()
	}

	restorePoint, err := DumpWithPrefix(ctx, conn, toolsBinDir, backupsDir, RestorePointPrefix, appVersion, pgVersion, logf)
	if err != nil {
		return fmt.Errorf("create restore point: %w", err)
	}

	applyAndCheck := func(ctx context.Context, path string) error {
		if aerr := RestoreDump(ctx, conn, toolsBinDir, path, logf); aerr != nil {
			return aerr
		}
		if hooks.Migrate != nil {
			if merr := hooks.Migrate(ctx); merr != nil {
				return fmt.Errorf("run migrations: %w", merr)
			}
		}
		if hooks.Verify != nil {
			if verr := hooks.Verify(ctx); verr != nil {
				return fmt.Errorf("post-restore verification: %w", verr)
			}
		}
		return nil
	}

	if ferr := applyAndCheck(ctx, dumpPath); ferr != nil {
		logf("restore: failed (%v), rolling back to %s", ferr, filepath.Base(restorePoint))
		// The original ctx may be cancelled/expired — the rollback must still run.
		if rberr := applyAndCheck(context.WithoutCancel(ctx), restorePoint); rberr != nil {
			return fmt.Errorf("restore failed: %w; ROLLBACK ALSO FAILED (restore point kept at %s): %v", ferr, restorePoint, rberr)
		}
		return fmt.Errorf("restore failed, database rolled back to its previous state: %w", ferr)
	}

	logf("restore: success (restore point kept at %s)", filepath.Base(restorePoint))
	return nil
}
