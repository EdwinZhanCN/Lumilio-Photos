package storage

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"
)

func TestCapacityDecisionUsesActualTargetVolumeAndSafetyMargin(t *testing.T) {
	info := StoragePathInfo{Writable: true, CapacityKnown: true, TotalBytes: 100 << 30, AvailableBytes: 20 << 30}
	decision := capacityDecision("repo", "/target", info, 16<<30)
	if decision.SafetyMargin != 5<<30 {
		t.Fatalf("safety margin = %d, want 5 GiB", decision.SafetyMargin)
	}
	if decision.Allowed {
		t.Fatalf("decision allowed with %d available and %d required", decision.AvailableBytes, decision.RequiredBytes)
	}

	info.AvailableBytes = 21 << 30
	if decision = capacityDecision("repo", "/target", info, 16<<30); !decision.Allowed {
		t.Fatalf("decision unexpectedly rejected: %+v", decision)
	}
}

func TestWritePreflightRejectsReplacedParentStorageIdentity(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	rootPath := filepath.Join(t.TempDir(), "default")
	initializeDefaultStorageForTest(t, manager, rootPath)
	repository, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}
	original, err := rootcfg.Load(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	replacement := rootcfg.New("Replacement")
	if err := replacement.Save(rootPath); err != nil {
		t.Fatal(err)
	}

	if _, err := manager.CheckRepositoryWriteCapacity(ctx, repository.RepoID.String(), 1); !errors.Is(err, ErrRepositoryUnavailable) {
		t.Fatalf("write preflight error = %v, want ErrRepositoryUnavailable", err)
	}
	if _, err := manager.files.Open(repository); !errors.Is(err, ErrRepositoryUnavailable) {
		t.Fatalf("RepositoryFS open error = %v, want ErrRepositoryUnavailable", err)
	}
	if _, err := manager.RenameRepository(ctx, repository.RepoID.String(), "Must Not Be Written",
		LifecycleRequest{RequestID: "rename-replaced-parent", Actor: "test"}); !errors.Is(err, ErrRepositoryUnavailable) {
		t.Fatalf("rename pre-write error = %v, want ErrRepositoryUnavailable", err)
	}
	diskRepository, err := repocfg.LoadConfigFromFile(repository.Path)
	if err != nil {
		t.Fatal(err)
	}
	if diskRepository.Name == "Must Not Be Written" {
		t.Fatal("rename wrote the repository marker after parent identity changed")
	}
	sidecarPath := filepath.Join(repository.Path, DefaultStructure.SidecarsDir, "asset-1.lumilio-sidecar")
	if err := manager.WriteRepositorySidecar(ctx, repository.RepoID.String(), "asset-1", []byte("{}")); !errors.Is(err, ErrRepositoryUnavailable) {
		t.Fatalf("sidecar pre-write error = %v, want ErrRepositoryUnavailable", err)
	}
	for _, path := range []string{sidecarPath, sidecarPath + ".tmp"} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("sidecar rejection left %s: %v", path, err)
		}
	}
	if matches, err := filepath.Glob(sidecarPath + ".tmp-*"); err != nil || len(matches) != 0 {
		t.Fatalf("sidecar rejection left temporary files %v (glob error %v)", matches, err)
	}
	root, err := manager.queries.GetRepositoryRoot(ctx, repository.RootID)
	if err != nil {
		t.Fatal(err)
	}
	if root.Status != dbtypes.RepositoryRootStatusError {
		t.Fatalf("parent status = %q, want error", root.Status)
	}
	if err := original.Save(rootPath); err != nil {
		t.Fatal(err)
	}
	runtimeStatus, err := manager.StorageRuntimeStatus(ctx)
	if err != nil || runtimeStatus.State != StorageRuntimeStateActive {
		t.Fatalf("explicit coordination status = %+v, error = %v", runtimeStatus, err)
	}
}

