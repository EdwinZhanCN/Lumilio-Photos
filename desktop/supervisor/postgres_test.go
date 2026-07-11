package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestPostgres(t *testing.T) *Postgres {
	t.Helper()
	root := t.TempDir()
	return NewPostgres(PostgresOptions{
		BinDir:       filepath.Join(root, "bin"), // never executed in these tests
		DataDir:      filepath.Join(root, "data"),
		Host:         filepath.Join(root, "run"),
		LogsDir:      filepath.Join(root, "logs"),
		User:         "lumilio",
		DBName:       "lumilio_test",
		PasswordFile: filepath.Join(root, "db_password"),
		Logf:         t.Logf,
	})
}

func TestPGHBAConfRequiresScram(t *testing.T) {
	for _, goos := range []string{"darwin", "linux", "windows"} {
		conf := pgHBAConf(goos)
		if strings.Contains(conf, "trust") {
			t.Errorf("pg_hba for %s must not use trust auth:\n%s", goos, conf)
		}
		if !strings.Contains(conf, "scram-sha-256") {
			t.Errorf("pg_hba for %s must require scram-sha-256:\n%s", goos, conf)
		}
	}
	if !strings.Contains(pgHBAConf("windows"), "host   all   all   127.0.0.1/32") {
		t.Error("windows pg_hba must scope the host rule to the IPv4 loopback")
	}
	if !strings.Contains(pgHBAConf("darwin"), "local   all   all") {
		t.Error("unix pg_hba must use a local (socket) rule")
	}
}

func TestDataDirStatus(t *testing.T) {
	writeVersion := func(t *testing.T, p *Postgres, v string) {
		t.Helper()
		if err := os.MkdirAll(p.dataDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p.dataDir, "PG_VERSION"), []byte(v+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("absent or empty dir is empty", func(t *testing.T) {
		p := newTestPostgres(t)
		if status, _ := p.DataDirStatus("17"); status != DataDirEmpty {
			t.Fatalf("absent dir: got %v, want DataDirEmpty", status)
		}
		if err := os.MkdirAll(p.dataDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if status, _ := p.DataDirStatus("17"); status != DataDirEmpty {
			t.Fatalf("empty dir: got %v, want DataDirEmpty", status)
		}
	})

	t.Run("matching version is valid", func(t *testing.T) {
		p := newTestPostgres(t)
		writeVersion(t, p, "17")
		status, found := p.DataDirStatus("17")
		if status != DataDirValid || found != "17" {
			t.Fatalf("got %v/%q, want DataDirValid/17", status, found)
		}
	})

	t.Run("empty expectation skips version check", func(t *testing.T) {
		p := newTestPostgres(t)
		writeVersion(t, p, "14")
		if status, _ := p.DataDirStatus(""); status != DataDirValid {
			t.Fatalf("got %v, want DataDirValid when expectation is empty", status)
		}
	})

	t.Run("other major version is a mismatch", func(t *testing.T) {
		p := newTestPostgres(t)
		writeVersion(t, p, "17")
		status, found := p.DataDirStatus("18")
		if status != DataDirVersionMismatch || found != "17" {
			t.Fatalf("got %v/%q, want DataDirVersionMismatch/17", status, found)
		}
	})

	t.Run("leftovers without PG_VERSION are incomplete", func(t *testing.T) {
		p := newTestPostgres(t)
		if err := os.MkdirAll(filepath.Join(p.dataDir, "base"), 0o700); err != nil {
			t.Fatal(err)
		}
		if status, _ := p.DataDirStatus("17"); status != DataDirIncomplete {
			t.Fatalf("got %v, want DataDirIncomplete", status)
		}
	})
}

func TestResetDataDir(t *testing.T) {
	p := newTestPostgres(t)
	leftover := filepath.Join(p.dataDir, "base", "1")
	if err := os.MkdirAll(leftover, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := p.ResetDataDir(); err != nil {
		t.Fatalf("ResetDataDir on incomplete dir: %v", err)
	}
	if _, err := os.Stat(leftover); !os.IsNotExist(err) {
		t.Fatal("leftovers should be removed")
	}
	if entries, err := os.ReadDir(p.dataDir); err != nil || len(entries) != 0 {
		t.Fatalf("data dir should exist and be empty after reset (entries=%v, err=%v)", entries, err)
	}

	// A directory holding an initialized cluster must never be reset.
	if err := os.WriteFile(filepath.Join(p.dataDir, "PG_VERSION"), []byte("17\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := p.ResetDataDir(); err == nil {
		t.Fatal("ResetDataDir must refuse an initialized data directory")
	}
}

func TestInitDBRequiresPasswordFile(t *testing.T) {
	p := newTestPostgres(t)
	if err := p.InitDB(context.Background()); err == nil || !strings.Contains(err.Error(), "password file") {
		t.Fatalf("InitDB without a password file should fail before exec, got: %v", err)
	}

	if err := os.WriteFile(p.passwordFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := p.InitDB(context.Background()); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("InitDB with an empty password file should fail before exec, got: %v", err)
	}
}

func TestPasswordReadsAndTrims(t *testing.T) {
	p := newTestPostgres(t)
	if _, err := p.password(); err == nil {
		t.Fatal("password() should fail when the file is missing")
	}
	if err := os.WriteFile(p.passwordFile, []byte("s3cret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pw, err := p.password()
	if err != nil || pw != "s3cret" {
		t.Fatalf("got %q/%v, want s3cret trimmed", pw, err)
	}
}
