package storage

import (
	"bytes"
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

// parentStorageLocationFailures covers the Storage Location faults that must
// never deny a registered child's own work. Each fault leaves the child path,
// marker, and original bytes intact.
var parentStorageLocationFailures = []string{"missing_marker", "invalid_marker", "replaced_marker", "offline", "error", "maintenance"}

// failParentStorageLocationForTest makes only the Storage Location unhealthy.
func failParentStorageLocationForTest(
	t *testing.T,
	manager *DefaultRepositoryManager,
	rootPath string,
	repository repo.Repository,
	failure string,
) {
	t.Helper()
	ctx := context.Background()
	var err error
	switch failure {
	case "missing_marker":
		err = os.Remove(filepath.Join(rootPath, rootcfg.FileName))
	case "invalid_marker":
		err = os.WriteFile(filepath.Join(rootPath, rootcfg.FileName), []byte("invalid marker"), 0o644)
	case "replaced_marker":
		err = rootcfg.New("Replacement Storage Location").Save(rootPath)
	default:
		storageLocation, loadErr := manager.queries.GetStorageLocation(ctx, repository.StorageLocationID)
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		_, err = manager.queries.UpdateStorageLocationFromDisk(ctx, repo.UpdateStorageLocationFromDiskParams{
			StorageLocationID: storageLocation.StorageLocationID, Name: storageLocation.Name, Status: dbtypes.StorageLocationStatus(failure),
			UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
		})
	}
	if err != nil {
		t.Fatal(err)
	}
}

// F09: write admission is owned by the child's own path and marker. A parent
// Storage Location fault never denies capacity preflight, sidecar publication,
// or rename, and child work never rewrites parent registration.
func TestChildWritePathsIgnoreParentStorageLocationFailure(t *testing.T) {
	for _, failure := range parentStorageLocationFailures {
		t.Run(failure, func(t *testing.T) {
			_, manager := newCatalogRepositoryManager(t)
			ctx := context.Background()
			rootPath := filepath.Join(t.TempDir(), "default")
			initializeDefaultStorageForTest(t, manager, rootPath)
			repository, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
			if err != nil {
				t.Fatal(err)
			}
			failParentStorageLocationForTest(t, manager, rootPath, repository, failure)
			parentBefore, err := manager.queries.GetStorageLocation(ctx, repository.StorageLocationID)
			if err != nil {
				t.Fatal(err)
			}

			sidecarData := []byte(`{"asset":"parent-independent"}`)
			if err := manager.WriteRepositorySidecar(ctx, repository.RepoID.String(), "asset-parent-independent", sidecarData); err != nil {
				t.Fatalf("sidecar write denied by parent %s: %v", failure, err)
			}
			stored, err := manager.ReadRepositorySidecar(ctx, repository.RepoID.String(), "asset-parent-independent")
			if err != nil || !bytes.Equal(stored, sidecarData) {
				t.Fatalf("sidecar round trip = %q, %v", stored, err)
			}

			renamed, err := manager.RenameRepository(ctx, repository.RepoID.String(), "Renamed Child",
				LifecycleRequest{RequestID: "rename-parent-independent-" + failure, Actor: "test"})
			if err != nil {
				t.Fatalf("rename denied by parent %s: %v", failure, err)
			}
			if renamed.Name != "Renamed Child" || renamed.RepoID != repository.RepoID {
				t.Fatalf("renamed repository = %+v", renamed)
			}
			diskRepository, err := repocfg.LoadConfigFromFile(repository.Path)
			if err != nil {
				t.Fatal(err)
			}
			if diskRepository.Name != "Renamed Child" || diskRepository.ID != repository.RepoID.String() {
				t.Fatalf("renamed marker = %+v", diskRepository)
			}

			// The host volume may legitimately veto a write on its own (for
			// example real low free space), so the preflight runs after the
			// other child work and is only required to never produce a
			// parent-attributed denial.
			decision, err := manager.CheckRepositoryWriteCapacity(ctx, repository.RepoID.String(), 1)
			if errors.Is(err, ErrRepositoryUnavailable) {
				t.Fatalf("write preflight denied by parent %s: %v", failure, err)
			}
			if !decision.Writable || !decision.CapacityKnown || decision.RepositoryPath != repository.Path {
				t.Fatalf("write preflight did not sample the child target: %+v", decision)
			}
			if err != nil && !errors.Is(err, ErrInsufficientSpace) {
				t.Fatalf("write preflight error = %v", err)
			}

			parentAfter, err := manager.queries.GetStorageLocation(ctx, repository.StorageLocationID)
			if err != nil {
				t.Fatal(err)
			}
			if parentAfter.Status != parentBefore.Status || parentAfter.UpdatedAt != parentBefore.UpdatedAt {
				t.Fatalf("child work changed parent registration: before=%+v after=%+v", parentBefore, parentAfter)
			}
		})
	}
}

// F09: parent Storage Location maintenance is not a write gate; the child's own
// repository maintenance barrier still is.
func TestSidecarWriteHonorsRepositoryMaintenanceNotParentStatus(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	initializeDefaultStorageForTest(t, manager, filepath.Join(t.TempDir(), "default"))
	repository, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}
	storageLocation, err := manager.queries.GetStorageLocation(ctx, repository.StorageLocationID)
	if err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := manager.queries.UpdateStorageLocationFromDisk(ctx, repo.UpdateStorageLocationFromDiskParams{
		StorageLocationID: storageLocation.StorageLocationID, Name: storageLocation.Name, Status: dbtypes.StorageLocationStatusMaintenance, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	rootSidecar := []byte(`{"asset":"storageLocation-maintenance"}`)
	if err := manager.WriteRepositorySidecar(ctx, repository.RepoID.String(), "asset-storageLocation", rootSidecar); err != nil {
		t.Fatalf("sidecar write denied by parent Storage Location maintenance: %v", err)
	}
	stored, err := manager.ReadRepositorySidecar(ctx, repository.RepoID.String(), "asset-storageLocation")
	if err != nil || !bytes.Equal(stored, rootSidecar) {
		t.Fatalf("sidecar written under parent maintenance = %q, %v", stored, err)
	}
	rootTarget := filepath.Join(repository.Path, DefaultStructure.SidecarsDir, "asset-storageLocation.lumilio-sidecar")
	if _, err := os.Stat(rootTarget); err != nil {
		t.Fatalf("sidecar written under parent maintenance is missing: %v", err)
	}

	if _, err := manager.queries.UpdateStorageLocationFromDisk(ctx, repo.UpdateStorageLocationFromDiskParams{
		StorageLocationID: storageLocation.StorageLocationID, Name: storageLocation.Name, Status: dbtypes.StorageLocationStatusActive, UpdatedAt: now,
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
	target := filepath.Join(repository.Path, DefaultStructure.SidecarsDir, "asset-repository.lumilio-sidecar")
	for _, path := range []string{target, target + ".tmp"} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("sidecar maintenance rejection left %s: %v", path, err)
		}
	}
	if matches, err := filepath.Glob(target + ".tmp-*"); err != nil || len(matches) != 0 {
		t.Fatalf("sidecar maintenance rejection left temporary files %v (glob error %v)", matches, err)
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
