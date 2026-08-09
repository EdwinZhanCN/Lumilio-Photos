package storage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func newCatalogRepositoryManager(t *testing.T) (*db.DB, *DefaultRepositoryManager) {
	t.Helper()
	ctx := context.Background()
	catalogDirectory := t.TempDir()
	if err := os.Chmod(catalogDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(catalogDirectory, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close(context.Background()) })
	if err := catalog.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	files := NewRepositoryFSFactory(NewRepositoryAccessCoordinator(), catalog.Queries)
	manager, err := NewRepositoryManager(catalog.SQL, catalog.Queries, zap.NewNop(), nil, files)
	if err != nil {
		t.Fatal(err)
	}
	return catalog, manager
}

func initializeDefaultStorageForTest(t *testing.T, manager *DefaultRepositoryManager, rootPath string) {
	t.Helper()
	ctx := context.Background()
	root, err := manager.EnsureDefaultRepositoryRoot(ctx, rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if root.MountFingerprint == "" || root.MountFingerprint != InspectStoragePath(rootPath).MountFingerprint {
		t.Fatalf("registered mount fingerprint = %q", root.MountFingerprint)
	}
	if _, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "test-primary:" + root.RootID.String(), Actor: "test",
		Name: "Primary", DirectoryName: "primary", Role: dbtypes.RepoRolePrimary,
		RootID: root.RootID.String(),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterRepositoryCopyRejectsPrimaryIdentityAsRegular(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	rootPath := filepath.Join(t.TempDir(), "default")
	initializeDefaultStorageForTest(t, manager, rootPath)
	primary, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}
	config, err := repocfg.LoadConfigFromFile(primary.Path)
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(t.TempDir(), "primary-copy")
	if err := os.MkdirAll(copyPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := config.SaveConfigToFile(copyPath); err != nil {
		t.Fatal(err)
	}
	if err := manager.dirManager.CreateStructure(copyPath); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RegisterRepositoryCopy(ctx, copyPath, nil, dbtypes.RepoRoleRegular); !errors.Is(err, ErrPathNotAllowed) {
		t.Fatalf("primary copy error = %v, want ErrPathNotAllowed", err)
	}
	loaded, err := repocfg.LoadConfigFromFile(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != primary.RepoID.String() {
		t.Fatalf("rejected primary copy identity changed to %s", loaded.ID)
	}
}

func TestCreateRepositoryLifecycleRequestIsDurablyIdempotent(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	rootPath := filepath.Join(t.TempDir(), "default")
	root, err := manager.EnsureDefaultRepositoryRoot(ctx, rootPath)
	if err != nil {
		t.Fatal(err)
	}
	actor, err := catalog.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "lifecycle-actor", Password: "test", DisplayName: "Lifecycle Actor", Role: "admin",
		WebauthnUserHandle: []byte("lifecycle-actor-handle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	spec := CreateRepositorySpec{
		RequestID: "create-primary-1", Actor: "test:user:1", ActorUserID: &actor.UserID, HostInstanceID: "web-server-1",
		Name: "Primary", DirectoryName: "primary", Role: dbtypes.RepoRolePrimary,
		RootID: root.RootID.String(),
	}
	first, err := manager.CreateRepository(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.CreateRepository(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	if first.Repository.RepoID != second.Repository.RepoID {
		t.Fatalf("idempotent retry returned %s, want %s", second.Repository.RepoID, first.Repository.RepoID)
	}

	changed := spec
	changed.Name = "Different payload"
	if _, err := manager.CreateRepository(ctx, changed); !errors.Is(err, ErrLifecycleRequestConflict) {
		t.Fatalf("changed payload error = %v, want ErrLifecycleRequestConflict", err)
	}

	var completed int
	if err := catalog.SQL.QueryRowContext(ctx, `
		SELECT count(*) FROM lifecycle_operations
		WHERE request_id = 'create-primary-1' AND status = 'completed'
	`).Scan(&completed); err != nil {
		t.Fatal(err)
	}
	if completed != 1 {
		t.Fatalf("completed lifecycle rows = %d, want 1", completed)
	}
	var audited int
	if err := catalog.SQL.QueryRowContext(ctx, `
		SELECT count(*) FROM lifecycle_audit_events
		WHERE request_id = 'create-primary-1' AND action = 'create_repository'
			  AND result = 'succeeded' AND actor = 'test:user:1'
			  AND actor_user_id = ? AND host_instance_id = 'web-server-1'
	`, actor.UserID).Scan(&audited); err != nil {
		t.Fatal(err)
	}
	if audited != 1 {
		t.Fatalf("durable successful audit rows = %d, want 1", audited)
	}
}

func TestCreateRepositoryRequiresAndAuditsStorageRiskConfirmation(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	riskyRootPath := filepath.Join(base, "Dropbox")
	if err := os.Mkdir(riskyRootPath, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := manager.AddRepositoryRoot(ctx, riskyRootPath, "Cloud-synced disk")
	if err != nil {
		t.Fatal(err)
	}
	spec := CreateRepositorySpec{
		RequestID: "risk-confirmation-create", Actor: "web:test-admin", Name: "Risk Archive",
		DirectoryName: "risk-archive", Role: dbtypes.RepoRoleRegular, RootID: root.RootID.String(),
	}
	if _, err := manager.CreateRepository(ctx, spec); !errors.Is(err, ErrRepositoryRiskConfirmationRequired) {
		t.Fatalf("unconfirmed risky create error = %v, want ErrRepositoryRiskConfirmationRequired", err)
	}
	if _, err := os.Stat(filepath.Join(riskyRootPath, "risk-archive")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unconfirmed risky create mutated disk: %v", err)
	}
	rejected, err := manager.ListLifecycleAudit(ctx, LifecycleAuditFilter{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	var rejectedRisk *LifecycleAuditEvent
	for index := range rejected {
		if rejected[index].RequestID == spec.RequestID {
			rejectedRisk = &rejected[index]
			break
		}
	}
	if rejectedRisk == nil || rejectedRisk.Result != AuditResultRejected ||
		rejectedRisk.FailureStage != "risk_confirmation" || rejectedRisk.ConfirmationType != "none" {
		t.Fatalf("rejected risk audit = %#v", rejectedRisk)
	}
	spec.RiskConfirmation = true
	created, err := manager.CreateRepository(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	events, err := manager.ListLifecycleAudit(ctx, LifecycleAuditFilter{
		TargetType: "repository", TargetID: created.Repository.RepoID.String(), Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ConfirmationType != "storage_risk" || events[0].Result != AuditResultSucceeded {
		t.Fatalf("risk confirmation audit = %#v", events)
	}
}

func TestRecoverLifecycleOperationsRollsBackUncommittedCreate(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	target := filepath.Join(t.TempDir(), "interrupted")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	config := repocfg.NewRepositoryConfig("Interrupted")
	if err := manager.dirManager.CreateStructure(target); err != nil {
		t.Fatal(err)
	}
	if err := config.SaveConfigToFile(target); err != nil {
		t.Fatal(err)
	}
	targetID := config.ID
	operation, _, err := manager.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: "interrupted-create", Kind: lifecycleKindCreateRepository,
		Payload: createRepositoryOperationPayload{
			Name: "Interrupted", Path: target, RootID: uuid.NewString(), Role: dbtypes.RepoRoleRegular,
			StorageStrategy: "date", DuplicateHandling: "rename",
		},
		Actor: "web:admin", HostInstanceID: "web-server-recovery", TargetType: "repository", TargetID: &targetID,
		RollbackData: createRepositoryRollbackData{Path: target, TargetCreated: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseFilesystemApplied,
		createRepositoryRollbackData{Path: target, TargetCreated: true}); err != nil {
		t.Fatal(err)
	}

	if err := manager.RecoverLifecycleOperations(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("interrupted create target still exists: %v", err)
	}
	recovered, err := manager.queries.GetLifecycleOperation(ctx, operation.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Status != lifecycleStatusRolledBack {
		t.Fatalf("recovered status = %q, want rolled_back", recovered.Status)
	}
	events, err := manager.ListLifecycleAudit(ctx, LifecycleAuditFilter{
		TargetType: "repository", TargetID: targetID, Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Result != AuditResultRecovered || events[0].Source != "recovery" ||
		events[0].FailureStage != lifecyclePhaseFailed || events[0].Actor != "web:admin" ||
		events[0].HostInstanceID != "web-server-recovery" || events[0].RequestID != "interrupted-create" {
		t.Fatalf("recovery audit = %#v", events)
	}
}

func TestLifecycleRecoveryClaimsEveryJournalOnlyTargetBeforeMutation(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	initializeDefaultStorageForTest(t, manager, filepath.Join(t.TempDir(), "default"))
	releaseOwnership, err := manager.AcquireRuntimeStorageOwnership(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseOwnership()

	tests := []struct {
		kind     string
		lockKind string
		payload  func(string) any
	}{
		{lifecycleKindCreateRepository, "repository", func(path string) any { return createRepositoryOperationPayload{Path: path} }},
		{lifecycleKindOpenRepository, "repository", func(path string) any { return openRepositoryOperationPayload{Path: path} }},
		{lifecycleKindRegisterRepositoryCopy, "repository", func(path string) any { return registerRepositoryCopyOperationPayload{Path: path} }},
		{lifecycleKindRenameRepository, "repository", func(path string) any { return renameRepositoryOperationPayload{Path: path} }},
		{lifecycleKindCreateStorageLocation, "root", func(path string) any { return createStorageLocationOperationPayload{Path: path} }},
		{lifecycleKindSwitchDefaultStorage, "root", func(path string) any { return switchDefaultStorageOperationPayload{NewPath: path} }},
		{lifecycleKindRelocateStorage, "root", func(path string) any { return switchDefaultStorageOperationPayload{NewPath: path} }},
	}
	for _, test := range tests {
		t.Run(test.kind, func(t *testing.T) {
			target := t.TempDir()
			var release func()
			var lockErr error
			if test.lockKind == "root" {
				release, lockErr = acquireRootPathLock(ctx, target, true)
			} else {
				release, lockErr = acquireRepositoryPathLock(ctx, target, true)
			}
			if lockErr != nil {
				t.Fatal(lockErr)
			}
			defer release()
			payload, marshalErr := marshalLifecycleJSON(test.payload(target))
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			recoveryCtx, cancel := context.WithTimeout(ctx, 25*time.Millisecond)
			defer cancel()
			err := manager.claimLifecycleRecoveryTargets(recoveryCtx, repo.LifecycleOperation{
				OperationID: uuid.NewString(), Kind: test.kind, Payload: payload,
			})
			if !errors.Is(err, ErrRepositoryLockUnavailable) {
				t.Fatalf("recovery lock error = %v", err)
			}
		})
	}
}

func TestStorageLocationAndCopyRegistrationUseDurableJournal(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))

	externalPath := filepath.Join(base, "external")
	if err := os.Mkdir(externalPath, 0o755); err != nil {
		t.Fatal(err)
	}
	request := LifecycleRequest{RequestID: "add-external-1", Actor: "test"}
	firstRoot, err := manager.AddRepositoryRoot(ctx, externalPath, "External", request)
	if err != nil {
		t.Fatal(err)
	}
	secondRoot, err := manager.AddRepositoryRoot(ctx, externalPath, "External", request)
	if err != nil {
		t.Fatal(err)
	}
	if firstRoot.RootID != secondRoot.RootID {
		t.Fatalf("Storage Location retry returned %s, want %s", secondRoot.RootID, firstRoot.RootID)
	}
	if _, err := manager.AddRepositoryRoot(ctx, externalPath, "Changed", request); !errors.Is(err, ErrLifecycleRequestConflict) {
		t.Fatalf("changed Storage Location request error = %v", err)
	}

	original, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "create-copy-source", Actor: "test", Name: "Source", DirectoryName: "source",
		Role: dbtypes.RepoRoleRegular, RootID: firstRoot.RootID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(firstRoot.Path, "copy")
	if err := os.Mkdir(copyPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := manager.dirManager.CreateStructure(copyPath); err != nil {
		t.Fatal(err)
	}
	copyConfig := original.Repository.Config
	if err := copyConfig.SaveConfigToFile(copyPath); err != nil {
		t.Fatal(err)
	}
	oldPrivateFile := filepath.Join(copyPath, DefaultStructure.SystemDir, "staging", "old-state")
	if err := os.MkdirAll(filepath.Dir(oldPrivateFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPrivateFile, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	var scanned []string
	manager.SetInitialScanEnqueuer(func(_ context.Context, repositoryID string) error {
		scanned = append(scanned, repositoryID)
		return nil
	})
	copyRequest := LifecycleRequest{RequestID: "register-copy-1", Actor: "test"}
	registered, err := manager.RegisterRepositoryCopy(ctx, copyPath, nil, dbtypes.RepoRoleRegular, copyRequest)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := manager.RegisterRepositoryCopy(ctx, copyPath, nil, dbtypes.RepoRoleRegular, copyRequest)
	if err != nil {
		t.Fatal(err)
	}
	if registered.RepoID != replayed.RepoID || registered.RepoID == original.Repository.RepoID {
		t.Fatalf("copy identities: source=%s first=%s retry=%s", original.Repository.RepoID, registered.RepoID, replayed.RepoID)
	}
	if len(scanned) != 1 || scanned[0] != registered.RepoID.String() {
		t.Fatalf("copy initial scans = %#v", scanned)
	}
	// Simulate the at-least-once handoff crash window after queue acceptance
	// but before its lifecycle receipt. Restart retry may enqueue once more, and
	// then persists the receipt so further retries are no-ops.
	if _, err := catalog.SQL.ExecContext(ctx, `
		UPDATE lifecycle_operations
		SET result = json_set(result, '$.initial_scan_queued', json('false'))
		WHERE request_id = ?
	`, copyRequest.RequestID); err != nil {
		t.Fatal(err)
	}
	if err := manager.RetryPendingInitialRepositoryScans(ctx); err != nil {
		t.Fatal(err)
	}
	if err := manager.RetryPendingInitialRepositoryScans(ctx); err != nil {
		t.Fatal(err)
	}
	if len(scanned) != 2 || scanned[1] != registered.RepoID.String() {
		t.Fatalf("durably retried copy scans = %#v", scanned)
	}
	recovered, err := filepath.Glob(filepath.Join(copyPath, DefaultStructure.SystemDir, "recovery", "copied-from-*", "staging", "old-state"))
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered) != 1 {
		t.Fatalf("isolated private-state files = %v", recovered)
	}
	var completed int
	if err := catalog.SQL.QueryRowContext(ctx, `
		SELECT count(*) FROM lifecycle_operations
		WHERE request_id IN ('add-external-1', 'register-copy-1') AND status = 'completed'
	`).Scan(&completed); err != nil {
		t.Fatal(err)
	}
	if completed != 2 {
		t.Fatalf("completed journal rows = %d, want 2", completed)
	}
}

func TestRemoveRepositoryClearsCatalogAndPreservesFiles(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	root, err := manager.queries.GetDefaultRepositoryRoot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "remove-regular-create", Actor: "test", Name: "Removable Archive",
		DirectoryName: "removable", Role: dbtypes.RepoRoleRegular, RootID: root.RootID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}

	originalPath := filepath.Join(created.Repository.Path, DefaultStructure.InboxDir, "kept.jpg")
	if err := os.WriteFile(originalPath, []byte("original-media"), 0o644); err != nil {
		t.Fatal(err)
	}
	privatePath := filepath.Join(created.Repository.Path, DefaultStructure.SystemDir, "staging", "kept-state")
	if err := os.MkdirAll(filepath.Dir(privatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(privatePath, []byte("private-state"), 0o644); err != nil {
		t.Fatal(err)
	}
	assetID := uuid.New()
	if _, err := catalog.SQL.ExecContext(ctx, `
		INSERT INTO assets (
			asset_id, type, original_filename, storage_path, mime_type,
			file_size, content_hash, upload_time, repository_id, updated_at
		) VALUES (?, 'PHOTO', 'kept.jpg', 'originals/kept.jpg', 'image/jpeg', ?, 'hash', 1, ?, 1)
	`, assetID, len("original-media"), created.Repository.RepoID); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.SQL.ExecContext(ctx, `
		INSERT INTO river_job (args, kind, state)
		VALUES (jsonb(?), 'process_semantic', 'available')
	`, `{"assetId":"`+assetID.String()+`"}`); err != nil {
		t.Fatal(err)
	}
	owner, err := catalog.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "removal-impact-owner", Password: "test", DisplayName: "Owner", Role: "admin",
		WebauthnUserHandle: []byte("removal-impact-owner-handle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	credentialID := uuid.New()
	if _, err := catalog.SQL.ExecContext(ctx, `
		INSERT INTO cloud_credentials (
			credential_id, provider, display_name, identity_hash, masked_identity,
			owner_id, created_at, updated_at
		) VALUES (?, 'icloud', 'Archive account', 'impact-identity', 'a***@example.com', ?, 1, 1)
	`, credentialID, owner.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.SQL.ExecContext(ctx, `
		INSERT INTO cloud_import_runs (
			run_id, repository_id, credential_id, owner_id, provider, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, 'icloud', 'completed', 1, 1)
	`, uuid.New(), created.Repository.RepoID, credentialID, owner.UserID); err != nil {
		t.Fatal(err)
	}

	impact, err := manager.PreviewRepositoryRemoval(ctx, created.Repository.RepoID.String())
	if err != nil {
		t.Fatal(err)
	}
	if impact.RepositoryName != "Removable Archive" || impact.AssetCount != 1 || impact.CatalogMediaBytes != int64(len("original-media")) {
		t.Fatalf("removal impact = %+v", impact)
	}
	if impact.CloudImportCount != 1 {
		t.Fatalf("cloud import receipt impact = %d, want 1", impact.CloudImportCount)
	}
	if impact.ActiveTaskCount != 1 {
		t.Fatalf("asset-scoped task impact = %d, want 1", impact.ActiveTaskCount)
	}
	if !impact.PrivateStateFound || impact.PrivateStateBytes < int64(len("private-state")) {
		t.Fatalf("private-state impact = %+v", impact)
	}
	manager.beforeRepositoryJobCleanup = func() {
		if _, updateErr := catalog.SQL.ExecContext(ctx, `UPDATE river_job SET state = 'running' WHERE json_extract(args, '$.assetId') = ?`, assetID.String()); updateErr != nil {
			t.Errorf("transition available asset job to running: %v", updateErr)
		}
	}
	if err := manager.RemoveRepository(ctx, created.Repository.RepoID.String()); !errors.Is(err, ErrRepositoryBusy) {
		t.Fatalf("running asset-scoped removal error = %v, want ErrRepositoryBusy", err)
	}
	manager.beforeRepositoryJobCleanup = nil
	if _, err := manager.queries.GetRepository(ctx, created.Repository.RepoID); err != nil {
		t.Fatalf("running-job rejection removed repository: %v", err)
	}
	if _, err := catalog.SQL.ExecContext(ctx, `UPDATE river_job SET state = 'available' WHERE json_extract(args, '$.assetId') = ?`, assetID.String()); err != nil {
		t.Fatal(err)
	}

	if err := manager.RemoveRepository(ctx, created.Repository.RepoID.String(), LifecycleRequest{
		RequestID: "remove-regular", Actor: "test",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.queries.GetRepository(ctx, created.Repository.RepoID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("removed repository lookup error = %v, want sql.ErrNoRows", err)
	}
	var assets int
	if err := catalog.SQL.QueryRowContext(ctx, "SELECT count(*) FROM assets WHERE asset_id = ?", assetID).Scan(&assets); err != nil {
		t.Fatal(err)
	}
	if assets != 0 {
		t.Fatalf("remaining catalog assets = %d, want 0", assets)
	}
	var remainingAssetJobs int
	if err := catalog.SQL.QueryRowContext(ctx, `
		SELECT count(*) FROM river_job
		WHERE json_extract(args, '$.assetId') = ?
	`, assetID.String()).Scan(&remainingAssetJobs); err != nil {
		t.Fatal(err)
	}
	if remainingAssetJobs != 0 {
		t.Fatalf("remaining asset-scoped jobs = %d, want 0", remainingAssetJobs)
	}
	var removalAudit int
	if err := catalog.SQL.QueryRowContext(ctx, `
		SELECT count(*) FROM lifecycle_audit_events
		WHERE request_id = 'remove-regular' AND action = 'remove_repository'
		  AND result = 'succeeded' AND confirmation_type = 'exact_repository_name'
		  AND old_path = ? AND json_extract(details, '$.files_preserved') = 1
	`, created.Repository.Path).Scan(&removalAudit); err != nil {
		t.Fatal(err)
	}
	if removalAudit != 1 {
		t.Fatalf("repository removal audit rows = %d, want 1", removalAudit)
	}
	for _, preservedPath := range []string{
		originalPath,
		privatePath,
		filepath.Join(created.Repository.Path, DefaultStructure.ConfigFile),
	} {
		if _, err := os.Stat(preservedPath); err != nil {
			t.Fatalf("preserved file %s: %v", preservedPath, err)
		}
	}

	primary, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.RemoveRepository(ctx, primary.RepoID.String()); !errors.Is(err, ErrPrimaryRepositoryNotRemovable) {
		t.Fatalf("primary removal error = %v, want ErrPrimaryRepositoryNotRemovable", err)
	}
}

func TestRemoveRepositoryHonorsContextWhileWaitingForLifecycleLeases(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	initializeDefaultStorageForTest(t, manager, filepath.Join(t.TempDir(), "default"))
	root, err := manager.queries.GetDefaultRepositoryRoot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "remove-context-create", Actor: "test", Name: "Context Archive",
		DirectoryName: "context-archive", Role: dbtypes.RepoRoleRegular, RootID: root.RootID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}

	rootRelease, err := manager.files.AccessCoordinator().AcquireRootMutationContext(ctx, root.RootID)
	if err != nil {
		t.Fatal(err)
	}
	deadlineContext, cancelDeadline := context.WithTimeout(ctx, 30*time.Millisecond)
	err = manager.RemoveRepository(deadlineContext, created.Repository.RepoID.String())
	cancelDeadline()
	rootRelease()
	if !errors.Is(err, ErrRepositoryBusy) {
		t.Fatalf("remove behind root mutation = %v, want ErrRepositoryBusy", err)
	}

	repositoryFS, err := manager.files.Open(*created.Repository)
	if err != nil {
		t.Fatal(err)
	}
	cancelContext, cancel := context.WithCancel(ctx)
	cancel()
	err = manager.RemoveRepository(cancelContext, created.Repository.RepoID.String())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("remove with cancelled context = %v, want context.Canceled", err)
	}
	if err := repositoryFS.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.queries.BeginRepositoryActivity(ctx, repo.BeginRepositoryActivityParams{
		RepoID: created.Repository.RepoID, Activity: dbtypes.RepositoryActivityScanning,
		UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	}); err != nil {
		t.Fatal(err)
	}
	if err := manager.RemoveRepository(ctx, created.Repository.RepoID.String()); !errors.Is(err, ErrRepositoryBusy) {
		t.Fatalf("remove during running worker = %v, want ErrRepositoryBusy", err)
	}
	if _, err := manager.queries.FinishRepositoryActivity(ctx, repo.FinishRepositoryActivityParams{
		RepoID: created.Repository.RepoID, Activity: dbtypes.RepositoryActivityScanning,
		UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.queries.GetRepository(ctx, created.Repository.RepoID); err != nil {
		t.Fatalf("repository changed after cancelled removal: %v", err)
	}
}
