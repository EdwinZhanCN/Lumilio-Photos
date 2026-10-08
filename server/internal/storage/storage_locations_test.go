package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"

	"go.uber.org/zap"
)

func TestEnsureDefaultStorageLocationFailsClosedWhenRegisteredPathDisappears(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	rootPath := filepath.Join(t.TempDir(), "default")
	storageLocation, err := manager.EnsureDefaultStorageLocation(ctx, rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(rootPath); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.EnsureDefaultStorageLocation(ctx, rootPath); !errors.Is(err, ErrStorageLocationOffline) {
		t.Fatalf("missing registered default error = %v, want ErrStorageLocationOffline", err)
	}
	if _, err := os.Stat(rootPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("startup recreated missing registered storageLocation %s: %v", storageLocation.StorageLocationID, err)
	}
	status, err := manager.StorageRuntimeStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StorageRuntimeStateDegraded || status.Reason != StorageRuntimeReasonRecoveryRequired {
		t.Fatalf("incomplete bootstrap runtime status = %#v", status)
	}
	recorded, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil || recorded.StorageLocationID != storageLocation.StorageLocationID {
		t.Fatalf("default identity was not preserved: storageLocation=%#v err=%v", recorded, err)
	}
}

func TestEnsureDefaultStorageLocationDoesNotReplaceMissingRegisteredMarker(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	rootPath := filepath.Join(t.TempDir(), "default")
	storageLocation, err := manager.EnsureDefaultStorageLocation(ctx, rootPath)
	if err != nil {
		t.Fatal(err)
	}
	markerPath := filepath.Join(rootPath, ".lumilioroot")
	if err := os.Remove(markerPath); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.EnsureDefaultStorageLocation(ctx, rootPath); !errors.Is(err, ErrStorageLocationInvalid) {
		t.Fatalf("missing registered marker error = %v, want ErrStorageLocationInvalid", err)
	}
	if _, err := os.Stat(markerPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("startup replaced missing marker for %s: %v", storageLocation.StorageLocationID, err)
	}
	recorded, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil || recorded.StorageLocationID != storageLocation.StorageLocationID {
		t.Fatalf("default identity was not preserved: storageLocation=%#v err=%v", recorded, err)
	}
}

func TestEnsureDefaultStorageLocationSwitchesOnlyToMovedIdentityWithFixedPrimary(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	oldPath := filepath.Join(base, "default-old")
	storageLocation, err := manager.EnsureDefaultStorageLocation(ctx, oldPath)
	if err != nil {
		t.Fatal(err)
	}
	primary, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "default-move-primary", Actor: "test", Name: "Primary",
		DirectoryName: "primary", Role: dbtypes.RepoRolePrimary, StorageLocationID: storageLocation.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}

	newPath := filepath.Join(base, "default-new")
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	moved, err := manager.EnsureDefaultStorageLocation(ctx, newPath, LifecycleRequest{
		RequestID: "desktop-default-switch", Actor: "desktop_host:settings", HostInstanceID: "desktop-instance-1",
		ConfirmationType: "portable_identity_match",
	})
	if err != nil {
		t.Fatal(err)
	}
	canonicalNewPath, err := CanonicalizeRepositoryPath(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if moved.StorageLocationID != storageLocation.StorageLocationID || moved.Path != canonicalNewPath {
		t.Fatalf("moved default storageLocation = %#v, want identity %s at %s", moved, storageLocation.StorageLocationID, canonicalNewPath)
	}
	recordedPrimary, err := manager.queries.GetRepository(ctx, primary.Repository.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	if recordedPrimary.Path != filepath.Join(canonicalNewPath, "primary") {
		t.Fatalf("moved primary path = %q", recordedPrimary.Path)
	}
	events, err := manager.ListLifecycleAudit(ctx, LifecycleAuditFilter{
		TargetType: "storage_location", TargetID: storageLocation.StorageLocationID.String(), Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	var switchAudit *LifecycleAuditEvent
	for index := range events {
		if events[index].RequestID == "desktop-default-switch" {
			switchAudit = &events[index]
			break
		}
	}
	if switchAudit == nil || switchAudit.Actor != "desktop_host:settings" ||
		switchAudit.HostInstanceID != "desktop-instance-1" || switchAudit.ConfirmationType != "portable_identity_match" ||
		switchAudit.OldPath == "" || switchAudit.NewPath != canonicalNewPath ||
		!strings.Contains(string(switchAudit.Details), `"repository_count":1`) ||
		!strings.Contains(string(switchAudit.Details), `"files_preserved":true`) {
		t.Fatalf("default switch audit = %#v", switchAudit)
	}
}

func TestRelocateStorageLocationRejectsOnlineOriginalIdentity(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	rootPath := filepath.Join(base, "external-original")
	if err := os.Mkdir(rootPath, 0o755); err != nil {
		t.Fatal(err)
	}
	storageLocation, err := manager.AddStorageLocation(ctx, rootPath, "Original")
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(base, "external-copy")
	if err := os.Mkdir(copyPath, 0o755); err != nil {
		t.Fatal(err)
	}
	marker, err := os.ReadFile(filepath.Join(storageLocation.Path, ".lumilioroot"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(copyPath, ".lumilioroot"), marker, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RelocateStorageLocation(ctx, storageLocation.StorageLocationID.String(), copyPath); err == nil {
		t.Fatal("relocate accepted a duplicate storageLocation while the registered original remained online")
	} else {
		var conflict *StorageLocationConflictError
		if !errors.As(err, &conflict) || len(conflict.Actions) != 0 {
			t.Fatalf("online-original error = %T %v", err, err)
		}
	}
}

func TestRecoverLifecycleOperationsRollsForwardInterruptedRootRelocate(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	oldPath := filepath.Join(base, "external-old")
	if err := os.Mkdir(oldPath, 0o755); err != nil {
		t.Fatal(err)
	}
	storageLocation, err := manager.AddStorageLocation(ctx, oldPath, "Archive")
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "recovery-storageLocation-child", Actor: "test", Name: "Child", DirectoryName: "child",
		Role: dbtypes.RepoRoleRegular, StorageLocationID: storageLocation.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(base, "external-new")
	if err := os.Rename(storageLocation.Path, newPath); err != nil {
		t.Fatal(err)
	}
	canonicalNewPath, err := CanonicalizeRepositoryPath(newPath)
	if err != nil {
		t.Fatal(err)
	}
	targetID := storageLocation.StorageLocationID.String()
	operation, _, err := manager.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: "interrupted-storageLocation-relocate", Kind: lifecycleKindRelocateStorage,
		Payload: switchDefaultStorageOperationPayload{StorageLocationID: storageLocation.StorageLocationID.String(), OldPath: storageLocation.Path, NewPath: canonicalNewPath},
		Actor:   "test", TargetType: "storage_location", TargetID: &targetID,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := manager.queries.UpdateStorageLocationFromDisk(ctx, repo.UpdateStorageLocationFromDiskParams{
		StorageLocationID: storageLocation.StorageLocationID, Name: storageLocation.Name, Status: dbtypes.StorageLocationStatusMaintenance, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.queries.BeginRepositoryMaintenance(ctx, repo.BeginRepositoryMaintenanceParams{
		RepoID: created.Repository.RepoID, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	restarted, err := NewRepositoryManager(catalog.SQL, catalog.Queries, zap.NewNop(), nil, NewRepositoryFSFactory(nil, catalog.Queries))
	if err != nil {
		t.Fatal(err)
	}
	releaseOwnership, err := restarted.AcquireRuntimeStorageOwnership(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseOwnership()
	competingProcess := startStorageLockProcess(t, canonicalNewPath, "storage_location")
	blockedCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	blockedErr := restarted.RecoverLifecycleOperations(blockedCtx)
	cancel()
	if !errors.Is(blockedErr, ErrRepositoryLockUnavailable) {
		t.Fatalf("recovery with second-instance new-path lock = %v", blockedErr)
	}
	stillMaintenance, err := restarted.queries.GetStorageLocation(ctx, storageLocation.StorageLocationID)
	if err != nil || stillMaintenance.Status != dbtypes.StorageLocationStatusMaintenance || stillMaintenance.Path != storageLocation.Path {
		t.Fatalf("blocked recovery mutated storageLocation = %#v, err=%v", stillMaintenance, err)
	}
	if err := competingProcess.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = competingProcess.Wait()
	if err := restarted.RecoverLifecycleOperations(ctx); err != nil {
		t.Fatal(err)
	}
	recoveredRoot, _ := restarted.queries.GetStorageLocation(ctx, storageLocation.StorageLocationID)
	recoveredChild, _ := restarted.queries.GetRepository(ctx, created.Repository.RepoID)
	if recoveredRoot.Path != canonicalNewPath || recoveredRoot.Status != dbtypes.StorageLocationStatusActive || recoveredChild.Path != filepath.Join(canonicalNewPath, "child") || recoveredChild.Reachability != dbtypes.RepositoryReachabilityActive {
		t.Fatalf("recovered relocate: storageLocation=%#v child=%#v", recoveredRoot, recoveredChild)
	}
	recoveredOperation, err := restarted.queries.GetLifecycleOperation(ctx, operation.OperationID)
	if err != nil || recoveredOperation.Status != lifecycleStatusCompleted {
		t.Fatalf("recovered operation = %#v, err=%v", recoveredOperation, err)
	}
}

func TestPathIsStrictlyInside(t *testing.T) {
	base := canonicalTempDir(t)
	storageLocation := filepath.Join(base, "photos")
	cases := []struct {
		name string
		path string
		want bool
	}{
		{name: "direct child", path: filepath.Join(storageLocation, "family"), want: true},
		{name: "nested child", path: filepath.Join(storageLocation, "family", "2026"), want: true},
		{name: "same path", path: storageLocation, want: false},
		{name: "parent", path: base, want: false},
		{name: "prefix sibling", path: filepath.Join(base, "photos-archive"), want: false},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := pathIsStrictlyInside(storageLocation, test.path); got != test.want {
				t.Fatalf("pathIsStrictlyInside(%q, %q) = %v, want %v", storageLocation, test.path, got, test.want)
			}
		})
	}
}

func TestPathIsDirectChild(t *testing.T) {
	base := canonicalTempDir(t)
	storageLocation := filepath.Join(base, "photos")
	cases := []struct {
		name string
		path string
		want bool
	}{
		{name: "direct child", path: filepath.Join(storageLocation, "family"), want: true},
		{name: "nested child", path: filepath.Join(storageLocation, "family", "2026"), want: false},
		{name: "same path", path: storageLocation, want: false},
		{name: "parent", path: base, want: false},
		{name: "prefix sibling", path: filepath.Join(base, "photos-archive"), want: false},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := pathIsDirectChild(storageLocation, test.path); got != test.want {
				t.Fatalf("pathIsDirectChild(%q, %q) = %v, want %v", storageLocation, test.path, got, test.want)
			}
		})
	}
}

func TestRelocatedRepositoryPath(t *testing.T) {
	base := canonicalTempDir(t)
	oldRoot := filepath.Join(base, "old")
	newRoot := filepath.Join(base, "new")
	repositoryPath := filepath.Join(oldRoot, "family")

	got, err := relocatedRepositoryPath(oldRoot, newRoot, repositoryPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(newRoot, "family"); got != want {
		t.Fatalf("relocated path = %q, want %q", got, want)
	}
	if _, err := relocatedRepositoryPath(oldRoot, newRoot, filepath.Join(base, "old-copy")); !errors.Is(err, ErrStorageLocationInvalid) {
		t.Fatalf("prefix sibling error = %v, want ErrStorageLocationInvalid", err)
	}
	if _, err := relocatedRepositoryPath(oldRoot, newRoot, filepath.Join(oldRoot, "family", "2026")); !errors.Is(err, ErrStorageLocationInvalid) {
		t.Fatalf("nested repository error = %v, want ErrStorageLocationInvalid", err)
	}
}

func TestRelocatedRepositoryPathAcrossWindowsDriveLetters(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows drive-letter semantics require a Windows filesystem runtime")
	}
	got, err := relocatedRepositoryPath(`D:\Lumilio`, `E:\Lumilio`, `D:\Lumilio\family`)
	if err != nil {
		t.Fatal(err)
	}
	if want := `E:\Lumilio\family`; got != want {
		t.Fatalf("relocated path = %q, want %q", got, want)
	}
}
