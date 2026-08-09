package storage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

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
		RequestID:   "host-action-add-root-1",
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
	boundAgain, err := manager.SetHostActionExpectedVersion(ctx, first.ActionID, first.NativeHostNonce(), 99)
	if err != nil || boundAgain.ExpectedVersion != 42 {
		t.Fatalf("expected version compare-and-set changed: action=%#v err=%v", boundAgain, err)
	}
	if _, err := manager.ExecuteHostAction(ctx, first.ActionID, "wrong-nonce", "desktop-1", filepath.Join(base, "external")); !errors.Is(err, ErrHostActionNonceInvalid) {
		t.Fatalf("wrong nonce error = %v, want ErrHostActionNonceInvalid", err)
	}

	external := filepath.Join(base, "external")
	if err := os.Mkdir(external, 0o755); err != nil {
		t.Fatal(err)
	}
	action, err := manager.ExecuteHostAction(ctx, first.ActionID, first.NativeHostNonce(), "desktop-1", external)
	if err != nil {
		t.Fatal(err)
	}
	if action.Status != HostActionSucceeded || action.Result == nil || action.Result.RootID == "" {
		t.Fatalf("completed action = %#v", action)
	}
	root, err := manager.GetRepositoryRoot(ctx, action.Result.RootID)
	if err != nil {
		t.Fatal(err)
	}
	canonicalExternal, err := CanonicalizeRepositoryPath(external)
	if err != nil {
		t.Fatal(err)
	}
	if root.Path != canonicalExternal || root.Name != "External Archive" {
		t.Fatalf("created Storage Location = %#v", root)
	}
	if pending, err = manager.ListPendingHostActions(ctx); err != nil || len(pending) != 0 {
		t.Fatalf("pending actions after approval = %#v, err=%v", pending, err)
	}
	auditEvents, err := manager.ListLifecycleAudit(ctx, LifecycleAuditFilter{
		TargetType: "storage_location", TargetID: action.Result.RootID, Limit: 20,
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
	if hostAudit.ActorUserID == nil || *hostAudit.ActorUserID != actor.UserID || hostAudit.HostInstanceID != "desktop-1" || hostAudit.RequestID != input.RequestID || hostAudit.Source != "desktop_host" || hostAudit.ConfirmationType != "native_directory_selection" || hostAudit.Result != AuditResultSucceeded {
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
	if _, err := manager.ExecuteHostAction(ctx, action.ActionID, action.NativeHostNonce(), "desktop-1", missing); err == nil {
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

func TestHostActionRequiresExplicitStorageRiskConfirmationBeforeMutation(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	actor, err := catalog.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "risk-confirmation-admin", Password: "test", DisplayName: "Admin", Role: "admin",
		WebauthnUserHandle: []byte("risk-confirmation-admin-handle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	selected := filepath.Join(base, "external")
	if err := os.Mkdir(selected, 0o755); err != nil {
		t.Fatal(err)
	}
	previousInspector := inspectHostActionStoragePath
	inspectHostActionStoragePath = func(path string) StoragePathInfo {
		info := InspectStoragePath(path)
		info.RiskWarnings = []string{"network_filesystem", "removable_storage", "cloud_sync_directory"}
		return info
	}
	t.Cleanup(func() { inspectHostActionStoragePath = previousInspector })
	action, err := manager.CreateHostAction(ctx, CreateHostActionInput{
		RequestID: "host-risk-two-phase", Kind: HostActionAuthorizeStorageLocation,
		Actor: "web:user:risk-confirmation-admin", ActorUserID: &actor.UserID,
		Summary: HostActionSummary{Name: "Risky Archive"}, TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	decision, err := manager.ExecuteHostAction(ctx, action.ActionID, action.NativeHostNonce(), "desktop-risk", selected)
	if err != nil {
		t.Fatalf("stage risk decision: %v", err)
	}
	if decision.Status != HostActionNeedsDecision || decision.Result == nil || decision.Result.Conflict == nil ||
		decision.Result.Conflict.Type != "storage_risk" || len(decision.Result.Conflict.RiskWarnings) != 3 {
		t.Fatalf("risk decision = %#v", decision)
	}
	if _, lookupErr := manager.queries.GetRepositoryRootByPath(ctx, selected); !errors.Is(lookupErr, sql.ErrNoRows) {
		t.Fatalf("unconfirmed risk mutated Storage Locations: %v", lookupErr)
	}
	if _, err := manager.ResolveHostAction(ctx, action.ActionID, "confirm_risk", false); !errors.Is(err, ErrRepositoryRiskConfirmationRequired) {
		t.Fatalf("false risk confirmation error = %v", err)
	}
	completed, err := manager.ResolveHostAction(ctx, action.ActionID, "confirm_risk", true)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != HostActionSucceeded || completed.Result == nil || completed.Result.RootID == "" {
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

func TestHostActionOpenAndLocateRequireRiskDecisionBeforeCatalogMutation(t *testing.T) {
	catalog, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	if _, err := catalog.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "host-risk-owner", Password: "test", DisplayName: "Owner", Role: "admin",
		WebauthnUserHandle: []byte("host-risk-owner-handle"),
	}); err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	initializeDefaultStorageForTest(t, manager, filepath.Join(base, "default"))
	defaultRoot, err := manager.queries.GetDefaultRepositoryRoot(ctx)
	if err != nil {
		t.Fatal(err)
	}

	previousInspector := inspectHostActionStoragePath
	inspectHostActionStoragePath = func(path string) StoragePathInfo {
		info := InspectStoragePath(path)
		info.RiskWarnings = []string{"network_filesystem"}
		info.MountFingerprint = "confirmed-test-mount"
		return info
	}
	t.Cleanup(func() { inspectHostActionStoragePath = previousInspector })

	openSource, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "host-risk-open-source", Actor: "test", Name: "Open Source", DirectoryName: "open-source",
		Role: dbtypes.RepoRoleRegular, RootID: defaultRoot.RootID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	openPath := openSource.Repository.Path
	if err := manager.RemoveRepository(ctx, openSource.Repository.RepoID.String(), LifecycleRequest{
		RequestID: "host-risk-open-detach", Actor: "test",
	}); err != nil {
		t.Fatal(err)
	}
	openAction, err := manager.CreateHostAction(ctx, CreateHostActionInput{
		RequestID: "host-risk-open", Kind: HostActionOpenRepository, TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	openDecision, err := manager.ExecuteHostAction(ctx, openAction.ActionID, openAction.NativeHostNonce(), "desktop-risk", openPath)
	if err != nil {
		t.Fatal(err)
	}
	assertStorageRiskDecision(t, openDecision)
	if _, lookupErr := manager.queries.GetRepositoryByPath(ctx, openPath); !errors.Is(lookupErr, sql.ErrNoRows) {
		t.Fatalf("unconfirmed open mutated catalog: %v", lookupErr)
	}
	openCompleted, err := manager.ResolveHostAction(ctx, openAction.ActionID, "confirm_risk", true)
	if err != nil || openCompleted.Status != HostActionSucceeded {
		t.Fatalf("confirmed open = %#v, err=%v", openCompleted, err)
	}

	relocateSource, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "host-risk-relocate-source", Actor: "test", Name: "Relocate Source", DirectoryName: "relocate-source",
		Role: dbtypes.RepoRoleRegular, RootID: defaultRoot.RootID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	oldRepositoryPath := relocateSource.Repository.Path
	newRepositoryPath := filepath.Join(defaultRoot.Path, "relocate-target")
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
	repositoryDecision, err := manager.ExecuteHostAction(ctx, locateRepositoryAction.ActionID, locateRepositoryAction.NativeHostNonce(), "desktop-risk", newRepositoryPath)
	if err != nil {
		t.Fatal(err)
	}
	assertStorageRiskDecision(t, repositoryDecision)
	stillOld, err := manager.GetRepository(relocateSource.Repository.RepoID.String())
	if err != nil || stillOld.Path != oldRepositoryPath {
		t.Fatalf("unconfirmed Repository locate changed path: %#v, err=%v", stillOld, err)
	}
	repositoryCompleted, err := manager.ResolveHostAction(ctx, locateRepositoryAction.ActionID, "confirm_risk", true)
	if err != nil || repositoryCompleted.Status != HostActionSucceeded {
		t.Fatalf("confirmed Repository locate = %#v, err=%v", repositoryCompleted, err)
	}

	oldRootPath := filepath.Join(base, "external-root")
	if err := os.Mkdir(oldRootPath, 0o755); err != nil {
		t.Fatal(err)
	}
	externalRoot, err := manager.AddRepositoryRoot(ctx, oldRootPath, "External")
	if err != nil {
		t.Fatal(err)
	}
	newRootPath := filepath.Join(base, "external-root-moved")
	if err := os.Rename(oldRootPath, newRootPath); err != nil {
		t.Fatal(err)
	}
	locateRootAction, err := manager.CreateHostAction(ctx, CreateHostActionInput{
		RequestID: "host-risk-locate-root", Kind: HostActionLocateStorageLocation,
		Summary: HostActionSummary{RootID: externalRoot.RootID.String()}, TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	rootDecision, err := manager.ExecuteHostAction(ctx, locateRootAction.ActionID, locateRootAction.NativeHostNonce(), "desktop-risk", newRootPath)
	if err != nil {
		t.Fatal(err)
	}
	assertStorageRiskDecision(t, rootDecision)
	if !containsString(rootDecision.Result.Conflict.RiskWarnings, "mount_fingerprint_changed") {
		t.Fatalf("mount fingerprint warning missing: %#v", rootDecision.Result.Conflict.RiskWarnings)
	}
	stillOldRoot, err := manager.GetRepositoryRoot(ctx, externalRoot.RootID.String())
	if err != nil || stillOldRoot.Path != externalRoot.Path {
		t.Fatalf("unconfirmed Storage Location locate changed path: %#v, err=%v", stillOldRoot, err)
	}
	rootCompleted, err := manager.ResolveHostAction(ctx, locateRootAction.ActionID, "confirm_risk", true)
	if err != nil || rootCompleted.Status != HostActionSucceeded {
		t.Fatalf("confirmed Storage Location locate = %#v, err=%v", rootCompleted, err)
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
	root, err := manager.queries.GetDefaultRepositoryRoot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	original, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "copy-audit-original", Actor: "test", Name: "Original", DirectoryName: "original",
		Role: dbtypes.RepoRoleRegular, RootID: root.RootID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	externalPath := filepath.Join(base, "external")
	if err := os.Mkdir(externalPath, 0o755); err != nil {
		t.Fatal(err)
	}
	external, err := manager.AddRepositoryRoot(ctx, externalPath, "External")
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
	decision, err := manager.ExecuteHostAction(ctx, action.ActionID, action.NativeHostNonce(), "desktop-copy", copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Status != HostActionNeedsDecision || decision.Result == nil || decision.Result.Conflict == nil {
		t.Fatalf("copy decision = %#v", decision)
	}
	completed, err := manager.ResolveHostAction(ctx, action.ActionID, "add_separate")
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
