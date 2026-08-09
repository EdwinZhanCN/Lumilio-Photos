package storage

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
)

const (
	repositoryLockHelperPathEnv = "LUMILIO_TEST_REPOSITORY_LOCK_PATH"
	repositoryLockHelperKindEnv = "LUMILIO_TEST_REPOSITORY_LOCK_KIND"
)

func TestRepositoryOSLockHelperProcess(t *testing.T) {
	helperPath := os.Getenv(repositoryLockHelperPathEnv)
	if helperPath == "" {
		return
	}
	var (
		release func()
		err     error
	)
	if os.Getenv(repositoryLockHelperKindEnv) == "root" {
		release, err = acquireRootPathLock(context.Background(), helperPath, true)
	} else {
		release, err = acquireRepositoryPathLock(context.Background(), helperPath, true)
	}
	if err != nil {
		os.Exit(2)
	}
	defer release()
	_, _ = os.Stdout.WriteString("locked\n")
	select {}
}

func startStorageLockProcess(t *testing.T, path, kind string) *exec.Cmd {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestRepositoryOSLockHelperProcess$")
	command.Env = append(os.Environ(), repositoryLockHelperPathEnv+"="+path, repositoryLockHelperKindEnv+"="+kind)
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if command.ProcessState == nil {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	})
	if line, readErr := bufio.NewReader(stdout).ReadString('\n'); readErr != nil || line != "locked\n" {
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatalf("lock helper readiness = %q, %v", line, readErr)
	}
	return command
}

func TestRepositoryOSLockCoordinatesIndependentCallers(t *testing.T) {
	repositoryPath := t.TempDir()
	firstRelease, err := acquireRepositoryPathLock(context.Background(), repositoryPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer firstRelease()
	lockInfo, err := InspectRepositoryLock(repositoryPath, "repository")
	if err != nil {
		t.Fatal(err)
	}
	if lockInfo.Holder == "" || lockInfo.AcquiredAt.IsZero() {
		t.Fatalf("lock diagnostics = %#v", lockInfo)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := acquireRepositoryPathLock(ctx, repositoryPath, true); !errors.Is(err, ErrRepositoryLockUnavailable) {
		t.Fatalf("exclusive lock error = %v, want ErrRepositoryLockUnavailable", err)
	}
	firstRelease()
	lockInfo, err = InspectRepositoryLock(repositoryPath, "repository")
	if err != nil {
		t.Fatal(err)
	}
	if lockInfo.Holder != "" || !lockInfo.AcquiredAt.IsZero() {
		t.Fatalf("released lock diagnostics = %#v, want no active holder", lockInfo)
	}

	exclusiveRelease, err := acquireRepositoryPathLock(context.Background(), repositoryPath, true)
	if err != nil {
		t.Fatal(err)
	}
	exclusiveRelease()
	if _, err := os.Stat(filepath.Join(repositoryPath, ".lumiliorepo.lock")); err != nil {
		t.Fatalf("lock evidence file: %v", err)
	}
}

func TestRepositoryOSLockCoordinatesProcessAndReleasesAfterCrash(t *testing.T) {
	repositoryPath := t.TempDir()
	command := startStorageLockProcess(t, repositoryPath, "repository")
	lockInfo, err := InspectRepositoryLock(repositoryPath, "repository")
	if err != nil {
		_ = command.Process.Kill()
		t.Fatal(err)
	}
	if lockInfo.Holder == "" {
		_ = command.Process.Kill()
		t.Fatal("active cross-process lock was not reported")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := acquireRepositoryPathLock(ctx, repositoryPath, true); !errors.Is(err, ErrRepositoryLockUnavailable) {
		_ = command.Process.Kill()
		t.Fatalf("cross-process lock error = %v", err)
	}
	if err := command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = command.Wait()
	lockInfo, err = InspectRepositoryLock(repositoryPath, "repository")
	if err != nil {
		t.Fatal(err)
	}
	if lockInfo.Holder != "" || !lockInfo.AcquiredAt.IsZero() {
		t.Fatalf("crashed lock diagnostics = %#v, want no active holder", lockInfo)
	}
	release, err := acquireRepositoryPathLock(context.Background(), repositoryPath, true)
	if err != nil {
		t.Fatalf("lock remained held after process crash: %v", err)
	}
	release()
}

func TestRuntimeStorageOwnershipRejectsSecondManager(t *testing.T) {
	catalog, first := newCatalogRepositoryManager(t)
	initializeDefaultStorageForTest(t, first, filepath.Join(t.TempDir(), "default"))
	second, err := NewRepositoryManager(catalog.SQL, catalog.Queries, zap.NewNop(), nil, NewRepositoryFSFactory(nil, catalog.Queries))
	if err != nil {
		t.Fatal(err)
	}
	release, err := first.AcquireRuntimeStorageOwnership(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := second.AcquireRuntimeStorageOwnership(ctx); !errors.Is(err, ErrRepositoryLockUnavailable) {
		t.Fatalf("second runtime ownership error = %v, want ErrRepositoryLockUnavailable", err)
	}
}

func TestNetworkFilesystemClassificationFailsClosed(t *testing.T) {
	for _, filesystem := range []string{"nfs", "nfs4", "cifs", "smbfs", "fuse.sshfs", "9p"} {
		if !networkFilesystem(filesystem) {
			t.Errorf("networkFilesystem(%q) = false", filesystem)
		}
	}
	if networkFilesystem("apfs") || networkFilesystem("ext4") {
		t.Fatal("local filesystem classified as network")
	}
}
