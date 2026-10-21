package storage

import (
	"context"
	"fmt"
	"strings"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
)

// StorageLocationAction is one command an administrator may run against a
// registered Storage Location. Like the Repository vocabulary it is closed:
// the Web menu, the native host, and the HTTP handlers consume the same set,
// and a command refuses exactly the actions the policy withholds.
type StorageLocationAction string

const (
	// LocationActionReconnect re-points a positively unavailable external
	// Storage Location, and every child Repository with it, at the tree that
	// carries its identity. It never moves data.
	LocationActionReconnect StorageLocationAction = "reconnect"
	// LocationActionDetach removes the registration of an empty external
	// Storage Location and preserves every file.
	LocationActionDetach StorageLocationAction = "detach"
)

// StorageLocationActions lists every action in a stable order.
var StorageLocationActions = []StorageLocationAction{LocationActionReconnect, LocationActionDetach}

// StorageLocationActionFacts are the facts the Location policy reads: the
// catalog row, its journal, and one read-only look at the registered path.
type StorageLocationActionFacts struct {
	Kind            dbtypes.StorageLocationKind
	Status          dbtypes.StorageLocationStatus
	RepositoryCount int64
	// PendingOperation is true while a journaled lifecycle operation that
	// targets this Storage Location is running.
	PendingOperation bool
	// Original is what the registered path currently holds.
	Original OriginalIdentityState
}

// StorageLocationActionVerdict is the policy's answer for one action.
type StorageLocationActionVerdict struct {
	Action       StorageLocationAction
	Blockers     []RepositoryActionBlocker
	Alternatives []RepositoryActionAlternative
}

// Allowed is true when nothing withholds the action.
func (v StorageLocationActionVerdict) Allowed() bool { return len(v.Blockers) == 0 }

// StorageLocationActionPolicy is the complete verdict set for one Location.
type StorageLocationActionPolicy struct {
	Verdicts []StorageLocationActionVerdict
}

// Verdict returns the verdict for one action; an unknown action is withheld.
func (p StorageLocationActionPolicy) Verdict(action StorageLocationAction) StorageLocationActionVerdict {
	for _, verdict := range p.Verdicts {
		if verdict.Action == action {
			return verdict
		}
	}
	return StorageLocationActionVerdict{Action: action, Blockers: []RepositoryActionBlocker{BlockerPendingOperation}}
}

// DeriveStorageLocationActions is the pure kind-aware Storage Location
// policy. The Default Storage Location follows the configured storage.path
// and is never reconnected or detached on its own; an external Location is
// reconnected only when its original is positively unavailable and detached
// only while it registers no Repository.
func DeriveStorageLocationActions(facts StorageLocationActionFacts) StorageLocationActionPolicy {
	verdicts := make([]StorageLocationActionVerdict, 0, len(StorageLocationActions))
	for _, action := range StorageLocationActions {
		verdict := StorageLocationActionVerdict{Action: action}
		block := func(blocker RepositoryActionBlocker, alternatives ...RepositoryActionAlternative) {
			verdict.Blockers = append(verdict.Blockers, blocker)
			verdict.Alternatives = append(verdict.Alternatives, alternatives...)
		}
		if facts.Kind != dbtypes.StorageLocationKindExternal {
			block(BlockerDefaultProtected, AlternativeDefaultConfiguration)
		}
		if facts.Status == dbtypes.StorageLocationStatusMaintenance {
			block(BlockerMaintenance, AlternativeWaitForIdle)
		}
		if facts.PendingOperation {
			block(BlockerPendingOperation, AlternativeWaitForIdle)
		}
		switch action {
		case LocationActionReconnect:
			switch facts.Original {
			case OriginalOnline:
				block(BlockerOriginalAvailable)
			case OriginalInaccessible:
				block(BlockerOriginalInaccessible, AlternativeCheckAccess)
			}
		case LocationActionDetach:
			if facts.RepositoryCount != 0 {
				block(BlockerRegisteredRepositories)
			}
		}
		verdict.Blockers = uniqueBlockers(verdict.Blockers)
		verdict.Alternatives = uniqueAlternatives(verdict.Alternatives)
		verdicts = append(verdicts, verdict)
	}
	return StorageLocationActionPolicy{Verdicts: verdicts}
}

