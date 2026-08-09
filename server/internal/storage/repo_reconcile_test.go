package storage

import (
	"os"
	"path/filepath"
	"testing"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func reconcileTestManager(t *testing.T) *DefaultRepositoryManager {
	t.Helper()
	manager, err := NewRepositoryManager(nil, nil, zap.NewNop(), nil, nil)
	if err != nil {
		t.Fatalf("NewRepositoryManager: %v", err)
	}
	return manager
}

// writeRepositoryAt creates a repository directory carrying id and returns the
// database row that would represent it.
func writeRepositoryAt(t *testing.T, path string, id uuid.UUID) repo.Repository {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	config := repocfg.NewRepositoryConfig("Test Repo")
	config.ID = id.String()
	if err := config.SaveConfigToFile(path); err != nil {
		t.Fatalf("save config: %v", err)
	}
	return repo.Repository{
		RepoID:       id,
		Path:         path,
		Reachability: dbtypes.RepositoryReachabilityActive,
	}
}

func TestInspectRepositoryOnDiskReportsActiveWhenIDMatches(t *testing.T) {
	manager := reconcileTestManager(t)
	id := uuid.New()
	row := writeRepositoryAt(t, filepath.Join(canonicalTempDir(t), "library"), id)

	status, config := manager.inspectRepositoryOnDisk(row)

	if status != dbtypes.RepositoryReachabilityActive {
		t.Fatalf("status = %q, want active", status)
	}
	if config == nil || config.ID != id.String() {
		t.Fatalf("config not returned for refresh: %+v", config)
	}
}

// An unplugged drive is offline, never error: the data is elsewhere, not gone.
func TestInspectRepositoryOnDiskReportsOfflineWhenPathMissing(t *testing.T) {
	manager := reconcileTestManager(t)
	id := uuid.New()
	row := repo.Repository{
		RepoID:       id,
		Path:         filepath.Join(canonicalTempDir(t), "unplugged", "library"),
		Reachability: dbtypes.RepositoryReachabilityActive,
	}

	status, config := manager.inspectRepositoryOnDisk(row)

	if status != dbtypes.RepositoryReachabilityOffline {
		t.Fatalf("status = %q, want offline", status)
	}
	if config != nil {
		t.Fatal("offline repository must not refresh the config cache")
	}
}

// A different repository at the recorded path is not something the server may
// resolve on its own.
func TestInspectRepositoryOnDiskReportsErrorWhenIDDiffers(t *testing.T) {
	manager := reconcileTestManager(t)
	row := writeRepositoryAt(t, filepath.Join(canonicalTempDir(t), "library"), uuid.New())
	row.RepoID = uuid.New()

	status, config := manager.inspectRepositoryOnDisk(row)

	if status != dbtypes.RepositoryReachabilityIdentityError {
		t.Fatalf("status = %q, want identity_error", status)
	}
	if config != nil {
		t.Fatal("mismatched repository must not refresh the config cache")
	}
}

func TestInspectRepositoryOnDiskReportsErrorWhenConfigUnparseable(t *testing.T) {
	manager := reconcileTestManager(t)
	dir := filepath.Join(canonicalTempDir(t), "library")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".lumiliorepo"), []byte("not: [valid"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	row := repo.Repository{
		RepoID:       uuid.New(),
		Path:         dir,
		Reachability: dbtypes.RepositoryReachabilityActive,
	}

	status, _ := manager.inspectRepositoryOnDisk(row)

	if status != dbtypes.RepositoryReachabilityIdentityError {
		t.Fatalf("status = %q, want identity_error", status)
	}
}

// A remounted drive must come back on its own, without user action.
func TestInspectRepositoryOnDiskRecoversFromOffline(t *testing.T) {
	manager := reconcileTestManager(t)
	id := uuid.New()
	path := filepath.Join(canonicalTempDir(t), "library")
	row := repo.Repository{
		RepoID:       id,
		Path:         path,
		Reachability: dbtypes.RepositoryReachabilityOffline,
	}

	if status, _ := manager.inspectRepositoryOnDisk(row); status != dbtypes.RepositoryReachabilityOffline {
		t.Fatalf("status = %q, want offline before remount", status)
	}

	writeRepositoryAt(t, path, id)

	if status, _ := manager.inspectRepositoryOnDisk(row); status != dbtypes.RepositoryReachabilityActive {
		t.Fatalf("status = %q, want active after remount", status)
	}
}

// A lifecycle operation owns maintenance until its journal commits or rolls
// back, so ordinary reachability reconciliation must not clear the barrier.
func TestReconcileRepositorySkipsMaintenance(t *testing.T) {
	manager := reconcileTestManager(t)
	row := repo.Repository{
		RepoID:       uuid.New(),
		Path:         filepath.Join(canonicalTempDir(t), "gone"),
		Reachability: dbtypes.RepositoryReachabilityMaintenance,
		Activity:     dbtypes.RepositoryActivityPaused,
	}

	// queries is nil, so any attempt to write would panic.
	if err := manager.reconcileRepository(t.Context(), row); err != nil {
		t.Fatalf("reconcileRepository returned error: %v", err)
	}
}
