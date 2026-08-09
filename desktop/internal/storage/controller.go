// Package storage adapts the in-process repository control handoff into the
// small, cache-backed surface needed by Tray and Settings. The cache is
// derived state: corruption is quarantined and never prevents Host startup.
package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"desktop/internal/platform"
	"desktop/internal/runtime"
	"desktop/internal/state"
)

const cacheSchemaVersion = 1

type cacheFile struct {
	SchemaVersion int                   `json:"schemaVersion"`
	Version       uint64                `json:"version"`
	Items         []dto.StorageShortcut `json:"items"`
}

type Options struct {
	Paths           platform.Paths
	Runtime         *runtime.Controller
	Store           *state.Store
	Operations      *operation.Registry
	PickDirectory   func(string) (string, error)
	OpenFileManager func(string, bool) error
}

type Controller struct {
	paths      platform.Paths
	runtime    *runtime.Controller
	store      *state.Store
	operations *operation.Registry

	openMu sync.Mutex
	open   func(string, bool) error
	pick   func(string) (string, error)
}

func NewController(options Options) *Controller {
	if options.Store == nil {
		options.Store = state.New()
	}
	if options.Operations == nil {
		options.Operations = operation.New()
	}
	return &Controller{paths: options.Paths, runtime: options.Runtime, store: options.Store, operations: options.Operations, pick: options.PickDirectory, open: options.OpenFileManager}
}

// SetPickDirectory installs the host-owned native directory picker. The
// callback is the only Wails/UI boundary here; the selected path is still
// treated as untrusted and canonicalized before it is returned to bindings.
func (c *Controller) SetPickDirectory(pick func(string) (string, error)) {
	c.openMu.Lock()
	c.pick = pick
	c.openMu.Unlock()
}

func (c *Controller) SetOpenFileManager(open func(string, bool) error) {
	c.openMu.Lock()
	c.open = open
	c.openMu.Unlock()
}

func (c *Controller) ListShortcuts(ctx context.Context) ([]dto.StorageShortcut, error) {
	if c.runtime != nil {
		if control := c.runtime.RepositoryControl(); control != nil {
			locations, err := control.ListStorageLocations(ctx)
			if err == nil {
				return c.refresh(locations)
			}
		}
	}
	cache, err := c.loadCache()
	if err != nil {
		return nil, nil
	}
	c.publishCache(cache)
	return cache.Items, nil
}

func (c *Controller) OpenLocation(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return operation.NewError(dto.ErrorInvalidArgument, "storage location ID is required")
	}
	items, err := c.ListShortcuts(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.ID != id {
			continue
		}
		path, err := validateShortcutPath(item.Path)
		if err != nil {
			return operation.NewError(dto.ErrorStorageLocationOffline, "storage location is offline or no longer authorized")
		}
		c.openMu.Lock()
		open := c.open
		c.openMu.Unlock()
		if open == nil {
			return operation.NewError(dto.ErrorRuntimeNotReady, "file manager is unavailable")
		}
		return open(path, false)
	}
	return operation.NewError(dto.ErrorInvalidArgument, "unknown storage location")
}

// PickLocation opens the native directory picker without touching repository
// state. A subsequent AddLocation call performs the durable registration.
func (c *Controller) PickLocation(title string) (string, error) {
	c.openMu.Lock()
	pick := c.pick
	c.openMu.Unlock()
	if pick == nil {
		return "", operation.NewError(dto.ErrorRuntimeNotReady, "directory picker is unavailable")
	}
	path, err := pick(strings.TrimSpace(title))
	if err != nil {
		return "", operation.NewError(dto.ErrorInvalidArgument, "directory selection was cancelled or unavailable")
	}
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	canonical, err := canonicalShortcutPath(path)
	if err != nil {
		return "", operation.NewError(dto.ErrorStorageLocationOffline, "selected directory is not available")
	}
	return canonical, nil
}

