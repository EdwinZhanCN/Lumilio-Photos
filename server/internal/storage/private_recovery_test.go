package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"

	"github.com/google/uuid"
)

const (
	leftoverTrashID   = "22222222-2222-4222-8222-222222222222"
	leftoverAssetID   = "33333333-3333-4333-8333-333333333333"
	leftoverNewerID   = "55555555-5555-4555-8555-555555555555"
	leftoverStudio    = ".lumilio/sidecars/" + leftoverAssetID + ".lumilio-sidecar"
	leftoverNewer     = ".lumilio/sidecars/" + leftoverNewerID + ".lumilio-sidecar"
	leftoverIncoming  = ".lumilio/staging/incoming/abandoned.part"
	leftoverFailed    = ".lumilio/staging/failed/old.part"
	leftoverOpsLog    = ".lumilio/logs/operations.log"
	leftoverUnknown   = ".lumilio/unknown-child/opaque.bin"
	leftoverLegacy    = ".lumilio/recovery/reopened-legacy/staging/x.part"
	leftoverTrashFile = ".lumilio/trash/files/" + leftoverTrashID + "/deleted.png"
	leftoverTrashInfo = ".lumilio/trash/info/" + leftoverTrashID + ".json"
)

// seedLeftoverPrivateState writes the private state a repository carries
// after a lost catalog: a trashed file with its format-1 info, a Studio
// record this build reads and one it does not, abandoned staging, logs with
// history, a derived face crop, opaque unknown data, and a directory an
// earlier build's blanket isolation left behind. The storage package cannot
// import testfixture (cycle), so the tree is written here.
func seedLeftoverPrivateState(t *testing.T, repositoryPath, repositoryID string) {
	t.Helper()
	write := func(relative string, data []byte) {
		t.Helper()
		target := filepath.Join(repositoryPath, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	deleted := []byte("deleted bytes")
	write(leftoverTrashFile, deleted)
	info, _ := json.Marshal(map[string]any{
		"format": 1, "trash_id": leftoverTrashID, "repository_id": repositoryID,
		"original_path": "inbox/deleted.png", "asset_id": "44444444-4444-4444-8444-444444444444",
		"hash_algorithm": "blake3-v1", "content_hash": strings.Repeat("ab", 32),
		"size": len(deleted), "mtime_ns": 1, "deleted_at": "2020-01-02T03:04:05Z", "actor": "test",
	})
	write(leftoverTrashInfo, info)
	studio, _ := json.Marshal(map[string]any{
		"version": 1, "asset_id": leftoverAssetID, "updated_at": "2020-01-02T03:04:05Z",
		"source":      map[string]any{"original_filename": "original.png", "storage_path": "inbox/original.png", "mime_type": "image/png", "file_size": 3, "hash": strings.Repeat("cd", 32)},
		"adjustments": map[string]any{"exposure": 0.5},
	})
	write(leftoverStudio, studio)
	newer, _ := json.Marshal(map[string]any{"version": 2, "asset_id": leftoverNewerID, "future": true})
	write(leftoverNewer, newer)
	write(leftoverIncoming, []byte("uncommitted incoming bytes\n"))
	write(leftoverFailed, []byte("failed staging bytes\n"))
	write(leftoverOpsLog, []byte("{\"history\":true}\n"))
	write(".lumilio/logs/error.log", nil)
	write(".lumilio/assets/faces/face.jpg", []byte("face crop"))
	write(leftoverUnknown, []byte("unknown private data\x00\xff"))
	write(leftoverLegacy, []byte("legacy isolation"))
}

// hashTree maps every regular file below root (slash-separated, relative) to
// its content hash; the proof that bytes were preserved.
func hashTree(t *testing.T, root string) map[string]string {
	t.Helper()
	hashes := map[string]string{}
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		hashes[filepath.ToSlash(relative)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hashes
}

func itemByPath(receipt RecoveryReceipt, itemPath string) (RecoveryItem, bool) {
	for _, item := range receipt.Items {
		if item.Path == itemPath {
			return item, true
		}
	}
	return RecoveryItem{}, false
}

func TestPlanPrivatePreservationClassifiesByClass(t *testing.T) {
	repositoryPath := t.TempDir()
	if plan, err := planPrivatePreservation(repositoryPath, RecoveryEventOpen, "test", "r", "id", "id", time.Now()); err != nil || !plan.Empty() || !plan.PrivateRootAbsent {
		t.Fatalf("plan without .lumilio = %+v, err=%v", plan, err)
	}
	if err := os.Mkdir(filepath.Join(repositoryPath, DefaultStructure.SystemDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if plan, err := planPrivatePreservation(repositoryPath, RecoveryEventOpen, "test", "r", "id", "id", time.Now()); err != nil || !plan.Empty() || plan.PrivateRootAbsent {
		t.Fatalf("plan with an empty .lumilio = %+v, err=%v", plan, err)
	}

	seedLeftoverPrivateState(t, repositoryPath, "11111111-1111-4111-8111-111111111111")
	// A receipt of an earlier registration and the bytes it moved are its own.
	earlier := uuid.NewString()
	earlierReceipt := RecoveryReceipt{ReceiptID: earlier, Event: RecoveryEventOpen, RecordedAt: time.Now()}
	data, err := earlierReceipt.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string][]byte{
		".lumilio/recovery/" + earlier + ".json":               data,
		".lumilio/recovery/" + earlier + "/staging/moved.part": []byte("moved earlier"),
	} {
		target := filepath.Join(repositoryPath, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	before := hashTree(t, repositoryPath)

	plan, err := planPrivatePreservation(repositoryPath, RecoveryEventRegisterCopy, "web:user:1", "req-1", "source-id", "new-id", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Empty() || plan.PrivateRootAbsent {
		t.Fatalf("plan = %+v", plan)
	}
	receipt := *plan.Receipt
	if receipt.Event != RecoveryEventRegisterCopy || receipt.SourceRepositoryID != "source-id" || receipt.RepositoryID != "new-id" || receipt.Actor != "web:user:1" {
		t.Fatalf("receipt header = %+v", receipt)
	}
	if receipt.Trash.Files != 2 || receipt.Logs.Files != 2 || receipt.Derived.Files != 1 || receipt.ItemsTruncated != 0 {
		t.Fatalf("summaries: trash=%+v logs=%+v derived=%+v truncated=%d", receipt.Trash, receipt.Logs, receipt.Derived, receipt.ItemsTruncated)
	}
	for _, summarized := range []string{leftoverTrashFile, leftoverTrashInfo, leftoverOpsLog, ".lumilio/assets/faces/face.jpg"} {
		if _, listed := itemByPath(receipt, summarized); listed {
			t.Fatalf("%s is listed as an item; it is summarized by class", summarized)
		}
	}
	studio, ok := itemByPath(receipt, leftoverStudio)
	if !ok || studio.Class != PrivateClassStudio || studio.Disposition != DispositionPreserved || studio.AssetID != leftoverAssetID {
		t.Fatalf("studio item = %+v", studio)
	}
	newer, ok := itemByPath(receipt, leftoverNewer)
	if !ok || newer.Class != PrivateClassStudio || newer.Disposition != DispositionUnreadable || !strings.Contains(newer.Detail, "version 2") {
		t.Fatalf("newer studio item = %+v", newer)
	}
	for _, staged := range []string{leftoverIncoming, leftoverFailed} {
		item, ok := itemByPath(receipt, staged)
		want := RepositoryRecoveryDirectory + "/" + receipt.ReceiptID + "/" + strings.TrimPrefix(staged, ".lumilio/")
		if !ok || item.Class != PrivateClassStaging || item.Disposition != DispositionMoved || item.MovedTo != want {
			t.Fatalf("staging item %s = %+v, want moved to %s", staged, item, want)
		}
	}
	if len(plan.Moves) != 2 {
		t.Fatalf("moves = %+v", plan.Moves)
	}
	unknown, ok := itemByPath(receipt, leftoverUnknown)
	if !ok || unknown.Class != PrivateClassUnknown || unknown.Disposition != DispositionPreserved {
		t.Fatalf("unknown item = %+v", unknown)
	}
	legacy, ok := itemByPath(receipt, leftoverLegacy)
	if !ok || legacy.Class != PrivateClassUnknown || legacy.Detail == "" {
		t.Fatalf("legacy isolation item = %+v", legacy)
	}
	if _, listed := itemByPath(receipt, ".lumilio/recovery/"+earlier+"/staging/moved.part"); listed {
		t.Fatal("bytes an earlier receipt moved were listed again")
	}
	if _, listed := itemByPath(receipt, ".lumilio/recovery/"+earlier+".json"); listed {
		t.Fatal("an earlier receipt was listed as unknown content")
	}
	ids := map[string]bool{}
	for _, item := range receipt.Items {
		if item.ItemID == "" || ids[item.ItemID] {
			t.Fatalf("item ids are not unique and stable: %+v", receipt.Items)
		}
		ids[item.ItemID] = true
	}
	// Planning is read-only.
	if after := hashTree(t, repositoryPath); len(after) != len(before) {
		t.Fatalf("planning changed the tree: before %d files, after %d", len(before), len(after))
	}
	for name, sum := range before {
		if hashTree(t, repositoryPath)[name] != sum {
			t.Fatalf("planning changed %s", name)
		}
	}
	// The receipt round-trips through its wire format.
	encoded, err := receipt.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ParseRecoveryReceipt(encoded)
	if err != nil || decoded.ReceiptID != receipt.ReceiptID || len(decoded.Items) != len(receipt.Items) {
		t.Fatalf("receipt round trip = %+v, err=%v", decoded, err)
	}
	if _, err := ParseRecoveryReceipt([]byte(`{"format":2,"receipt_id":"` + receipt.ReceiptID + `"}`)); !errors.Is(err, ErrRecoveryReceiptFormat) {
		t.Fatalf("newer receipt format error = %v", err)
	}
}

// assertPreservedByClass checks the outcome every registration owes the
// private tree: every file that was there is still there with its bytes,
// staging now under the receipt, and the receipt names it all.
func assertPreservedByClass(t *testing.T, repositoryPath string, before map[string]string, event RecoveryEvent) RecoveryReceipt {
	t.Helper()
	after := hashTree(t, repositoryPath)
	receipts, err := filepath.Glob(filepath.Join(repositoryPath, filepath.FromSlash(RepositoryRecoveryDirectory), "*.json"))
	if err != nil || len(receipts) != 1 {
		t.Fatalf("receipts = %v, err=%v", receipts, err)
	}
	data, err := os.ReadFile(receipts[0])
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := ParseRecoveryReceipt(data)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Event != event {
		t.Fatalf("receipt event = %s, want %s", receipt.Event, event)
	}
	movedTo := map[string]string{}
	for _, item := range receipt.Items {
		if item.MovedTo != "" {
			movedTo[item.Path] = item.MovedTo
		}
	}
	for name, sum := range before {
		current := name
		if to, moved := movedTo[name]; moved {
			current = to
			if _, stillThere := after[name]; stillThere {
				t.Fatalf("%s was moved to %s but is still at its source", name, to)
			}
		}
		if after[current] != sum {
			t.Fatalf("%s: bytes changed or lost (now at %s: %q, want %q)", name, current, after[current], sum)
		}
	}
	if len(movedTo) != 2 {
		t.Fatalf("moved items = %v, want the two staging files", movedTo)
	}
	return receipt
}

func TestOpenRepositoryPreservesPrivateStateByClass(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	initializeDefaultStorageForTest(t, manager, filepath.Join(t.TempDir(), "default"))
	storageLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "create-leftover", Actor: "test", Name: "Leftover", DirectoryName: "leftover",
		Role: dbtypes.RepoRoleRegular, StorageLocationID: storageLocation.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	repositoryPath := created.Repository.Path
	repositoryID := created.Repository.RepoID.String()
	if _, err := manager.RemoveRepository(ctx, repositoryID); err != nil {
		t.Fatal(err)
	}
	seedLeftoverPrivateState(t, repositoryPath, repositoryID)
	before := hashTree(t, repositoryPath)

	request := LifecycleRequest{RequestID: "open-leftover", Actor: "web:user:7"}
	opened, err := manager.OpenRepository(ctx, repositoryPath, nil, dbtypes.RepoRoleRegular, request)
	if err != nil {
		t.Fatal(err)
	}
	receipt := assertPreservedByClass(t, repositoryPath, before, RecoveryEventOpen)
	if receipt.SourceRepositoryID != repositoryID || receipt.RepositoryID != repositoryID || receipt.RequestID != "open-leftover" {
		t.Fatalf("open receipt = %+v", receipt)
	}
	if receipt.Trash.Files != 2 {
		t.Fatalf("trash summary = %+v", receipt.Trash)
	}
	// The fresh layout exists around the preserved state, and the logs keep
	// their history rather than being truncated by the layout creation.
	for _, directory := range []string{DefaultStructure.IncomingDir, DefaultStructure.FailedDir, DefaultStructure.SidecarsDir} {
		if info, err := os.Stat(filepath.Join(repositoryPath, filepath.FromSlash(directory))); err != nil || !info.IsDir() {
			t.Fatalf("layout directory %s missing after open: %v", directory, err)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(repositoryPath, filepath.FromSlash(DefaultStructure.IncomingDir))); len(entries) != 0 {
		t.Fatalf("incoming staging still holds %d entries after open", len(entries))
	}
	// Replaying the same request neither remints a receipt nor repeats a move.
	replayed, err := manager.OpenRepository(ctx, repositoryPath, nil, dbtypes.RepoRoleRegular, request)
	if err != nil || replayed.RepoID != opened.RepoID {
		t.Fatalf("replay = %v, err=%v", replayed, err)
	}
	assertPreservedByClass(t, repositoryPath, before, RecoveryEventOpen)
	var audits int
	if err := catalog.SQL.QueryRowContext(ctx, `SELECT count(*) FROM lifecycle_audit_events WHERE action = 'open_repository' AND result = 'succeeded'`).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("open audits = %d, err=%v", audits, err)
	}
}

func TestPrivatePreservationRollsBackACrashBetweenMoves(t *testing.T) {
	repositoryPath := t.TempDir()
	seedLeftoverPrivateState(t, repositoryPath, "11111111-1111-4111-8111-111111111111")
	before := hashTree(t, repositoryPath)
	plan, err := planPrivatePreservation(repositoryPath, RecoveryEventOpen, "test", "r", "id", "id", time.Now())
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
	defer func() { testHookAfterPrivateStateMove = nil }()
	if err := applyPrivatePreservation(repositoryPath, plan); !errors.Is(err, crash) {
		t.Fatalf("apply error = %v, want the injected crash", err)
	}
	// One staging file moved, the receipt is written: the state a crash leaves.
	if _, err := os.Stat(filepath.Join(repositoryPath, filepath.FromSlash(plan.Moves[0].To))); err != nil {
		t.Fatalf("first move did not happen: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repositoryPath, filepath.FromSlash(plan.Moves[1].From))); err != nil {
		t.Fatalf("second file moved despite the crash: %v", err)
	}
	if err := rollbackPrivateState(repositoryPath, plan); err != nil {
		t.Fatal(err)
	}
	after := hashTree(t, repositoryPath)
	if len(after) != len(before) {
		t.Fatalf("rollback left %d files, want %d: %v", len(after), len(before), after)
	}
	for name, sum := range before {
		if after[name] != sum {
			t.Fatalf("rollback did not restore %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(repositoryPath, filepath.FromSlash(RepositoryRecoveryDirectory), plan.ReceiptID()+".json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("receipt survived the rollback: %v", err)
	}
	// Rolling back twice is harmless.
	if err := rollbackPrivateState(repositoryPath, plan); err != nil {
		t.Fatal(err)
	}
}

func TestPrivatePreservationRetryFinishesAnInterruptedPlanWithoutRepeatingMoves(t *testing.T) {
	repositoryPath := t.TempDir()
	seedLeftoverPrivateState(t, repositoryPath, "11111111-1111-4111-8111-111111111111")
	before := hashTree(t, repositoryPath)
	plan, err := planPrivatePreservation(repositoryPath, RecoveryEventOpen, "test", "r", "id", "id", time.Now())
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
	if err := applyPrivatePreservation(repositoryPath, plan); !errors.Is(err, crash) {
		t.Fatalf("apply error = %v", err)
	}
	testHookAfterPrivateStateMove = nil
	// A move that stopped between link and unlink leaves the file under both
	// names; the retry keeps the destination and drops the source.
	second := plan.Moves[1]
	if err := os.Link(filepath.Join(repositoryPath, filepath.FromSlash(second.From)), filepath.Join(repositoryPath, filepath.FromSlash(second.To))); err != nil {
		t.Skipf("hard links unavailable here: %v", err)
	}
	if err := applyPrivatePreservation(repositoryPath, plan); err != nil {
		t.Fatalf("retry = %v", err)
	}
	after := hashTree(t, repositoryPath)
	for _, move := range plan.Moves {
		if _, there := after[move.From]; there {
			t.Fatalf("%s is still at its source after the retry", move.From)
		}
		if after[move.To] != before[move.From] {
			t.Fatalf("%s bytes differ after the retry", move.To)
		}
	}
	if receipts, _ := filepath.Glob(filepath.Join(repositoryPath, filepath.FromSlash(RepositoryRecoveryDirectory), "*.json")); len(receipts) != 1 {
		t.Fatalf("receipts after retry = %v", receipts)
	}
	// A third apply is a no-op.
	if err := applyPrivatePreservation(repositoryPath, plan); err != nil {
		t.Fatalf("idempotent apply = %v", err)
	}
	// A different file at a destination is a conflict, never replaced.
	conflict := privatePreservationPlan{Receipt: plan.Receipt, Moves: []privateStateMove{{From: leftoverUnknown, To: plan.Moves[0].To}}}
	if err := applyPrivatePreservation(repositoryPath, conflict); !errors.Is(err, ErrRepositoryDestinationExists) {
		t.Fatalf("conflicting move error = %v", err)
	}
	if hashTree(t, repositoryPath)[plan.Moves[0].To] != before[plan.Moves[0].From] {
		t.Fatal("a conflicting move replaced the destination")
	}
}

func TestLifecycleRecoveryRollsBackAnOpenInterruptedAfterItsFilesystemPhase(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	initializeDefaultStorageForTest(t, manager, filepath.Join(t.TempDir(), "default"))
	storageLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "create-interrupted", Actor: "test", Name: "Interrupted", DirectoryName: "interrupted",
		Role: dbtypes.RepoRoleRegular, StorageLocationID: storageLocation.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	repositoryPath := created.Repository.Path
	repositoryID := created.Repository.RepoID.String()
	if _, err := manager.RemoveRepository(ctx, repositoryID); err != nil {
		t.Fatal(err)
	}
	seedLeftoverPrivateState(t, repositoryPath, repositoryID)
	before := hashTree(t, repositoryPath)

	// The journal an open leaves when the process dies after the private
	// tree was changed and before the catalog row was inserted.
	config, err := repocfg.LoadConfigFromFile(repositoryPath)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := planPrivatePreservation(repositoryPath, RecoveryEventOpen, "test", "open-crash", config.ID, config.ID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	operation, _, err := manager.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: "open-crash", Kind: lifecycleKindOpenRepository,
		Payload: openRepositoryOperationPayload{Path: repositoryPath, StorageLocationID: storageLocation.StorageLocationID.String(), Role: dbtypes.RepoRoleRegular},
		Actor:   "test", TargetType: "repository", TargetID: &config.ID,
		RollbackData: registerRepositoryCopyRollbackData{Preservation: plan},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhasePrepared, registerRepositoryCopyRollbackData{Preservation: plan}); err != nil {
		t.Fatal(err)
	}
	if err := applyPrivatePreservation(repositoryPath, plan); err != nil {
		t.Fatal(err)
	}
	if err := manager.dirManager.CreateStructure(repositoryPath); err != nil {
		t.Fatal(err)
	}
	if err := manager.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseFilesystemApplied, registerRepositoryCopyRollbackData{Preservation: plan}); err != nil {
		t.Fatal(err)
	}

	if err := manager.RecoverLifecycleOperations(ctx); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := catalog.SQL.QueryRowContext(ctx, `SELECT status FROM lifecycle_operations WHERE operation_id = ?`, operation.OperationID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != lifecycleStatusRolledBack {
		t.Fatalf("recovered open status = %s, want rolled_back", status)
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
		t.Fatal("an interrupted open registered the repository")
	}
	var rows []repo.LifecycleOperation
	if rows, err = manager.queries.ListIncompleteLifecycleOperations(ctx); err != nil || len(rows) != 0 {
		t.Fatalf("incomplete operations after recovery = %d, err=%v", len(rows), err)
	}
}

func TestCreateStructureKeepsExistingLogs(t *testing.T) {
	repositoryPath := t.TempDir()
	seedLeftoverPrivateState(t, repositoryPath, "11111111-1111-4111-8111-111111111111")
	if err := NewDirectoryManager().CreateStructure(repositoryPath); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(repositoryPath, filepath.FromSlash(leftoverOpsLog)))
	if err != nil || string(content) != "{\"history\":true}\n" {
		t.Fatalf("operations log after CreateStructure = %q, err=%v", content, err)
	}
}
