package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"server/internal/db/dbtypes"

	"github.com/google/uuid"
)

type HostActionKind string
type HostActionStatus string

var inspectHostActionStoragePath = InspectStoragePath

const (
	HostActionAuthorizeStorageLocation HostActionKind = "authorize_storage_location"
	HostActionOpenRepository           HostActionKind = "open_repository"
	HostActionLocateStorageLocation    HostActionKind = "locate_storage_location"
	HostActionLocateRepository         HostActionKind = "locate_repository"

	HostActionPending       HostActionStatus = "pending"
	HostActionRunning       HostActionStatus = "running"
	HostActionNeedsDecision HostActionStatus = "needs_decision"
	HostActionSucceeded     HostActionStatus = "succeeded"
	HostActionFailed        HostActionStatus = "failed"
	HostActionCancelled     HostActionStatus = "cancelled"
	HostActionExpired       HostActionStatus = "expired"
)

var (
	ErrHostActionConflict       = errors.New("host action request ID was reused with a different request")
	ErrHostActionNotPending     = errors.New("host action is not pending")
	ErrHostActionNonceInvalid   = errors.New("host action nonce is invalid")
	ErrHostActionDecisionNeeded = errors.New("host action requires a recovery decision")
)

type HostActionSummary struct {
	Name         string `json:"name,omitempty"`
	Purpose      string `json:"purpose,omitempty"`
	RootID       string `json:"root_id,omitempty"`
	RepositoryID string `json:"repository_id,omitempty"`
}

type HostActionConflict struct {
	Type           string   `json:"type"`
	RepositoryID   string   `json:"repository_id,omitempty"`
	RootID         string   `json:"root_id,omitempty"`
	RegisteredPath string   `json:"registered_path,omitempty"`
	RequestedPath  string   `json:"requested_path,omitempty"`
	Actions        []string `json:"actions,omitempty"`
	RiskWarnings   []string `json:"risk_warnings,omitempty"`
}

type HostActionResult struct {
	RepositoryID string              `json:"repository_id,omitempty"`
	RootID       string              `json:"root_id,omitempty"`
	Name         string              `json:"name,omitempty"`
	Conflict     *HostActionConflict `json:"conflict,omitempty"`
}

type HostAction struct {
	ActionID        string
	RequestID       string
	Kind            HostActionKind
	Actor           string
	ActorUserID     *int32
	HostInstanceID  string
	SessionID       string
	Summary         HostActionSummary
	ExpectedVersion uint64
	Status          HostActionStatus
	Result          *HostActionResult
	ErrorCode       string
	ErrorMessage    string
	ExpiresAt       time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CompletedAt     *time.Time

	// These fields never cross the shared HTTP contract. They are consumed only
	// by the in-process native-host control plane.
	nonce        string
	selectedPath string
	requestHash  string
}

type CreateHostActionInput struct {
	RequestID       string
	Kind            HostActionKind
	Actor           string
	ActorUserID     *int32
	SessionID       string
	Summary         HostActionSummary
	ExpectedVersion uint64
	TTL             time.Duration
}

// NativeHostNonce is intentionally available only to the in-process control
// adapter; HTTP DTOs never call it or serialize the backing field.
func (a HostAction) NativeHostNonce() string { return a.nonce }

