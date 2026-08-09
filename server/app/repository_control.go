package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"

	"github.com/google/uuid"
)

// RepositoryControl is the narrow in-process storage control plane exposed to
// the Desktop host after a native filesystem grant. It is not routed over the
// shared HTTP API.
type RepositoryControl interface {
	ListStorageLocations(ctx context.Context) ([]StorageLocationInfo, error)
	AddStorageLocation(ctx context.Context, requestID, path, name string) (StorageLocationInfo, []string, error)
	ResolveStorageLocationConflict(ctx context.Context, rootID, path string) (StorageLocationInfo, error)
	RemoveStorageLocation(ctx context.Context, id string) error
	AttachRepository(ctx context.Context, path string) (RepositoryInfo, error)
	ResolveRepositoryConflict(ctx context.Context, action, repositoryID, path string, requestID ...string) (RepositoryInfo, error)
	ListPendingHostActions(ctx context.Context) ([]HostActionInfo, error)
	SetHostActionExpectedVersion(ctx context.Context, actionID, nonce string, version uint64) (HostActionInfo, error)
	ExecuteHostAction(ctx context.Context, actionID, nonce, hostInstanceID, selectedPath string, riskConfirmation bool) (HostActionInfo, error)
	CancelHostAction(ctx context.Context, actionID string) (HostActionInfo, error)
}

type StorageLocationInfo struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Path                 string `json:"path"`
	Kind                 string `json:"kind"`
	Status               string `json:"status"`
	RepositoryCount      int64  `json:"repositoryCount"`
	ActiveOperationCount int64  `json:"activeOperationCount"`
	CanRemove            bool   `json:"canRemove"`
	RemovalBlockedBy     string `json:"removalBlockedBy,omitempty"`
	FilesPreserved       bool   `json:"filesPreserved"`
	Writable             bool   `json:"writable"`
	CapacityKnown        bool   `json:"capacityKnown"`
	TotalBytes           uint64 `json:"totalBytes,omitempty"`
	AvailableBytes       uint64 `json:"availableBytes,omitempty"`
	Filesystem           string `json:"filesystem,omitempty"`
}

type RepositoryInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	Reachability string `json:"reachability"`
	Activity     string `json:"activity"`
}

type HostActionInfo struct {
	ID              string    `json:"id"`
	RequestID       string    `json:"requestId"`
	Kind            string    `json:"kind"`
	Actor           string    `json:"actor"`
	Purpose         string    `json:"purpose"`
	Name            string    `json:"name"`
	ExpectedVersion uint64    `json:"expectedVersion"`
	Nonce           string    `json:"nonce"`
	Status          string    `json:"status"`
	RiskWarnings    []string  `json:"riskWarnings,omitempty"`
	ExpiresAt       time.Time `json:"expiresAt"`
}

type RepositoryIdentityConflict struct {
	RepositoryID   string   `json:"repositoryId"`
	RegisteredPath string   `json:"registeredPath"`
	RequestedPath  string   `json:"requestedPath"`
	Actions        []string `json:"actions"`
}

type StorageLocationIdentityConflict struct {
	RootID         string   `json:"rootId"`
	RegisteredPath string   `json:"registeredPath"`
	RequestedPath  string   `json:"requestedPath"`
	Actions        []string `json:"actions"`
}

func (e *StorageLocationIdentityConflict) Error() string {
	return fmt.Sprintf("storage location %s is already registered at %s", e.RootID, e.RegisteredPath)
}

func (e *RepositoryIdentityConflict) Error() string {
	return fmt.Sprintf("repository %s is already registered at %s", e.RepositoryID, e.RegisteredPath)
}

type repositoryControl struct{ manager storage.RepositoryManager }

var ErrHostOwnerUnavailable = errors.New("Host Owner unavailable; complete initial administrator setup before attaching a repository")

func newRepositoryControl(manager storage.RepositoryManager) RepositoryControl {
	return &repositoryControl{manager: manager}
}

func (c *repositoryControl) ListStorageLocations(ctx context.Context) ([]StorageLocationInfo, error) {
	roots, err := c.manager.ListRepositoryRoots(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]StorageLocationInfo, 0, len(roots))
	for _, root := range roots {
		impact, err := c.manager.PreviewRepositoryRootRemoval(ctx, root.RootID.String())
		if err != nil {
			return nil, err
		}
		items = append(items, storageLocationInfo(root, impact))
	}
	return items, nil
}

