package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"
)

func TestOpenRepositoryRejectsUnavailableCloudPlaceholderBeforeCatalogMutation(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	root, err := manager.queries.GetDefaultRepositoryRoot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "create-placeholder", Actor: "test", Name: "Placeholder Archive",
		DirectoryName: "placeholder-archive", Role: dbtypes.RepoRoleRegular, RootID: root.RootID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	repositoryPath := created.Repository.Path
	if err := manager.RemoveRepository(ctx, created.Repository.RepoID.String()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repositoryPath, ".family.jpg.icloud"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.OpenRepository(ctx, repositoryPath, nil, dbtypes.RepoRoleRegular,
		LifecycleRequest{RequestID: "open-placeholder", Actor: "test"}); !errors.Is(err, ErrUnavailableCloudPlaceholder) {
		t.Fatalf("open placeholder Repository error = %v, want ErrUnavailableCloudPlaceholder", err)
	}
	if _, err := manager.GetRepository(created.Repository.RepoID.String()); err == nil {
		t.Fatal("placeholder Repository was registered")
	}
}

func TestOpenRepositoryIsolatesPrivateStateAndSchedulesInitialScan(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	root, err := manager.queries.GetDefaultRepositoryRoot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "create-reopen-source", Actor: "test", Name: "Reopened Archive",
		DirectoryName: "reopened", Role: dbtypes.RepoRoleRegular, RootID: root.RootID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	oldState := filepath.Join(created.Repository.Path, DefaultStructure.SystemDir, "staging", "old-state")
	if err := os.MkdirAll(filepath.Dir(oldState), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldState, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := manager.RemoveRepository(ctx, created.Repository.RepoID.String()); err != nil {
		t.Fatal(err)
	}

	var scanned []string
	manager.SetInitialScanEnqueuer(func(_ context.Context, repositoryID string) error {
		scanned = append(scanned, repositoryID)
		return nil
	})
	request := LifecycleRequest{RequestID: "open-repository-1", Actor: "test:user:1"}
	opened, err := manager.OpenRepository(ctx, created.Repository.Path, nil, dbtypes.RepoRoleRegular, request)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := manager.OpenRepository(ctx, created.Repository.Path, nil, dbtypes.RepoRoleRegular, request)
	if err != nil {
		t.Fatal(err)
	}
	if opened.RepoID != created.Repository.RepoID || replayed.RepoID != opened.RepoID {
		t.Fatalf("repository identities: created=%s opened=%s replayed=%s", created.Repository.RepoID, opened.RepoID, replayed.RepoID)
	}
	if len(scanned) != 1 || scanned[0] != opened.RepoID.String() {
		t.Fatalf("initial scans = %#v", scanned)
	}
	// Simulate a crash after lifecycle completion but before the durable queue
	// receipt was recorded. Startup/runtime retry must enqueue exactly once and
	// then persist the receipt so a subsequent retry is a no-op.
	if _, err := catalog.SQL.ExecContext(ctx, `
		UPDATE lifecycle_operations
		SET result = json_set(result, '$.initial_scan_queued', json('false'))
		WHERE request_id = ?
	`, request.RequestID); err != nil {
		t.Fatal(err)
	}
	if err := manager.RetryPendingInitialRepositoryScans(ctx); err != nil {
		t.Fatal(err)
	}
	if err := manager.RetryPendingInitialRepositoryScans(ctx); err != nil {
		t.Fatal(err)
	}
	if len(scanned) != 2 || scanned[1] != opened.RepoID.String() {
		t.Fatalf("durably retried initial scans = %#v", scanned)
	}
	recovered, err := filepath.Glob(filepath.Join(
		opened.Path, DefaultStructure.SystemDir, "recovery", "reopened-*", "staging", "old-state",
	))
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered) != 1 {
		t.Fatalf("reopened private state = %#v", recovered)
	}
	if _, err := os.Stat(filepath.Join(opened.Path, DefaultStructure.SystemDir, "staging", "old-state")); !os.IsNotExist(err) {
		t.Fatalf("old private state remained active: %v", err)
	}
}

func TestRenameRepositoryChangesOnlyDisplayName(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	initializeDefaultStorageForTest(t, manager, filepath.Join(t.TempDir(), "default"))
	root, err := manager.queries.GetDefaultRepositoryRoot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "rename-only", Actor: "test", Name: "Before", DirectoryName: "stable-folder",
		Role: dbtypes.RepoRoleRegular, RootID: root.RootID.String(), StorageStrategy: "cas", DuplicateHandling: "rename",
	})
	if err != nil {
		t.Fatal(err)
	}
	renamed, err := manager.RenameRepository(ctx, created.Repository.RepoID.String(), "After", LifecycleRequest{
		RequestID: "rename-before-after", Actor: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Name != "After" || renamed.Path != created.Repository.Path || renamed.Config.StorageStrategy != "cas" || renamed.Config.LocalSettings.HandleDuplicateFilenames != "rename" {
		t.Fatalf("rename changed immutable repository fields: before=%#v after=%#v", created.Repository, renamed)
	}
	disk, err := repocfg.LoadConfigFromFile(renamed.Path)
	if err != nil {
		t.Fatal(err)
	}
	if disk.Name != "After" || disk.StorageStrategy != "cas" || disk.ID != renamed.RepoID.String() {
		t.Fatalf("renamed marker = %#v", disk)
	}
	replayed, err := manager.RenameRepository(ctx, created.Repository.RepoID.String(), "After", LifecycleRequest{
		RequestID: "rename-before-after", Actor: "test",
	})
	if err != nil || replayed.Name != "After" {
		t.Fatalf("idempotent rename replay = %+v, error = %v", replayed, err)
	}
	operation, err := manager.queries.GetLifecycleOperationByRequestID(ctx, "rename-before-after")
	if err != nil || operation.Kind != lifecycleKindRenameRepository || operation.Status != lifecycleStatusCompleted {
		t.Fatalf("rename journal = %+v, error = %v", operation, err)
	}

	crashConfig := *disk
	crashConfig.Name = "Recovered After Crash"
	crashOperation, _, err := manager.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: "rename-crash-recovery", Kind: lifecycleKindRenameRepository,
		Payload: renameRepositoryOperationPayload{
			RepositoryID: renamed.RepoID.String(), Path: renamed.Path, NewName: crashConfig.Name,
		},
		Actor: "test", TargetType: "repository", TargetID: ptrString(renamed.RepoID.String()),
		RollbackData: renameRepositoryRollbackData{PreviousConfig: *disk},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := crashConfig.SaveConfigToFile(renamed.Path); err != nil {
		t.Fatal(err)
	}
	if err := manager.updateLifecycleOperationPhase(ctx, crashOperation.OperationID, lifecyclePhaseFilesystemApplied,
		renameRepositoryRollbackData{PreviousConfig: *disk}); err != nil {
		t.Fatal(err)
	}
	if err := manager.RecoverLifecycleOperations(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := manager.queries.GetRepository(ctx, renamed.RepoID)
	if err != nil || recovered.Name != crashConfig.Name {
		t.Fatalf("recovered rename = %+v, error = %v", recovered, err)
	}

	// Simulate the other crash boundary: marker and catalog committed, but the
	// lifecycle operation was not marked complete. Recovery must recognize and
	// complete the same durable result rather than rejecting or rewriting any
	// immutable repository fields.
	committedConfig := crashConfig
	committedConfig.Name = "Recovered After Catalog Commit"
	committedOperation, _, err := manager.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: "rename-catalog-commit-recovery", Kind: lifecycleKindRenameRepository,
		Payload: renameRepositoryOperationPayload{
			RepositoryID: renamed.RepoID.String(), Path: renamed.Path, NewName: committedConfig.Name,
		},
		Actor: "test", TargetType: "repository", TargetID: ptrString(renamed.RepoID.String()),
		RollbackData: renameRepositoryRollbackData{PreviousConfig: crashConfig},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := committedConfig.SaveConfigToFile(renamed.Path); err != nil {
		t.Fatal(err)
	}
	if err := manager.updateLifecycleOperationPhase(ctx, committedOperation.OperationID, lifecyclePhaseFilesystemApplied,
		renameRepositoryRollbackData{PreviousConfig: crashConfig}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.queries.UpdateRepository(ctx, repo.UpdateRepositoryParams{
		RepoID: renamed.RepoID, Name: committedConfig.Name, Config: committedConfig,
		DefaultOwnerID: recovered.DefaultOwnerID, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	}); err != nil {
		t.Fatal(err)
	}
	if err := manager.RecoverLifecycleOperations(ctx); err != nil {
		t.Fatal(err)
	}
	completed, err := manager.queries.GetLifecycleOperation(ctx, committedOperation.OperationID)
	if err != nil || completed.Status != lifecycleStatusCompleted {
		t.Fatalf("catalog-committed rename recovery = %+v, error = %v", completed, err)
	}
}

func ptrString(value string) *string { return &value }
