package storage

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"

	"github.com/google/uuid"
)

// denyingObserver reads the host filesystem except below the denied prefixes,
// where every read and stat fails with fs.ErrPermission: the registered path
// of an original whose disk is mounted but not readable by the Server.
type denyingObserver struct {
	OSStorageObserver
	denied []string
}

func (o *denyingObserver) blocks(p string) bool {
	clean := filepath.Clean(p)
	for _, prefix := range o.denied {
		if clean == prefix || strings.HasPrefix(clean, prefix+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (o *denyingObserver) ReadFile(p string) ([]byte, error) {
	if o.blocks(p) {
		return nil, &fs.PathError{Op: "open", Path: p, Err: fs.ErrPermission}
	}
	return o.OSStorageObserver.ReadFile(p)
}

func (o *denyingObserver) Stat(p string) (fs.FileInfo, error) {
	if o.blocks(p) {
		return nil, &fs.PathError{Op: "stat", Path: p, Err: fs.ErrPermission}
	}
	return o.OSStorageObserver.Stat(p)
}

// externalLocation registers a plain external Storage Location under base.
func externalLocation(t *testing.T, manager *DefaultRepositoryManager, base, name string) *repo.StorageLocation {
	t.Helper()
	path := filepath.Join(base, name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	storageLocation, err := manager.AddStorageLocation(context.Background(), path, name)
	if err != nil {
		t.Fatal(err)
	}
	return storageLocation
}

// regularUnder creates an acknowledged regular Repository under a Location.
func regularUnder(t *testing.T, manager *DefaultRepositoryManager, storageLocation *repo.StorageLocation, directory string) repo.Repository {
	t.Helper()
	created, err := createAcknowledgedForTest(t, manager, CreateRepositorySpec{
		RequestID: "reconnect-" + directory + "-" + uuid.NewString(), Actor: "test", Name: directory,
		DirectoryName: directory, Role: dbtypes.RepoRoleRegular, StorageLocationID: storageLocation.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return *created.Repository
}

// moveTree moves a Repository tree to another Location the way an operator
// does with the Server stopped or the disk unplugged: the bytes move, the
// catalog does not know.
func moveTree(t *testing.T, repository repo.Repository, storageLocation *repo.StorageLocation) string {
	t.Helper()
	newPath := filepath.Join(storageLocation.Path, filepath.Base(repository.Path))
	if err := os.Rename(repository.Path, newPath); err != nil {
		t.Fatal(err)
	}
	return newPath
}

// A registered path that cannot be read proves nothing about the original.
// The command, the native task, and the policy all withhold the reconnect
// with original_inaccessible instead of treating the denial as absence.
func TestReconnectRefusesAnInaccessibleOriginal(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	diskA := externalLocation(t, manager, base, "disk-a")
	diskB := externalLocation(t, manager, base, "disk-b")
	repository := regularUnder(t, manager, diskA, "travel")
	// A same-identity copy sits on disk B; only the unreadable original
	// stands between it and a reconnect.
	copyPath := filepath.Join(diskB.Path, "travel")
	if err := os.CopyFS(copyPath, os.DirFS(repository.Path)); err != nil {
		t.Fatal(err)
	}
	manager.SetObservationBackend(&denyingObserver{denied: []string{repository.Path}})
	t.Cleanup(func() { manager.SetObservationBackend(OSStorageObserver{}) })

	for _, projected := range []dbtypes.RepositoryReachability{dbtypes.RepositoryReachabilityActive, dbtypes.RepositoryReachabilityOffline} {
		now := dbtypes.NewTimestamp(time.Now().UTC())
		if _, err := manager.queries.UpdateRepositoryReachability(ctx, repo.UpdateRepositoryReachabilityParams{RepoID: repository.RepoID, Reachability: projected, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		_, err := manager.ReconnectRepository(ctx, repository.RepoID.String(), copyPath, LifecycleRequest{RequestID: "reconnect-denied-" + string(projected), Actor: "test"})
		var unavailable *RepositoryActionUnavailableError
		if !errors.As(err, &unavailable) || unavailable.Action != ActionReconnect ||
			len(unavailable.Blockers) != 1 || unavailable.Blockers[0] != BlockerOriginalInaccessible ||
			len(unavailable.Alternatives) != 1 || unavailable.Alternatives[0] != AlternativeCheckAccess {
			t.Fatalf("projected %s, denied original: reconnect error = %v, want original_inaccessible/check_access", projected, err)
		}
		still, err := manager.GetRepository(repository.RepoID.String())
		if err != nil || still.Path != repository.Path || still.StorageLocationID != diskA.StorageLocationID {
			t.Fatalf("refused reconnect changed the row: %#v err=%v", still, err)
		}
	}

	// The native task carries the same verdict.
	action, err := manager.CreateHostAction(ctx, CreateHostActionInput{
		RequestID: "host-locate-denied", Kind: HostActionLocateRepository,
		Summary: HostActionSummary{RepositoryID: repository.RepoID.String()}, TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	refused, err := manager.ExecuteHostAction(ctx, action.ActionID, action.NativeHostNonce(), "desktop-denied", copyPath, nil)
	var unavailable *RepositoryActionUnavailableError
	if !errors.As(err, &unavailable) || refused.Status != HostActionFailed || refused.ErrorCode != hostActionCodeActionUnavailable ||
		refused.Result == nil || refused.Result.Refusal == nil ||
		!containsString(refused.Result.Refusal.Blockers, string(BlockerOriginalInaccessible)) ||
		!containsString(refused.Result.Refusal.Alternatives, string(AlternativeCheckAccess)) {
		t.Fatalf("native reconnect with a denied original = %#v err=%v", refused.Result, err)
	}

	// Once the registered path reads again as positively absent, the same
	// destination is reconnected: denial delayed the decision, it did not
	// pre-empt it.
	manager.SetObservationBackend(OSStorageObserver{})
	if err := os.RemoveAll(repository.Path); err != nil {
		t.Fatal(err)
	}
	reconnected, err := manager.ReconnectRepository(ctx, repository.RepoID.String(), copyPath, LifecycleRequest{RequestID: "reconnect-after-access", Actor: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if reconnected.RepoID != repository.RepoID || reconnected.StorageLocationID != diskB.StorageLocationID || reconnected.Path != canonicalTestPath(t, copyPath) {
		t.Fatalf("reconnected = %#v", reconnected)
	}
}

// A reconnect is a journaled operation: the same request replays the same
// result without a second mutation, a different target under that request
// is a conflict, and startup recovery settles an interrupted one from the
// catalog alone, since the command moves no file.
func TestReconnectRepositoryIsJournaledReplayedAndRecovered(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	diskA := externalLocation(t, manager, base, "disk-a")
	diskB := externalLocation(t, manager, base, "disk-b")
	repository := regularUnder(t, manager, diskA, "journal")
	newPath := canonicalTestPath(t, moveTree(t, repository, diskB))

	request := LifecycleRequest{RequestID: "reconnect-journal-1", Actor: "web:user:1", ConfirmationType: "update_location"}
	first, err := manager.ReconnectRepository(ctx, repository.RepoID.String(), newPath, request)
	if err != nil {
		t.Fatal(err)
	}
	operation, err := manager.queries.GetLifecycleOperationByRequestID(ctx, request.RequestID)
	if err != nil || operation.Kind != lifecycleKindReconnectRepository || operation.Status != lifecycleStatusCompleted ||
		operation.TargetID == nil || *operation.TargetID != repository.RepoID.String() {
		t.Fatalf("reconnect journal = %#v err=%v", operation, err)
	}
	replayed, err := manager.ReconnectRepository(ctx, repository.RepoID.String(), newPath, request)
	if err != nil || replayed.Path != first.Path || replayed.RepoID != first.RepoID {
		t.Fatalf("replayed reconnect = %#v err=%v", replayed, err)
	}
	var count int
	if err := manager.readerDatabase.QueryRowContext(ctx, `SELECT count(*) FROM lifecycle_operations WHERE kind = ?`, lifecycleKindReconnectRepository).Scan(&count); err != nil || count != 1 {
		t.Fatalf("reconnect operations after replay = %d err=%v", count, err)
	}
	if _, err := manager.ReconnectRepository(ctx, repository.RepoID.String(), filepath.Join(diskA.Path, "journal"), request); !errors.Is(err, ErrLifecycleRequestConflict) {
		t.Fatalf("same request, different destination = %v, want ErrLifecycleRequestConflict", err)
	}
	// Audit: the command's row and the journal's row both name the request
	// with the administrator's confirmation.
	events, err := manager.ListLifecycleAudit(ctx, LifecycleAuditFilter{TargetType: "repository", TargetID: repository.RepoID.String(), Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	audited := 0
	for _, event := range events {
		if event.RequestID == request.RequestID && event.Result == AuditResultSucceeded && event.ConfirmationType == "update_location" {
			audited++
		}
	}
	if audited == 0 {
		t.Fatalf("no succeeded audit row with the confirmation for the reconnect: %#v", events)
	}

	// Interrupted before the catalog commit: the row is still at the old
	// path, nothing is rolled back, and the journal is closed as recovered.
	again := moveTree(t, *first, diskA)
	againCanonical := canonicalTestPath(t, again)
	interrupted, _, err := manager.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: "reconnect-crash-before", Kind: lifecycleKindReconnectRepository,
		Payload: reconnectRepositoryOperationPayload{
			RepositoryID: repository.RepoID.String(), OldPath: first.Path, NewPath: againCanonical,
			StorageLocationID: diskA.StorageLocationID.String(), ConfirmationType: "update_location",
		},
		Actor: "web:user:1", TargetType: "repository", TargetID: ptrString(repository.RepoID.String()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.RecoverLifecycleOperations(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := manager.queries.GetLifecycleOperation(ctx, interrupted.OperationID)
	if err != nil || recovered.Status == lifecycleStatusRunning || recovered.Status == lifecycleStatusCompleted {
		t.Fatalf("interrupted reconnect after recovery = %#v err=%v", recovered, err)
	}
	row, err := manager.GetRepository(repository.RepoID.String())
	if err != nil || row.Path != first.Path {
		t.Fatalf("recovery moved the row of an uncommitted reconnect: %#v err=%v", row, err)
	}
	// The administrator retries and succeeds; the retry is its own request.
	retried, err := manager.ReconnectRepository(ctx, repository.RepoID.String(), againCanonical, LifecycleRequest{RequestID: "reconnect-retry", Actor: "web:user:1"})
	if err != nil || retried.Path != againCanonical {
		t.Fatalf("retry after recovery = %#v err=%v", retried, err)
	}

	// Interrupted after the catalog commit: the row is already at the new
	// path, so recovery completes the operation.
	finalPath := canonicalTestPath(t, moveTree(t, *retried, diskB))
	committed, _, err := manager.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: "reconnect-crash-after", Kind: lifecycleKindReconnectRepository,
		Payload: reconnectRepositoryOperationPayload{
			RepositoryID: repository.RepoID.String(), OldPath: retried.Path, NewPath: finalPath,
			StorageLocationID: diskB.StorageLocationID.String(), ConfirmationType: "update_location",
		},
		Actor: "web:user:1", TargetType: "repository", TargetID: ptrString(repository.RepoID.String()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.queries.UpdateRepositoryPath(ctx, repo.UpdateRepositoryPathParams{
		RepoID: repository.RepoID, Path: finalPath, StorageLocationID: diskB.StorageLocationID,
		Reachability: dbtypes.RepositoryReachabilityActive, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	}); err != nil {
		t.Fatal(err)
	}
	if err := manager.RecoverLifecycleOperations(ctx); err != nil {
		t.Fatal(err)
	}
	settled, err := manager.queries.GetLifecycleOperation(ctx, committed.OperationID)
	if err != nil || settled.Status != lifecycleStatusCompleted {
		t.Fatalf("committed reconnect after recovery = %#v err=%v", settled, err)
	}
}

// Reconcile decides reachability only from positive evidence: a path it
// cannot read keeps the row as it is, an absent one is offline.
func TestReconcileKeepsReachabilityWhenTheRegisteredPathCannotBeRead(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	diskA := externalLocation(t, manager, base, "disk-a")
	repository := regularUnder(t, manager, diskA, "reconcile")

	manager.SetObservationBackend(&denyingObserver{denied: []string{repository.Path}})
	if err := manager.ReconcileAll(ctx); err != nil {
		t.Fatal(err)
	}
	row, err := manager.GetRepository(repository.RepoID.String())
	if err != nil || row.Reachability != dbtypes.RepositoryReachabilityActive {
		t.Fatalf("denied path changed reachability: %#v err=%v", row, err)
	}
	policy, err := manager.RepositoryActionPolicyFor(ctx, *row)
	if err != nil || policy.Verdict(ActionReconnect).Allowed() {
		t.Fatalf("a row whose path is denied offers reconnect: %+v err=%v", policy.Verdict(ActionReconnect), err)
	}

	manager.SetObservationBackend(OSStorageObserver{})
	if err := os.RemoveAll(repository.Path); err != nil {
		t.Fatal(err)
	}
	if err := manager.ReconcileAll(ctx); err != nil {
		t.Fatal(err)
	}
	row, err = manager.GetRepository(repository.RepoID.String())
	if err != nil || row.Reachability != dbtypes.RepositoryReachabilityOffline {
		t.Fatalf("absent path reachability = %#v err=%v", row, err)
	}
	// An offline row whose path becomes unreadable stays offline, never
	// flipping to identity_error or active on a failed read.
	manager.SetObservationBackend(&denyingObserver{denied: []string{repository.Path}})
	t.Cleanup(func() { manager.SetObservationBackend(OSStorageObserver{}) })
	if err := manager.ReconcileAll(ctx); err != nil {
		t.Fatal(err)
	}
	row, err = manager.GetRepository(repository.RepoID.String())
	if err != nil || row.Reachability != dbtypes.RepositoryReachabilityOffline {
		t.Fatalf("denied offline row reachability = %#v err=%v", row, err)
	}
}

// The Storage Location policy is kind-aware and pure: the Default Storage
// Location is protected, an external one reconnects only from a positively
// unavailable original and detaches only while empty.
func TestDeriveStorageLocationActionsIsKindAware(t *testing.T) {
	cases := []struct {
		name      string
		facts     StorageLocationActionFacts
		reconnect []RepositoryActionBlocker
		detach    []RepositoryActionBlocker
	}{
		{"default location", StorageLocationActionFacts{Kind: dbtypes.StorageLocationKindDefault, Status: dbtypes.StorageLocationStatusActive, Original: OriginalAbsent},
			[]RepositoryActionBlocker{BlockerDefaultProtected}, []RepositoryActionBlocker{BlockerDefaultProtected}},
		{"external online with children", StorageLocationActionFacts{Kind: dbtypes.StorageLocationKindExternal, Status: dbtypes.StorageLocationStatusActive, RepositoryCount: 2, Original: OriginalOnline},
			[]RepositoryActionBlocker{BlockerOriginalAvailable}, []RepositoryActionBlocker{BlockerRegisteredRepositories}},
		{"external absent with children", StorageLocationActionFacts{Kind: dbtypes.StorageLocationKindExternal, Status: dbtypes.StorageLocationStatusActive, RepositoryCount: 2, Original: OriginalAbsent},
			nil, []RepositoryActionBlocker{BlockerRegisteredRepositories}},
		{"external identity differs, empty", StorageLocationActionFacts{Kind: dbtypes.StorageLocationKindExternal, Status: dbtypes.StorageLocationStatusActive, Original: OriginalIdentityDiffers},
			nil, nil},
		{"external inaccessible", StorageLocationActionFacts{Kind: dbtypes.StorageLocationKindExternal, Status: dbtypes.StorageLocationStatusActive, Original: OriginalInaccessible},
			[]RepositoryActionBlocker{BlockerOriginalInaccessible}, nil},
		{"external in maintenance", StorageLocationActionFacts{Kind: dbtypes.StorageLocationKindExternal, Status: dbtypes.StorageLocationStatusMaintenance, Original: OriginalAbsent},
			[]RepositoryActionBlocker{BlockerMaintenance}, []RepositoryActionBlocker{BlockerMaintenance}},
		{"external with a running operation", StorageLocationActionFacts{Kind: dbtypes.StorageLocationKindExternal, Status: dbtypes.StorageLocationStatusActive, PendingOperation: true, Original: OriginalAbsent},
			[]RepositoryActionBlocker{BlockerPendingOperation}, []RepositoryActionBlocker{BlockerPendingOperation}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy := DeriveStorageLocationActions(tc.facts)
			if got := policy.Verdict(LocationActionReconnect).Blockers; strings.Join(BlockerCodes(got), ",") != strings.Join(BlockerCodes(tc.reconnect), ",") {
				t.Fatalf("reconnect blockers = %v, want %v", got, tc.reconnect)
			}
			if got := policy.Verdict(LocationActionDetach).Blockers; strings.Join(BlockerCodes(got), ",") != strings.Join(BlockerCodes(tc.detach), ",") {
				t.Fatalf("detach blockers = %v, want %v", got, tc.detach)
			}
			for _, verdict := range policy.Verdicts {
				if len(verdict.Blockers) == 0 && len(verdict.Alternatives) != 0 {
					t.Fatalf("allowed %s carries alternatives %v", verdict.Action, verdict.Alternatives)
				}
			}
		})
	}
}

// The Location reconnect command and the native task refuse exactly what the
// policy withholds, and admit a positively unavailable original so that only
// the destination can refuse it.
func TestStorageLocationReconnectAgreesWithThePolicy(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	defaultLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(base, "elsewhere")
	if err := os.Mkdir(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}

	// The Default Storage Location is protected by both entries.
	_, err = manager.ReconnectStorageLocation(ctx, defaultLocation.StorageLocationID.String(), elsewhere, LifecycleRequest{RequestID: "reconnect-default", Actor: "test"})
	var unavailable *StorageLocationActionUnavailableError
	if !errors.As(err, &unavailable) || unavailable.Action != LocationActionReconnect ||
		!containsString(BlockerCodes(unavailable.Blockers), string(BlockerDefaultProtected)) ||
		!containsString(AlternativeCodes(unavailable.Alternatives), string(AlternativeDefaultConfiguration)) ||
		!errors.Is(err, ErrStorageLocationNotRemovable) {
		t.Fatalf("default Location reconnect = %v", err)
	}
	action, err := manager.CreateHostAction(ctx, CreateHostActionInput{
		RequestID: "host-locate-default-root", Kind: HostActionLocateStorageLocation,
		Summary: HostActionSummary{StorageLocationID: defaultLocation.StorageLocationID.String()}, TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	refused, err := manager.ExecuteHostAction(ctx, action.ActionID, action.NativeHostNonce(), "desktop-policy", elsewhere, nil)
	if !errors.As(err, &unavailable) || refused.Status != HostActionFailed || refused.ErrorCode != hostActionCodeActionUnavailable ||
		refused.Result == nil || refused.Result.Refusal == nil || refused.Result.Refusal.Action != string(LocationActionReconnect) ||
		!containsString(refused.Result.Refusal.Blockers, string(BlockerDefaultProtected)) {
		t.Fatalf("native default Location reconnect = %#v err=%v", refused.Result, err)
	}

	// An external Location whose registered path cannot be read admits
	// nothing; once it is positively absent, only the destination decides.
	external := externalLocation(t, manager, base, "external")
	destination := filepath.Join(base, "external-moved")
	if err := os.CopyFS(destination, os.DirFS(external.Path)); err != nil {
		t.Fatal(err)
	}
	manager.SetObservationBackend(&denyingObserver{denied: []string{external.Path}})
	t.Cleanup(func() { manager.SetObservationBackend(OSStorageObserver{}) })
	_, err = manager.ReconnectStorageLocation(ctx, external.StorageLocationID.String(), destination, LifecycleRequest{RequestID: "reconnect-denied-root", Actor: "test"})
	if !errors.As(err, &unavailable) || len(unavailable.Blockers) != 1 || unavailable.Blockers[0] != BlockerOriginalInaccessible ||
		len(unavailable.Alternatives) != 1 || unavailable.Alternatives[0] != AlternativeCheckAccess {
		t.Fatalf("denied external Location reconnect = %v", err)
	}
	policy, err := manager.StorageLocationActionPolicyFor(ctx, *external)
	if err != nil || policy.Verdict(LocationActionReconnect).Allowed() {
		t.Fatalf("policy for a denied Location = %+v err=%v", policy, err)
	}
	manager.SetObservationBackend(OSStorageObserver{})
	if err := os.RemoveAll(external.Path); err != nil {
		t.Fatal(err)
	}
	policy, err = manager.StorageLocationActionPolicyFor(ctx, *external)
	if err != nil || !policy.Verdict(LocationActionReconnect).Allowed() {
		t.Fatalf("policy for an absent Location = %+v err=%v", policy, err)
	}
	// A destination carrying another identity is refused by the destination
	// check, never by the policy: admission has passed.
	other := externalLocation(t, manager, base, "other-identity")
	_, err = manager.ReconnectStorageLocation(ctx, external.StorageLocationID.String(), other.Path, LifecycleRequest{RequestID: "reconnect-wrong-root", Actor: "test"})
	if !errors.Is(err, ErrStorageLocationInvalid) || errors.As(err, &unavailable) {
		t.Fatalf("wrong-identity destination = %v, want ErrStorageLocationInvalid", err)
	}
	reconnected, err := manager.ReconnectStorageLocation(ctx, external.StorageLocationID.String(), destination, LifecycleRequest{RequestID: "reconnect-root-ok", Actor: "test"})
	if err != nil || reconnected.Path != canonicalTestPath(t, destination) {
		t.Fatalf("absent original reconnect = %#v err=%v", reconnected, err)
	}
	journal, err := manager.queries.GetLifecycleOperationByRequestID(ctx, "reconnect-root-ok")
	if err != nil || journal.Kind != lifecycleKindReconnectStorage || journal.Status != lifecycleStatusCompleted {
		t.Fatalf("Location reconnect journal = %#v err=%v", journal, err)
	}
}

// A tree moved while this process holds its runtime lock carries the lock
// file with it. The reconnect recognizes its own lock at the new path rather
// than refusing ownership to itself, for a Repository and for a whole
// Storage Location with its children; a second process still cannot claim
// the reconnected tree.
func TestReconnectFollowsATreeMovedWhileItsRuntimeLockIsHeld(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	releaseOwnership, err := manager.AcquireRuntimeStorageOwnership(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseOwnership()
	diskA := externalLocation(t, manager, base, "disk-a")
	diskB := externalLocation(t, manager, base, "disk-b")
	repository := regularUnder(t, manager, diskA, "held")
	if _, err := os.Stat(filepath.Join(repository.Path, ".lumiliorepo.lock")); err != nil {
		t.Fatalf("the created Repository holds no runtime lock file: %v", err)
	}
	newPath := moveTree(t, repository, diskB)
	reconnected, err := manager.ReconnectRepository(ctx, repository.RepoID.String(), newPath, LifecycleRequest{RequestID: "reconnect-held", Actor: "test"})
	if err != nil {
		t.Fatalf("reconnect of a tree moved while held = %v", err)
	}
	manager.ownershipMu.Lock()
	_, oldHeld := manager.ownership["repository:"+repository.Path]
	_, newHeld := manager.ownership["repository:"+reconnected.Path]
	manager.ownershipMu.Unlock()
	if oldHeld || !newHeld {
		t.Fatalf("ownership keys after reconnect: old=%t new=%t", oldHeld, newHeld)
	}
	// Another process cannot take the reconnected tree.
	if _, err := (OSRepositoryLockProvider{}).Acquire(cancelledContext(), filepath.Join(reconnected.Path, ".lumiliorepo.lock"), true); !errors.Is(err, ErrRepositoryLockUnavailable) {
		t.Fatalf("second holder on the reconnected tree = %v, want ErrRepositoryLockUnavailable", err)
	}

	// The same for a whole Location: root and child locks move together.
	diskC := externalLocation(t, manager, base, "disk-c")
	child := regularUnder(t, manager, diskC, "child")
	movedRoot := filepath.Join(base, "disk-c-moved")
	if err := os.Rename(diskC.Path, movedRoot); err != nil {
		t.Fatal(err)
	}
	movedLocation, err := manager.ReconnectStorageLocation(ctx, diskC.StorageLocationID.String(), movedRoot, LifecycleRequest{RequestID: "reconnect-held-root", Actor: "test"})
	if err != nil {
		t.Fatalf("reconnect of a Location moved while held = %v", err)
	}
	movedChild, err := manager.GetRepository(child.RepoID.String())
	if err != nil || movedChild.Path != filepath.Join(movedLocation.Path, "child") {
		t.Fatalf("child after Location reconnect = %#v err=%v", movedChild, err)
	}
	manager.ownershipMu.Lock()
	_, childHeld := manager.ownership["repository:"+movedChild.Path]
	_, staleChild := manager.ownership["repository:"+child.Path]
	manager.ownershipMu.Unlock()
	if !childHeld || staleChild {
		t.Fatalf("child ownership after Location reconnect: new=%t stale=%t", childHeld, staleChild)
	}
}

func cancelledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}
