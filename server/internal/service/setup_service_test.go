package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"server/config"
)

type fakeRotator struct {
	calls    int
	username string
	password string
	err      error
}

func (f *fakeRotator) RotatePassword(_ context.Context, username, newPassword string) error {
	f.calls++
	f.username = username
	f.password = newPassword
	return f.err
}

func newTestSetupService(t *testing.T, rotator DBCredentialRotator) (*SetupService, string) {
	t.Helper()
	dir := t.TempDir()
	return &SetupService{
		dbConfig: config.DatabaseConfig{
			Host: "db", Port: "5432", User: "postgres", Password: "postgres",
			DBName: "lumiliophotos", SSL: "disable",
		},
		rotator:    rotator,
		secretPath: filepath.Join(dir, "secrets", "db_password"),
		configPath: filepath.Join(dir, "config", "system.toml"),
		now:        func() time.Time { return time.Unix(1700000000, 0).UTC() },
	}, dir
}

func TestSetupService_Initialize_RotatesPersistsAndMarksInitialized(t *testing.T) {
	rotator := &fakeRotator{}
	svc, _ := newTestSetupService(t, rotator)

	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.Initialized {
		t.Fatal("expected uninitialized system before setup")
	}

	result, err := svc.Initialize(context.Background(), SetupRequest{
		SiteName:      "My Library",
		AdminUsername: "admin",
	})
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}

	// 32-char high-entropy alphanumeric password handed to the rotator.
	if rotator.calls != 1 {
		t.Fatalf("expected rotator called once, got %d", rotator.calls)
	}
	if rotator.username != "postgres" {
		t.Fatalf("expected ALTER USER on postgres, got %q", rotator.username)
	}
	if len(rotator.password) != generatedPasswordLength {
		t.Fatalf("expected %d-char password, got %d", generatedPasswordLength, len(rotator.password))
	}
	if !regexp.MustCompile(`^[A-Za-z0-9]+$`).MatchString(rotator.password) {
		t.Fatalf("password is not alphanumeric: %q", rotator.password)
	}
	if result.PasswordLength != generatedPasswordLength {
		t.Fatalf("unexpected reported password length %d", result.PasswordLength)
	}

	// Secret persisted with 0600 perms and matching the rotated password.
	secretInfo, err := os.Stat(svc.secretPath)
	if err != nil {
		t.Fatalf("stat secret: %v", err)
	}
	if perm := secretInfo.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected secret perms 0600, got %o", perm)
	}
	secretBytes, err := os.ReadFile(svc.secretPath)
	if err != nil {
		t.Fatalf("read secret: %v", err)
	}
	if strings.TrimSpace(string(secretBytes)) != rotator.password {
		t.Fatal("persisted secret does not match rotated password")
	}

	// Non-sensitive metadata persisted as TOML, and never the secret.
	configInfo, err := os.Stat(svc.configPath)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if perm := configInfo.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected config perms 0600, got %o", perm)
	}
	configBytes, err := os.ReadFile(svc.configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	configText := string(configBytes)
	for _, want := range []string{"My Library", "admin", "initialized = true", "rotated = true"} {
		if !strings.Contains(configText, want) {
			t.Fatalf("system config missing %q:\n%s", want, configText)
		}
	}
	if strings.Contains(configText, rotator.password) {
		t.Fatal("system config must never contain the database password")
	}

	// Status flips to initialized; a second setup is refused.
	status, err = svc.Status(context.Background())
	if err != nil {
		t.Fatalf("status after init: %v", err)
	}
	if !status.Initialized {
		t.Fatal("expected initialized system after setup")
	}
	if _, err := svc.Initialize(context.Background(), SetupRequest{SiteName: "x"}); !errors.Is(err, ErrSystemAlreadyInitialized) {
		t.Fatalf("expected ErrSystemAlreadyInitialized, got %v", err)
	}
}

func TestSetupService_Initialize_RotationFailureLeavesSystemUninitialized(t *testing.T) {
	rotator := &fakeRotator{err: errors.New("connection refused")}
	svc, _ := newTestSetupService(t, rotator)

	if _, err := svc.Initialize(context.Background(), SetupRequest{SiteName: "x"}); err == nil {
		t.Fatal("expected error when rotation fails")
	}

	if _, err := os.Stat(svc.secretPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("secret must not be written when rotation fails")
	}
	if _, err := os.Stat(svc.configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("system config must not be written when rotation fails")
	}
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.Initialized {
		t.Fatal("system must remain uninitialized after a failed setup")
	}
}

func TestGenerateHighEntropyPassword_UniqueAndSized(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 64; i++ {
		pw, err := generateHighEntropyPassword(generatedPasswordLength)
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if len(pw) != generatedPasswordLength {
			t.Fatalf("expected length %d, got %d", generatedPasswordLength, len(pw))
		}
		if _, dup := seen[pw]; dup {
			t.Fatal("generated duplicate password")
		}
		seen[pw] = struct{}{}
	}
}