func (c *Controller) ListHostActions(ctx context.Context) ([]dto.HostActionTicket, error) {
	control, err := c.repositoryControl()
	if err != nil {
		return nil, err
	}
	actions, err := control.ListPendingHostActions(ctx)
	if err != nil {
		return nil, err
	}
	currentVersion := c.store.Get().Storage.Version
	for index := range actions {
		if actions[index].ExpectedVersion == 0 && currentVersion != 0 {
			bound, bindErr := control.SetHostActionExpectedVersion(ctx, actions[index].ID, actions[index].Nonce, currentVersion)
			if bindErr != nil {
				return nil, bindErr
			}
			actions[index] = bound
		}
	}
	items := make([]dto.HostActionTicket, 0, len(actions))
	for _, action := range actions {
		items = append(items, dto.HostActionTicket{
			ID: action.ID, RequestID: action.RequestID, Kind: action.Kind, Actor: action.Actor,
			Purpose: action.Purpose, Name: action.Name, ExpectedVersion: action.ExpectedVersion,
			Nonce: action.Nonce, Status: action.Status, RiskWarnings: action.RiskWarnings, ExpiresAt: action.ExpiresAt,
		})
	}
	return items, nil
}

func (c *Controller) ApproveHostAction(requestID string, expectedVersion uint64, actionID, nonce string) (dto.OperationReceipt, error) {
	if existing, ok := c.operations.ReceiptForRequest(requestID); ok {
		return existing, nil
	}
	control, err := c.repositoryControl()
	if err != nil {
		return dto.OperationReceipt{}, err
	}
	currentVersion := c.store.Get().Storage.Version
	if expectedVersion != 0 && expectedVersion != currentVersion {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorStaleVersion, "storage summary version is stale")
	}
	actions, err := control.ListPendingHostActions(context.Background())
	if err != nil {
		return dto.OperationReceipt{}, err
	}
	var selected *runtime.HostAction
	for i := range actions {
		if actions[i].ID == actionID && actions[i].Nonce == nonce {
			selected = &actions[i]
			break
		}
	}
	if selected == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorInvalidArgument, "host action is no longer pending")
	}
	if selected.ExpectedVersion != 0 && selected.ExpectedVersion != currentVersion {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorStaleVersion, "host action expected a different storage version")
	}
	riskConfirmation := selected.Status == "needs_decision" && len(selected.RiskWarnings) > 0
	canonical := ""
	if !riskConfirmation {
		c.openMu.Lock()
		pick := c.pick
		c.openMu.Unlock()
		if pick == nil {
			return dto.OperationReceipt{}, operation.NewError(dto.ErrorRuntimeNotReady, "directory picker is unavailable")
		}
		path, pickErr := pick(hostActionPickerTitle(*selected))
		if pickErr != nil || strings.TrimSpace(path) == "" {
			return dto.OperationReceipt{}, operation.NewError(dto.ErrorInvalidArgument, "directory selection was cancelled")
		}
		canonical, err = canonicalShortcutPath(path)
		if err != nil {
			return dto.OperationReceipt{}, operation.NewError(dto.ErrorStorageLocationOffline, "selected directory is unavailable")
		}
	}
	receipt, err := c.operations.Accept(requestID, "host-action:"+actionID, currentVersion, false)
	if err != nil {
		return dto.OperationReceipt{}, err
	}
	instanceID := c.store.Get().InstanceID
	go func() {
		_, callErr := control.ExecuteHostAction(context.Background(), actionID, nonce, instanceID, canonical, riskConfirmation)
		if callErr == nil {
			if locations, listErr := control.ListStorageLocations(context.Background()); listErr == nil {
				_, callErr = c.refresh(locations)
			}
		}
		if callErr != nil {
			_ = c.operations.Fail(receipt.OperationID, operation.WithOperation(operation.NewError(dto.ErrorRecoveryRequired, "native host action failed"), receipt.OperationID))
		} else {
			_ = c.operations.Succeed(receipt.OperationID)
		}
		c.syncOperations()
	}()
	c.syncOperations()
	return receipt, nil
}

func (c *Controller) DeclineHostAction(actionID string) error {
	control, err := c.repositoryControl()
	if err != nil {
		return err
	}
	_, err = control.CancelHostAction(context.Background(), strings.TrimSpace(actionID))
	return err
}

func hostActionPickerTitle(action runtime.HostAction) string {
	switch action.Kind {
	case "authorize_storage_location":
		return "Authorize Storage Location"
	case "open_repository":
		return "Open Existing Repository"
	case "locate_storage_location":
		return "Locate Storage Location"
	case "locate_repository":
		return "Locate Repository"
	default:
		return "Choose Folder"
	}
}

