package backup

import (
	"compress/gzip"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// smokeCluster spins up a throwaway scram-authed cluster on a Unix socket and
// returns its Conn plus the bin dir. Skips when PostgreSQL is unavailable.
func smokeCluster(t *testing.T, port string) (Conn, string) {
	t.Helper()
	binDir := os.Getenv("LUMILIO_PG_BIN_DIR")
	if binDir == "" {
		if p, err := exec.LookPath("initdb"); err == nil {
			binDir = filepath.Dir(p)
		} else {
			t.Skip("no PostgreSQL binaries found (set LUMILIO_PG_BIN_DIR) — skipping smoke test")
		}
	}
	if os.Geteuid() == 0 {
		t.Skip("initdb cannot run as root — skipping smoke test")
	}

	ctx := context.Background()
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	sockDir, err := os.MkdirTemp("/tmp", "lumilio-restore-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sockDir) })

	run := func(name string, args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, filepath.Join(binDir, name), args...)
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		if out, err := cmd.CombinedOutput(); err != nil {
			if name == "initdb" && (strings.Contains(string(out), "could not create shared memory segment") || strings.Contains(string(out), "Operation not permitted")) {
				t.Skipf("PostgreSQL initdb cannot create shared memory in this environment: %v\n%s", err, out)
			}
			t.Fatalf("%s: %v\n%s", name, err, out)
		}
	}

	pwFile := filepath.Join(root, "pw")
	if err := os.WriteFile(pwFile, []byte("restore-smoke-pw"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("initdb", "-D", dataDir, "-U", "lumilio", "--auth=scram-sha-256", "--pwfile="+pwFile, "--encoding=UTF8", "--locale=C")
	run("pg_ctl", "start", "-w", "-t", "60", "-D", dataDir, "-l", filepath.Join(root, "pg.log"),
		"-o", "-p "+port+" -c listen_addresses='' -c unix_socket_directories="+sockDir)
	t.Cleanup(func() { run("pg_ctl", "stop", "-m", "fast", "-w", "-D", dataDir) })

	return Conn{Host: sockDir, Port: port, User: "lumilio", Password: "restore-smoke-pw", DBName: "postgres"}, binDir
}

func smokePSQL(t *testing.T, conn Conn, binDir, sql string) string {
	t.Helper()
	cmd := exec.Command(filepath.Join(binDir, "psql"),
		"--host", conn.Host, "--port", conn.Port, "--username", conn.User, "--dbname", conn.DBName,
		"--tuples-only", "--no-align", "-c", sql)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+conn.Password, "LC_ALL=C")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("psql %q: %v\n%s", sql, err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestRestoreSmoke exercises the full loop: seed → dump → mutate → restore →
// original data back.
func TestRestoreSmoke(t *testing.T) {
	conn, binDir := smokeCluster(t, "5491")
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "backups")

	smokePSQL(t, conn, binDir, "CREATE TABLE albums (id int primary key, title text); INSERT INTO albums VALUES (1, 'holiday');")

	dump, err := Dump(ctx, conn, binDir, dir, "smoke", "17.0", t.Logf)
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}

	smokePSQL(t, conn, binDir, "UPDATE albums SET title = 'wrecked'; CREATE TABLE junk (id int);")

	if err := RestoreDump(ctx, conn, binDir, dump, t.Logf); err != nil {
		t.Fatalf("RestoreDump: %v", err)
	}

	if got := smokePSQL(t, conn, binDir, "SELECT title FROM albums WHERE id = 1"); got != "holiday" {
		t.Fatalf("albums.title after restore = %q, want %q", got, "holiday")
	}
	if got := smokePSQL(t, conn, binDir, "SELECT to_regclass('public.junk') IS NULL"); got != "t" {
		t.Fatal("table created after the dump must be gone after restore")
	}
}

// TestRestoreWithRollbackSmoke verifies a corrupt dump triggers rollback to
// the restore point and the database keeps its pre-restore state.
func TestRestoreWithRollbackSmoke(t *testing.T) {
	conn, binDir := smokeCluster(t, "5492")
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "backups")

	smokePSQL(t, conn, binDir, "CREATE TABLE albums (id int primary key, title text); INSERT INTO albums VALUES (1, 'precious');")

	// A syntactically broken gzip "dump".
	badPath := filepath.Join(dir, FileName(nowForTest(), "smoke", "17.0"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(badPath)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	if _, err := gz.Write([]byte("THIS IS NOT SQL;\n")); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	verified := false
	hooks := RestoreHooks{
		Verify: func(context.Context) error { verified = true; return nil },
	}
	err = RestoreWithRollback(ctx, conn, binDir, dir, badPath, "smoke", "17.0", hooks, t.Logf)
	if err == nil {
		t.Fatal("restoring a corrupt dump must fail")
	}
	if !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("error should report rollback, got: %v", err)
	}
	if !verified {
		t.Fatal("rollback path must also run verification")
	}

	if got := smokePSQL(t, conn, binDir, "SELECT title FROM albums WHERE id = 1"); got != "precious" {
		t.Fatalf("albums.title after rollback = %q, want %q", got, "precious")
	}
}

func nowForTest() (t time.Time) { return time.Now() }
