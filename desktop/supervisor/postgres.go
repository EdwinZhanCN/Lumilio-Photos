package supervisor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Postgres manages the lifecycle of the private, bundled PostgreSQL instance:
// initdb, configuration, start/stop, readiness, and database creation. On unix
// it only ever speaks over a Unix socket (no TCP port is opened), so it cannot
// collide with a system PostgreSQL or be reached from the network. Windows
// PostgreSQL has no Unix sockets, so there the postmaster binds the IPv4
// loopback only (127.0.0.1:<port>).
type Postgres struct {
	binDir       string // directory containing initdb/pg_ctl/postgres/pg_isready/createdb
	dataDir      string
	host         string // unix socket directory, or 127.0.0.1 on Windows
	logsDir      string
	port         string
	user         string
	dbName       string
	passwordFile string
	logf         func(string, ...any)
}

// PostgresOptions configures a Postgres controller.
type PostgresOptions struct {
	BinDir  string
	DataDir string
	// Host is the connection target: a Unix socket directory on unix hosts,
	// or 127.0.0.1 on Windows (use Paths.DBHost).
	Host         string
	LogsDir      string
	Port         string
	User         string
	DBName       string
	PasswordFile string
	Logf         func(string, ...any)
}

// NewPostgres builds a Postgres controller from options, filling in defaults.
func NewPostgres(opts PostgresOptions) *Postgres {
	logf := opts.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	port := opts.Port
	if port == "" {
		port = pgPort
	}
	return &Postgres{
		binDir:       opts.BinDir,
		dataDir:      opts.DataDir,
		host:         opts.Host,
		logsDir:      opts.LogsDir,
		port:         port,
		user:         opts.User,
		dbName:       opts.DBName,
		passwordFile: opts.PasswordFile,
		logf:         logf,
	}
}

func (p *Postgres) bin(name string) string {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(p.binDir, name)
}

// IsInitialized reports whether dataDir already contains an initialized cluster.
func (p *Postgres) IsInitialized() bool {
	_, err := os.Stat(filepath.Join(p.dataDir, "PG_VERSION"))
	return err == nil
}

// DataDirStatus classifies the on-disk state of the data directory before
// startup, so the caller can decide between a normal start, initdb, clearing a
// half-written directory, or refusing to touch data written by a different
// PostgreSQL major version.
type DataDirStatus int

const (
	// DataDirEmpty: the directory is absent or empty — run initdb.
	DataDirEmpty DataDirStatus = iota
	// DataDirValid: an initialized cluster matching the expected major version.
	DataDirValid
	// DataDirVersionMismatch: a cluster initialized by a different PostgreSQL
	// major version. Starting the bundled binaries against it would FATAL, and
	// silently re-initializing would destroy user data.
	DataDirVersionMismatch
	// DataDirIncomplete: leftover files without PG_VERSION — an initdb that
	// crashed midway. The directory never held a valid cluster, so it is safe
	// to clear and re-init.
	DataDirIncomplete
)

// DataDirStatus inspects the data directory and reports how startup should
// proceed. expectedMajor is the bundled PostgreSQL major version (e.g. "17");
// when empty, the version check is skipped and any initialized cluster counts
// as valid. The returned string is the major version found in PG_VERSION, if
// any.
func (p *Postgres) DataDirStatus(expectedMajor string) (DataDirStatus, string) {
	data, err := os.ReadFile(filepath.Join(p.dataDir, "PG_VERSION"))
	if err == nil {
		found := strings.TrimSpace(string(data))
		if expectedMajor == "" || found == expectedMajor {
			return DataDirValid, found
		}
		return DataDirVersionMismatch, found
	}
	entries, err := os.ReadDir(p.dataDir)
	if err != nil || len(entries) == 0 {
		return DataDirEmpty, ""
	}
	return DataDirIncomplete, ""
}

// ResetDataDir clears a never-valid data directory (see DataDirIncomplete) so
// initdb can run on a clean slate — initdb refuses a non-empty target, so
// without this a crashed first-run initdb would wedge every later launch. It
// refuses to remove an initialized cluster.
func (p *Postgres) ResetDataDir() error {
	if p.IsInitialized() {
		return fmt.Errorf("refusing to reset initialized data directory %s", p.dataDir)
	}
	if err := os.RemoveAll(p.dataDir); err != nil {
		return fmt.Errorf("reset data dir %s: %w", p.dataDir, err)
	}
	if err := os.MkdirAll(p.dataDir, 0o700); err != nil {
		return fmt.Errorf("recreate data dir %s: %w", p.dataDir, err)
	}
	return nil
}