func (rm *DefaultRepositoryManager) CreateHostAction(ctx context.Context, input CreateHostActionInput) (HostAction, error) {
	if err := validateHostActionInput(input); err != nil {
		return HostAction{}, err
	}
	requestID := strings.TrimSpace(input.RequestID)
	if requestID == "" {
		requestID = uuid.NewString()
	}
	input.RequestID = requestID
	requestBytes, err := json.Marshal(struct {
		Kind            HostActionKind    `json:"kind"`
		Summary         HostActionSummary `json:"summary"`
		ExpectedVersion uint64            `json:"expected_version"`
		SessionID       string            `json:"session_id"`
	}{input.Kind, input.Summary, input.ExpectedVersion, input.SessionID})
	if err != nil {
		return HostAction{}, fmt.Errorf("encode host action request: %w", err)
	}
	digest := sha256.Sum256(requestBytes)
	requestHash := hex.EncodeToString(digest[:])
	if existing, err := rm.getHostActionByRequestID(ctx, requestID); err == nil {
		if existing.requestHash != requestHash {
			return HostAction{}, ErrHostActionConflict
		}
		return existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return HostAction{}, err
	}

	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return HostAction{}, fmt.Errorf("generate host action nonce: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes)
	summary, err := json.Marshal(input.Summary)
	if err != nil {
		return HostAction{}, fmt.Errorf("encode host action summary: %w", err)
	}
	ttl := input.TTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	if ttl > 30*time.Minute {
		ttl = 30 * time.Minute
	}
	now := time.Now().UTC()
	actionID := uuid.NewString()
	actor := strings.TrimSpace(input.Actor)
	if actor == "" {
		actor = "web:admin"
	}
	_, err = rm.database.ExecContext(ctx, `
		INSERT INTO host_actions (
			action_id, request_id, request_hash, kind, actor, actor_user_id,
			session_id, request_summary, expected_version, nonce, status,
			expires_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?, ?)
	`, actionID, requestID, requestHash, input.Kind, actor, input.ActorUserID,
		strings.TrimSpace(input.SessionID), string(summary), input.ExpectedVersion, nonce,
		dbtypes.NewTimestamp(now.Add(ttl)), dbtypes.NewTimestamp(now), dbtypes.NewTimestamp(now))
	if err != nil {
		if existing, readErr := rm.getHostActionByRequestID(ctx, requestID); readErr == nil {
			if existing.requestHash != requestHash {
				return HostAction{}, ErrHostActionConflict
			}
			return existing, nil
		}
		return HostAction{}, fmt.Errorf("create host action: %w", err)
	}
	return rm.GetHostAction(ctx, actionID)
}

func validateHostActionInput(input CreateHostActionInput) error {
	switch input.Kind {
	case HostActionAuthorizeStorageLocation, HostActionOpenRepository:
	case HostActionLocateStorageLocation:
		if _, err := uuid.Parse(strings.TrimSpace(input.Summary.RootID)); err != nil {
			return fmt.Errorf("locate Storage Location requires a valid root_id: %w", err)
		}
	case HostActionLocateRepository:
		if _, err := uuid.Parse(strings.TrimSpace(input.Summary.RepositoryID)); err != nil {
			return fmt.Errorf("locate repository requires a valid repository_id: %w", err)
		}
	default:
		return fmt.Errorf("unsupported host action kind %q", input.Kind)
	}
	return nil
}

func (rm *DefaultRepositoryManager) GetHostAction(ctx context.Context, id string) (HostAction, error) {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return HostAction{}, fmt.Errorf("invalid host action id: %w", err)
	}
	return rm.scanHostAction(rm.database.QueryRowContext(ctx, hostActionSelect+" WHERE action_id = ?", id))
}

func (rm *DefaultRepositoryManager) getHostActionByRequestID(ctx context.Context, requestID string) (HostAction, error) {
	return rm.scanHostAction(rm.database.QueryRowContext(ctx, hostActionSelect+" WHERE request_id = ?", requestID))
}