func (c *repositoryControl) AddStorageLocation(ctx context.Context, requestID, path, name string) (StorageLocationInfo, []string, error) {
	root, err := c.manager.AddRepositoryRoot(ctx, path, name, desktopLifecycleRequest(requestID, "native_directory_selection", false))
	if err != nil {
		var conflict *storage.RepositoryRootConflictError
		if errors.As(err, &conflict) {
			return StorageLocationInfo{}, nil, &StorageLocationIdentityConflict{
				RootID: conflict.RootID, RegisteredPath: conflict.RegisteredPath,
				RequestedPath: conflict.RequestedPath, Actions: conflict.Actions,
			}
		}
		return StorageLocationInfo{}, nil, err
	}
	impact, err := c.manager.PreviewRepositoryRootRemoval(ctx, root.RootID.String())
	if err != nil {
		return StorageLocationInfo{}, nil, err
	}
	return storageLocationInfo(*root, impact), storage.RepositoryRootWarnings(root.Path), nil
}

func (c *repositoryControl) ResolveStorageLocationConflict(ctx context.Context, rootID, path string) (StorageLocationInfo, error) {
	root, err := c.manager.RelocateRepositoryRoot(ctx, rootID, path, desktopLifecycleRequest("", "update_location", true))
	if err != nil {
		return StorageLocationInfo{}, err
	}
	impact, err := c.manager.PreviewRepositoryRootRemoval(ctx, root.RootID.String())
	if err != nil {
		return StorageLocationInfo{}, err
	}
	return storageLocationInfo(*root, impact), nil
}

func (c *repositoryControl) RemoveStorageLocation(ctx context.Context, id string) error {
	return c.manager.DeleteRepositoryRoot(ctx, id, desktopLifecycleRequest("", "remove_from_lumilio", false))
}

func (c *repositoryControl) AttachRepository(ctx context.Context, path string) (RepositoryInfo, error) {
	ownerID, err := c.requireHostOwnerID(ctx)
	if err != nil {
		return RepositoryInfo{}, err
	}
	repository, err := c.manager.OpenRepository(ctx, path, ownerID, dbtypes.RepoRoleRegular,
		desktopLifecycleRequest("", "native_directory_selection", false))
	if err != nil {
		var conflict *storage.RepositoryConflictError
		if errors.As(err, &conflict) {
			return RepositoryInfo{}, &RepositoryIdentityConflict{
				RepositoryID: conflict.RepositoryID, RegisteredPath: conflict.RegisteredPath,
				RequestedPath: conflict.RequestedPath, Actions: conflict.Actions,
			}
		}
		return RepositoryInfo{}, err
	}
	return repositoryInfo(repository), nil
}

func (c *repositoryControl) ResolveRepositoryConflict(ctx context.Context, action, repositoryID, path string, requestIDs ...string) (RepositoryInfo, error) {
	var repository *repo.Repository
	var err error
	switch action {
	case "relocate":
		requestID := ""
		if len(requestIDs) > 0 {
			requestID = requestIDs[0]
		}
		repository, err = c.manager.RelocateRepository(ctx, repositoryID, path,
			desktopLifecycleRequest(requestID, "update_location", true))
	case "copy":
		var ownerID *int32
		ownerID, err = c.requireHostOwnerID(ctx)
		if err == nil {
			requestID := ""
			if len(requestIDs) > 0 {
				requestID = requestIDs[0]
			}
			repository, err = c.manager.RegisterRepositoryCopy(ctx, path, ownerID, dbtypes.RepoRoleRegular,
				desktopLifecycleRequest(requestID, "independent_identity", true))
		}
	default:
		return RepositoryInfo{}, fmt.Errorf("unknown repository conflict action %q", action)
	}
	if err != nil {
		return RepositoryInfo{}, err
	}
	return repositoryInfo(repository), nil
}

