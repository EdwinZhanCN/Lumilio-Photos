package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"
)

// leftoverPrimaryForTest registers a default Storage Location whose fixed
// primary child was created by an earlier catalog and then lost: the marker
// and the private state are on disk, the catalog knows neither. It is the
// state a reinstall finds (R3). The returned catalog has no primary.
func leftoverPrimaryForTest(t *testing.T, catalog *db.DB, manager *DefaultRepositoryManager) (repositoryPath, repositoryID string) {
	t.Helper()
	ctx := context.Background()
	rootPath := filepath.Join(t.TempDir(), "default")
	storageLocation, err := manager.EnsureDefaultStorageLocation(ctx, rootPath)
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "create-lost-primary", Actor: "test", Name: "Lost Primary", DirectoryName: "primary",
		Role: dbtypes.RepoRolePrimary, StorageLocationID: storageLocation.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	repositoryPath, repositoryID = created.Repository.Path, created.Repository.RepoID.String()
	if err := os.WriteFile(filepath.Join(repositoryPath, "inbox", "original.png"), []byte("original bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The catalog forgets the primary the way a lost app state does: the
	// row is gone, every file stays. (The primary is protected from detach.)
	if _, err := catalog.SQL.ExecContext(ctx, `DELETE FROM repositories WHERE repo_id = ?`, created.Repository.RepoID); err != nil {
		t.Fatal(err)
	}
	if count, err := manager.queries.CountPrimaryRepositories(ctx); err != nil || count != 0 {
		t.Fatalf("primary count after loss = %d, err=%v", count, err)
	}
	seedLeftoverPrivateState(t, repositoryPath, repositoryID)
	return repositoryPath, repositoryID
}

func adoptReviewed(t *testing.T, manager *DefaultRepositoryManager, review *SetupPrimaryAssessment, requestID string) (*AdoptPrimaryResult, error) {
	t.Helper()
	expected := ""
	if review.Existing != nil {
		expected = review.Existing.RepositoryID
	}
	// No users row exists in this catalog, so the owner stays unset here;
	// the handler tests cover the Host Owner assignment.
	return manager.AdoptPrimaryRepository(context.Background(), AdoptPrimarySpec{
		RequestID: requestID, Actor: "web:user:7",
		ExpectedRepositoryID: expected, AssessmentToken: review.Token, AcknowledgedRisks: RiskCodes(review.Risks),
	})
}

// R3: explicit adoption keeps the identity and the originals, assigns the
// owner, preserves the private state under an adopt receipt, creates
// exactly one primary, queues the scan, replays idempotently, and never
// runs on a review the administrator did not see.
func TestAdoptPrimaryRepositoryKeepsIdentityAndPreservesPrivateState(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	repositoryPath, repositoryID := leftoverPrimaryForTest(t, catalog, manager)
	var scans []string
	manager.SetInitialScanEnqueuer(func(_ context.Context, id string) error {
		scans = append(scans, id)
		return nil
	})
	before := hashTree(t, repositoryPath)

	review, err := manager.AssessSetupPrimaryTarget(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if review.State != SetupPrimaryAdoptable || !review.Adoptable() || review.Existing == nil || review.Existing.RepositoryID != repositoryID ||
		review.Existing.Name != "Lost Primary" || len(review.LayoutMissing) != 0 || review.Token == "" {
		t.Fatalf("setup review = %+v (existing %+v)", review, review.Existing)
	}
	if got := review.Actions; len(got) != 2 || got[0] != SetupActionAdopt || got[1] != SetupActionFreshElsewhere {
		t.Fatalf("actions = %v", got)
	}
	if p := review.Recovery; p == nil || p.TrashFiles != 2 || p.StudioRecords != 2 || p.StagingFiles != 2 || p.UnknownFiles < 1 || p.LogFiles < 1 {
		t.Fatalf("recovery preview = %+v", review.Recovery)
	}
	if after := hashTree(t, repositoryPath); len(after) != len(before) {
		t.Fatal("the review changed the tree")
	}

	// No review token: the Problem carries the token to retry with, even
	// though the local target reports no risk.
	var consent *StorageRiskConfirmationError
	_, err = manager.AdoptPrimaryRepository(ctx, AdoptPrimarySpec{RequestID: "adopt-no-token", Actor: "web:user:7", ExpectedRepositoryID: repositoryID})
	if !errors.As(err, &consent) || consent.Token != review.Token {
		t.Fatalf("adoption without a review = %v", err)
	}
	// A review of another identity does not adopt this one.
	_, err = manager.AdoptPrimaryRepository(ctx, AdoptPrimarySpec{
		RequestID: "adopt-wrong-identity", Actor: "web:user:7", ExpectedRepositoryID: "99999999-9999-4999-8999-999999999999", AssessmentToken: review.Token,
	})
	if !errors.Is(err, ErrStorageAssessmentStale) {
		t.Fatalf("adoption with a different expected identity = %v, want ErrStorageAssessmentStale", err)
	}
	if count, _ := manager.queries.CountPrimaryRepositories(ctx); count != 0 {
		t.Fatal("a refused adoption registered a primary")
	}

	adopted, err := adoptReviewed(t, manager, review, "adopt-primary")
	if err != nil {
		t.Fatal(err)
	}
	if adopted.Replayed || adopted.Repository.RepoID.String() != repositoryID || adopted.Repository.Role != dbtypes.RepoRolePrimary ||
		adopted.Repository.Path != repositoryPath || adopted.Repository.DefaultOwnerID != nil ||
		adopted.ReceiptID == "" || adopted.Receipt == nil || adopted.Receipt.Event != RecoveryEventAdopt {
		t.Fatalf("adopted = %+v receipt=%+v", adopted.Repository, adopted.Receipt)
	}
	receipt := assertPreservedByClass(t, repositoryPath, before, RecoveryEventAdopt)
	if receipt.ReceiptID != adopted.ReceiptID || receipt.SourceRepositoryID != repositoryID || receipt.RepositoryID != repositoryID || receipt.Trash.Files != 2 {
		t.Fatalf("adopt receipt = %+v", receipt)
	}
	after := hashTree(t, repositoryPath)
	for _, name := range []string{".lumiliorepo", "inbox/original.png"} {
		if after[name] != before[name] {
			t.Fatalf("adoption rewrote %s", name)
		}
	}
	if count, _ := manager.queries.CountPrimaryRepositories(ctx); count != 1 {
		t.Fatalf("primary count after adoption = %d", count)
	}
	if len(scans) != 1 || scans[0] != repositoryID {
		t.Fatalf("initial scans = %v", scans)
	}
	status, err := manager.StorageRuntimeStatus(ctx)
	if err != nil || status.State != StorageRuntimeStateActive {
		t.Fatalf("runtime status after adoption = %+v err=%v", status, err)
	}

	replayed, err := adoptReviewed(t, manager, review, "adopt-primary")
	if err != nil || !replayed.Replayed || replayed.Repository.RepoID.String() != repositoryID || replayed.ReceiptID != adopted.ReceiptID || replayed.Receipt == nil {
		t.Fatalf("replay = %+v err=%v", replayed, err)
	}
	assertPreservedByClass(t, repositoryPath, before, RecoveryEventAdopt)
	if len(scans) != 1 {
		t.Fatalf("replay queued another scan: %v", scans)
	}
	// A new request against the now-registered identity is the conflict the
	// Storage page understands, never a second registration.
	var conflict *RepositoryConflictError
	if _, err := adoptReviewed(t, manager, review, "adopt-primary-again"); !errors.As(err, &conflict) || conflict.RepositoryID != repositoryID {
		t.Fatalf("second adoption = %v, want the registered-identity conflict", err)
	}
	if count, _ := manager.queries.CountPrimaryRepositories(ctx); count != 1 {
		t.Fatalf("primary count after the refused second adoption = %d", count)
	}
	var audits int
	if err := catalog.SQL.QueryRowContext(ctx, `SELECT count(*) FROM lifecycle_audit_events WHERE action = 'adopt_primary_repository' AND result = 'succeeded'`).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("adopt audits = %d, err=%v", audits, err)
	}
	if review, err := manager.AssessSetupPrimaryTarget(ctx); err != nil || review.State != SetupPrimaryRegistered || review.Adoptable() {
		t.Fatalf("review after adoption = %+v err=%v", review, err)
	}
}

// R12: a compatible marker whose layout is missing is adopted with the
// missing directories added and nothing else; a file where a directory
// belongs blocks the repair and leaves every byte in place.
func TestAdoptPrimaryRepairsAMissingLayoutButNotAnObstructedOne(t *testing.T) {
	t.Run("missing private workspace is completed", func(t *testing.T) {
		catalog, manager := newCatalogRepositoryManager(t)
		ctx := context.Background()
		repositoryPath, repositoryID := leftoverPrimaryForTest(t, catalog, manager)
		if err := os.RemoveAll(filepath.Join(repositoryPath, DefaultStructure.SystemDir)); err != nil {
			t.Fatal(err)
		}
		before := hashTree(t, repositoryPath)
		review, err := manager.AssessSetupPrimaryTarget(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if review.State != SetupPrimaryLayoutRepairable || len(review.LayoutMissing) != 1 || review.LayoutMissing[0] != DefaultStructure.SystemDir || review.Existing.RepositoryID != repositoryID {
			t.Fatalf("review = %+v missing=%v", review.State, review.LayoutMissing)
		}
		adopted, err := adoptReviewed(t, manager, review, "adopt-repair")
		if err != nil {
			t.Fatal(err)
		}
		if adopted.ReceiptID != "" || adopted.Receipt != nil || len(adopted.LayoutRepaired) != 1 {
			t.Fatalf("repair result = %+v", adopted)
		}
		for _, directory := range []string{DefaultStructure.IncomingDir, DefaultStructure.SidecarsDir, DefaultStructure.InboxDir} {
			if info, err := os.Stat(filepath.Join(repositoryPath, filepath.FromSlash(directory))); err != nil || !info.IsDir() {
				t.Fatalf("layout directory %s missing after repair: %v", directory, err)
			}
		}
		after := hashTree(t, repositoryPath)
		for name, sum := range before {
			if after[name] != sum {
				t.Fatalf("repair changed %s", name)
			}
		}
		if recorded, err := manager.GetRepository(repositoryID); err != nil || recorded.Role != dbtypes.RepoRolePrimary {
			t.Fatalf("repaired primary = %+v err=%v", recorded, err)
		}
	})
	t.Run("a file in place of inbox blocks the repair", func(t *testing.T) {
		catalog, manager := newCatalogRepositoryManager(t)
		ctx := context.Background()
		repositoryPath, _ := leftoverPrimaryForTest(t, catalog, manager)
		if err := os.RemoveAll(filepath.Join(repositoryPath, DefaultStructure.SystemDir)); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(filepath.Join(repositoryPath, DefaultStructure.InboxDir)); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repositoryPath, DefaultStructure.InboxDir), []byte("not a directory"), 0o644); err != nil {
			t.Fatal(err)
		}
		before := hashTree(t, repositoryPath)
		review, err := manager.AssessSetupPrimaryTarget(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = adoptReviewed(t, manager, review, "adopt-obstructed")
		if !errors.Is(err, ErrRepositoryLayoutObstructed) {
			t.Fatalf("obstructed repair = %v, want ErrRepositoryLayoutObstructed", err)
		}
		after := hashTree(t, repositoryPath)
		if len(after) != len(before) {
			t.Fatalf("obstructed repair changed the tree: %v", after)
		}
		for name, sum := range before {
			if after[name] != sum {
				t.Fatalf("obstructed repair changed %s", name)
			}
		}
		if _, err := os.Stat(filepath.Join(repositoryPath, DefaultStructure.SystemDir)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("obstructed repair created the private workspace: %v", err)
		}
		if count, _ := manager.queries.CountPrimaryRepositories(ctx); count != 0 {
			t.Fatal("obstructed repair registered a primary")
		}
	})
}

// Adoption is journaled like every other registration: a crash between two
// staging moves rolls the tree back to what the user had, leaves no primary,
// and a retry finishes from the same facts.
func TestAdoptPrimaryRollsBackACrashBetweenMovesAndRetries(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	repositoryPath, repositoryID := leftoverPrimaryForTest(t, catalog, manager)
	before := hashTree(t, repositoryPath)
	review, err := manager.AssessSetupPrimaryTarget(ctx)
	if err != nil {
		t.Fatal(err)
	}
	crash := errors.New("simulated crash")
	testHookAfterPrivateStateMove = func(moved int) error {
		if moved == 1 {
			return crash
		}
		return nil
	}
	_, err = adoptReviewed(t, manager, review, "adopt-crash")
	testHookAfterPrivateStateMove = nil
	if !errors.Is(err, crash) {
		t.Fatalf("adoption error = %v, want the injected crash", err)
	}
	after := hashTree(t, repositoryPath)
	if len(after) != len(before) {
		t.Fatalf("rollback did not restore the tree: %d files, want %d", len(after), len(before))
	}
	for name, sum := range before {
		if after[name] != sum {
			t.Fatalf("rollback did not restore %s", name)
		}
	}
	if count, _ := manager.queries.CountPrimaryRepositories(ctx); count != 0 {
		t.Fatal("a crashed adoption registered a primary")
	}
	var status string
	if err := catalog.SQL.QueryRowContext(ctx, `SELECT status FROM lifecycle_operations WHERE request_id = 'adopt-crash'`).Scan(&status); err != nil || status != lifecycleStatusRolledBack {
		t.Fatalf("crashed adoption status = %q err=%v", status, err)
	}
	retried, err := adoptReviewed(t, manager, review, "adopt-retry")
	if err != nil || retried.Repository.RepoID.String() != repositoryID {
		t.Fatalf("retry = %+v err=%v", retried, err)
	}
	assertPreservedByClass(t, repositoryPath, before, RecoveryEventAdopt)
}

// Startup journal recovery undoes an adoption the process did not finish
// after its filesystem phase and leaves no primary behind.
func TestLifecycleRecoveryRollsBackAnAdoptionInterruptedAfterItsFilesystemPhase(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	repositoryPath, repositoryID := leftoverPrimaryForTest(t, catalog, manager)
	storageLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	before := hashTree(t, repositoryPath)
	config, err := repocfg.LoadConfigFromFile(repositoryPath)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := planPrivatePreservation(repositoryPath, RecoveryEventAdopt, "test", "adopt-interrupted", config.ID, config.ID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	rollback := adoptPrimaryRollbackData{Preservation: plan}
	operation, _, err := manager.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: "adopt-interrupted", Kind: lifecycleKindAdoptPrimaryRepository,
		Payload: adoptPrimaryOperationPayload{Path: repositoryPath, StorageLocationID: storageLocation.StorageLocationID.String(), RepositoryID: config.ID},
		Actor:   "test", TargetType: "repository", TargetID: &config.ID, RollbackData: rollback,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhasePrepared, rollback); err != nil {
		t.Fatal(err)
	}
	if err := applyPrivatePreservation(repositoryPath, plan); err != nil {
		t.Fatal(err)
	}
	if err := manager.dirManager.CreateStructure(repositoryPath); err != nil {
		t.Fatal(err)
	}
	if err := manager.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseFilesystemApplied, rollback); err != nil {
		t.Fatal(err)
	}

	if err := manager.RecoverLifecycleOperations(ctx); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := catalog.SQL.QueryRowContext(ctx, `SELECT status FROM lifecycle_operations WHERE operation_id = ?`, operation.OperationID).Scan(&status); err != nil || status != lifecycleStatusRolledBack {
		t.Fatalf("recovered adoption status = %q err=%v", status, err)
	}
	after := hashTree(t, repositoryPath)
	for name, sum := range before {
		if after[name] != sum {
			t.Fatalf("recovery did not restore %s", name)
		}
	}
	if receipts, _ := filepath.Glob(filepath.Join(repositoryPath, filepath.FromSlash(RepositoryRecoveryDirectory), "*.json")); len(receipts) != 0 {
		t.Fatalf("receipt survived the recovery rollback: %v", receipts)
	}
	if _, err := manager.GetRepository(repositoryID); err == nil {
		t.Fatal("an interrupted adoption registered the primary")
	}
	// The next setup review offers the adoption again.
	if review, err := manager.AssessSetupPrimaryTarget(ctx); err != nil || review.State != SetupPrimaryAdoptable {
		t.Fatalf("review after recovery = %+v err=%v", review, err)
	}
}

// Smaller decision "Fresh default during setup": before the Primary
// Repository exists, a different configured path becomes a fresh Default
// Storage Location and the previous tree is left untouched; once the primary
// exists the same change is the identity-preserving move it always was.
func TestFreshDefaultDuringSetupReplacesAnUnusedRegistration(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	oldPath := filepath.Join(base, "default-old")
	oldLocation, err := manager.EnsureDefaultStorageLocation(ctx, oldPath)
	if err != nil {
		t.Fatal(err)
	}
	// Something a user may have left in the old tree, without a Repository.
	if err := os.WriteFile(filepath.Join(oldPath, "notes.txt"), []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldBefore := hashTree(t, oldPath)

	// A corrupt root marker at the new path is refused and changes nothing.
	corruptPath := filepath.Join(base, "default-corrupt")
	if err := os.MkdirAll(corruptPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(corruptPath, rootcfg.FileName), []byte("version: [unterminated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.EnsureDefaultStorageLocation(ctx, corruptPath); !errors.Is(err, ErrStorageLocationInvalid) {
		t.Fatalf("corrupt fresh target = %v, want ErrStorageLocationInvalid", err)
	}
	if recorded, err := manager.queries.GetDefaultStorageLocation(ctx); err != nil || recorded.StorageLocationID != oldLocation.StorageLocationID {
		t.Fatalf("refused fresh switch changed the registration: %+v err=%v", recorded, err)
	}

	newPath := filepath.Join(base, "default-new")
	fresh, err := manager.EnsureDefaultStorageLocation(ctx, newPath, LifecycleRequest{
		RequestID: "fresh-default-1", Actor: "desktop_host:settings", HostInstanceID: "desktop-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	canonicalNew, _ := CanonicalizeRepositoryPath(newPath)
	if fresh.StorageLocationID == oldLocation.StorageLocationID || fresh.Kind != dbtypes.StorageLocationKindDefault || fresh.Path != canonicalNew {
		t.Fatalf("fresh default = %+v (old %s)", fresh, oldLocation.StorageLocationID)
	}
	if marker, err := rootcfg.Load(newPath); err != nil || marker.ID != fresh.StorageLocationID.String() {
		t.Fatalf("fresh marker = %+v err=%v", marker, err)
	}
	if recorded, err := manager.queries.GetDefaultStorageLocation(ctx); err != nil || recorded.StorageLocationID != fresh.StorageLocationID {
		t.Fatalf("default registration = %+v err=%v", recorded, err)
	}
	if _, err := manager.queries.GetStorageLocation(ctx, oldLocation.StorageLocationID); err == nil {
		t.Fatal("the unused old default registration survived the fresh switch")
	}
	oldAfter := hashTree(t, oldPath)
	if len(oldAfter) != len(oldBefore) {
		t.Fatalf("fresh switch touched the old tree: %v", oldAfter)
	}
	for name, sum := range oldBefore {
		if oldAfter[name] != sum {
			t.Fatalf("fresh switch changed %s in the old tree", name)
		}
	}
	// The same path on the next start is the registered default.
	if again, err := manager.EnsureDefaultStorageLocation(ctx, newPath); err != nil || again.StorageLocationID != fresh.StorageLocationID {
		t.Fatalf("restart on the fresh default = %+v err=%v", again, err)
	}
	var confirmation string
	if err := catalog.SQL.QueryRowContext(ctx, `SELECT json_extract(payload, '$.confirmation_type') FROM lifecycle_operations WHERE request_id = 'fresh-default-1'`).Scan(&confirmation); err != nil || confirmation != "fresh_default" {
		t.Fatalf("fresh switch journal = %q err=%v", confirmation, err)
	}

	// A folder that already carries an unregistered compatible root keeps its identity.
	markedPath := filepath.Join(base, "default-marked")
	if err := os.MkdirAll(markedPath, 0o755); err != nil {
		t.Fatal(err)
	}
	existingRoot := rootcfg.New("Earlier default")
	if err := existingRoot.Save(markedPath); err != nil {
		t.Fatal(err)
	}
	marked, err := manager.EnsureDefaultStorageLocation(ctx, markedPath)
	if err != nil || marked.StorageLocationID.String() != existingRoot.ID {
		t.Fatalf("fresh switch onto a marked folder = %+v err=%v", marked, err)
	}

	// Once the primary exists, a different path must prove the moved identity.
	if _, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "fresh-primary", Actor: "test", Name: "Primary", DirectoryName: "primary",
		Role: dbtypes.RepoRolePrimary, StorageLocationID: marked.StorageLocationID.String(),
	}); err != nil {
		t.Fatal(err)
	}
	laterPath := filepath.Join(base, "default-later")
	if _, err := manager.EnsureDefaultStorageLocation(ctx, laterPath); err == nil {
		t.Fatal("a different path after primary completion was accepted as a fresh default")
	}
	if recorded, err := manager.queries.GetDefaultStorageLocation(ctx); err != nil || recorded.StorageLocationID != marked.StorageLocationID {
		t.Fatalf("established default changed: %+v err=%v", recorded, err)
	}
}

// A fresh-default switch the process did not finish is undone at startup:
// the marker it created is removed and the previous registration stays.
func TestLifecycleRecoveryRollsBackAnInterruptedFreshDefaultSwitch(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	oldPath := filepath.Join(base, "default-old")
	oldLocation, err := manager.EnsureDefaultStorageLocation(ctx, oldPath)
	if err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(base, "default-new")
	if err := os.MkdirAll(newPath, 0o755); err != nil {
		t.Fatal(err)
	}
	created := rootcfg.New("Default storage")
	if err := created.Save(newPath); err != nil {
		t.Fatal(err)
	}
	targetID := oldLocation.StorageLocationID.String()
	rollback := createStorageLocationRollbackData{Path: newPath, MarkerCreated: true}
	operation, _, err := manager.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: "fresh-crash", Kind: lifecycleKindSwitchDefaultStorage,
		Payload: switchDefaultStorageOperationPayload{
			StorageLocationID: targetID, OldPath: oldLocation.Path, NewPath: newPath,
			ConfirmationType: "fresh_default", NewStorageLocationID: created.ID,
		},
		Actor: "test", TargetType: "storage_location", TargetID: &targetID, RollbackData: rollback,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseFilesystemApplied, rollback); err != nil {
		t.Fatal(err)
	}
	if err := manager.RecoverLifecycleOperations(ctx); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := catalog.SQL.QueryRowContext(ctx, `SELECT status FROM lifecycle_operations WHERE operation_id = ?`, operation.OperationID).Scan(&status); err != nil || status != lifecycleStatusRolledBack {
		t.Fatalf("recovered fresh switch status = %q err=%v", status, err)
	}
	if _, err := os.Stat(filepath.Join(newPath, rootcfg.FileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("created marker survived the rollback: %v", err)
	}
	if recorded, err := manager.queries.GetDefaultStorageLocation(ctx); err != nil || recorded.StorageLocationID != oldLocation.StorageLocationID {
		t.Fatalf("previous default lost: %+v err=%v", recorded, err)
	}
}
