package storage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"
)

func TestHostActionIsDurableIdempotentAndNativeApproved(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	actor, err := catalog.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "native-approval-admin", Password: "test", DisplayName: "Admin", Role: "admin",
		WebauthnUserHandle: []byte("native-approval-admin-handle"),
	})
	if err != nil {
		t.Fatal(err)
	}

	input := CreateHostActionInput{
		RequestID:   "host-action-add-storageLocation-1",
		Kind:        HostActionAuthorizeStorageLocation,
		Actor:       "web:user:native-approval-admin",
		ActorUserID: &actor.UserID,
		SessionID:   "session-1",
		Summary: HostActionSummary{
			Name:    "External Archive",
			Purpose: "Store an archive on an external disk",
		},
		TTL: 10 * time.Minute,
	}
	first, err := manager.CreateHostAction(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.CreateHostAction(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.ActionID != second.ActionID {
		t.Fatalf("idempotent action ID = %q, want %q", second.ActionID, first.ActionID)
	}
	if first.NativeHostNonce() == "" {
		t.Fatal("native host nonce is empty")
	}

	changed := input
	changed.Summary.Name = "Different Archive"
	if _, err := manager.CreateHostAction(ctx, changed); !errors.Is(err, ErrHostActionConflict) {
		t.Fatalf("changed idempotent request error = %v, want ErrHostActionConflict", err)
	}

	pending, err := manager.ListPendingHostActions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].ActionID != first.ActionID {
		t.Fatalf("pending actions = %#v", pending)
	}
	bound, err := manager.SetHostActionExpectedVersion(ctx, first.ActionID, first.NativeHostNonce(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if bound.ExpectedVersion != 42 {
		t.Fatalf("bound expected version = %d, want 42", bound.ExpectedVersion)
	}
	// Presenting the request again after the storage snapshot moved rebinds
	// it; a binding never leaves a request permanently un-approvable.
	boundAgain, err := manager.SetHostActionExpectedVersion(ctx, first.ActionID, first.NativeHostNonce(), 99)
	if err != nil || boundAgain.ExpectedVersion != 99 {
		t.Fatalf("expected version rebind on presentation: action=%#v err=%v", boundAgain, err)
	}
	if _, err := manager.SetHostActionExpectedVersion(ctx, first.ActionID, "wrong-nonce", 7); !errors.Is(err, ErrHostActionNonceInvalid) {
		t.Fatalf("rebind without the nonce error = %v", err)
	}
	if _, err := manager.ExecuteHostAction(ctx, first.ActionID, "wrong-nonce", "desktop-1", filepath.Join(base, "external"), nil); !errors.Is(err, ErrHostActionNonceInvalid) {
		t.Fatalf("wrong nonce error = %v, want ErrHostActionNonceInvalid", err)
	}

	external := filepath.Join(base, "external")
	if err := os.Mkdir(external, 0o755); err != nil {
		t.Fatal(err)
	}
	manager.SetObservationBackend(&injectedObserver{target: canonicalTestPath(t, external)})
	// A plain internal disk outside the Default Storage Location is still a
	// reviewed destination: the non-default risk is acknowledged by code.
	review, err := manager.ExecuteHostAction(ctx, first.ActionID, first.NativeHostNonce(), "desktop-1", external, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertStorageRiskDecision(t, review)
	if got := review.Result.Conflict.RiskWarnings; !reflect.DeepEqual(got, []string{"non_default_location"}) {
		t.Fatalf("authorize risk codes = %v", got)
	}
	action, err := manager.ResolveHostAction(ctx, first.ActionID, "confirm_risk", review.Result.Conflict.RiskWarnings)
	if err != nil {
		t.Fatal(err)
	}
	if action.Status != HostActionSucceeded || action.Result == nil || action.Result.StorageLocationID == "" {
		t.Fatalf("completed action = %#v", action)
	}
	storageLocation, err := manager.GetStorageLocation(ctx, action.Result.StorageLocationID)
	if err != nil {
		t.Fatal(err)
	}
	canonicalExternal, err := CanonicalizeRepositoryPath(external)
	if err != nil {
		t.Fatal(err)
	}
	if storageLocation.Path != canonicalExternal || storageLocation.Name != "External Archive" {
		t.Fatalf("created Storage Location = %#v", storageLocation)
	}
	if pending, err = manager.ListPendingHostActions(ctx); err != nil || len(pending) != 0 {
		t.Fatalf("pending actions after approval = %#v, err=%v", pending, err)
	}
	auditEvents, err := manager.ListLifecycleAudit(ctx, LifecycleAuditFilter{
		TargetType: "storage_location", TargetID: action.Result.StorageLocationID, Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	var hostAudit *LifecycleAuditEvent
	for index := range auditEvents {
		if auditEvents[index].OperationID == first.ActionID {
			hostAudit = &auditEvents[index]
			break
		}
	}
	if hostAudit == nil {
		t.Fatalf("host action audit not found: %#v", auditEvents)
	}
	if hostAudit.ActorUserID == nil || *hostAudit.ActorUserID != actor.UserID || hostAudit.HostInstanceID != "desktop-1" || hostAudit.RequestID != input.RequestID || hostAudit.Source != "desktop_host" || hostAudit.ConfirmationType != "storage_risk_confirmed" || hostAudit.Result != AuditResultSucceeded {
		t.Fatalf("host action audit context = %#v", hostAudit)
	}
}

func TestHostActionCancellationAndExpiry(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()

	cancellable, err := manager.CreateHostAction(ctx, CreateHostActionInput{
		RequestID: "cancel-host-action", Kind: HostActionOpenRepository, TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := manager.CancelHostAction(ctx, cancellable.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != HostActionCancelled || cancelled.CompletedAt == nil {
		t.Fatalf("cancelled action = %#v", cancelled)
	}

	expiring, err := manager.CreateHostAction(ctx, CreateHostActionInput{
		RequestID: "expire-host-action", Kind: HostActionOpenRepository, TTL: time.Nanosecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	if _, err := manager.ListPendingHostActions(ctx); err != nil {
		t.Fatal(err)
	}
	expired, err := manager.GetHostAction(ctx, expiring.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	if expired.Status != HostActionExpired || expired.ErrorCode != "expired" {
		t.Fatalf("expired action = %#v", expired)
	}
}

func TestListPendingHostActionsStaysAvailableWhileWriterIsBusy(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	manager.readerDatabase = catalog.ReaderSQL
	if _, err := manager.CreateHostAction(ctx, CreateHostActionInput{
		RequestID: "writer-busy-host-action", Kind: HostActionOpenRepository, TTL: time.Minute,
	}); err != nil {
		t.Fatal(err)
	}

	writer, err := catalog.SQL.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	if _, err := writer.ExecContext(ctx, `UPDATE system_state SET updated_at = updated_at WHERE id = 1`); err != nil {
		t.Fatal(err)
	}

	readCtx, readCancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer readCancel()
	actions, err := manager.ListPendingHostActions(readCtx)
	if err != nil {
		t.Fatalf("pending host action list waited for the writer connection: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("pending host actions = %d, want 1", len(actions))
	}
}

func TestHostActionFailurePersistsTerminalState(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	action, err := manager.CreateHostAction(ctx, CreateHostActionInput{
		RequestID: "fail-host-action", Kind: HostActionOpenRepository, TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := manager.ExecuteHostAction(ctx, action.ActionID, action.NativeHostNonce(), "desktop-1", missing, nil); err == nil {
		t.Fatal("invalid selected path unexpectedly succeeded")
	}
	failed, err := manager.GetHostAction(ctx, action.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != HostActionFailed || failed.CompletedAt == nil || failed.ErrorCode == "" {
		t.Fatalf("failed action = %#v", failed)
	}
}

// injectedObserver reproduces the facts of a native host's disk for one
// directory without hardware: a macOS mount by default, optionally removable,
// replaced, or on a network filesystem. Every other path keeps the real
// filesystem. It is the in-package twin of testfixture.Observer, which cannot
// be imported here without a cycle.
type injectedObserver struct {
	OSStorageObserver
	target     string
	removable  bool
	remote     bool
	filesystem string
	device     string
	platform   string
}

func (o *injectedObserver) matches(p string) bool {
	clean := filepath.Clean(p)
	return clean == o.target || filepath.Dir(clean) == o.target
}

func (o *injectedObserver) MountInfo(p string) (MountFacts, error) {
	facts, err := o.OSStorageObserver.MountInfo(p)
	if o.matches(p) {
		facts.Removable = o.removable
		facts.Remote = o.remote
		facts.MountID, facts.MountSource, facts.MountPath = "injected-1", "/dev/injected1", o.target
		facts.Device = o.device
		if facts.Device == "" {
			facts.Device = "injected1"
		}
		facts.Platform = o.platform
		if facts.Platform == "" {
			facts.Platform = "darwin"
		}
	}
	return facts, err
}

func (o *injectedObserver) StatFS(p string) (VolumeFacts, error) {
	facts, err := o.OSStorageObserver.StatFS(p)
	if o.matches(p) && o.filesystem != "" {
		facts.Filesystem = o.filesystem
	}
	return facts, err
}

func canonicalTestPath(t *testing.T, p string) string {
	t.Helper()
	canonical, err := CanonicalizeRepositoryPath(p)
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}

func createHostActionAdmin(t *testing.T, catalog *db.DB, username string) repo.User {
	t.Helper()
	actor, err := catalog.Queries.CreateUser(context.Background(), repo.CreateUserParams{
		Username: username, Password: "test", DisplayName: "Admin", Role: "admin",
		WebauthnUserHandle: []byte(username + "-handle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return actor
}

// The native review is the shared assessment: a removable external disk is
// a risk the administrator acknowledges by code, for this task only, and
// nothing is registered until every reported code is acknowledged.
func TestHostActionRequiresAcknowledgedRiskCodesBeforeMutation(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	actor := createHostActionAdmin(t, catalog, "risk-confirmation-admin")
	selected := filepath.Join(base, "external")
	if err := os.Mkdir(selected, 0o755); err != nil {
		t.Fatal(err)
	}
	manager.SetObservationBackend(&injectedObserver{target: canonicalTestPath(t, selected), removable: true})

	action, err := manager.CreateHostAction(ctx, CreateHostActionInput{
		RequestID: "host-risk-two-phase", Kind: HostActionAuthorizeStorageLocation,
		Actor: "web:user:risk-confirmation-admin", ActorUserID: &actor.UserID,
		Summary: HostActionSummary{Name: "Risky Archive"}, TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	decision, err := manager.ExecuteHostAction(ctx, action.ActionID, action.NativeHostNonce(), "desktop-risk", selected, nil)
	if err != nil {
		t.Fatalf("stage risk decision: %v", err)
	}
	assertStorageRiskDecision(t, decision)
	if got := decision.Result.Conflict.RiskWarnings; !reflect.DeepEqual(got, []string{"removable_storage", "non_default_location"}) {
		t.Fatalf("risk codes = %v, want the shared removable and non-default codes", got)
	}
	if decision.Result.Conflict.Fingerprint == "" || decision.ErrorCode != "risk_confirmation" {
		t.Fatalf("risk decision row = %#v", decision)
	}
	if _, lookupErr := manager.queries.GetStorageLocationByPath(ctx, canonicalTestPath(t, selected)); !errors.Is(lookupErr, sql.ErrNoRows) {
		t.Fatalf("unconfirmed risk mutated Storage Locations: %v", lookupErr)
	}
	var consent *StorageRiskConfirmationError
	if _, err := manager.ResolveHostAction(ctx, action.ActionID, "confirm_risk", nil); !errors.As(err, &consent) || len(consent.Risks) != 2 {
		t.Fatalf("empty acknowledgement error = %v", err)
	}
	if _, err := manager.ResolveHostAction(ctx, action.ActionID, "confirm_risk", []string{"removable_storage"}); !errors.As(err, &consent) {
		t.Fatalf("partial acknowledgement error = %v", err)
	}
	if _, err := manager.ResolveHostAction(ctx, action.ActionID, "update_location", []string{"removable_storage", "non_default_location"}); !errors.Is(err, ErrRepositoryRiskConfirmationRequired) {
		t.Fatalf("wrong resolution for a risk review error = %v", err)
	}
	if _, lookupErr := manager.queries.GetStorageLocationByPath(ctx, canonicalTestPath(t, selected)); !errors.Is(lookupErr, sql.ErrNoRows) {
		t.Fatalf("refused acknowledgements mutated Storage Locations: %v", lookupErr)
	}
	completed, err := manager.ResolveHostAction(ctx, action.ActionID, "confirm_risk", []string{"non_default_location", "removable_storage", "unknown_extra"})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != HostActionSucceeded || completed.Result == nil || completed.Result.StorageLocationID == "" {
		t.Fatalf("confirmed host action = %#v", completed)
	}
	events, err := manager.ListLifecycleAudit(ctx, LifecycleAuditFilter{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range events {
		if event.OperationID == action.ActionID && event.ConfirmationType == "storage_risk_confirmed" &&
			event.ActorUserID != nil && *event.ActorUserID == actor.UserID && event.HostInstanceID == "desktop-risk" {
			found = true
		}
	}
	if !found {
		t.Fatalf("confirmed risk host audit missing: %#v", events)
	}
}

// Network storage is refused by the shared classifier on the native path
// exactly as on the HTTP path: the task fails with the typed Problem before
// anything is written, and no decision can confirm past it.
func TestHostActionRefusesNetworkStorageWithoutADecision(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	actor := createHostActionAdmin(t, catalog, "network-admin")
	selected := filepath.Join(base, "nas-share")
	if err := os.Mkdir(selected, 0o755); err != nil {
		t.Fatal(err)
	}
	manager.SetObservationBackend(&injectedObserver{target: canonicalTestPath(t, selected), remote: true, filesystem: "cifs"})

	action, err := manager.CreateHostAction(ctx, CreateHostActionInput{
		RequestID: "host-network-refused", Kind: HostActionAuthorizeStorageLocation,
		Actor: "web:user:network-admin", ActorUserID: &actor.UserID,
		Summary: HostActionSummary{Name: "NAS"}, TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	refused, err := manager.ExecuteHostAction(ctx, action.ActionID, action.NativeHostNonce(), "desktop-network", selected, nil)
	if !errors.Is(err, ErrStorageUnsupportedFilesystem) {
		t.Fatalf("network target error = %v, want ErrStorageUnsupportedFilesystem", err)
	}
	if refused.Status != HostActionFailed || refused.ErrorCode != string(ProblemUnsupportedFilesystem) || refused.Result != nil {
		t.Fatalf("refused task = %#v", refused)
	}
	if _, lookupErr := manager.queries.GetStorageLocationByPath(ctx, canonicalTestPath(t, selected)); !errors.Is(lookupErr, sql.ErrNoRows) {
		t.Fatalf("refused network target mutated Storage Locations: %v", lookupErr)
	}
	if _, err := manager.ResolveHostAction(ctx, action.ActionID, "confirm_risk", []string{"network_filesystem"}); !errors.Is(err, ErrHostActionDecisionNeeded) {
		t.Fatalf("a refusal must not be confirmable, got %v", err)
	}
	if _, err := manager.ExecuteHostAction(ctx, action.ActionID, action.NativeHostNonce(), "desktop-network", selected, []string{"network_filesystem"}); !errors.Is(err, ErrHostActionNotPending) {
		t.Fatalf("a failed task must not run again, got %v", err)
	}
}

// An acknowledgement belongs to the facts it was given for. When the disk
// behind the selected folder is replaced between the review and the
// decision, the decision is stale, the current review is presented, and the
// catalog is unchanged until that review is acknowledged.
func TestHostActionStaleAcknowledgementPresentsTheCurrentReview(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	actor := createHostActionAdmin(t, catalog, "stale-admin")
	selected := filepath.Join(base, "external")
	if err := os.Mkdir(selected, 0o755); err != nil {
		t.Fatal(err)
	}
	observer := &injectedObserver{target: canonicalTestPath(t, selected), removable: true, device: "disk-a"}
	manager.SetObservationBackend(observer)

	action, err := manager.CreateHostAction(ctx, CreateHostActionInput{
		RequestID: "host-stale-review", Kind: HostActionAuthorizeStorageLocation,
		Actor: "web:user:stale-admin", ActorUserID: &actor.UserID,
		Summary: HostActionSummary{Name: "Archive"}, TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := manager.ExecuteHostAction(ctx, action.ActionID, action.NativeHostNonce(), "desktop-stale", selected, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertStorageRiskDecision(t, first)
	acknowledged := first.Result.Conflict.RiskWarnings

	observer.device = "disk-b"
	stale, err := manager.ResolveHostAction(ctx, action.ActionID, "confirm_risk", acknowledged)
	if !errors.Is(err, ErrStorageAssessmentStale) {
		t.Fatalf("replaced disk under an acknowledgement error = %v, want ErrStorageAssessmentStale", err)
	}
	assertStorageRiskDecision(t, stale)
	if stale.Result.Conflict.Fingerprint == first.Result.Conflict.Fingerprint {
		t.Fatal("a replaced disk kept the review fingerprint")
	}
	if _, lookupErr := manager.queries.GetStorageLocationByPath(ctx, canonicalTestPath(t, selected)); !errors.Is(lookupErr, sql.ErrNoRows) {
		t.Fatalf("stale acknowledgement mutated Storage Locations: %v", lookupErr)
	}
	completed, err := manager.ResolveHostAction(ctx, action.ActionID, "confirm_risk", stale.Result.Conflict.RiskWarnings)
	if err != nil || completed.Status != HostActionSucceeded {
		t.Fatalf("acknowledging the current review = %#v, err=%v", completed, err)
	}
}

// Open and both reconnect kinds stop at the same shared review before any
// catalog mutation; a reconnect whose selected tree sits on a different mount
// than the registration recorded adds the continuity risk.
func TestHostActionOpenAndLocateRequireRiskDecisionBeforeCatalogMutation(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	createHostActionAdmin(t, catalog, "host-risk-owner")
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	defaultStorageLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	openSource, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "host-risk-open-source", Actor: "test", Name: "Open Source", DirectoryName: "open-source",
		Role: dbtypes.RepoRoleRegular, StorageLocationID: defaultStorageLocation.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	relocateSource, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "host-risk-relocate-source", Actor: "test", Name: "Relocate Source", DirectoryName: "relocate-source",
		Role: dbtypes.RepoRoleRegular, StorageLocationID: defaultStorageLocation.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	openPath := openSource.Repository.Path
	if _, err := manager.RemoveRepository(ctx, openSource.Repository.RepoID.String(), LifecycleRequest{
		RequestID: "host-risk-open-detach", Actor: "test",
	}); err != nil {
		t.Fatal(err)
	}
	// From here on the default disk reads as removable: every native task on
	// it stops at the shared review before the catalog changes.
	manager.SetObservationBackend(&injectedObserver{target: canonicalTestPath(t, defaultStorageLocation.Path), removable: true})
	openAction, err := manager.CreateHostAction(ctx, CreateHostActionInput{
		RequestID: "host-risk-open", Kind: HostActionOpenRepository, TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	openDecision, err := manager.ExecuteHostAction(ctx, openAction.ActionID, openAction.NativeHostNonce(), "desktop-risk", openPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertStorageRiskDecision(t, openDecision)
	if got := openDecision.Result.Conflict.RiskWarnings; !reflect.DeepEqual(got, []string{"removable_storage"}) {
		t.Fatalf("open under the default Location risk codes = %v, want only removable_storage", got)
	}
	if _, lookupErr := manager.queries.GetRepositoryByPath(ctx, openPath); !errors.Is(lookupErr, sql.ErrNoRows) {
		t.Fatalf("unconfirmed open mutated catalog: %v", lookupErr)
	}
	openCompleted, err := manager.ResolveHostAction(ctx, openAction.ActionID, "confirm_risk", openDecision.Result.Conflict.RiskWarnings)
	if err != nil || openCompleted.Status != HostActionSucceeded {
		t.Fatalf("confirmed open = %#v, err=%v", openCompleted, err)
	}

	oldRepositoryPath := relocateSource.Repository.Path
	newRepositoryPath := filepath.Join(defaultStorageLocation.Path, "relocate-target")
	if err := os.Rename(oldRepositoryPath, newRepositoryPath); err != nil {
		t.Fatal(err)
	}
	locateRepositoryAction, err := manager.CreateHostAction(ctx, CreateHostActionInput{
		RequestID: "host-risk-locate-repository", Kind: HostActionLocateRepository,
		Summary: HostActionSummary{RepositoryID: relocateSource.Repository.RepoID.String()}, TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	repositoryDecision, err := manager.ExecuteHostAction(ctx, locateRepositoryAction.ActionID, locateRepositoryAction.NativeHostNonce(), "desktop-risk", newRepositoryPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertStorageRiskDecision(t, repositoryDecision)
	stillOld, err := manager.GetRepository(relocateSource.Repository.RepoID.String())
	if err != nil || stillOld.Path != oldRepositoryPath {
		t.Fatalf("unconfirmed Repository locate changed path: %#v, err=%v", stillOld, err)
	}
	repositoryCompleted, err := manager.ResolveHostAction(ctx, locateRepositoryAction.ActionID, "confirm_risk", repositoryDecision.Result.Conflict.RiskWarnings)
	if err != nil || repositoryCompleted.Status != HostActionSucceeded {
		t.Fatalf("confirmed Repository locate = %#v, err=%v", repositoryCompleted, err)
	}

	oldRootPath := filepath.Join(base, "external-storageLocation")
	if err := os.Mkdir(oldRootPath, 0o755); err != nil {
		t.Fatal(err)
	}
	externalRoot, err := manager.AddStorageLocation(ctx, oldRootPath, "External")
	if err != nil {
		t.Fatal(err)
	}
	newRootPath := filepath.Join(base, "external-storageLocation-moved")
	if err := os.Rename(oldRootPath, newRootPath); err != nil {
		t.Fatal(err)
	}
	// The registration remembers another mount than the one the moved tree
	// now sits on: the continuity risk joins the review.
	if _, err := manager.queries.UpdateStorageLocationMountFingerprint(ctx, repo.UpdateStorageLocationMountFingerprintParams{
		StorageLocationID: externalRoot.StorageLocationID, MountFingerprint: "registered-on-another-disk",
		UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	}); err != nil {
		t.Fatal(err)
	}
	locateRootAction, err := manager.CreateHostAction(ctx, CreateHostActionInput{
		RequestID: "host-risk-locate-storageLocation", Kind: HostActionLocateStorageLocation,
		Summary: HostActionSummary{StorageLocationID: externalRoot.StorageLocationID.String()}, TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	rootDecision, err := manager.ExecuteHostAction(ctx, locateRootAction.ActionID, locateRootAction.NativeHostNonce(), "desktop-risk", newRootPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertStorageRiskDecision(t, rootDecision)
	if !containsString(rootDecision.Result.Conflict.RiskWarnings, string(RiskMountChanged)) {
		t.Fatalf("mount continuity risk missing: %#v", rootDecision.Result.Conflict.RiskWarnings)
	}
	stillOldRoot, err := manager.GetStorageLocation(ctx, externalRoot.StorageLocationID.String())
	if err != nil || stillOldRoot.Path != externalRoot.Path {
		t.Fatalf("unconfirmed Storage Location locate changed path: %#v, err=%v", stillOldRoot, err)
	}
	rootCompleted, err := manager.ResolveHostAction(ctx, locateRootAction.ActionID, "confirm_risk", rootDecision.Result.Conflict.RiskWarnings)
	if err != nil || rootCompleted.Status != HostActionSucceeded {
		t.Fatalf("confirmed Storage Location locate = %#v, err=%v", rootCompleted, err)
	}
}

// The native host goes through the same role-aware policy as HTTP: a
// reconnect the policy withholds fails the task under the action-unavailable
// Problem with the verdict's blockers, instead of a generic host failure.
func TestHostActionLocateRefusalCarriesThePolicyBlockers(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	primary, err := manager.queries.GetPrimaryRepository(ctx)
	if err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(base, "elsewhere")
	if err := os.Mkdir(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	action, err := manager.CreateHostAction(ctx, CreateHostActionInput{
		RequestID: "host-locate-primary", Kind: HostActionLocateRepository,
		Summary: HostActionSummary{RepositoryID: primary.RepoID.String()}, TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	refused, err := manager.ExecuteHostAction(ctx, action.ActionID, action.NativeHostNonce(), "desktop-policy", elsewhere, nil)
	var unavailable *RepositoryActionUnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("primary reconnect through the native host error = %v, want the policy refusal", err)
	}
	if refused.Status != HostActionFailed || refused.ErrorCode != hostActionCodeActionUnavailable ||
		refused.Result == nil || refused.Result.Refusal == nil || refused.Result.Refusal.Action != string(ActionReconnect) ||
		!containsString(refused.Result.Refusal.Blockers, string(BlockerPrimaryProtected)) {
		t.Fatalf("refused task = %#v result=%#v", refused, refused.Result)
	}
	still, err := manager.GetRepository(primary.RepoID.String())
	if err != nil || still.Path != primary.Path {
		t.Fatalf("refused reconnect moved the primary: %#v err=%v", still, err)
	}
}

func assertStorageRiskDecision(t *testing.T, action HostAction) {
	t.Helper()
	if action.Status != HostActionNeedsDecision || action.Result == nil || action.Result.Conflict == nil ||
		action.Result.Conflict.Type != "storage_risk" || len(action.Result.Conflict.RiskWarnings) == 0 {
		t.Fatalf("storage risk decision = %#v", action)
	}
}

func TestHostActionAddSeparateCarriesActorHostAndIndependentIdentityAudit(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	actor, err := catalog.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "copy-audit-admin", Password: "test", DisplayName: "Admin", Role: "admin",
		WebauthnUserHandle: []byte("copy-audit-admin-handle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	storageLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	original, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "copy-audit-original", Actor: "test", Name: "Original", DirectoryName: "original",
		Role: dbtypes.RepoRoleRegular, StorageLocationID: storageLocation.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	externalPath := filepath.Join(base, "external")
	if err := os.Mkdir(externalPath, 0o755); err != nil {
		t.Fatal(err)
	}
	external, err := manager.AddStorageLocation(ctx, externalPath, "External")
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(external.Path, "copy")
	if err := os.Mkdir(copyPath, 0o755); err != nil {
		t.Fatal(err)
	}
	config, err := repocfg.LoadConfigFromFile(original.Repository.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := config.SaveConfigToFile(copyPath); err != nil {
		t.Fatal(err)
	}
	if err := manager.dirManager.CreateStructure(copyPath); err != nil {
		t.Fatal(err)
	}
	action, err := manager.CreateHostAction(ctx, CreateHostActionInput{
		RequestID: "host-copy-audit", Kind: HostActionOpenRepository, Actor: "web:user:copy-audit-admin",
		ActorUserID: &actor.UserID, TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Opening under an external Storage Location is reviewed first; the
	// identity conflict follows once that review is acknowledged.
	review, err := manager.ExecuteHostAction(ctx, action.ActionID, action.NativeHostNonce(), "desktop-copy", copyPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertStorageRiskDecision(t, review)
	if got := review.Result.Conflict.RiskWarnings; !reflect.DeepEqual(got, []string{"non_default_location"}) {
		t.Fatalf("open under an external Location risk codes = %v", got)
	}
	decision, err := manager.ResolveHostAction(ctx, action.ActionID, "confirm_risk", review.Result.Conflict.RiskWarnings)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Status != HostActionNeedsDecision || decision.Result == nil || decision.Result.Conflict == nil || decision.Result.Conflict.Type != "repository_identity" {
		t.Fatalf("copy decision = %#v", decision)
	}
	completed, err := manager.ResolveHostAction(ctx, action.ActionID, "add_separate", nil)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != HostActionSucceeded || completed.Result == nil || completed.Result.RepositoryID == "" {
		t.Fatalf("copy completion = %#v", completed)
	}
	events, err := manager.ListLifecycleAudit(ctx, LifecycleAuditFilter{
		TargetType: "repository", TargetID: completed.Result.RepositoryID, Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	journalFound, hostFound := false, false
	for _, event := range events {
		if event.ActorUserID == nil || *event.ActorUserID != actor.UserID || event.HostInstanceID != "desktop-copy" {
			continue
		}
		if event.Action == lifecycleKindRegisterRepositoryCopy && event.ConfirmationType == "independent_identity" {
			journalFound = true
		}
		if event.OperationID == action.ActionID && event.Source == "desktop_host" && event.ConfirmationType == "independent_identity" {
			hostFound = true
		}
	}
	if !journalFound || !hostFound {
		t.Fatalf("copy lifecycle/host audit pair missing: %#v", events)
	}
}

func TestRecoverHostActionsReturnsInterruptedRunningTaskToPending(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	actor, err := catalog.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "host-action-owner", Password: "test", DisplayName: "Owner", Role: "admin",
		WebauthnUserHandle: []byte("host-action-owner-handle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	actorID := actor.UserID
	action, err := manager.CreateHostAction(ctx, CreateHostActionInput{
		RequestID: "recover-running-host-action", Kind: HostActionOpenRepository, Actor: "web:user:17",
		ActorUserID: &actorID, TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.SQL.ExecContext(ctx, `UPDATE host_actions SET status = 'running', host_instance_id = 'old-host', selected_path = '/stale' WHERE action_id = ?`, action.ActionID); err != nil {
		t.Fatal(err)
	}
	if err := manager.RecoverHostActions(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := manager.GetHostAction(ctx, action.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Status != HostActionPending || recovered.HostInstanceID != "" || recovered.selectedPath != "" || recovered.ErrorCode != "interrupted" {
		t.Fatalf("recovered action = %#v", recovered)
	}
	actions, err := manager.ListHostActionsForActor(ctx, actorID)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].ActionID != action.ActionID {
		t.Fatalf("actor actions = %#v", actions)
	}
}
