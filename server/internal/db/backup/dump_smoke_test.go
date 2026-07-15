package backup

import (
	"compress/gzip"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDumpSmoke exercises a real pg_dump round trip: init a throwaway cluster,
// create a table, Dump, gunzip, and check the DDL survived. Skipped when no
// PostgreSQL binaries are available (mirrors the desktop supervisor's
// lifecycle smoke test). Requires a non-root user, like initdb itself.
func TestDumpSmoke(t *testing.T) {
	binDir := os.Getenv("LUMILIO_PG_BIN_DIR")
	if binDir == "" {
		if p, err := exec.LookPath("initdb"); err == nil {
			binDir = filepath.Dir(p)
		} else {
			t.Skip("no PostgreSQL binaries found (set LUMILIO_PG_BIN_DIR) — skipping dump smoke test")
		}
	}
	if os.Geteuid() == 0 {
		t.Skip("initdb cannot run as root — skipping dump smoke test")
	}

	ctx := context.Background()
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	sockDir, err := os.MkdirTemp("/tmp", "lumilio-dump-")
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
	if err := os.WriteFile(pwFile, []byte("dump-smoke-pw"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("initdb", "-D", dataDir, "-U", "lumilio", "--auth=scram-sha-256", "--pwfile="+pwFile, "--encoding=UTF8", "--locale=C")
	run("pg_ctl", "start", "-w", "-t", "60", "-D", dataDir, "-l", filepath.Join(root, "pg.log"),
		"-o", "-p 5490 -c listen_addresses='' -c unix_socket_directories="+sockDir)
	t.Cleanup(func() { run("pg_ctl", "stop", "-m", "fast", "-w", "-D", dataDir) })

	psql := exec.CommandContext(ctx, filepath.Join(binDir, "psql"),
		"--host", sockDir, "--port", "5490", "--username", "lumilio", "--dbname", "postgres",
		"-c", "CREATE TABLE smoke_assets (id int primary key, note text); INSERT INTO smoke_assets VALUES (1, 'hello');")
	psql.Env = append(os.Environ(), "PGPASSWORD=dump-smoke-pw")
	if out, err := psql.CombinedOutput(); err != nil {
		t.Fatalf("psql seed: %v\n%s", err, out)
	}

	conn := Conn{Host: sockDir, Port: "5490", User: "lumilio", Password: "dump-smoke-pw", DBName: "postgres"}
	destDir := filepath.Join(root, "backups")
	path, err := Dump(ctx, conn, binDir, destDir, "smoke", "17.0", t.Logf)
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if !IsRoutineName(filepath.Base(path)) {
		t.Fatalf("dump filename %q is not a routine backup name", filepath.Base(path))
	}
	if latest, ok := LatestRoutine(destDir); !ok || time.Since(latest) > time.Minute {
		t.Fatalf("LatestRoutine after dump = %v/%v", latest, ok)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("dump is not valid gzip: %v", err)
	}
	sql, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("read dump: %v", err)
	}
	for _, want := range []string{"CREATE TABLE public.smoke_assets", "hello", "DROP TABLE IF EXISTS"} {
		if !strings.Contains(string(sql), want) {
			t.Errorf("dump missing %q", want)
		}
	}
}
