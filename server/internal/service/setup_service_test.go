package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

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

	result, err := svc.Initialize(context.Background(), SetupRequest{})
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

	// Setup rotated the credential; a second setup is refused (the rotated
	// secret on disk is the db_rotated gate).
	if _, err := svc.Initialize(context.Background(), SetupRequest{}); !errors.Is(err, ErrSystemAlreadyInitialized) {
		t.Fatalf("expected ErrSystemAlreadyInitialized, got %v", err)
	}
}

func TestSetupService_Initialize_RotationFailureLeavesSystemUninitialized(t *testing.T) {
	rotator := &fakeRotator{err: errors.New("connection refused")}
	svc, _ := newTestSetupService(t, rotator)

	if _, err := svc.Initialize(context.Background(), SetupRequest{}); err == nil {
		t.Fatal("expected error when rotation fails")
	}

	if _, err := os.Stat(svc.secretPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("secret must not be written when rotation fails")
	}
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.Initialized {
		t.Fatal("system must remain uninitialized after a failed setup")
	}
}

func TestSetupService_Initialize_SecretWriteFailureDoesNotRotate(t *testing.T) {
	rotator := &fakeRotator{}
	svc, dir := newTestSetupService(t, rotator)

	blockingFile := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(blockingFile, []byte("blocked"), 0o600); err != nil {
		t.Fatalf("write blocking file: %v", err)
	}
	svc.secretPath = filepath.Join(blockingFile, "db_password")

	if _, err := svc.Initialize(context.Background(), SetupRequest{}); err == nil {
		t.Fatal("expected secret write error")
	}
	if rotator.calls != 0 {
		t.Fatalf("database password must not rotate after secret write failure, got %d calls", rotator.calls)
	}
}

func TestSetupService_Initialize_SerializesConcurrentCalls(t *testing.T) {
	rotator := &fakeRotator{}
	svc, _ := newTestSetupService(t, rotator)

	const callers = 2
	errs := make([]error, callers)
	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			_, errs[index] = svc.Initialize(context.Background(), SetupRequest{})
		}(i)
	}

	close(start)
	wg.Wait()

	successes := 0
	alreadyInitialized := 0
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrSystemAlreadyInitialized):
			alreadyInitialized++
		default:
			t.Fatalf("unexpected initialize error: %v", err)
		}
	}

	if successes != 1 || alreadyInitialized != callers-1 {
		t.Fatalf("expected one success and %d initialized refusals, got successes=%d initialized=%d errs=%v",
			callers-1, successes, alreadyInitialized, errs)
	}
	if rotator.calls != 1 {
		t.Fatalf("expected one password rotation, got %d", rotator.calls)
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
