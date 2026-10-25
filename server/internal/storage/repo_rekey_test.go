package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/pathsemantics"

	"github.com/google/uuid"
)

var (
	insensitiveNFC = pathsemantics.Semantics{Case: pathsemantics.CaseInsensitive, Normalization: pathsemantics.NormalizationNFC}
	sensitiveNone  = pathsemantics.Semantics{Case: pathsemantics.CaseSensitive, Normalization: pathsemantics.NormalizationNone}
)

// pretendVolume makes every tree below prefix report rule, while every other
// tree keeps the measured rule of the host's temporary filesystem. It stands
// in for an exFAT or case-sensitive disk on the ordinary CI runner; the
// filesystem-image jobs measure the real volumes.
func pretendVolume(manager *DefaultRepositoryManager, prefix string, rule pathsemantics.Semantics) {
	manager.SetVolumeQualifier(func(root string) (pathsemantics.Semantics, error) {
		if root == prefix || strings.HasPrefix(root, prefix+string(filepath.Separator)) {
			return rule, nil
		}
		return pathsemantics.Qualify(root)
	})
}

// requireCaseSensitiveSource skips a test whose fixture needs two files that
// differ only by case on the host's temporary filesystem.
func requireCaseSensitiveSource(t *testing.T, repository repo.Repository) {
	t.Helper()
	if repository.PathCase != pathsemantics.CaseSensitive {
		t.Skipf("the host temporary filesystem is case-insensitive (%s); the filesystem-image jobs cover this fixture", RepositoryVolumeRule(repository))
	}
}

// indexEntries writes catalog rows for relative paths under the repository's
// own rule, directories as present and files as pending_hash, the way a walk
// records a tree before hashing. Each file also exists on disk, so the
// reconnect moves real bytes.
func indexEntries(t *testing.T, manager *DefaultRepositoryManager, repository repo.Repository, paths ...string) map[string]repo.RepositoryEntry {
	t.Helper()
	ctx := context.Background()
	rule := RepositoryVolumeRule(repository)
	now := dbtypes.NewTimestamp(time.Now().UTC())
	rows := map[string]repo.RepositoryEntry{}
	insert := func(relative, kind string) {
		if _, done := rows[relative]; done {
			return
		}
		key, err := rule.PathKey(relative)
		if err != nil {
			t.Fatal(err)
		}
		parentKey, err := rule.PathKey(parentPathOf(relative))
		if err != nil {
			t.Fatal(err)
		}
		state := entryStatePresent
		if kind == "file" {
			state = entryStatePendingHash
		}
		row, err := manager.queries.InsertRepositoryEntry(ctx, repo.InsertRepositoryEntryParams{
			EntryID: uuid.New(), RepositoryID: repository.RepoID, Path: relative, PathKey: key, ParentKey: parentKey,
			Kind: kind, Size: 3, MtimeNs: now.Time.UnixNano(), StatCheckedNs: now.Time.UnixNano(), State: state, UpdatedAt: now,
		})
		if err != nil {
			t.Fatalf("index %s: %v", relative, err)
		}
		rows[relative] = row
	}
	for _, relative := range paths {
		parts := strings.Split(relative, "/")
		for depth := 1; depth < len(parts); depth++ {
			insert(strings.Join(parts[:depth], "/"), "directory")
		}
		target := filepath.Join(repository.Path, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(relative), 0o644); err != nil {
			t.Fatal(err)
		}
		insert(relative, "file")
	}
	return rows
}