// InitDB initializes a fresh cluster owned by the configured user. The
// superuser password is set from passwordFile at initdb time (--pwfile), so
// the cluster requires scram-sha-256 auth from its very first start (see
// pgHBAConf) and is never open on trust. This matters on Windows, where the
// loopback listener is reachable by every local user and process.
func (p *Postgres) InitDB(ctx context.Context) error {
	if info, err := os.Stat(p.passwordFile); err != nil {
		return fmt.Errorf("initdb: password file must be generated first: %w", err)
	} else if info.Size() == 0 {
		return fmt.Errorf("initdb: password file %s is empty", p.passwordFile)
	}
	p.logf("initdb: initializing cluster at %s", p.dataDir)
	return p.run(ctx, "initdb",
		"-D", p.dataDir,
		"-U", p.user,
		"--auth=scram-sha-256",
		"--pwfile="+p.passwordFile,
		"--encoding=UTF8",
		"--locale=C",
	)
}

// WriteConfigs (over)writes postgresql.conf and pg_hba.conf. postgresql.conf is
// rewritten every launch so settings (especially the socket directory, which
// may fall back to /tmp) stay in sync with the resolved paths.
func (p *Postgres) WriteConfigs() error {
	if runtime.GOOS != "windows" {
		if err := os.MkdirAll(p.host, 0o700); err != nil {
			return fmt.Errorf("create socket dir %s: %w", p.host, err)
		}
	}
	if err := os.MkdirAll(p.logsDir, 0o700); err != nil {
		return fmt.Errorf("create logs dir %s: %w", p.logsDir, err)
	}

	conf := fmt.Sprintf(`# Generated by Lumilio Photos desktop supervisor. Do not edit by hand.
%s
port = %s
shared_buffers = 64MB
work_mem = 4MB
maintenance_work_mem = 16MB
# Comfortably above the API connection pool (sized to CPU count) plus River's
# per-queue producers and PostgreSQL's reserved superuser connections. The
# plan's original 10 caused transient "too many clients" errors at startup.
max_connections = 50
wal_level = minimal
max_wal_senders = 0
logging_collector = on
log_directory = '%s'
log_filename = 'postgresql-%%Y-%%m-%%d.log'
log_rotation_age = 1d
log_rotation_size = 10MB
`, pgListenConf(runtime.GOOS, p.host), p.port, pgConfPathValue(p.logsDir))

	if err := os.WriteFile(filepath.Join(p.dataDir, "postgresql.conf"), []byte(conf), 0o600); err != nil {
		return fmt.Errorf("write postgresql.conf: %w", err)
	}

	if err := os.WriteFile(filepath.Join(p.dataDir, "pg_hba.conf"), []byte(pgHBAConf(runtime.GOOS)), 0o600); err != nil {
		return fmt.Errorf("write pg_hba.conf: %w", err)
	}
	return nil
}

// pgListenConf returns the listener configuration: unix hosts speak only over
// the Unix socket (no TCP), Windows binds the IPv4 loopback only because its
// PostgreSQL builds have no Unix socket support.
func pgListenConf(goos, host string) string {
	if goos == "windows" {
		return "listen_addresses = '127.0.0.1'"
	}
	return fmt.Sprintf("listen_addresses = ''\nunix_socket_directories = '%s'", pgConfPathValue(host))
}

// pgHBAConf returns client auth rules matching the listener setup. Every rule
// requires scram-sha-256, never trust: on unix the 0700 socket directory is
// already a same-user boundary, but on Windows the loopback listener is
// reachable by every local user and process, so password auth is the only
// thing between the cluster and other users on the machine. ("local" lines are
// rejected by Unix-socket-less Windows builds, hence the per-OS split.)
func pgHBAConf(goos string) string {
	if goos == "windows" {
		return "# Generated by Lumilio Photos desktop supervisor.\n" +
			"host   all   all   127.0.0.1/32   scram-sha-256\n"
	}
	return "# Generated by Lumilio Photos desktop supervisor.\nlocal   all   all   scram-sha-256\n"
}

