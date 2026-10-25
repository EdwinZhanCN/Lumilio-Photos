package storage

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
)

// RepositoryAction is one command an administrator may run against a
// registered Repository. The vocabulary is closed: the Web menu, the native
// host, and the HTTP handlers all consume the same set, and a command refuses
// exactly the actions the policy withholds.
type RepositoryAction string

const (
	// ActionVerify queues an authoritative scan.
	ActionVerify RepositoryAction = "verify"
	// ActionRename changes the display name in the marker and the catalog.
	ActionRename RepositoryAction = "rename"
	// ActionDetach removes the registration and preserves every file.
	ActionDetach RepositoryAction = "detach"
	// ActionReconnect re-points a positively unavailable registration at the
	// tree that carries its identity. It never moves data.
	ActionReconnect RepositoryAction = "reconnect"
)

// RepositoryActions lists every action in a stable order.
var RepositoryActions = []RepositoryAction{ActionVerify, ActionRename, ActionDetach, ActionReconnect}

// RepositoryActionBlocker names why an action is withheld. Codes are stable
// public vocabulary; presenters localize them.
type RepositoryActionBlocker string

const (
	// BlockerPrimaryProtected: the Primary Repository is fixed at
	// <default>/primary and follows the Default Storage Location; it is never
	// renamed, detached, or reconnected on its own.
	BlockerPrimaryProtected RepositoryActionBlocker = "primary_protected"
	// BlockerRepositoryOffline: the original is positively unavailable.
	BlockerRepositoryOffline RepositoryActionBlocker = "repository_offline"
	// BlockerIdentityProblem: the marker at the registered path is missing,
	// corrupt, or belongs to another Repository.
	BlockerIdentityProblem RepositoryActionBlocker = "identity_problem"
	// BlockerRecoveryRequired: an interrupted mutation must be recovered first.
	BlockerRecoveryRequired RepositoryActionBlocker = "recovery_required"
	// BlockerMaintenance: a detach or removal is in progress.
	BlockerMaintenance RepositoryActionBlocker = "maintenance"
	// BlockerRepositoryBusy: scanning, importing, or processing holds the
	// Repository; the action waits for idle.
	BlockerRepositoryBusy RepositoryActionBlocker = "repository_busy"
	// BlockerPendingOperation: a journaled lifecycle operation targeting this
	// Repository is still running.
	BlockerPendingOperation RepositoryActionBlocker = "pending_operation"
	// BlockerOriginalAvailable: the original is online, so there is nothing
	// to reconnect; a copy elsewhere is a separate review.
	BlockerOriginalAvailable RepositoryActionBlocker = "original_available"
	// BlockerOriginalInaccessible: the registered path could not be read
	// (permission denied or an unknown failure). Denial is not absence, so
	// nothing is concluded about the original and no reconnect is admitted
	// on it.
	BlockerOriginalInaccessible RepositoryActionBlocker = "original_inaccessible"
	// BlockerDefaultProtected: the Default Storage Location follows the
	// configured storage.path; it is never reconnected or detached on its own.
	BlockerDefaultProtected RepositoryActionBlocker = "default_protected"
	// BlockerRegisteredRepositories: a Storage Location with registered
	// Repositories cannot be detached.
	BlockerRegisteredRepositories RepositoryActionBlocker = "registered_repositories"
)

// RepositoryActionAlternative names a legal entry the administrator can take
// instead of a withheld action.
type RepositoryActionAlternative string

const (
	// AlternativeDefaultConfiguration: the Primary follows the Default Storage
	// Location; its location changes through the configuration journey.
	AlternativeDefaultConfiguration RepositoryActionAlternative = "default_configuration"
	// AlternativeRecoverOperation: recover the interrupted operation first.
	AlternativeRecoverOperation RepositoryActionAlternative = "recover_operation"
	// AlternativeWaitForIdle: retry once current work finishes.
	AlternativeWaitForIdle RepositoryActionAlternative = "wait_for_idle"
	// AlternativeCheckAccess: fix access to the registered path, then
	// reassess; the catalog and the tree are both left as they are.
	AlternativeCheckAccess RepositoryActionAlternative = "check_access"
)

// RepositoryActionFacts are the catalog facts the action policy reads. They
// are projections already in the storage view; the policy performs no I/O.
type RepositoryActionFacts struct {
	Role         dbtypes.RepoRole
	Reachability dbtypes.RepositoryReachability
	Activity     dbtypes.RepositoryActivity
	// PendingOperation is true while a journaled lifecycle operation that
	// targets this Repository is running.
	PendingOperation bool
	// OriginalInaccessible is set by a command that read the registered path
	// and could not: the original is neither online nor positively absent.
	// Ordinary projections leave it false; they perform no I/O.
	OriginalInaccessible bool
}

