package app

import (
	"context"
	"errors"
	"fmt"

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
	AddStorageLocation(ctx context.Context, path, name string) (StorageLocationInfo, []string, error)
	ResolveStorageLocationConflict(ctx context.Context, rootID, path string) (StorageLocationInfo, error)
	RemoveStorageLocation(ctx context.Context, id string) error
	AttachRepository(ctx context.Context, path string) (RepositoryInfo, error)
	ResolveRepositoryConflict(ctx context.Context, action, repositoryID, path string) (RepositoryInfo, error)
}

type StorageLocationInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
}

type RepositoryInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Path   string `json:"path"`
	Status string `json:"status"`
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
		items = append(items, storageLocationInfo(root))
	}
	return items, nil
}

func (c *repositoryControl) AddStorageLocation(ctx context.Context, path, name string) (StorageLocationInfo, []string, error) {
	root, err := c.manager.AddRepositoryRoot(ctx, path, name)
	if err != nil {
		var conflict *storage.RepositoryRootConflictError
		if errors.As(err, &conflict) {
			return StorageLocationInfo{}, nil, &StorageLocationIdentityConflict{
				RootID: conflict.RootID, RegisteredPath: conflict.RegisteredPath,
				RequestedPath: conflict.RequestedPath, Actions: []string{"relocate"},
			}
		}
		return StorageLocationInfo{}, nil, err
	}
	return storageLocationInfo(*root), storage.RepositoryRootWarnings(root.Path), nil
}

func (c *repositoryControl) ResolveStorageLocationConflict(ctx context.Context, rootID, path string) (StorageLocationInfo, error) {
	root, err := c.manager.RelocateRepositoryRoot(ctx, rootID, path)
	if err != nil {
		return StorageLocationInfo{}, err
	}
	return storageLocationInfo(*root), nil
}

func (c *repositoryControl) RemoveStorageLocation(ctx context.Context, id string) error {
	return c.manager.DeleteRepositoryRoot(ctx, id)
}

func (c *repositoryControl) AttachRepository(ctx context.Context, path string) (RepositoryInfo, error) {
	ownerID, err := c.requireHostOwnerID(ctx)
	if err != nil {
		return RepositoryInfo{}, err
	}
	repository, err := c.manager.AddRepository(path, ownerID, dbtypes.RepoRoleRegular)
	if err != nil {
		var conflict *storage.RepositoryConflictError
		if errors.As(err, &conflict) {
			return RepositoryInfo{}, &RepositoryIdentityConflict{
				RepositoryID: conflict.RepositoryID, RegisteredPath: conflict.RegisteredPath,
				RequestedPath: conflict.RequestedPath, Actions: []string{"relocate", "copy"},
			}
		}
		return RepositoryInfo{}, err
	}
	return repositoryInfo(repository), nil
}

func (c *repositoryControl) ResolveRepositoryConflict(ctx context.Context, action, repositoryID, path string) (RepositoryInfo, error) {
	var repository *repo.Repository
	var err error
	switch action {
	case "relocate":
		repository, err = c.manager.RelocateRepository(ctx, repositoryID, path)
	case "copy":
		var ownerID *int32
		ownerID, err = c.requireHostOwnerID(ctx)
		if err == nil {
			repository, err = c.manager.RegisterRepositoryCopy(ctx, path, ownerID, dbtypes.RepoRoleRegular)
		}
	default:
		return RepositoryInfo{}, fmt.Errorf("unknown repository conflict action %q", action)
	}
	if err != nil {
		return RepositoryInfo{}, err
	}
	return repositoryInfo(repository), nil
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

func storageLocationInfo(root repo.RepositoryRoot) StorageLocationInfo {
	id := ""
	if root.RootID.Valid {
		id = uuid.UUID(root.RootID.Bytes).String()
	}
	return StorageLocationInfo{ID: id, Name: root.Name, Path: root.Path, Kind: string(root.Kind), Status: string(root.Status)}
}

func repositoryInfo(repository *repo.Repository) RepositoryInfo {
	if repository == nil {
		return RepositoryInfo{}
	}
	id := ""
	if repository.RepoID.Valid {
		id = uuid.UUID(repository.RepoID.Bytes).String()
	}
	return RepositoryInfo{ID: id, Name: repository.Name, Path: repository.Path, Status: string(repository.Status)}
}