// HandleStaleState recovers from a previous unclean shutdown (force quit or
// crash) by either stopping a still-running leftover postmaster or clearing a
// stale postmaster.pid so a fresh start is not refused.
func (p *Postgres) HandleStaleState(ctx context.Context) error {
	pidFile := filepath.Join(p.dataDir, "postmaster.pid")
	if _, err := os.Stat(pidFile); err != nil {
		return nil // no pid file, nothing to recover
	}

	switch p.statusCode(ctx) {
	case 0:
		// Still running from a previous session; stop it cleanly.
		p.logf("recovery: leftover postgres is running, stopping")
		return p.Stop(ctx)
	default:
		// Not running but pid file present → stale; remove it.
		p.logf("recovery: removing stale postmaster.pid")
		if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove stale pid file: %w", err)
		}
		return nil
	}
}

// statusCode returns the pg_ctl status exit code (0 = running, 3 = not running,
// 4 = bad data dir). Any non-ExitError is treated as "not running".
func (p *Postgres) statusCode(ctx context.Context) int {
	cmd := exec.CommandContext(ctx, p.bin("pg_ctl"), "status", "-D", p.dataDir)
	hideConsole(cmd)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		return 3
	}
	return 0
}

// Start launches the postmaster and waits (pg_ctl -w) for it to accept
// connections.
func (p *Postgres) Start(ctx context.Context) error {
	p.logf("postgres: starting")
	logPath := filepath.Join(p.logsDir, "postgres.log")
	// pg_ctl start spawns the long-lived postmaster as a grandchild. On Windows
	// that grandchild inherits pg_ctl's stdout/stderr handles; if those are an
	// os/exec capture pipe, the pipe never reaches EOF while the postmaster lives,
	// so the stdout-copier — and therefore cmd.Wait / Start — blocks forever even
	// though the server is already accepting connections (the tray hangs at
	// "starting database"). Direct pg_ctl's own output to a real file instead: Go
	// then passes the file handle directly (no pipe, no copier goroutine), so Run
	// returns as soon as pg_ctl itself exits, and the postmaster inheriting the
	// file handle is harmless.
	ctlLogPath := filepath.Join(p.logsDir, "pg_ctl.log")
	err := p.runToFile(ctx, ctlLogPath, "pg_ctl",
		"start",
		"-D", p.dataDir,
		"-l", logPath,
		"-w",
		"-t", "60",
	)
	if err == nil {
		return nil
	}
	// pg_ctl's own failure message is only "could not start server / Examine
	// the log output"; the actual postmaster error lives in postgres.log. Fold
	// both tails into the error so the real cause is visible without hunting for
	// the log file (which the setup UI cannot open).
	msg := strings.TrimSpace(tailFile(ctlLogPath, 4096))
	if tail := tailFile(logPath, 4096); tail != "" {
		msg = strings.TrimSpace(msg + "\n--- postgres.log ---\n" + tail)
	}
	return fmt.Errorf("pg_ctl start: %w\n%s", err, msg)
}

// tailFile returns up to maxBytes of the end of the file at path, or "" if it
// cannot be read. Used to surface the postmaster log tail on a start failure.
func tailFile(path string, maxBytes int64) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return ""
	}
	if size := info.Size(); size > maxBytes {
		if _, err := f.Seek(size-maxBytes, 0); err != nil {
			return ""
		}
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// Stop performs a fast shutdown (rollback in-flight, then exit) bounded by a
// timeout.
func (p *Postgres) Stop(ctx context.Context) error {
	p.logf("postgres: stopping")
	return p.run(ctx, "pg_ctl",
		"stop",
		"-D", p.dataDir,
		"-m", "fast",
		"-w",
		"-t", "30",
	)
}

// WaitReady polls pg_isready with exponential backoff until the server accepts
// connections or the timeout elapses.
func (p *Postgres) WaitReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	backoff := 100 * time.Millisecond
	var lastErr error
	for {
		if err := p.isReady(ctx); err == nil {
			p.logf("postgres: ready")
			return nil
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("postgres not ready after %s: %w", timeout, lastErr)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 2*time.Second {
			backoff *= 2
		}
	}
}

func (p *Postgres) isReady(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, p.bin("pg_isready"),
		"-h", p.host,
		"-p", p.port,
	)
	hideConsole(cmd)
	return cmd.Run()
}

