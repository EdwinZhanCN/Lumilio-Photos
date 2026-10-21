package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"
)

func allowedSet(policy RepositoryActionPolicy) map[RepositoryAction]bool {
	out := map[RepositoryAction]bool{}
	for _, action := range policy.Allowed() {
		out[action] = true
	}
	return out
}

// R10: the role-aware policy never invents a protected action, whatever the
// other facts say, and withholds mutation while the Repository is busy,
// pending, in recovery, or in maintenance.
func TestDeriveRepositoryActionsIsRoleAware(t *testing.T) {
	cases := []struct {
		name    string
		facts   RepositoryActionFacts
		allowed []RepositoryAction
	}{
		{"regular active idle", RepositoryActionFacts{Role: dbtypes.RepoRoleRegular, Reachability: dbtypes.RepositoryReachabilityActive, Activity: dbtypes.RepositoryActivityIdle}, []RepositoryAction{ActionVerify, ActionRename, ActionDetach}},
		{"regular scanning", RepositoryActionFacts{Role: dbtypes.RepoRoleRegular, Reachability: dbtypes.RepositoryReachabilityActive, Activity: dbtypes.RepositoryActivityScanning}, []RepositoryAction{ActionVerify, ActionRename}},
		{"regular offline", RepositoryActionFacts{Role: dbtypes.RepoRoleRegular, Reachability: dbtypes.RepositoryReachabilityOffline, Activity: dbtypes.RepositoryActivityIdle}, []RepositoryAction{ActionDetach, ActionReconnect}},
		{"regular offline with stale activity", RepositoryActionFacts{Role: dbtypes.RepoRoleRegular, Reachability: dbtypes.RepositoryReachabilityOffline, Activity: dbtypes.RepositoryActivityImporting}, []RepositoryAction{ActionDetach, ActionReconnect}},
		{"regular identity error", RepositoryActionFacts{Role: dbtypes.RepoRoleRegular, Reachability: dbtypes.RepositoryReachabilityIdentityError, Activity: dbtypes.RepositoryActivityIdle}, []RepositoryAction{ActionDetach, ActionReconnect}},
		{"regular recovery required", RepositoryActionFacts{Role: dbtypes.RepoRoleRegular, Reachability: dbtypes.RepositoryReachabilityRecoveryRequired, Activity: dbtypes.RepositoryActivityIdle}, nil},
		{"regular maintenance", RepositoryActionFacts{Role: dbtypes.RepoRoleRegular, Reachability: dbtypes.RepositoryReachabilityMaintenance, Activity: dbtypes.RepositoryActivityIdle}, nil},
		{"regular pending operation", RepositoryActionFacts{Role: dbtypes.RepoRoleRegular, Reachability: dbtypes.RepositoryReachabilityActive, Activity: dbtypes.RepositoryActivityIdle, PendingOperation: true}, []RepositoryAction{ActionVerify}},
		{"primary active idle", RepositoryActionFacts{Role: dbtypes.RepoRolePrimary, Reachability: dbtypes.RepositoryReachabilityActive, Activity: dbtypes.RepositoryActivityIdle}, []RepositoryAction{ActionVerify}},
		{"primary offline", RepositoryActionFacts{Role: dbtypes.RepoRolePrimary, Reachability: dbtypes.RepositoryReachabilityOffline, Activity: dbtypes.RepositoryActivityIdle}, nil},
		{"primary identity error", RepositoryActionFacts{Role: dbtypes.RepoRolePrimary, Reachability: dbtypes.RepositoryReachabilityIdentityError, Activity: dbtypes.RepositoryActivityIdle}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy := DeriveRepositoryActions(tc.facts)
			want := map[RepositoryAction]bool{}
			for _, action := range tc.allowed {
				want[action] = true
			}
			for _, verdict := range policy.Verdicts {
				if verdict.Allowed() != want[verdict.Action] {
					t.Fatalf("%s: allowed = %t, blockers %v; want allowed %t", verdict.Action, verdict.Allowed(), verdict.Blockers, want[verdict.Action])
				}
				if !verdict.Allowed() && len(verdict.Blockers) == 0 {
					t.Fatalf("%s withheld without a blocker", verdict.Action)
				}
			}
			if tc.facts.Role == dbtypes.RepoRolePrimary {
				for _, action := range []RepositoryAction{ActionRename, ActionDetach, ActionReconnect} {
					verdict := policy.Verdict(action)
					if verdict.Blockers[0] != BlockerPrimaryProtected || len(verdict.Alternatives) == 0 || verdict.Alternatives[0] != AlternativeDefaultConfiguration {
						t.Fatalf("primary %s verdict = %+v, want primary_protected with the default configuration alternative first", action, verdict)
					}
				}
			}
		})
	}
}

