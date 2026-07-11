package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// pgBinDirForTest resolves a PostgreSQL bin directory for the lifecycle smoke
// test, or skips when none is available (e.g. CI without PostgreSQL). The major
// version need not match the bundled one — this only exercises the lifecycle.
func pgBinDirForTest(t *testing.T) string {
	t.Helper()
	if v := os.Getenv("LUMILIO_PG_BIN_DIR"); v != "" {
		return v
	}
	if p, err := exec.LookPath("initdb"); err == nil {
		return filepath.Dir(p)
	}
	t.Skip("no PostgreSQL binaries found (set LUMILIO_PG_BIN_DIR) — skipping lifecycle smoke test")
	return ""
}

// TestPostgresLifecycleSmoke exercises the real initdb → start → pg_isready →
// createdb → stop cycle against a local PostgreSQL, covering validation item #1
// of the desktop exec plan. It is skipped automatically when no PostgreSQL is
// installed.
func TestPostgresLifecycleSmoke(t *testing.T) {
	binDir := pgBinDirForTest(t)

	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	logsDir := filepath.Join(root, "logs")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(logsDir, 0o700); err != nil {
		t.Fatal(err)
	}

	// Use a short /tmp socket directory to stay within the Unix socket path
	// limit regardless of where the test temp dir lives.
	sockDir, err := os.MkdirTemp("/tmp", "lumilio-smoke-")
	if err != nil {
		t.Fatalf("create socket dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sockDir) })

	// initdb sets the superuser password from this file (--pwfile); the cluster
	// requires scram-sha-256 from its first start, so it must exist up front.
	passwordFile := filepath.Join(root, "db_password")
	if err := os.WriteFile(passwordFile, []byte("smoke-test-password"), 0o600); err != nil {
		t.Fatalf("write password file: %v", err)
	}

	pg := NewPostgres(PostgresOptions{
		BinDir:       binDir,
		DataDir:      dataDir,
		Host:         sockDir,
		LogsDir:      logsDir,
		Port:         "5489",
		User:         "lumilio",
		DBName:       "lumilio_smoke",
		PasswordFile: passwordFile,
		Logf:         t.Logf,
	})

	ctx := context.Background()

	if pg.IsInitialized() {
		t.Fatal("fresh data dir should not report initialized")
	}
	if err := pg.InitDB(ctx); err != nil {
		if strings.Contains(err.Error(), "could not create shared memory segment") ||
			strings.Contains(err.Error(), "Operation not permitted") {
			t.Skipf("PostgreSQL initdb cannot create shared memory in this environment: %v", err)
		}
		t.Fatalf("InitDB: %v", err)
	}
	if !pg.IsInitialized() {
		t.Fatal("data dir should report initialized after InitDB")
	}
	if err := pg.WriteConfigs(); err != nil {
		t.Fatalf("WriteConfigs: %v", err)
	}
	if err := pg.HandleStaleState(ctx); err != nil {
		t.Fatalf("HandleStaleState: %v", err)
	}
	if err := pg.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Always attempt a stop so a failure mid-test does not leave a postmaster.
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), pgStopTimeout)
		defer cancel()
		_ = pg.Stop(stopCtx)
	})

	if err := pg.WaitReady(ctx, 30*time.Second); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
	if err := pg.CreateDB(ctx); err != nil {
		t.Fatalf("CreateDB: %v", err)
	}
	// CreateDB must be idempotent across launches.
	if err := pg.CreateDB(ctx); err != nil {
		t.Fatalf("CreateDB (idempotent re-run): %v", err)
	}

	// The cluster must actually enforce scram: a wrong password has to be
	// rejected, otherwise the generated pg_hba silently regressed to trust.
	wrongCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	wrongDSN := fmt.Sprintf(
		"host=%s port=5489 user=lumilio password=not-the-password dbname=postgres sslmode=disable", sockDir)
	if conn, err := pgx.Connect(wrongCtx, wrongDSN); err == nil {
		_ = conn.Close(wrongCtx)
		t.Fatal("connection with a wrong password was accepted — auth regressed to trust")
	}

	if err := pg.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