func TestSidecarWriteRejectsRootAndRepositoryMaintenance(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	initializeDefaultStorageForTest(t, manager, filepath.Join(t.TempDir(), "default"))
	repository, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}
	root, err := manager.queries.GetRepositoryRoot(ctx, repository.RootID)
	if err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := manager.queries.UpdateRepositoryRootFromDisk(ctx, repo.UpdateRepositoryRootFromDiskParams{
		RootID: root.RootID, Name: root.Name, Status: dbtypes.RepositoryRootStatusMaintenance, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := manager.WriteRepositorySidecar(ctx, repository.RepoID.String(), "asset-root", []byte("{}")); err == nil {
		t.Fatal("sidecar write accepted root maintenance")
	}
	if _, err := manager.queries.UpdateRepositoryRootFromDisk(ctx, repo.UpdateRepositoryRootFromDiskParams{
		RootID: root.RootID, Name: root.Name, Status: dbtypes.RepositoryRootStatusActive, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.queries.BeginRepositoryMaintenance(ctx, repo.BeginRepositoryMaintenanceParams{
		RepoID: repository.RepoID, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := manager.WriteRepositorySidecar(ctx, repository.RepoID.String(), "asset-repository", []byte("{}")); !errors.Is(err, ErrRepositoryBusy) {
		t.Fatalf("sidecar repository-maintenance error = %v, want ErrRepositoryBusy", err)
	}
	for _, assetID := range []string{"asset-root", "asset-repository"} {
		target := filepath.Join(repository.Path, DefaultStructure.SidecarsDir, assetID+".lumilio-sidecar")
		for _, path := range []string{target, target + ".tmp"} {
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("sidecar maintenance rejection left %s: %v", path, err)
			}
		}
		if matches, err := filepath.Glob(target + ".tmp-*"); err != nil || len(matches) != 0 {
			t.Fatalf("sidecar maintenance rejection left temporary files %v (glob error %v)", matches, err)
		}
	}
}

func TestSidecarWriteHonorsContextWhileRepositoryLeaseIsBusy(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	initializeDefaultStorageForTest(t, manager, filepath.Join(t.TempDir(), "default"))
	repository, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}
	releaseMutation := manager.files.AccessCoordinator().AcquireMutation(repository.RepoID)
	defer releaseMutation()

	requestCtx, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
	defer cancel()
	assetID := "asset-busy"
	err = manager.WriteRepositorySidecar(requestCtx, repository.RepoID.String(), assetID, []byte("{}"))
	if !errors.Is(err, ErrRepositoryBusy) {
		t.Fatalf("sidecar busy error = %v, want ErrRepositoryBusy", err)
	}
	target := filepath.Join(repository.Path, DefaultStructure.SidecarsDir, assetID+".lumilio-sidecar")
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("sidecar busy rejection left target %s: %v", target, err)
	}
	if matches, err := filepath.Glob(target + ".tmp-*"); err != nil || len(matches) != 0 {
		t.Fatalf("sidecar busy rejection left temporary files %v (glob error %v)", matches, err)
	}
	current, err := manager.queries.GetRepository(ctx, repository.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Activity != dbtypes.RepositoryActivityIdle {
		t.Fatalf("repository activity = %q after cancelled sidecar wait, want idle", current.Activity)
	}
}

func TestCapacityDecisionRejectsReadOnlyAndAllowsUnknownCapacity(t *testing.T) {
	readOnly := capacityDecision("repo", "/target", StoragePathInfo{Writable: false}, 1)
	if readOnly.Allowed {
		t.Fatal("read-only target was allowed")
	}
	unknown := capacityDecision("repo", "/target", StoragePathInfo{Writable: true}, 1<<40)
	if !unknown.Allowed || unknown.CapacityKnown {
		t.Fatalf("unknown-capacity decision = %+v", unknown)
	}
}

func TestKnownSizePreflightPausesRepositoryBeforeWriting(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	initializeDefaultStorageForTest(t, manager, filepath.Join(t.TempDir(), "default"))
	repository, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}

	decision, err := manager.CheckRepositoryWriteCapacity(ctx, repository.RepoID.String(), math.MaxUint64)
	if !errors.Is(err, ErrInsufficientSpace) {
		t.Fatalf("capacity error = %v, want ErrInsufficientSpace", err)
	}
	if decision.Allowed {
		t.Fatalf("overflowing known write was allowed: %+v", decision)
	}
	updated, err := manager.queries.GetRepository(ctx, repository.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Activity != dbtypes.RepositoryActivityPaused {
		t.Fatalf("repository activity = %q, want paused", updated.Activity)
	}
	if updated.PauseReason != "low_space" {
		t.Fatalf("repository pause reason = %q, want low_space", updated.PauseReason)
	}
}

func TestCapacityRecoveryResumesOnlyLowSpacePause(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	initializeDefaultStorageForTest(t, manager, filepath.Join(t.TempDir(), "default"))
	repository, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := manager.queries.PauseRepositoryForLowSpace(ctx, repo.PauseRepositoryForLowSpaceParams{
		RepoID: repository.RepoID, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	paused, err := manager.queries.GetRepository(ctx, repository.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.resumeRepositoryAfterCapacityRecovery(ctx, paused, StoragePathInfo{
		Writable: true, CapacityKnown: true, TotalBytes: 10 << 30, AvailableBytes: 10 << 30,
	}); err != nil {
		t.Fatal(err)
	}
	recovered, err := manager.queries.GetRepository(ctx, repository.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Activity != dbtypes.RepositoryActivityIdle || recovered.PauseReason != "" {
		t.Fatalf("capacity recovery state = activity %q reason %q", recovered.Activity, recovered.PauseReason)
	}

	if _, err := manager.queries.UpdateRepositoryActivity(ctx, repo.UpdateRepositoryActivityParams{
		RepoID: repository.RepoID, Activity: dbtypes.RepositoryActivityPaused, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	manualBefore, err := manager.queries.GetRepository(ctx, repository.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed, err := manager.resumeRepositoryAfterCapacityRecovery(ctx, manualBefore, StoragePathInfo{
		Writable: true, CapacityKnown: true, TotalBytes: 10 << 30, AvailableBytes: 10 << 30,
	}); err != nil || resumed {
		t.Fatalf("manual pause resume result = %v, error = %v", resumed, err)
	}
	manual, err := manager.queries.GetRepository(ctx, repository.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	if manual.Activity != dbtypes.RepositoryActivityPaused || manual.PauseReason != "manual" {
		t.Fatalf("manual pause was incorrectly resumed: activity %q reason %q", manual.Activity, manual.PauseReason)
	}
}
