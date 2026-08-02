package runtime

import (
	"context"

	"server/app"
)

// StorageControl is the Desktop-safe projection of Server's in-process
// repository control. The server package remains behind the runtime adapter;
// storage and host code only see these DTO-shaped values.
type StorageControl interface {
	ListStorageLocations(context.Context) ([]StorageLocation, error)
	AddStorageLocation(context.Context, string, string) (StorageLocation, []string, error)
	AttachRepository(context.Context, string) (Repository, error)
}

type StorageLocation struct {
	ID     string
	Name   string
	Path   string
	Kind   string
	Status string
}

type Repository struct {
	ID     string
	Name   string
	Path   string
	Status string
}

type repositoryControlAdapter struct{ inner app.RepositoryControl }

func (a repositoryControlAdapter) ListStorageLocations(ctx context.Context) ([]StorageLocation, error) {
	items, err := a.inner.ListStorageLocations(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]StorageLocation, 0, len(items))
	for _, item := range items {
		result = append(result, StorageLocation{
			ID: item.ID, Name: item.Name, Path: item.Path, Kind: item.Kind, Status: item.Status,
		})
	}
	return result, nil
}

func (a repositoryControlAdapter) AddStorageLocation(ctx context.Context, path, name string) (StorageLocation, []string, error) {
	item, warnings, err := a.inner.AddStorageLocation(ctx, path, name)
	if err != nil {
		return StorageLocation{}, nil, err
	}
	return StorageLocation{ID: item.ID, Name: item.Name, Path: item.Path, Kind: item.Kind, Status: item.Status}, warnings, nil
}

func (a repositoryControlAdapter) AttachRepository(ctx context.Context, path string) (Repository, error) {
	item, err := a.inner.AttachRepository(ctx, path)
	if err != nil {
		return Repository{}, err
	}
	return Repository{ID: item.ID, Name: item.Name, Path: item.Path, Status: item.Status}, nil
}

func adaptRepositoryControl(value app.RepositoryControl) StorageControl {
	if value == nil {
		return nil
	}
	return repositoryControlAdapter{inner: value}
}