// RepositoryActionVerdict is the policy's answer for one action.
type RepositoryActionVerdict struct {
	Action       RepositoryAction
	Blockers     []RepositoryActionBlocker
	Alternatives []RepositoryActionAlternative
}

// Allowed is true when nothing withholds the action.
func (v RepositoryActionVerdict) Allowed() bool { return len(v.Blockers) == 0 }

// RepositoryActionPolicy is the complete verdict set for one Repository.
type RepositoryActionPolicy struct {
	Verdicts []RepositoryActionVerdict
}

// Allowed lists the actions the policy admits, in the stable order.
func (p RepositoryActionPolicy) Allowed() []RepositoryAction {
	var out []RepositoryAction
	for _, verdict := range p.Verdicts {
		if verdict.Allowed() {
			out = append(out, verdict.Action)
		}
	}
	return out
}

// Verdict returns the verdict for one action; an unknown action is withheld.
func (p RepositoryActionPolicy) Verdict(action RepositoryAction) RepositoryActionVerdict {
	for _, verdict := range p.Verdicts {
		if verdict.Action == action {
			return verdict
		}
	}
	return RepositoryActionVerdict{Action: action, Blockers: []RepositoryActionBlocker{BlockerPendingOperation}}
}

// DeriveRepositoryActions is the pure role-aware action policy (#242 plan,
// P3). Commands admit an action only when this policy allows it, re-derived
// under their barriers, so a menu built from the same verdicts never offers
// a command the Server would refuse on the same facts.
func DeriveRepositoryActions(facts RepositoryActionFacts) RepositoryActionPolicy {
	primary := normalizeRepoRole(facts.Role) == dbtypes.RepoRolePrimary
	verdicts := make([]RepositoryActionVerdict, 0, len(RepositoryActions))
	for _, action := range RepositoryActions {
		verdict := RepositoryActionVerdict{Action: action}
		block := func(blocker RepositoryActionBlocker, alternatives ...RepositoryActionAlternative) {
			verdict.Blockers = append(verdict.Blockers, blocker)
			verdict.Alternatives = append(verdict.Alternatives, alternatives...)
		}
		if primary && action != ActionVerify {
			block(BlockerPrimaryProtected, AlternativeDefaultConfiguration)
		}
		switch facts.Reachability {
		case dbtypes.RepositoryReachabilityRecoveryRequired:
			block(BlockerRecoveryRequired, AlternativeRecoverOperation)
		case dbtypes.RepositoryReachabilityMaintenance:
			block(BlockerMaintenance, AlternativeWaitForIdle)
		case dbtypes.RepositoryReachabilityOffline:
			if action == ActionVerify || action == ActionRename {
				block(BlockerRepositoryOffline)
			}
		case dbtypes.RepositoryReachabilityIdentityError:
			if action == ActionVerify || action == ActionRename {
				block(BlockerIdentityProblem)
			}
		default:
			if action == ActionReconnect && !facts.OriginalInaccessible {
				block(BlockerOriginalAvailable)
			}
		}
		if facts.PendingOperation && action != ActionVerify {
			block(BlockerPendingOperation, AlternativeWaitForIdle)
		}
		if facts.OriginalInaccessible && action == ActionReconnect {
			block(BlockerOriginalInaccessible, AlternativeCheckAccess)
		}
		// Detach and reconnect need an idle Repository unless the original is
		// positively unavailable, where stale activity cannot be finished.
		if (action == ActionDetach || action == ActionReconnect) && facts.Activity != dbtypes.RepositoryActivityIdle &&
			!repositoryReachabilityAllowsStaleActivityDetach(facts.Reachability) {
			block(BlockerRepositoryBusy, AlternativeWaitForIdle)
		}
		verdict.Blockers = uniqueBlockers(verdict.Blockers)
		verdict.Alternatives = uniqueAlternatives(verdict.Alternatives)
		verdicts = append(verdicts, verdict)
	}
	return RepositoryActionPolicy{Verdicts: verdicts}
}

