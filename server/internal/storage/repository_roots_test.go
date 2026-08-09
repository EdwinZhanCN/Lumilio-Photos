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

func TestEnsureDefaultRepositoryRootFailsClosedWhenRegisteredPathDisappears(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	rootPath := filepath.Join(t.TempDir(), "default")
	root, err := manager.EnsureDefaultRepositoryRoot(ctx, rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(rootPath); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.EnsureDefaultRepositoryRoot(ctx, rootPath); !errors.Is(err, ErrRepositoryRootOffline) {
		t.Fatalf("missing registered default error = %v, want ErrRepositoryRootOffline", err)
	}
	if _, err := os.Stat(rootPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("startup recreated missing registered root %s: %v", root.RootID, err)
	}
	status, err := manager.StorageRuntimeStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StorageRuntimeStateDegraded || status.Reason != StorageRuntimeReasonRecoveryRequired {
		t.Fatalf("missing default runtime status = %#v", status)
	}
	recorded, err := manager.queries.GetDefaultRepositoryRoot(ctx)
	if err != nil || recorded.RootID != root.RootID {
		t.Fatalf("default identity was not preserved: root=%#v err=%v", recorded, err)
	}
}

func TestEnsureDefaultRepositoryRootDoesNotReplaceMissingRegisteredMarker(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	rootPath := filepath.Join(t.TempDir(), "default")
	root, err := manager.EnsureDefaultRepositoryRoot(ctx, rootPath)
	if err != nil {
		t.Fatal(err)
	}
	markerPath := filepath.Join(rootPath, ".lumilioroot")
	if err := os.Remove(markerPath); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.EnsureDefaultRepositoryRoot(ctx, rootPath); !errors.Is(err, ErrRepositoryRootInvalid) {
		t.Fatalf("missing registered marker error = %v, want ErrRepositoryRootInvalid", err)
	}
	if _, err := os.Stat(markerPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("startup replaced missing marker for %s: %v", root.RootID, err)
	}
	recorded, err := manager.queries.GetDefaultRepositoryRoot(ctx)
	if err != nil || recorded.RootID != root.RootID {
		t.Fatalf("default identity was not preserved: root=%#v err=%v", recorded, err)
	}
}

func TestEnsureDefaultRepositoryRootSwitchesOnlyToMovedIdentityWithFixedPrimary(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	oldPath := filepath.Join(base, "default-old")
	root, err := manager.EnsureDefaultRepositoryRoot(ctx, oldPath)
	if err != nil {
		t.Fatal(err)
	}
	primary, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "default-move-primary", Actor: "test", Name: "Primary",
		DirectoryName: "primary", Role: dbtypes.RepoRolePrimary, RootID: root.RootID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}

	newPath := filepath.Join(base, "default-new")
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	moved, err := manager.EnsureDefaultRepositoryRoot(ctx, newPath, LifecycleRequest{
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
	if moved.RootID != root.RootID || moved.Path != canonicalNewPath {
		t.Fatalf("moved default root = %#v, want identity %s at %s", moved, root.RootID, canonicalNewPath)
	}
	recordedPrimary, err := manager.queries.GetRepository(ctx, primary.Repository.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	if recordedPrimary.Path != filepath.Join(canonicalNewPath, "primary") {
		t.Fatalf("moved primary path = %q", recordedPrimary.Path)
	}
	events, err := manager.ListLifecycleAudit(ctx, LifecycleAuditFilter{
		TargetType: "storage_location", TargetID: root.RootID.String(), Limit: 20,
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
	root, err := manager.AddRepositoryRoot(ctx, rootPath, "Original")
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(base, "external-copy")
	if err := os.Mkdir(copyPath, 0o755); err != nil {
		t.Fatal(err)
	}
	marker, err := os.ReadFile(filepath.Join(root.Path, ".lumilioroot"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(copyPath, ".lumilioroot"), marker, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RelocateRepositoryRoot(ctx, root.RootID.String(), copyPath); err == nil {
		t.Fatal("relocate accepted a duplicate root while the registered original remained online")
	} else {
		var conflict *RepositoryRootConflictError
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
	root, err := manager.AddRepositoryRoot(ctx, oldPath, "Archive")
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "recovery-root-child", Actor: "test", Name: "Child", DirectoryName: "child",
		Role: dbtypes.RepoRoleRegular, RootID: root.RootID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(base, "external-new")
	if err := os.Rename(root.Path, newPath); err != nil {
		t.Fatal(err)
	}
	canonicalNewPath, err := CanonicalizeRepositoryPath(newPath)
	if err != nil {
		t.Fatal(err)
	}
	targetID := root.RootID.String()
	operation, _, err := manager.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: "interrupted-root-relocate", Kind: lifecycleKindRelocateStorage,
		Payload: switchDefaultStorageOperationPayload{RootID: root.RootID.String(), OldPath: root.Path, NewPath: canonicalNewPath},
		Actor:   "test", TargetType: "storage_location", TargetID: &targetID,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := manager.queries.UpdateRepositoryRootFromDisk(ctx, repo.UpdateRepositoryRootFromDiskParams{
		RootID: root.RootID, Name: root.Name, Status: dbtypes.RepositoryRootStatusMaintenance, UpdatedAt: now,
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
	competingProcess := startStorageLockProcess(t, canonicalNewPath, "root")
	blockedCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	blockedErr := restarted.RecoverLifecycleOperations(blockedCtx)
	cancel()
	if !errors.Is(blockedErr, ErrRepositoryLockUnavailable) {
		t.Fatalf("recovery with second-instance new-path lock = %v", blockedErr)
	}
	stillMaintenance, err := restarted.queries.GetRepositoryRoot(ctx, root.RootID)
	if err != nil || stillMaintenance.Status != dbtypes.RepositoryRootStatusMaintenance || stillMaintenance.Path != root.Path {
		t.Fatalf("blocked recovery mutated root = %#v, err=%v", stillMaintenance, err)
	}
	if err := competingProcess.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = competingProcess.Wait()
	if err := restarted.RecoverLifecycleOperations(ctx); err != nil {
		t.Fatal(err)
	}
	recoveredRoot, _ := restarted.queries.GetRepositoryRoot(ctx, root.RootID)
	recoveredChild, _ := restarted.queries.GetRepository(ctx, created.Repository.RepoID)
	if recoveredRoot.Path != canonicalNewPath || recoveredRoot.Status != dbtypes.RepositoryRootStatusActive || recoveredChild.Path != filepath.Join(canonicalNewPath, "child") || recoveredChild.Reachability != dbtypes.RepositoryReachabilityActive {
		t.Fatalf("recovered relocate: root=%#v child=%#v", recoveredRoot, recoveredChild)
	}
	recoveredOperation, err := restarted.queries.GetLifecycleOperation(ctx, operation.OperationID)
	if err != nil || recoveredOperation.Status != lifecycleStatusCompleted {
		t.Fatalf("recovered operation = %#v, err=%v", recoveredOperation, err)
	}
}

func TestPathIsStrictlyInside(t *testing.T) {
	base := canonicalTempDir(t)
	root := filepath.Join(base, "photos")
	cases := []struct {
		name string
		path string
		want bool
	}{
		{name: "direct child", path: filepath.Join(root, "family"), want: true},
		{name: "nested child", path: filepath.Join(root, "family", "2026"), want: true},
		{name: "same path", path: root, want: false},
		{name: "parent", path: base, want: false},
		{name: "prefix sibling", path: filepath.Join(base, "photos-archive"), want: false},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := pathIsStrictlyInside(root, test.path); got != test.want {
				t.Fatalf("pathIsStrictlyInside(%q, %q) = %v, want %v", root, test.path, got, test.want)
			}
		})
	}
}

func TestPathIsDirectChild(t *testing.T) {
	base := canonicalTempDir(t)
	root := filepath.Join(base, "photos")
	cases := []struct {
		name string
		path string
		want bool
	}{
		{name: "direct child", path: filepath.Join(root, "family"), want: true},
		{name: "nested child", path: filepath.Join(root, "family", "2026"), want: false},
		{name: "same path", path: root, want: false},
		{name: "parent", path: base, want: false},
		{name: "prefix sibling", path: filepath.Join(base, "photos-archive"), want: false},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := pathIsDirectChild(root, test.path); got != test.want {
				t.Fatalf("pathIsDirectChild(%q, %q) = %v, want %v", root, test.path, got, test.want)
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
	if _, err := relocatedRepositoryPath(oldRoot, newRoot, filepath.Join(base, "old-copy")); !errors.Is(err, ErrRepositoryRootInvalid) {
		t.Fatalf("prefix sibling error = %v, want ErrRepositoryRootInvalid", err)
	}
	if _, err := relocatedRepositoryPath(oldRoot, newRoot, filepath.Join(oldRoot, "family", "2026")); !errors.Is(err, ErrRepositoryRootInvalid) {
		t.Fatalf("nested repository error = %v, want ErrRepositoryRootInvalid", err)
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