func (rm *DefaultRepositoryManager) ListPendingHostActions(ctx context.Context) ([]HostAction, error) {
	if err := rm.expirePendingHostActions(ctx); err != nil {
		return nil, err
	}
	rows, err := rm.database.QueryContext(ctx, hostActionSelect+" WHERE status IN ('pending', 'needs_decision') ORDER BY created_at")
	if err != nil {
		return nil, fmt.Errorf("list pending host actions: %w", err)
	}
	defer rows.Close()
	var actions []HostAction
	for rows.Next() {
		action, err := rm.scanHostAction(rows)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	return actions, rows.Err()
}

func (rm *DefaultRepositoryManager) expirePendingHostActions(ctx context.Context) error {
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := rm.database.ExecContext(ctx, `
		UPDATE host_actions
		SET status = 'expired', error_code = 'expired',
			error_message = 'Native host approval expired', updated_at = ?, completed_at = ?
		WHERE status IN ('pending', 'needs_decision') AND expires_at <= ?
	`, now, now, now); err != nil {
		return fmt.Errorf("expire host actions: %w", err)
	}
	return nil
}

// SetHostActionExpectedVersion binds a pending request to the Desktop storage
// snapshot on which it was presented. Only the native control plane has the
// nonce needed to perform this one-time compare-and-set.
func (rm *DefaultRepositoryManager) SetHostActionExpectedVersion(ctx context.Context, actionID, nonce string, version uint64) (HostAction, error) {
	if version == 0 {
		return HostAction{}, errors.New("expected storage version must be non-zero")
	}
	action, err := rm.GetHostAction(ctx, actionID)
	if err != nil {
		return HostAction{}, err
	}
	if subtle.ConstantTimeCompare([]byte(action.nonce), []byte(strings.TrimSpace(nonce))) != 1 {
		return HostAction{}, ErrHostActionNonceInvalid
	}
	if action.Status != HostActionPending {
		return HostAction{}, ErrHostActionNotPending
	}
	if action.ExpectedVersion != 0 {
		return action, nil
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	result, err := rm.database.ExecContext(ctx, `
		UPDATE host_actions SET expected_version = ?, updated_at = ?
		WHERE action_id = ? AND status = 'pending' AND expected_version = 0
	`, version, now, actionID)
	if err != nil {
		return HostAction{}, fmt.Errorf("bind host action expected version: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return rm.GetHostAction(ctx, actionID)
	}
	return rm.GetHostAction(ctx, actionID)
}

// RecoverHostActions makes a native operation that was interrupted after it
// was claimed available for approval again. The filesystem/catalog mutations
// reached by ExecuteHostAction are independently idempotent or validate their
// already-applied result, so replay is safer than leaving an eternal `running`
// row that neither Web nor Desktop can finish.
func (rm *DefaultRepositoryManager) RecoverHostActions(ctx context.Context) error {
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := rm.database.ExecContext(ctx, `
		UPDATE host_actions
		SET status = 'pending', host_instance_id = '', selected_path = NULL,
			error_code = 'interrupted',
			error_message = 'The previous Desktop execution was interrupted; approve the task again',
			updated_at = ?
		WHERE status = 'running'
	`, now); err != nil {
		return fmt.Errorf("recover interrupted host actions: %w", err)
	}
	return nil
}

// ListHostActionsForActor returns durable unfinished work owned by one Web
// actor. It is intentionally actor-scoped so a refreshed browser can recover
// its tasks without exposing another administrator's workflow.
func (rm *DefaultRepositoryManager) ListHostActionsForActor(ctx context.Context, actorUserID int32) ([]HostAction, error) {
	if err := rm.expirePendingHostActions(ctx); err != nil {
		return nil, err
	}
	rows, err := rm.database.QueryContext(ctx, hostActionSelect+`
		WHERE actor_user_id = ? AND status IN ('pending', 'running', 'needs_decision')
		ORDER BY created_at DESC`, actorUserID)
	if err != nil {
		return nil, fmt.Errorf("list actor host actions: %w", err)
	}
	defer rows.Close()
	var actions []HostAction
	for rows.Next() {
		action, scanErr := rm.scanHostAction(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		actions = append(actions, action)
	}
	return actions, rows.Err()
}

func (rm *DefaultRepositoryManager) ExecuteHostAction(ctx context.Context, actionID, nonce, hostInstanceID, selectedPath string, riskConfirmation ...bool) (HostAction, error) {
	action, err := rm.GetHostAction(ctx, actionID)
	if err != nil {
		return HostAction{}, err
	}
	if subtle.ConstantTimeCompare([]byte(action.nonce), []byte(strings.TrimSpace(nonce))) != 1 {
		return HostAction{}, ErrHostActionNonceInvalid
	}
	confirmedRisk := len(riskConfirmation) > 0 && riskConfirmation[0]
	resumingRiskDecision := action.Status == HostActionNeedsDecision && action.Result != nil &&
		action.Result.Conflict != nil && action.Result.Conflict.Type == "storage_risk"
	if action.Status != HostActionPending && !resumingRiskDecision {
		return HostAction{}, ErrHostActionNotPending
	}
	if resumingRiskDecision {
		if !confirmedRisk {
			return HostAction{}, ErrHostActionDecisionNeeded
		}
		selectedPath = action.selectedPath
	}
	if !time.Now().UTC().Before(action.ExpiresAt) {
		_ = rm.finishHostAction(ctx, actionID, HostActionExpired, nil, "expired", "Native host approval expired")
		return HostAction{}, ErrHostActionNotPending
	}
	cleanPath, err := CanonicalizeRepositoryPath(selectedPath)
	if err != nil {
		return rm.failHostAction(ctx, actionID, "invalid_path", "The selected directory is unavailable", err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	expectedStatus := HostActionPending
	if resumingRiskDecision {
		expectedStatus = HostActionNeedsDecision
	}
	result, err := rm.database.ExecContext(ctx, `
		UPDATE host_actions SET status = 'running', host_instance_id = ?, selected_path = ?, updated_at = ?
		WHERE action_id = ? AND status = ?
	`, strings.TrimSpace(hostInstanceID), cleanPath, now, actionID, expectedStatus)
	if err != nil {
		return HostAction{}, fmt.Errorf("claim host action: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return HostAction{}, ErrHostActionNotPending
	}
	warnings := rm.hostActionRiskWarnings(ctx, action, cleanPath)
	if len(warnings) > 0 && !confirmedRisk {
		return rm.storeHostActionConflict(ctx, action, cleanPath, HostActionConflict{
			Type: "storage_risk", RequestedPath: cleanPath,
			Actions: []string{"confirm_risk"}, RiskWarnings: warnings,
		}, ErrRepositoryRiskConfirmationRequired)
	}

	var actionResult HostActionResult
	switch action.Kind {
	case HostActionAuthorizeStorageLocation:
		root, callErr := rm.AddRepositoryRoot(ctx, cleanPath, action.Summary.Name,
			LifecycleRequest{RequestID: "host-action:" + action.ActionID, Actor: "desktop_host:" + hostInstanceID, ActorUserID: action.ActorUserID, HostInstanceID: hostInstanceID, ConfirmationType: hostActionConfirmationType(confirmedRisk, "native_directory_selection"), RiskConfirmation: confirmedRisk})
		if callErr != nil {
			var conflict *RepositoryRootConflictError
			if errors.As(callErr, &conflict) {
				return rm.storeHostActionConflict(ctx, action, cleanPath, HostActionConflict{
					Type: "storage_location_identity", RootID: conflict.RootID,
					RegisteredPath: conflict.RegisteredPath, RequestedPath: conflict.RequestedPath,
					Actions: conflict.Actions,
				}, callErr)
			}
			return rm.failHostAction(ctx, actionID, "storage_location_failed", "Storage Location could not be added", callErr)
		}
		actionResult = HostActionResult{RootID: root.RootID.String(), Name: root.Name}
	case HostActionOpenRepository:
		ownerID, callErr := rm.HostOwnerID(ctx)
		if callErr == nil && ownerID == nil {
			callErr = errors.New("Host Owner is unavailable")
		}
		var repositoryID string
		if callErr == nil {
			repository, addErr := rm.OpenRepository(ctx, cleanPath, ownerID, dbtypes.RepoRoleRegular,
				LifecycleRequest{RequestID: "host-action-open:" + action.ActionID, Actor: action.Actor, ActorUserID: action.ActorUserID, HostInstanceID: hostInstanceID, ConfirmationType: hostActionConfirmationType(confirmedRisk, "native_directory_selection"), RiskConfirmation: confirmedRisk})
			callErr = addErr
			if repository != nil {
				repositoryID = repository.RepoID.String()
				actionResult = HostActionResult{RepositoryID: repositoryID, Name: repository.Name}
			}
		}
		if callErr != nil {
			var conflict *RepositoryConflictError
			if errors.As(callErr, &conflict) {
				return rm.storeHostActionConflict(ctx, action, cleanPath, HostActionConflict{
					Type: "repository_identity", RepositoryID: conflict.RepositoryID,
					RegisteredPath: conflict.RegisteredPath, RequestedPath: conflict.RequestedPath,
					Actions: conflict.Actions,
				}, callErr)
			}
			return rm.failHostAction(ctx, actionID, "open_repository_failed", "Repository could not be opened", callErr)
		}
	case HostActionLocateStorageLocation:
		root, callErr := rm.RelocateRepositoryRoot(ctx, action.Summary.RootID, cleanPath, LifecycleRequest{
			RequestID: "host-action-relocate-root:" + action.ActionID, Actor: action.Actor,
			ActorUserID: action.ActorUserID, HostInstanceID: hostInstanceID,
			ConfirmationType: hostActionConfirmationType(confirmedRisk, "native_directory_selection"), RiskConfirmation: confirmedRisk,
		})
		if callErr != nil {
			return rm.failHostAction(ctx, actionID, "locate_storage_location_failed", "Storage Location could not be reconnected", callErr)
		}
		actionResult = HostActionResult{RootID: root.RootID.String(), Name: root.Name}
	case HostActionLocateRepository:
		repository, callErr := rm.RelocateRepository(ctx, action.Summary.RepositoryID, cleanPath, LifecycleRequest{
			RequestID: "host-action-relocate:" + action.ActionID, Actor: action.Actor,
			ActorUserID: action.ActorUserID, HostInstanceID: hostInstanceID,
			ConfirmationType: hostActionConfirmationType(confirmedRisk, "native_directory_selection"), RiskConfirmation: confirmedRisk,
		})
		if callErr != nil {
			return rm.failHostAction(ctx, actionID, "locate_repository_failed", "Repository location could not be updated", callErr)
		}
		actionResult = HostActionResult{RepositoryID: repository.RepoID.String(), Name: repository.Name}
	}
	if err := rm.finishHostAction(ctx, actionID, HostActionSucceeded, &actionResult, "", ""); err != nil {
		return HostAction{}, err
	}
	return rm.GetHostAction(ctx, actionID)
}

func (rm *DefaultRepositoryManager) ResolveHostAction(ctx context.Context, actionID, resolution string, riskConfirmation ...bool) (HostAction, error) {
	action, err := rm.GetHostAction(ctx, actionID)
	if err != nil {
		return HostAction{}, err
	}
	if action.Status != HostActionNeedsDecision || action.Result == nil || action.Result.Conflict == nil {
		return HostAction{}, ErrHostActionDecisionNeeded
	}
	conflict := action.Result.Conflict
	if conflict.Type == "storage_risk" {
		confirmed := len(riskConfirmation) > 0 && riskConfirmation[0]
		if strings.TrimSpace(resolution) != "confirm_risk" || !confirmed {
			return HostAction{}, ErrRepositoryRiskConfirmationRequired
		}
		return rm.ExecuteHostAction(ctx, actionID, action.nonce, action.HostInstanceID, action.selectedPath, true)
	}
	protocolAction := ""
	switch strings.TrimSpace(resolution) {
	case "update_location":
		protocolAction = "relocate"
	case "add_separate":
		protocolAction = "copy"
	default:
		return HostAction{}, fmt.Errorf("unsupported host action resolution %q", resolution)
	}
	if !containsString(conflict.Actions, protocolAction) {
		return HostAction{}, fmt.Errorf("resolution %q is not allowed for this conflict", resolution)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	result, err := rm.database.ExecContext(ctx, `
		UPDATE host_actions SET status = 'running', updated_at = ?
		WHERE action_id = ? AND status = 'needs_decision'
	`, now, actionID)
	if err != nil {
		return HostAction{}, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return HostAction{}, ErrHostActionDecisionNeeded
	}

	var actionResult HostActionResult
	if conflict.Type == "storage_location_identity" {
		if protocolAction != "relocate" {
			return rm.failHostAction(ctx, actionID, "unsupported_resolution", "A copied Storage Location cannot be registered automatically", nil)
		}
		root, callErr := rm.RelocateRepositoryRoot(ctx, conflict.RootID, action.selectedPath, LifecycleRequest{
			RequestID: "host-action-resolve-relocate-root:" + action.ActionID, Actor: action.Actor,
			ActorUserID: action.ActorUserID, HostInstanceID: action.HostInstanceID,
			ConfirmationType: "update_location",
		})
		if callErr != nil {
			return rm.failHostAction(ctx, actionID, "resolution_failed", "Storage Location location could not be updated", callErr)
		}
		actionResult = HostActionResult{RootID: root.RootID.String(), Name: root.Name}
	} else {
		var repositoryID string
		if protocolAction == "relocate" {
			repository, callErr := rm.RelocateRepository(ctx, conflict.RepositoryID, action.selectedPath, LifecycleRequest{
				RequestID: "host-action-resolve-relocate:" + action.ActionID, Actor: action.Actor,
				ActorUserID: action.ActorUserID, HostInstanceID: action.HostInstanceID,
				ConfirmationType: "update_location",
			})
			if callErr != nil {
				return rm.failHostAction(ctx, actionID, "resolution_failed", "Repository location could not be updated", callErr)
			}
			repositoryID = repository.RepoID.String()
			actionResult = HostActionResult{RepositoryID: repositoryID, Name: repository.Name}
		} else {
			ownerID, callErr := rm.HostOwnerID(ctx)
			if callErr == nil && ownerID == nil {
				callErr = errors.New("Host Owner is unavailable")
			}
			var repositoryName string
			if callErr == nil {
				repository, copyErr := rm.RegisterRepositoryCopy(ctx, action.selectedPath, ownerID, dbtypes.RepoRoleRegular,
					LifecycleRequest{RequestID: "host-action-resolve:" + actionID, Actor: action.Actor,
						ActorUserID: action.ActorUserID, HostInstanceID: action.HostInstanceID,
						ConfirmationType: "independent_identity", RiskConfirmation: true})
				callErr = copyErr
				if repository != nil {
					repositoryID = repository.RepoID.String()
					repositoryName = repository.Name
				}
			}
			if callErr != nil {
				return rm.failHostAction(ctx, actionID, "resolution_failed", "Repository copy could not be added separately", callErr)
			}
			actionResult = HostActionResult{RepositoryID: repositoryID, Name: repositoryName}
		}
	}
	if err := rm.finishHostAction(ctx, actionID, HostActionSucceeded, &actionResult, "", ""); err != nil {
		return HostAction{}, err
	}
	return rm.GetHostAction(ctx, actionID)
}

func hostActionConfirmationType(riskConfirmed bool, fallback string) string {
	if riskConfirmed {
		return "storage_risk_confirmed"
	}
	return fallback
}

func (rm *DefaultRepositoryManager) hostActionRiskWarnings(ctx context.Context, action HostAction, selectedPath string) []string {
	info := inspectHostActionStoragePath(selectedPath)
	warnings := append([]string(nil), info.RiskWarnings...)
	switch action.Kind {
	case HostActionOpenRepository:
		if rootID, err := rm.repositoryRootIDForPath(ctx, selectedPath); err == nil {
			if root, rootErr := rm.queries.GetRepositoryRoot(ctx, rootID); rootErr == nil {
				warnings = repositoryCandidateRiskWarnings(root, selectedPath, info)
			}
		}
	case HostActionLocateRepository:
		if repositoryID, err := uuid.Parse(strings.TrimSpace(action.Summary.RepositoryID)); err == nil {
			if repository, repositoryErr := rm.queries.GetRepository(ctx, repositoryID); repositoryErr == nil {
				if root, rootErr := rm.queries.GetRepositoryRoot(ctx, repository.RootID); rootErr == nil {
					warnings = repositoryCandidateRiskWarnings(root, selectedPath, info)
				}
			}
		}
	case HostActionLocateStorageLocation:
		if rootID, err := uuid.Parse(strings.TrimSpace(action.Summary.RootID)); err == nil {
			if root, rootErr := rm.queries.GetRepositoryRoot(ctx, rootID); rootErr == nil &&
				root.MountFingerprint != "" && info.MountFingerprint != "" && root.MountFingerprint != info.MountFingerprint {
				warnings = append(warnings, "mount_fingerprint_changed")
			}
		}
	}
	return uniqueStrings(warnings)
}

func (rm *DefaultRepositoryManager) CancelHostAction(ctx context.Context, actionID string) (HostAction, error) {
	now := dbtypes.NewTimestamp(time.Now().UTC())
	result, err := rm.database.ExecContext(ctx, `
		UPDATE host_actions
		SET status = 'cancelled', error_code = 'cancelled', error_message = 'Cancelled by administrator',
			updated_at = ?, completed_at = ?
		WHERE action_id = ? AND status IN ('pending', 'needs_decision')
	`, now, now, actionID)
	if err != nil {
		return HostAction{}, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return HostAction{}, ErrHostActionNotPending
	}
	return rm.GetHostAction(ctx, actionID)
}

func (rm *DefaultRepositoryManager) storeHostActionConflict(ctx context.Context, action HostAction, selectedPath string, conflict HostActionConflict, cause error) (HostAction, error) {
	result := HostActionResult{Conflict: &conflict}
	if len(conflict.Actions) == 0 {
		return rm.failHostAction(ctx, action.ActionID, "duplicate_identity_online", "The original and selected copy are both online", cause)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return HostAction{}, err
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	_, err = rm.database.ExecContext(ctx, `
		UPDATE host_actions
		SET status = 'needs_decision', selected_path = ?, result = ?,
			error_code = 'identity_conflict', error_message = ?, updated_at = ?
		WHERE action_id = ?
	`, selectedPath, string(encoded), "A repository or Storage Location identity is already registered", now, action.ActionID)
	if err != nil {
		return HostAction{}, err
	}
	return rm.GetHostAction(ctx, action.ActionID)
}

func (rm *DefaultRepositoryManager) failHostAction(ctx context.Context, actionID, code, message string, cause error) (HostAction, error) {
	if cause != nil && strings.TrimSpace(message) == "" {
		message = cause.Error()
	}
	if err := rm.finishHostAction(ctx, actionID, HostActionFailed, nil, code, message); err != nil {
		return HostAction{}, err
	}
	action, err := rm.GetHostAction(ctx, actionID)
	if err != nil {
		return HostAction{}, err
	}
	if cause != nil {
		return action, cause
	}
	return action, errors.New(message)
}

func (rm *DefaultRepositoryManager) finishHostAction(ctx context.Context, actionID string, status HostActionStatus, result *HostActionResult, code, message string) error {
	var encoded any
	if result != nil {
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		encoded = string(data)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	tx, err := rm.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin host action completion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	action, err := rm.scanHostAction(tx.QueryRowContext(ctx, hostActionSelect+" WHERE action_id = ?", actionID))
	if err != nil {
		return err
	}
	updateResult, err := tx.ExecContext(ctx, `
		UPDATE host_actions
		SET status = ?, result = ?, error_code = NULLIF(?, ''), error_message = NULLIF(?, ''),
			updated_at = ?, completed_at = ?
		WHERE action_id = ? AND status NOT IN ('succeeded', 'failed', 'cancelled', 'expired')
	`, status, encoded, code, message, now, now, actionID)
	if err != nil {
		return err
	}
	if changed, _ := updateResult.RowsAffected(); changed == 0 {
		return tx.Commit()
	}
	if _, err := recordLifecycleAuditWithQueries(ctx, rm.queries.WithTx(tx), hostActionAuditInput(action, status, result, code, message)); err != nil {
		return err
	}
	return tx.Commit()
}

func hostActionAuditInput(action HostAction, status HostActionStatus, result *HostActionResult, code, message string) LifecycleAuditInput {
	targetType, targetID := "runtime_config", ""
	switch action.Kind {
	case HostActionAuthorizeStorageLocation, HostActionLocateStorageLocation:
		targetType, targetID = "storage_location", action.Summary.RootID
	case HostActionOpenRepository, HostActionLocateRepository:
		targetType, targetID = "repository", action.Summary.RepositoryID
	}
	if result != nil {
		if result.RootID != "" {
			targetType, targetID = "storage_location", result.RootID
		}
		if result.RepositoryID != "" {
			targetType, targetID = "repository", result.RepositoryID
		}
	}
	confirmation := "native_directory_selection"
	oldPath := ""
	if action.Result != nil && action.Result.Conflict != nil {
		oldPath = action.Result.Conflict.RegisteredPath
		confirmation = "update_location"
		if action.Result.Conflict.Type == "storage_risk" {
			confirmation = "storage_risk_confirmed"
		} else if result != nil && action.Result.Conflict.RepositoryID != "" && result.RepositoryID != action.Result.Conflict.RepositoryID {
			confirmation = "independent_identity"
		}
	}
	auditResult := AuditResultRejected
	if status == HostActionSucceeded {
		auditResult = AuditResultSucceeded
	} else if status == HostActionFailed {
		auditResult = AuditResultFailed
	}
	return LifecycleAuditInput{
		Actor: action.Actor, ActorUserID: action.ActorUserID, HostInstanceID: action.HostInstanceID,
		RequestID: action.RequestID, OperationID: action.ActionID, Action: string(action.Kind),
		TargetType: targetType, TargetID: targetID, Source: "desktop_host", ConfirmationType: confirmation,
		OldPath: oldPath, NewPath: action.selectedPath, Result: auditResult, FailureStage: code,
		Details: map[string]any{"host_action_status": status, "message": message, "expected_version": action.ExpectedVersion},
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

const hostActionSelect = `
	SELECT action_id, request_id, request_hash, kind, actor, actor_user_id,
		host_instance_id, session_id, request_summary, expected_version,
		nonce, status, selected_path, result, error_code, error_message,
		expires_at, created_at, updated_at, completed_at
	FROM host_actions`

type hostActionScanner interface {
	Scan(...any) error
}

func (rm *DefaultRepositoryManager) scanHostAction(row hostActionScanner) (HostAction, error) {
	var action HostAction
	var actorUserID sql.NullInt64
	var expectedVersion int64
	var summary string
	var selectedPath, result, errorCode, errorMessage sql.NullString
	var expiresAt, createdAt, updatedAt dbtypes.Timestamp
	var completedAt dbtypes.Timestamp
	if err := row.Scan(
		&action.ActionID, &action.RequestID, &action.requestHash, &action.Kind, &action.Actor, &actorUserID,
		&action.HostInstanceID, &action.SessionID, &summary, &expectedVersion,
		&action.nonce, &action.Status, &selectedPath, &result, &errorCode, &errorMessage,
		&expiresAt, &createdAt, &updatedAt, &completedAt,
	); err != nil {
		return HostAction{}, err
	}
	if actorUserID.Valid {
		value := int32(actorUserID.Int64)
		action.ActorUserID = &value
	}
	if expectedVersion >= 0 {
		action.ExpectedVersion = uint64(expectedVersion)
	}
	if err := json.Unmarshal([]byte(summary), &action.Summary); err != nil {
		return HostAction{}, fmt.Errorf("decode host action summary: %w", err)
	}
	if selectedPath.Valid {
		action.selectedPath = selectedPath.String
	}
	if result.Valid {
		var decoded HostActionResult
		if err := json.Unmarshal([]byte(result.String), &decoded); err != nil {
			return HostAction{}, fmt.Errorf("decode host action result: %w", err)
		}
		action.Result = &decoded
	}
	action.ErrorCode = errorCode.String
	action.ErrorMessage = errorMessage.String
	action.ExpiresAt = expiresAt.Time
	action.CreatedAt = createdAt.Time
	action.UpdatedAt = updatedAt.Time
	if completedAt.Valid {
		value := completedAt.Time
		action.CompletedAt = &value
	}
	return action, nil
}