func desktopLifecycleRequest(requestID, confirmationType string, riskConfirmation bool) storage.LifecycleRequest {
	hostInstanceID, _ := os.Hostname()
	hostInstanceID = strings.TrimSpace(hostInstanceID)
	if hostInstanceID == "" {
		hostInstanceID = "desktop-host"
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = uuid.NewString()
	}
	return storage.LifecycleRequest{
		RequestID: requestID, Actor: "desktop_host:" + hostInstanceID, HostInstanceID: hostInstanceID,
		ConfirmationType: confirmationType, RiskConfirmation: riskConfirmation,
	}
}

func (c *repositoryControl) ListPendingHostActions(ctx context.Context) ([]HostActionInfo, error) {
	actions, err := c.manager.ListPendingHostActions(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]HostActionInfo, 0, len(actions))
	for _, action := range actions {
		items = append(items, hostActionInfo(action))
	}
	return items, nil
}

func (c *repositoryControl) SetHostActionExpectedVersion(ctx context.Context, actionID, nonce string, version uint64) (HostActionInfo, error) {
	action, err := c.manager.SetHostActionExpectedVersion(ctx, actionID, nonce, version)
	if err != nil {
		return HostActionInfo{}, err
	}
	return hostActionInfo(action), nil
}

func (c *repositoryControl) ExecuteHostAction(ctx context.Context, actionID, nonce, hostInstanceID, selectedPath string, riskConfirmation bool) (HostActionInfo, error) {
	action, err := c.manager.ExecuteHostAction(ctx, actionID, nonce, hostInstanceID, selectedPath, riskConfirmation)
	if err != nil {
		if errors.Is(err, storage.ErrRepositoryRiskConfirmationRequired) && action.Status == storage.HostActionNeedsDecision {
			return hostActionInfo(action), nil
		}
		return HostActionInfo{}, err
	}
	return hostActionInfo(action), nil
}

func (c *repositoryControl) CancelHostAction(ctx context.Context, actionID string) (HostActionInfo, error) {
	action, err := c.manager.CancelHostAction(ctx, actionID)
	if err != nil {
		return HostActionInfo{}, err
	}
	return hostActionInfo(action), nil
}

func (c *repositoryControl) requireHostOwnerID(ctx context.Context) (*int32, error) {
	ownerID, err := c.manager.HostOwnerID(ctx)
	if err != nil {
		return nil, err
	}
	if ownerID == nil {
		return nil, ErrHostOwnerUnavailable
	}
	return ownerID, nil
}

func storageLocationInfo(root repo.RepositoryRoot, impact storage.RepositoryRootRemovalImpact) StorageLocationInfo {
	pathInfo := storage.InspectStoragePath(root.Path)
	return StorageLocationInfo{
		ID: root.RootID.String(), Name: root.Name, Path: root.Path, Kind: string(root.Kind), Status: string(root.Status),
		RepositoryCount: impact.RepositoryCount, ActiveOperationCount: impact.ActiveOperationCount,
		CanRemove: impact.CanRemove, RemovalBlockedBy: impact.BlockingReason, FilesPreserved: impact.FilesPreserved,
		Writable: pathInfo.Writable, CapacityKnown: pathInfo.CapacityKnown,
		TotalBytes: pathInfo.TotalBytes, AvailableBytes: pathInfo.AvailableBytes, Filesystem: pathInfo.Filesystem,
	}
}

func repositoryInfo(repository *repo.Repository) RepositoryInfo {
	if repository == nil {
		return RepositoryInfo{}
	}
	return RepositoryInfo{
		ID:           repository.RepoID.String(),
		Name:         repository.Name,
		Path:         repository.Path,
		Reachability: string(repository.Reachability),
		Activity:     string(repository.Activity),
	}
}

func hostActionInfo(action storage.HostAction) HostActionInfo {
	var warnings []string
	if action.Result != nil && action.Result.Conflict != nil {
		warnings = action.Result.Conflict.RiskWarnings
	}
	return HostActionInfo{
		ID: action.ActionID, RequestID: action.RequestID, Kind: string(action.Kind),
		Actor: action.Actor, Purpose: action.Summary.Purpose, Name: action.Summary.Name,
		ExpectedVersion: action.ExpectedVersion, Nonce: action.NativeHostNonce(),
		Status: string(action.Status), RiskWarnings: warnings, ExpiresAt: action.ExpiresAt,
	}
}