// StorageLocationActionUnavailableError is the typed refusal a Location
// command returns when the policy withholds its action on the facts it
// re-read under its barriers. It carries only stable codes, never host paths.
type StorageLocationActionUnavailableError struct {
	StorageLocationID string
	Action            StorageLocationAction
	Blockers          []RepositoryActionBlocker
	Alternatives      []RepositoryActionAlternative
}

func (e *StorageLocationActionUnavailableError) Error() string {
	return fmt.Sprintf("storage location action %s is unavailable: %s", e.Action, strings.Join(BlockerCodes(e.Blockers), ", "))
}

// Is lets callers that only know the legacy sentinel errors keep matching.
func (e *StorageLocationActionUnavailableError) Is(target error) bool {
	for _, blocker := range e.Blockers {
		switch {
		case target == ErrStorageLocationNotRemovable && blocker == BlockerDefaultProtected:
			return true
		case target == ErrStorageLocationInUse && blocker == BlockerRegisteredRepositories && e.Action == LocationActionDetach:
			return true
		case target == ErrRepositoryBusy && (blocker == BlockerPendingOperation || blocker == BlockerMaintenance):
			return true
		}
	}
	return false
}

// storageLocationActionFacts projects a catalog row, its journal, and one
// read of the registered root marker into policy facts.
func (rm *DefaultRepositoryManager) storageLocationActionFacts(ctx context.Context, storageLocation repo.StorageLocation) (StorageLocationActionFacts, error) {
	facts := StorageLocationActionFacts{Kind: storageLocation.Kind, Status: storageLocation.Status}
	if rm.readerDatabase != nil {
		if err := rm.readerDatabase.QueryRowContext(ctx,
			"SELECT count(*) FROM repositories WHERE storage_location_id = ?", storageLocation.StorageLocationID,
		).Scan(&facts.RepositoryCount); err != nil {
			return facts, fmt.Errorf("count Storage Location repositories: %w", err)
		}
		var running int
		if err := rm.readerDatabase.QueryRowContext(ctx, `
			SELECT count(*) FROM lifecycle_operations
			WHERE target_type = 'storage_location' AND target_id = ? AND status = 'running'
		`, storageLocation.StorageLocationID.String()).Scan(&running); err != nil {
			return facts, fmt.Errorf("inspect Storage Location lifecycle operations: %w", err)
		}
		facts.PendingOperation = running != 0
	}
	facts.Original = rm.storageLocationOriginalState(storageLocation.Path, storageLocation.StorageLocationID.String())
	return facts, nil
}

// StorageLocationActionPolicyFor derives the policy for a registered Storage
// Location from its row, its journal, and the marker at its registered path.
func (rm *DefaultRepositoryManager) StorageLocationActionPolicyFor(ctx context.Context, storageLocation repo.StorageLocation) (StorageLocationActionPolicy, error) {
	facts, err := rm.storageLocationActionFacts(ctx, storageLocation)
	if err != nil {
		return StorageLocationActionPolicy{}, err
	}
	return DeriveStorageLocationActions(facts), nil
}

// admitStorageLocationAction refuses a Location command the policy withholds
// on the facts it reads now; commands call it before and under their
// barriers.
func (rm *DefaultRepositoryManager) admitStorageLocationAction(ctx context.Context, storageLocation repo.StorageLocation, action StorageLocationAction) error {
	policy, err := rm.StorageLocationActionPolicyFor(ctx, storageLocation)
	if err != nil {
		return err
	}
	verdict := policy.Verdict(action)
	if verdict.Allowed() {
		return nil
	}
	return &StorageLocationActionUnavailableError{
		StorageLocationID: storageLocation.StorageLocationID.String(), Action: action,
		Blockers: verdict.Blockers, Alternatives: verdict.Alternatives,
	}
}