// actionFixture holds a default Location with its Primary and makes fresh
// regular Repositories whose catalog facts a test can set directly.
type actionFixture struct {
	t       *testing.T
	ctx     context.Context
	manager *DefaultRepositoryManager
	root    string
	primary repo.Repository
	seq     int
}

func newActionFixture(t *testing.T) *actionFixture {
	t.Helper()
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "default")
	initializeDefaultStorageForTest(t, manager, root)
	primary, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return &actionFixture{t: t, ctx: ctx, manager: manager, root: root, primary: primary}
}

func (f *actionFixture) regular(name string) repo.Repository {
	f.t.Helper()
	f.seq++
	created, err := f.manager.CreateRepository(f.ctx, CreateRepositorySpec{
		RequestID: fmt.Sprintf("action-%s-%d", name, f.seq), Actor: "test", Name: name,
		DirectoryName: fmt.Sprintf("%s-%d", name, f.seq), Role: dbtypes.RepoRoleRegular,
		StorageLocationID: f.primary.StorageLocationID.String(),
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return *created.Repository
}

func (f *actionFixture) setFacts(id uuid.UUID, reachability dbtypes.RepositoryReachability, activity dbtypes.RepositoryActivity) {
	f.t.Helper()
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := f.manager.queries.UpdateRepositoryReachability(f.ctx, repo.UpdateRepositoryReachabilityParams{RepoID: id, Reachability: reachability, UpdatedAt: now}); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.manager.queries.UpdateRepositoryActivity(f.ctx, repo.UpdateRepositoryActivityParams{RepoID: id, Activity: activity, UpdatedAt: now}); err != nil {
		f.t.Fatal(err)
	}
}

// plantRunningOperation journals a running operation against the Repository,
// the state a crash leaves behind, and returns a function that ends it.
func (f *actionFixture) plantRunningOperation(repository repo.Repository) func() {
	f.t.Helper()
	id := repository.RepoID.String()
	operation, _, err := f.manager.beginLifecycleOperation(f.ctx, lifecycleBeginInput{
		RequestID: "planted-" + uuid.NewString(), Kind: lifecycleKindRenameRepository,
		Payload: renameRepositoryOperationPayload{RepositoryID: id, Path: repository.Path, NewName: "planted"},
		Actor:   "test", TargetType: "repository", TargetID: &id,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return func() {
		_ = f.manager.failLifecycleOperation(f.ctx, operation.OperationID, true, errors.New("test planted"), nil)
	}
}

func (f *actionFixture) row(id uuid.UUID) repo.Repository {
	f.t.Helper()
	row, err := f.manager.queries.GetRepository(f.ctx, id)
	if err != nil {
		f.t.Fatal(err)
	}
	return row
}

// Capability and command agreement over generated role, reachability,
// activity, and journal combinations: a command refuses exactly when the
// policy on the same facts withholds its action, with the same blockers, and
// never refuses on policy grounds when the policy allows it.
func TestCommandsAgreeWithTheActionPolicy(t *testing.T) {
	f := newActionFixture(t)
	reachabilities := []dbtypes.RepositoryReachability{
		dbtypes.RepositoryReachabilityActive, dbtypes.RepositoryReachabilityOffline, dbtypes.RepositoryReachabilityIdentityError,
		dbtypes.RepositoryReachabilityRecoveryRequired, dbtypes.RepositoryReachabilityMaintenance,
	}
	activities := []dbtypes.RepositoryActivity{dbtypes.RepositoryActivityIdle, dbtypes.RepositoryActivityScanning}
	type command struct {
		action RepositoryAction
		run    func(repo.Repository) error
	}
	destination := filepath.Join(t.TempDir(), "nowhere")
	commands := []command{
		{ActionRename, func(r repo.Repository) error {
			_, err := f.manager.RenameRepository(f.ctx, r.RepoID.String(), "Agreed", LifecycleRequest{RequestID: "agree-rename-" + uuid.NewString(), Actor: "test"})
			return err
		}},
		{ActionDetach, func(r repo.Repository) error {
			_, err := f.manager.RemoveRepository(f.ctx, r.RepoID.String(), LifecycleRequest{RequestID: "agree-detach-" + uuid.NewString(), Actor: "test"})
			return err
		}},
		{ActionReconnect, func(r repo.Repository) error {
			_, err := f.manager.ReconnectRepository(f.ctx, r.RepoID.String(), destination, LifecycleRequest{RequestID: "agree-reconnect-" + uuid.NewString(), Actor: "test"})
			return err
		}},
	}
	for _, role := range []dbtypes.RepoRole{dbtypes.RepoRoleRegular, dbtypes.RepoRolePrimary} {
		for _, reachability := range reachabilities {
			for _, activity := range activities {
				for _, pending := range []bool{false, true} {
					for _, cmd := range commands {
						name := fmt.Sprintf("%s/%s/%s/pending=%t/%s", role, reachability, activity, pending, cmd.action)
						t.Run(name, func(t *testing.T) {
							var subject repo.Repository
							if role == dbtypes.RepoRolePrimary {
								subject = f.primary
							} else {
								subject = f.regular("agree")
							}
							f.setFacts(subject.RepoID, reachability, activity)
							var endPlanted func()
							if pending {
								endPlanted = f.plantRunningOperation(subject)
							}
							t.Cleanup(func() {
								if endPlanted != nil {
									endPlanted()
								}
								if role == dbtypes.RepoRolePrimary {
									f.setFacts(subject.RepoID, dbtypes.RepositoryReachabilityActive, dbtypes.RepositoryActivityIdle)
								}
							})
							policy := DeriveRepositoryActions(RepositoryActionFacts{Role: role, Reachability: reachability, Activity: activity, PendingOperation: pending})
							verdict := policy.Verdict(cmd.action)
							err := cmd.run(f.row(subject.RepoID))
							var unavailable *RepositoryActionUnavailableError
							refused := errors.As(err, &unavailable)
							if verdict.Allowed() {
								if refused {
									t.Fatalf("policy allows %s but the command refused: %v", cmd.action, err)
								}
								return
							}
							// A reconnect withheld only because the original is online is
							// what the command proves by reading the marker itself.
							if cmd.action == ActionReconnect && len(verdict.Blockers) == 1 && verdict.Blockers[0] == BlockerOriginalAvailable {
								if !errors.Is(err, ErrRepositoryOriginalOnline) {
									t.Fatalf("reconnect with the original online = %v, want ErrRepositoryOriginalOnline", err)
								}
								return
							}
							if !refused {
								t.Fatalf("policy withholds %s (%v) but the command did not refuse on policy grounds: %v", cmd.action, verdict.Blockers, err)
							}
							if unavailable.Action != cmd.action || fmt.Sprint(unavailable.Blockers) != fmt.Sprint(verdict.Blockers) {
								t.Fatalf("command refusal = %+v, policy verdict = %+v", unavailable, verdict)
							}
							if role == dbtypes.RepoRolePrimary {
								switch cmd.action {
								case ActionDetach:
									if !errors.Is(err, ErrPrimaryRepositoryNotRemovable) {
										t.Fatalf("primary detach refusal does not match the legacy sentinel: %v", err)
									}
								case ActionReconnect:
									if !errors.Is(err, ErrPathNotAllowed) {
										t.Fatalf("primary reconnect refusal does not match the legacy sentinel: %v", err)
									}
								}
							}
						})
					}
				}
			}
		}
	}
}

// markerBytes reads the complete marker at a Repository path.
func markerBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(path, ".lumiliorepo"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// moveRepositoryTree copies the marker and layout of a Repository to another
// Location and removes the original, the external move a reconnect answers.
func (f *actionFixture) moveRepositoryTree(repository repo.Repository, locationPath string) string {
	f.t.Helper()
	newPath := filepath.Join(locationPath, filepath.Base(repository.Path))
	if err := os.MkdirAll(newPath, 0o755); err != nil {
		f.t.Fatal(err)
	}
	config, err := repocfg.LoadConfigFromFile(repository.Path)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := config.SaveConfigToFile(newPath); err != nil {
		f.t.Fatal(err)
	}
	if err := f.manager.dirManager.CreateStructure(newPath); err != nil {
		f.t.Fatal(err)
	}
	if err := os.RemoveAll(repository.Path); err != nil {
		f.t.Fatal(err)
	}
	return newPath
}

// R5: a Rename parked before its lease while a reconnect moves the tree uses
// the row and path it finds under the lease. Only the raw name changes; the
// identity and every other marker field survive; the old path is untouched.
func TestRenameRefreshesTheLeasedRowAfterAReconnect(t *testing.T) {
	f := newActionFixture(t)
	elsewhere := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.Mkdir(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	location, err := f.manager.AddStorageLocation(f.ctx, elsewhere, "Elsewhere")
	if err != nil {
		t.Fatal(err)
	}
	repository := f.regular("moving")
	originalPath := repository.Path
	before, err := repocfg.LoadConfigFromFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}

	parked := make(chan struct{})
	release := make(chan struct{})
	testHookBeforeRenameLease = func() {
		close(parked)
		<-release
	}
	t.Cleanup(func() { testHookBeforeRenameLease = nil })
	type outcome struct {
		result *RepositoryRenameResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := f.manager.RenameRepository(f.ctx, repository.RepoID.String(), "Renamed After Move",
			LifecycleRequest{RequestID: "rename-race", Actor: "test"})
		done <- outcome{result, err}
	}()
	<-parked
	testHookBeforeRenameLease = nil
	newPath := f.moveRepositoryTree(repository, location.Path)
	if _, err := f.manager.ReconnectRepository(f.ctx, repository.RepoID.String(), newPath,
		LifecycleRequest{RequestID: "reconnect-during-rename", Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	close(release)
	got := <-done
	if got.err != nil {
		t.Fatalf("rename after reconnect = %v", got.err)
	}
	canonicalNew, err := CanonicalizeRepositoryPath(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.result.Repository.Path != canonicalNew || got.result.Repository.Name != "Renamed After Move" {
		t.Fatalf("renamed row = path %q name %q, want the reconnected path", got.result.Repository.Path, got.result.Repository.Name)
	}
	after, err := repocfg.LoadConfigFromFile(canonicalNew)
	if err != nil {
		t.Fatal(err)
	}
	if after.Name != "Renamed After Move" || after.ID != before.ID || after.Version != before.Version ||
		after.StorageStrategy != before.StorageStrategy || !after.CreatedAt.Equal(before.CreatedAt) ||
		after.LocalSettings != before.LocalSettings {
		t.Fatalf("rename changed more than the raw name: before %+v after %+v", before, after)
	}
	if _, err := os.Stat(originalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rename recreated the old path: stat = %v", err)
	}
	if got.result.Receipt.Kind != lifecycleKindRenameRepository || got.result.Receipt.Status != lifecycleStatusCompleted || got.result.Receipt.Replayed {
		t.Fatalf("receipt = %+v", got.result.Receipt)
	}

	// Replay: the same payload returns the same receipt; a different payload
	// under the same request ID conflicts.
	replayed, err := f.manager.RenameRepository(f.ctx, repository.RepoID.String(), "Renamed After Move", LifecycleRequest{RequestID: "rename-race", Actor: "test"})
	if err != nil || !replayed.Receipt.Replayed || replayed.Receipt.OperationID != got.result.Receipt.OperationID {
		t.Fatalf("replay = %+v, %v; want the same operation replayed", replayed, err)
	}
	if _, err := f.manager.RenameRepository(f.ctx, repository.RepoID.String(), "Different", LifecycleRequest{RequestID: "rename-race", Actor: "test"}); !errors.Is(err, ErrLifecycleRequestConflict) {
		t.Fatalf("changed payload under the same request = %v, want ErrLifecycleRequestConflict", err)
	}
}

// R5/R12: a marker replaced by another identity between the projected check
// and the lease is refused as a mismatch and its bytes are never touched.
func TestRenameRefusesAReplacedIdentityWithoutOverwriting(t *testing.T) {
	f := newActionFixture(t)
	repository := f.regular("replaced")
	foreign, err := repocfg.LoadConfigFromFile(repository.Path)
	if err != nil {
		t.Fatal(err)
	}
	foreign.ID = uuid.NewString()
	foreign.Name = "Someone Else"
	testHookBeforeRenameLease = func() {
		if err := foreign.SaveConfigToFile(repository.Path); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { testHookBeforeRenameLease = nil })
	_, err = f.manager.RenameRepository(f.ctx, repository.RepoID.String(), "Hijack", LifecycleRequest{RequestID: "rename-replaced", Actor: "test"})
	if !errors.Is(err, ErrRepositoryIDMismatch) {
		t.Fatalf("rename over a replaced marker = %v, want ErrRepositoryIDMismatch", err)
	}
	current, err := repocfg.LoadConfigFromFile(repository.Path)
	if err != nil || current.ID != foreign.ID || current.Name != "Someone Else" {
		t.Fatalf("replaced marker was touched: %+v, %v", current, err)
	}
	if f.row(repository.RepoID).Name != repository.Name {
		t.Fatal("catalog name changed although the rename was refused")
	}
	if _, err := f.manager.queries.GetLifecycleOperationByRequestID(f.ctx, "rename-replaced"); err == nil {
		t.Fatal("a refused rename must not leave a journal entry")
	}
}

// R12 command guard: when the catalog commit fails, the rollback restores the
// previous complete marker only while it still carries this identity. A
// marker replaced in between is left alone and the operation is marked as
// requiring recovery instead of being rolled back over the replacement.
func TestRenameRollbackNeverOverwritesAReplacedIdentity(t *testing.T) {
	f := newActionFixture(t)
	t.Run("rollback restores the previous marker", func(t *testing.T) {
		repository := f.regular("rollback")
		before := markerBytes(t, repository.Path)
		testHookBeforeRenameCatalogCommit = func() error { return errors.New("injected catalog failure") }
		t.Cleanup(func() { testHookBeforeRenameCatalogCommit = nil })
		if _, err := f.manager.RenameRepository(f.ctx, repository.RepoID.String(), "Never Committed", LifecycleRequest{RequestID: "rename-rollback", Actor: "test"}); err == nil {
			t.Fatal("rename succeeded although the catalog commit failed")
		}
		if !bytes.Equal(markerBytes(t, repository.Path), before) {
			t.Fatal("rollback did not restore the previous complete marker byte for byte")
		}
		operation, err := f.manager.queries.GetLifecycleOperationByRequestID(f.ctx, "rename-rollback")
		if err != nil || operation.Status != lifecycleStatusRolledBack {
			t.Fatalf("operation = %+v, %v; want rolled_back", operation, err)
		}
		if f.row(repository.RepoID).Name != repository.Name {
			t.Fatal("catalog name changed although the commit failed")
		}
	})
	t.Run("rollback refuses a replaced identity", func(t *testing.T) {
		repository := f.regular("rollback-replaced")
		foreign, err := repocfg.LoadConfigFromFile(repository.Path)
		if err != nil {
			t.Fatal(err)
		}
		foreign.ID = uuid.NewString()
		testHookBeforeRenameCatalogCommit = func() error {
			if err := foreign.SaveConfigToFile(repository.Path); err != nil {
				return err
			}
			return errors.New("injected catalog failure")
		}
		t.Cleanup(func() { testHookBeforeRenameCatalogCommit = nil })
		replaced := markerBytes(t, repository.Path)
		if _, err := f.manager.RenameRepository(f.ctx, repository.RepoID.String(), "Never Committed", LifecycleRequest{RequestID: "rename-rollback-replaced", Actor: "test"}); err == nil {
			t.Fatal("rename succeeded although the catalog commit failed")
		}
		if current := markerBytes(t, repository.Path); bytes.Equal(current, replaced) {
			// The hook wrote the foreign marker after the rename wrote its own;
			// what must hold is that the foreign identity is still there.
			t.Fatal("test wiring: the foreign marker was never written")
		} else if loaded, loadErr := repocfg.LoadConfigFromFile(repository.Path); loadErr != nil || loaded.ID != foreign.ID {
			t.Fatalf("rollback overwrote the replaced identity: %+v, %v", loaded, loadErr)
		}
		operation, err := f.manager.queries.GetLifecycleOperationByRequestID(f.ctx, "rename-rollback-replaced")
		if err != nil || operation.Status != lifecycleStatusFailed || operation.Phase != lifecyclePhaseRollbackRequired {
			t.Fatalf("operation = %+v, %v; want failed with rollback_required", operation, err)
		}
	})
}

// Detach is journaled: a retry with the same request finds the same receipt
// although the row is gone; another Repository under that request ID
// conflicts; an interrupted detach is rolled back at startup with the
// Repository intact.
func TestDetachIsJournaledAndReplaysItsReceipt(t *testing.T) {
	f := newActionFixture(t)
	repository := f.regular("detach")
	other := f.regular("other")
	receipt, err := f.manager.RemoveRepository(f.ctx, repository.RepoID.String(), LifecycleRequest{RequestID: "detach-once", Actor: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Kind != lifecycleKindDetachRepository || receipt.Status != lifecycleStatusCompleted || receipt.Replayed || receipt.RequestID != "detach-once" {
		t.Fatalf("receipt = %+v", receipt)
	}
	if _, err := os.Stat(repository.Path); err != nil {
		t.Fatalf("detach touched the files: %v", err)
	}
	replayed, err := f.manager.RemoveRepository(f.ctx, repository.RepoID.String(), LifecycleRequest{RequestID: "detach-once", Actor: "test"})
	if err != nil || !replayed.Replayed || replayed.OperationID != receipt.OperationID {
		t.Fatalf("replayed detach = %+v, %v; want the same operation", replayed, err)
	}
	if _, err := f.manager.RemoveRepository(f.ctx, other.RepoID.String(), LifecycleRequest{RequestID: "detach-once", Actor: "test"}); !errors.Is(err, ErrLifecycleRequestConflict) {
		t.Fatalf("another repository under the same request = %v, want ErrLifecycleRequestConflict", err)
	}

	id := other.RepoID.String()
	interrupted, _, err := f.manager.beginLifecycleOperation(f.ctx, lifecycleBeginInput{
		RequestID: "detach-interrupted", Kind: lifecycleKindDetachRepository,
		Payload: detachRepositoryOperationPayload{RepositoryID: id, Path: other.Path, Name: other.Name},
		Actor:   "test", TargetType: "repository", TargetID: &id,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.manager.RecoverLifecycleOperations(f.ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := f.manager.queries.GetLifecycleOperation(f.ctx, interrupted.OperationID)
	if err != nil || recovered.Status != lifecycleStatusRolledBack {
		t.Fatalf("interrupted detach after recovery = %+v, %v; want rolled_back", recovered, err)
	}
	if _, err := f.manager.queries.GetRepository(f.ctx, other.RepoID); err != nil {
		t.Fatalf("recovery removed a repository whose detach never committed: %v", err)
	}
}