func (c *Controller) repositoryControl() (runtime.StorageControl, error) {
	if c.runtime == nil {
		return nil, operation.NewError(dto.ErrorRepositoryControlUnavailable, "repository control is unavailable")
	}
	control := c.runtime.RepositoryControl()
	if control == nil {
		return nil, operation.NewError(dto.ErrorRepositoryControlUnavailable, "repository control is unavailable")
	}
	return control, nil
}

func (c *Controller) syncOperations() {
	items := c.operations.Snapshot()
	c.store.Commit(func(snapshot *dto.DesktopSnapshot) { snapshot.Operations = items })
}

func (c *Controller) refresh(locations []runtime.StorageLocation) ([]dto.StorageShortcut, error) {
	items := make([]dto.StorageShortcut, 0, len(locations))
	for _, location := range locations {
		path, pathErr := canonicalShortcutPath(location.Path)
		status := strings.TrimSpace(location.Status)
		canOpen := pathErr == nil && !strings.EqualFold(status, "offline") && !strings.EqualFold(status, "error")
		if path == "" {
			path = cleanCandidatePath(location.Path)
		}
		items = append(items, dto.StorageShortcut{
			ID: location.ID, Name: location.Name, Path: path, Kind: location.Kind, Status: status, CanOpen: canOpen,
			RepositoryCount: location.RepositoryCount, ActiveOperationCount: location.ActiveOperationCount,
			CanRemove: location.CanRemove, RemovalBlockedBy: location.RemovalBlockedBy, FilesPreserved: location.FilesPreserved,
			Writable: location.Writable, CapacityKnown: location.CapacityKnown, TotalBytes: location.TotalBytes,
			AvailableBytes: location.AvailableBytes, Filesystem: location.Filesystem,
		})
	}
	cache, err := c.loadCache()
	if err != nil {
		cache = cacheFile{SchemaVersion: cacheSchemaVersion}
	}
	if !reflect.DeepEqual(cache.Items, items) {
		cache.Version++
		cache.Items = items
		if err := c.saveCache(cache); err != nil {
			return items, err
		}
	}
	c.publishCache(cache)
	return items, nil
}

func (c *Controller) publishCache(cache cacheFile) {
	items := append([]dto.StorageShortcut(nil), cache.Items...)
	c.store.Commit(func(snapshot *dto.DesktopSnapshot) {
		snapshot.Storage.Version = cache.Version
		snapshot.Storage.Count = len(items)
	})
}

func (c *Controller) loadCache() (cacheFile, error) {
	data, err := os.ReadFile(c.paths.ShortcutsFile)
	if errors.Is(err, fs.ErrNotExist) {
		return cacheFile{SchemaVersion: cacheSchemaVersion}, nil
	}
	if err != nil {
		return cacheFile{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var cache cacheFile
	if err := decoder.Decode(&cache); err != nil || cache.SchemaVersion != cacheSchemaVersion {
		_ = os.Rename(c.paths.ShortcutsFile, c.paths.ShortcutsFile+fmt.Sprintf(".corrupt.%d", time.Now().UnixNano()))
		return cacheFile{SchemaVersion: cacheSchemaVersion}, fmt.Errorf("storage shortcut cache is invalid")
	}
	return cache, nil
}

func (c *Controller) saveCache(cache cacheFile) error {
	cache.SchemaVersion = cacheSchemaVersion
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return platform.WriteAtomic(c.paths.ShortcutsFile, append(data, '\n'), 0o600)
}

func validateShortcutPath(path string) (string, error) {
	clean, err := canonicalShortcutPath(path)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(clean, ".lumilioroot")); err != nil {
		return "", err
	}
	return clean, nil
}

func canonicalShortcutPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("storage path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(abs)
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil {
		return clean, err
	}
	resolved = filepath.Clean(resolved)
	info, err := os.Stat(resolved)
	if err != nil {
		return resolved, err
	}
	if !info.IsDir() {
		return resolved, errors.New("storage path is not a directory")
	}
	return resolved, nil
}

func cleanCandidatePath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	return filepath.Clean(abs)
}

var _ interface {
	ListShortcuts(context.Context) ([]dto.StorageShortcut, error)
	OpenLocation(context.Context, string) error
} = (*Controller)(nil)