// CreateDB creates the application database if it does not already exist. It is
// idempotent: existence is checked against the cluster catalog first (see
// databaseExists) so it does not depend on parsing createdb's translated
// "already exists" message — critical because the bundle ships no psql and a
// localized Windows PostgreSQL could emit that message in the OS language,
// which would otherwise fail every launch after the first.
func (p *Postgres) CreateDB(ctx context.Context) error {
	if exists, err := p.databaseExists(ctx); err != nil {
		p.logf("createdb: existence check failed, falling back to createdb: %v", err)
	} else if exists {
		return nil
	}

	password, err := p.password()
	if err != nil {
		return err
	}
	out, err := p.outputEnv(ctx, []string{"PGPASSWORD=" + password}, "createdb",
		"-h", p.host,
		"-p", p.port,
		"-U", p.user,
		p.dbName,
	)
	if err == nil {
		p.logf("createdb: created database %q", p.dbName)
		return nil
	}
	// Locale-independent primary check above already handled the common case; the
	// English match remains only as a best-effort fallback (LC_ALL=C output).
	if strings.Contains(out, "already exists") {
		return nil
	}
	return fmt.Errorf("createdb %q: %w (%s)", p.dbName, err, strings.TrimSpace(out))
}

// password reads the cluster superuser password from the password file.
func (p *Postgres) password() (string, error) {
	data, err := os.ReadFile(p.passwordFile)
	if err != nil {
		return "", fmt.Errorf("read db password file: %w", err)
	}
	pw := strings.TrimSpace(string(data))
	if pw == "" {
		return "", fmt.Errorf("db password file %s is empty", p.passwordFile)
	}
	return pw, nil
}

// databaseExists reports whether the application database is present, by querying
// the cluster catalog on the maintenance ("postgres") database, authenticating
// with the generated password (the cluster requires scram-sha-256 everywhere).
func (p *Postgres) databaseExists(ctx context.Context) (bool, error) {
	connCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	password, err := p.password()
	if err != nil {
		return false, err
	}
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable", p.host, p.port, p.user, password)
	conn, err := pgx.Connect(connCtx, dsn)
	if err != nil {
		return false, err
	}
	defer conn.Close(connCtx)

	var exists bool
	if err := conn.QueryRow(connCtx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)", p.dbName,
	).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// run executes a bundled binary, surfacing combined output on failure.
func (p *Postgres) run(ctx context.Context, name string, args ...string) error {
	out, err := p.output(ctx, name, args...)
	if err != nil {
		return fmt.Errorf("%s: %w\n%s", name, err, strings.TrimSpace(out))
	}
	return nil
}

// runToFile executes a bundled binary with its stdout/stderr directed to a real
// file at outPath rather than a captured pipe. This is required for commands that
// leave a long-lived daemon behind (pg_ctl start): with a pipe, the daemon's
// inherited write handle would keep the pipe open and block cmd.Wait forever on
// Windows. With a plain file, Wait only waits for the direct child to exit.
func (p *Postgres) runToFile(ctx context.Context, outPath, name string, args ...string) error {
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", outPath, err)
	}
	defer f.Close()

	cmd := exec.CommandContext(ctx, p.bin(name), args...)
	hideConsole(cmd)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LC_MESSAGES=C", "LANG=C")
	cmd.Stdout = f
	cmd.Stderr = f
	return cmd.Run()
}

func (p *Postgres) output(ctx context.Context, name string, args ...string) (string, error) {
	return p.outputEnv(ctx, nil, name, args...)
}

// outputEnv is output with extra environment entries (e.g. PGPASSWORD for the
// client tools now that the cluster requires password auth).
func (p *Postgres) outputEnv(ctx context.Context, extraEnv []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, p.bin(name), args...)
	hideConsole(cmd)
	// Force the C locale for messages so the PostgreSQL tools emit English
	// ASCII rather than the OS-locale encoding (e.g. GBK on a Chinese Windows),
	// which would render as mojibake in the setup UI. The cluster's own
	// encoding/locale is fixed at initdb time (--encoding=UTF8 --locale=C) and
	// is unaffected by these message-only overrides.
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LC_MESSAGES=C", "LANG=C")
	cmd.Env = append(cmd.Env, extraEnv...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

// pgConfEscape escapes single quotes for a PostgreSQL configuration string value.
func pgConfEscape(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// pgConfPathValue formats a filesystem path as a PostgreSQL configuration string
// value. PostgreSQL accepts forward slashes on every platform, and its config
// parser treats backslashes inside quoted strings as escape characters — so a
// raw Windows path like C:\Users\...\logs would be silently mangled (\U, \l, …),
// producing a bad log_directory and a postmaster that FATALs at startup while
// initdb (which never reads the file) succeeds. Normalizing to forward slashes
// avoids that; single quotes are still doubled. The backslash rewrite is
// unconditional (not host-OS dependent) so a Windows path is normalized even
// when the conf is generated or tested on another platform.
func pgConfPathValue(path string) string {
	return pgConfEscape(strings.ReplaceAll(path, `\`, "/"))
}