func listEntries(t *testing.T, manager *DefaultRepositoryManager, repositoryID uuid.UUID) map[string]repo.ListRepositoryEntriesForRekeyRow {
	t.Helper()
	rows := map[string]repo.ListRepositoryEntriesForRekeyRow{}
	after := uuid.Nil
	for {
		page, err := manager.queries.ListRepositoryEntriesForRekey(context.Background(), repo.ListRepositoryEntriesForRekeyParams{
			RepositoryID: repositoryID, AfterEntryID: after, PageSize: 100,
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range page {
			rows[row.Path] = row
		}
		if len(page) < 100 {
			return rows
		}
		after = page[len(page)-1].EntryID
	}
}

// A destination volume that reads two indexed paths as one name refuses
// the reconnect before any row or byte changes, naming the pair.
func TestReconnectRefusesADestinationThatCannotTellIndexedNamesApart(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	diskA := externalLocation(t, manager, base, "disk-a")
	exfat := externalLocation(t, manager, base, "exfat")
	repository := regularUnder(t, manager, diskA, "collide")
	requireCaseSensitiveSource(t, repository)
	before := indexEntries(t, manager, repository, "Trips/Photo.jpg", "Trips/photo.jpg", "Trips/Beach.jpg")
	pretendVolume(manager, exfat.Path, insensitiveNFC)

	newPath := canonicalTestPath(t, moveTree(t, repository, exfat))
	request := LifecycleRequest{RequestID: "reconnect-collide", Actor: "web:user:1"}
	_, err := manager.ReconnectRepository(ctx, repository.RepoID.String(), newPath, request)
	var collision *PathKeyCollisionError
	if !errors.As(err, &collision) {
		t.Fatalf("reconnect onto a colliding volume = %v, want PathKeyCollisionError", err)
	}
	// Entries are read in entry_id order, so which of the pair is named
	// first is not fixed; the pair is.
	if collision.Count != 1 || len(collision.Examples) != 1 {
		t.Fatalf("collision = %+v", collision)
	}
	if pair := collision.Examples[0]; !(pair.First == "Trips/Photo.jpg" && pair.Second == "Trips/photo.jpg") && !(pair.First == "Trips/photo.jpg" && pair.Second == "Trips/Photo.jpg") {
		t.Fatalf("collision pair = %+v", pair)
	}
	if !collision.Destination.Equal(insensitiveNFC) {
		t.Fatalf("collision names destination %s", collision.Destination)
	}
	row, err := manager.GetRepository(repository.RepoID.String())
	if err != nil || row.Path != repository.Path || row.Reachability != dbtypes.RepositoryReachabilityActive || !RepositoryVolumeRule(*row).Equal(RepositoryVolumeRule(repository)) {
		t.Fatalf("refused reconnect changed the row: %#v err=%v", row, err)
	}
	after := listEntries(t, manager, repository.RepoID)
	if len(after) != len(before) {
		t.Fatalf("refused reconnect changed the index: %d rows, want %d", len(after), len(before))
	}
	for relative, original := range before {
		current := after[relative]
		if current.PathKey != original.PathKey || current.ParentKey != original.ParentKey || current.Revision != original.Revision {
			t.Fatalf("refused reconnect rewrote %s: %+v", relative, current)
		}
	}
	for relative := range before {
		if _, statErr := os.Stat(filepath.Join(newPath, filepath.FromSlash(relative))); statErr != nil {
			t.Fatalf("refused reconnect touched the bytes of %s: %v", relative, statErr)
		}
	}
	if _, lookupErr := manager.queries.GetLifecycleOperationByRequestID(ctx, request.RequestID); lookupErr == nil {
		t.Fatal("a refused preflight opened a journal entry")
	}
	// The administrator renames one of the pair on the moved tree; the
	// index still holds both rows, so the refusal stands until the index
	// agrees (a scan of the destination would mark the renamed row missing).
	if err := os.Rename(filepath.Join(newPath, "Trips", "photo.jpg"), filepath.Join(newPath, "Trips", "photo-2.jpg")); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ReconnectRepository(ctx, repository.RepoID.String(), newPath, LifecycleRequest{RequestID: "reconnect-collide-2", Actor: "web:user:1"}); !errors.As(err, &collision) {
		t.Fatalf("reconnect with a renamed byte but an unchanged index = %v, want PathKeyCollisionError", err)
	}
}

// A reconnect onto a volume with another name rule rekeys every entry after
// the cutover: identities, paths, and bindings stay, only the comparison
// keys change, in more than one bounded batch, and the repository leaves
// maintenance with the destination rule recorded.
func TestReconnectRekeysTheIndexForTheDestinationVolume(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	diskA := externalLocation(t, manager, base, "disk-a")
	exfat := externalLocation(t, manager, base, "exfat")
	repository := regularUnder(t, manager, diskA, "rekey")
	source := RepositoryVolumeRule(repository)
	if source.Equal(insensitiveNFC) {
		t.Skipf("the host temporary filesystem already follows %s; the filesystem-image jobs cover this fixture", source)
	}
	paths := []string{"Trips/Café.jpg", "Trips/Beach.JPG"}
	for index := 0; index < rekeyBatchRows+50; index++ {
		paths = append(paths, fmt.Sprintf("Archive/IMG_%04d.JPG", index))
	}
	before := indexEntries(t, manager, repository, paths...)
	pretendVolume(manager, exfat.Path, insensitiveNFC)

	newPath := canonicalTestPath(t, moveTree(t, repository, exfat))
	request := LifecycleRequest{RequestID: "reconnect-rekey", Actor: "web:user:1"}
	reconnected, err := manager.ReconnectRepository(ctx, repository.RepoID.String(), newPath, request)
	if err != nil {
		t.Fatal(err)
	}
	if reconnected.Path != newPath || reconnected.Reachability != dbtypes.RepositoryReachabilityActive || reconnected.Activity != dbtypes.RepositoryActivityIdle || !RepositoryVolumeRule(*reconnected).Equal(insensitiveNFC) {
		t.Fatalf("reconnected row = %#v", reconnected)
	}
	operation, err := manager.queries.GetLifecycleOperationByRequestID(ctx, request.RequestID)
	if err != nil || operation.Status != lifecycleStatusCompleted {
		t.Fatalf("rekeying reconnect journal = %#v err=%v", operation, err)
	}
	after := listEntries(t, manager, repository.RepoID)
	if len(after) != len(before) {
		t.Fatalf("rekey changed the row count: %d, want %d", len(after), len(before))
	}
	for relative, original := range before {
		current, ok := after[relative]
		if !ok || current.EntryID != original.EntryID {
			t.Fatalf("rekey replaced the entry of %s: %+v", relative, current)
		}
		wantKey, _ := insensitiveNFC.PathKey(relative)
		wantParent, _ := insensitiveNFC.PathKey(parentPathOf(relative))
		if current.PathKey != wantKey || current.ParentKey != wantParent {
			t.Fatalf("%s keyed %q/%q, want %q/%q", relative, current.PathKey, current.ParentKey, wantKey, wantParent)
		}
		changed := wantKey != original.PathKey || wantParent != original.ParentKey
		if changed && current.Revision != original.Revision+1 {
			t.Fatalf("%s revision %d after rekey, want %d", relative, current.Revision, original.Revision+1)
		}
		if !changed && current.Revision != original.Revision {
			t.Fatalf("%s was rewritten although its keys did not change", relative)
		}
	}
	// The replay returns the same result without a second rekey.
	replayed, err := manager.ReconnectRepository(ctx, repository.RepoID.String(), newPath, request)
	if err != nil || replayed.Path != newPath {
		t.Fatalf("replay = %#v err=%v", replayed, err)
	}
	if again := listEntries(t, manager, repository.RepoID); again["Trips/Beach.JPG"].Revision != after["Trips/Beach.JPG"].Revision {
		t.Fatal("replay rekeyed the index again")
	}
	// Back onto the sensitive disk: the keys follow the volume again.
	backPath := canonicalTestPath(t, moveTree(t, *reconnected, diskA))
	returned, err := manager.ReconnectRepository(ctx, repository.RepoID.String(), backPath, LifecycleRequest{RequestID: "reconnect-rekey-back", Actor: "web:user:1"})
	if err != nil || !RepositoryVolumeRule(*returned).Equal(source) {
		t.Fatalf("reconnect back = %#v err=%v", returned, err)
	}
	for relative, original := range before {
		current := listEntries(t, manager, repository.RepoID)[relative]
		if current.PathKey != original.PathKey || current.ParentKey != original.ParentKey {
			t.Fatalf("%s keyed %q after returning, want %q", relative, current.PathKey, original.PathKey)
		}
	}
}

// A crash between the cutover and the end of the rekey leaves the repository
// in maintenance with the operation running; startup recovery finishes the
// idempotent pass and releases the repository.
func TestInterruptedRekeyIsFinishedByRecovery(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	diskA := externalLocation(t, manager, base, "disk-a")
	exfat := externalLocation(t, manager, base, "exfat")
	repository := regularUnder(t, manager, diskA, "crash")
	if RepositoryVolumeRule(repository).Equal(insensitiveNFC) {
		t.Skipf("the host temporary filesystem already follows %s; the filesystem-image jobs cover this fixture", insensitiveNFC)
	}
	before := indexEntries(t, manager, repository, "Trips/One.JPG", "Trips/Two.JPG", "Trips/Nested/Three.JPG")
	pretendVolume(manager, exfat.Path, insensitiveNFC)
	newPath := canonicalTestPath(t, moveTree(t, repository, exfat))

	// The cutover transaction, as the command commits it, and nothing after.
	operation, _, err := manager.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: "reconnect-crash-mid-rekey", Kind: lifecycleKindReconnectRepository,
		Payload: reconnectRepositoryOperationPayload{
			RepositoryID: repository.RepoID.String(), OldPath: repository.Path, NewPath: newPath,
			StorageLocationID: exfat.StorageLocationID.String(), ConfirmationType: "update_location",
			Rekey: true, PathCase: string(insensitiveNFC.Case), PathNormalization: string(insensitiveNFC.Normalization),
		},
		Actor: "web:user:1", TargetType: "repository", TargetID: ptrString(repository.RepoID.String()),
	})
	if err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := manager.queries.BeginRepositoryMaintenance(ctx, repo.BeginRepositoryMaintenanceParams{RepoID: repository.RepoID, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.queries.UpdateRepositoryPathSemantics(ctx, repo.UpdateRepositoryPathSemanticsParams{
		RepoID: repository.RepoID, Path: newPath, StorageLocationID: exfat.StorageLocationID,
		Reachability: dbtypes.RepositoryReachabilityMaintenance,
		PathCase:     insensitiveNFC.Case, PathNormalization: insensitiveNFC.Normalization, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := manager.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseCatalogCommitted, nil); err != nil {
		t.Fatal(err)
	}
	// Half the work was done before the crash.
	if _, err := manager.queries.RekeyRepositoryEntryCAS(ctx, repo.RekeyRepositoryEntryCASParams{
		PathKey: "trips", ParentKey: "", UpdatedAt: now, EntryID: before["Trips"].EntryID, ExpectedRevision: before["Trips"].Revision,
	}); err != nil {
		t.Fatal(err)
	}

	// Until recovery runs, the repository is in maintenance: no scan, upload,
	// or Trash command can key against a half-rebuilt index.
	if row, err := manager.GetRepository(repository.RepoID.String()); err != nil || row.Reachability != dbtypes.RepositoryReachabilityMaintenance {
		t.Fatalf("interrupted rekey left reachability %s err=%v", row.Reachability, err)
	}
	if err := manager.RecoverLifecycleOperations(ctx); err != nil {
		t.Fatal(err)
	}
	settled, err := manager.queries.GetLifecycleOperation(ctx, operation.OperationID)
	if err != nil || settled.Status != lifecycleStatusCompleted {
		t.Fatalf("interrupted rekey after recovery = %#v err=%v", settled, err)
	}
	row, err := manager.GetRepository(repository.RepoID.String())
	if err != nil || row.Reachability != dbtypes.RepositoryReachabilityActive || row.Activity != dbtypes.RepositoryActivityIdle || row.Path != newPath {
		t.Fatalf("recovered row = %#v err=%v", row, err)
	}
	for relative, original := range before {
		current := listEntries(t, manager, repository.RepoID)[relative]
		wantKey, _ := insensitiveNFC.PathKey(relative)
		wantParent, _ := insensitiveNFC.PathKey(parentPathOf(relative))
		if current.EntryID != original.EntryID || current.PathKey != wantKey || current.ParentKey != wantParent {
			t.Fatalf("%s after recovery = %+v, want keys %q/%q", relative, current, wantKey, wantParent)
		}
	}
	// A second recovery pass has nothing to do.
	if err := manager.RecoverLifecycleOperations(ctx); err != nil {
		t.Fatal(err)
	}
}

// A destination whose name rule cannot be qualified, or whose directories
// disagree, never receives a reconnect.
func TestReconnectRefusesAnUnqualifiedDestination(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	diskA := externalLocation(t, manager, base, "disk-a")
	mixed := externalLocation(t, manager, base, "mixed")
	repository := regularUnder(t, manager, diskA, "unqualified")
	indexEntries(t, manager, repository, "Trips/One.JPG")
	manager.SetVolumeQualifier(func(root string) (pathsemantics.Semantics, error) {
		if strings.HasPrefix(root, mixed.Path) {
			return pathsemantics.Semantics{}, fmt.Errorf("%w: %s", pathsemantics.ErrMixed, root)
		}
		return pathsemantics.Qualify(root)
	})
	newPath := canonicalTestPath(t, moveTree(t, repository, mixed))
	_, err := manager.ReconnectRepository(ctx, repository.RepoID.String(), newPath, LifecycleRequest{RequestID: "reconnect-mixed", Actor: "web:user:1"})
	if !errors.Is(err, pathsemantics.ErrUnqualified) || !errors.Is(err, pathsemantics.ErrMixed) {
		t.Fatalf("reconnect onto a mixed tree = %v, want ErrMixed", err)
	}
	row, err := manager.GetRepository(repository.RepoID.String())
	if err != nil || row.Path != repository.Path {
		t.Fatalf("refused reconnect changed the row: %#v err=%v", row, err)
	}
	// Registration is held to the same rule: a tree that cannot be qualified
	// is not opened.
	stranger := filepath.Join(mixed.Path, "stranger")
	if err := os.MkdirAll(stranger, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.OpenRepository(ctx, stranger, nil, dbtypes.RepoRoleRegular, LifecycleRequest{RequestID: "open-mixed", Actor: "web:user:1"}); err == nil {
		t.Fatal("an unmarked tree on an unqualified volume was opened")
	}
}

// A Storage Location reconnect qualifies every child's destination: one
// collision refuses the whole Location with nothing changed, and a clean
// tree rekeys every child that needs it before any leaves maintenance.
func TestStorageLocationReconnectRekeysEveryChildOnTheNewVolume(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	disk := externalLocation(t, manager, base, "disk")
	alpha := regularUnder(t, manager, disk, "alpha")
	beta := regularUnder(t, manager, disk, "beta")
	requireCaseSensitiveSource(t, alpha)
	alphaBefore := indexEntries(t, manager, alpha, "Trips/One.JPG", "Trips/Two.JPG")
	betaBefore := indexEntries(t, manager, beta, "Pics/Same.jpg", "Pics/same.jpg")
	movedPath := filepath.Join(base, "disk-on-exfat")
	pretendVolume(manager, movedPath, insensitiveNFC)
	if err := os.Rename(disk.Path, movedPath); err != nil {
		t.Fatal(err)
	}
	canonicalMoved := canonicalTestPath(t, movedPath)

	var collision *PathKeyCollisionError
	_, err := manager.ReconnectStorageLocation(ctx, disk.StorageLocationID.String(), movedPath, LifecycleRequest{RequestID: "location-collide", Actor: "web:user:1"})
	if !errors.As(err, &collision) || collision.RepositoryID != beta.RepoID.String() {
		t.Fatalf("Location reconnect with a colliding child = %v, want beta's PathKeyCollisionError", err)
	}
	for _, repository := range []repo.Repository{alpha, beta} {
		row, err := manager.GetRepository(repository.RepoID.String())
		if err != nil || row.Path != repository.Path || row.Reachability != dbtypes.RepositoryReachabilityActive {
			t.Fatalf("refused Location reconnect changed %s: %#v err=%v", repository.Name, row, err)
		}
	}
	if location, err := manager.queries.GetStorageLocation(ctx, disk.StorageLocationID); err != nil || location.Path != disk.Path || location.Status != dbtypes.StorageLocationStatusActive {
		t.Fatalf("refused Location reconnect changed the Location: %#v err=%v", location, err)
	}
	if entries := listEntries(t, manager, beta.RepoID); entries["Pics/same.jpg"].PathKey != betaBefore["Pics/same.jpg"].PathKey {
		t.Fatal("refused Location reconnect rewrote beta's keys")
	}

	// With beta's pair resolved in the index, the whole tree reconnects and
	// both children are rekeyed for the new volume.
	if _, err := manager.readerDatabase.ExecContext(ctx, `DELETE FROM repository_entries WHERE entry_id = ?`, betaBefore["Pics/same.jpg"].EntryID.String()); err != nil {
		t.Fatal(err)
	}
	reconnected, err := manager.ReconnectStorageLocation(ctx, disk.StorageLocationID.String(), movedPath, LifecycleRequest{RequestID: "location-rekey", Actor: "web:user:1"})
	if err != nil || reconnected.Path != canonicalMoved || reconnected.Status != dbtypes.StorageLocationStatusActive {
		t.Fatalf("Location reconnect = %#v err=%v", reconnected, err)
	}
	for _, item := range []struct {
		repository repo.Repository
		before     map[string]repo.RepositoryEntry
	}{{alpha, alphaBefore}, {beta, betaBefore}} {
		row, err := manager.GetRepository(item.repository.RepoID.String())
		if err != nil || row.Reachability != dbtypes.RepositoryReachabilityActive || !RepositoryVolumeRule(*row).Equal(insensitiveNFC) ||
			row.Path != filepath.Join(canonicalMoved, filepath.Base(item.repository.Path)) {
			t.Fatalf("%s after Location rekey = %#v err=%v", item.repository.Name, row, err)
		}
		after := listEntries(t, manager, item.repository.RepoID)
		for relative, original := range item.before {
			current, ok := after[relative]
			if relative == "Pics/same.jpg" {
				continue
			}
			wantKey, _ := insensitiveNFC.PathKey(relative)
			if !ok || current.EntryID != original.EntryID || current.PathKey != wantKey {
				t.Fatalf("%s/%s after Location rekey = %+v, want key %q", item.repository.Name, relative, current, wantKey)
			}
		}
	}
	operation, err := manager.queries.GetLifecycleOperationByRequestID(ctx, "location-rekey")
	if err != nil || operation.Status != lifecycleStatusCompleted {
		t.Fatalf("Location rekey journal = %#v err=%v", operation, err)
	}
}