func uniqueBlockers(values []RepositoryActionBlocker) []RepositoryActionBlocker {
	seen := make(map[RepositoryActionBlocker]bool, len(values))
	var out []RepositoryActionBlocker
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func uniqueAlternatives(values []RepositoryActionAlternative) []RepositoryActionAlternative {
	seen := make(map[RepositoryActionAlternative]bool, len(values))
	var out []RepositoryActionAlternative
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

// RepositoryActionUnavailableError is the typed refusal a command returns
// when the policy withholds its action on the facts it re-read under its
// barriers. It carries only stable codes, never host paths.
type RepositoryActionUnavailableError struct {
	RepositoryID string
	Action       RepositoryAction
	Blockers     []RepositoryActionBlocker
	Alternatives []RepositoryActionAlternative
}

func (e *RepositoryActionUnavailableError) Error() string {
	codes := make([]string, 0, len(e.Blockers))
	for _, blocker := range e.Blockers {
		codes = append(codes, string(blocker))
	}
	return fmt.Sprintf("repository action %s is unavailable: %s", e.Action, strings.Join(codes, ", "))
}

// Is lets callers that only know the legacy sentinel errors keep matching:
// a protected Primary detach is still ErrPrimaryRepositoryNotRemovable and a
// busy Repository is still ErrRepositoryBusy.
func (e *RepositoryActionUnavailableError) Is(target error) bool {
	for _, blocker := range e.Blockers {
		switch {
		case target == ErrPrimaryRepositoryNotRemovable && blocker == BlockerPrimaryProtected && e.Action == ActionDetach:
			return true
		case target == ErrPathNotAllowed && blocker == BlockerPrimaryProtected && e.Action == ActionReconnect:
			return true
		case target == ErrRepositoryBusy && (blocker == BlockerRepositoryBusy || blocker == BlockerPendingOperation || blocker == BlockerMaintenance):
			return true
		case target == ErrRepositoryOffline && blocker == BlockerRepositoryOffline:
			return true
		}
	}
	return false
}

// BlockerCodes and AlternativeCodes are the wire forms of a verdict.
func BlockerCodes(values []RepositoryActionBlocker) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, string(value))
	}
	return out
}

func AlternativeCodes(values []RepositoryActionAlternative) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, string(value))
	}
	return out
}

// ErrRepositoryOwnershipUnavailable reports that another process holds the
// runtime ownership lock of a Repository tree, so a mutation that needs
// exclusive ownership cannot start.
var ErrRepositoryOwnershipUnavailable = errors.New("repository ownership is held elsewhere")

// LifecycleReceipt is what a journaled command returns: the durable operation
// the request became, whether it was replayed, and its status. A client that
// retries with the same request ID receives the same receipt.
type LifecycleReceipt struct {
	OperationID string
	RequestID   string
	Kind        string
	Status      string
	Replayed    bool
}

func receiptFor(operation repo.LifecycleOperation, replayed bool) LifecycleReceipt {
	return LifecycleReceipt{
		OperationID: operation.OperationID, RequestID: operation.RequestID,
		Kind: operation.Kind, Status: operation.Status, Replayed: replayed,
	}
}

// repositoryActionFacts projects a catalog row and its journal into policy
// facts. It reads the catalog only.
func (rm *DefaultRepositoryManager) repositoryActionFacts(ctx context.Context, repository repo.Repository) (RepositoryActionFacts, error) {
	pending, err := rm.hasRunningLifecycleOperation(ctx, repository.RepoID.String())
	if err != nil {
		return RepositoryActionFacts{}, err
	}
	return RepositoryActionFacts{
		Role: repository.Role, Reachability: repository.Reachability,
		Activity: repository.Activity, PendingOperation: pending,
	}, nil
}

func (rm *DefaultRepositoryManager) hasRunningLifecycleOperation(ctx context.Context, repositoryID string) (bool, error) {
	if rm.readerDatabase == nil {
		return false, nil
	}
	var running int
	if err := rm.readerDatabase.QueryRowContext(ctx, `
		SELECT count(*) FROM lifecycle_operations
		WHERE target_type = 'repository' AND target_id = ? AND status = 'running'
	`, repositoryID).Scan(&running); err != nil {
		return false, fmt.Errorf("inspect repository lifecycle operations: %w", err)
	}
	return running != 0, nil
}

// RepositoryActionPolicyFor derives the policy for a registered Repository
// from current catalog facts; the storage view publishes it per row.
func (rm *DefaultRepositoryManager) RepositoryActionPolicyFor(ctx context.Context, repository repo.Repository) (RepositoryActionPolicy, error) {
	facts, err := rm.repositoryActionFacts(ctx, repository)
	if err != nil {
		return RepositoryActionPolicy{}, err
	}
	return DeriveRepositoryActions(facts), nil
}

// admitRepositoryAction re-derives the policy from the row the caller holds
// under its barrier and refuses with the typed error when the action is
// withheld. Callers pass the row they re-read after acquiring the lease.
func (rm *DefaultRepositoryManager) admitRepositoryAction(ctx context.Context, repository repo.Repository, action RepositoryAction) error {
	policy, err := rm.RepositoryActionPolicyFor(ctx, repository)
	if err != nil {
		return err
	}
	verdict := policy.Verdict(action)
	if verdict.Allowed() {
		return nil
	}
	return &RepositoryActionUnavailableError{
		RepositoryID: repository.RepoID.String(), Action: action,
		Blockers: verdict.Blockers, Alternatives: verdict.Alternatives,
	}
}

// sortedActions returns the wire form of the allowed actions.
func sortedActions(actions []RepositoryAction) []string {
	out := make([]string, 0, len(actions))
	for _, action := range actions {
		out = append(out, string(action))
	}
	sort.Strings(out)
	return out
}
