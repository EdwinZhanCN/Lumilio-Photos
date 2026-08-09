package runtime

import (
	"context"
	"time"

	"server/app"
)

// StorageControl is the Desktop-safe projection of Server's in-process
// repository control. The server package remains behind the runtime adapter;
// storage and host code only see these DTO-shaped values.
type StorageControl interface {
	ListStorageLocations(context.Context) ([]StorageLocation, error)
	ListPendingHostActions(context.Context) ([]HostAction, error)
	SetHostActionExpectedVersion(context.Context, string, string, uint64) (HostAction, error)
	ExecuteHostAction(context.Context, string, string, string, string, bool) (HostAction, error)
	CancelHostAction(context.Context, string) (HostAction, error)
}

type StorageLocation struct {
	ID                   string
	Name                 string
	Path                 string
	Kind                 string
	Status               string
	RepositoryCount      int64
	ActiveOperationCount int64
	CanRemove            bool
	RemovalBlockedBy     string
	FilesPreserved       bool
	Writable             bool
	CapacityKnown        bool
	TotalBytes           uint64
	AvailableBytes       uint64
	Filesystem           string
}

type Repository struct {
	ID           string
	Name         string
	Path         string
	Reachability string
	Activity     string
}

type HostAction struct {
	ID              string
	RequestID       string
	Kind            string
	Actor           string
	Purpose         string
	Name            string
	ExpectedVersion uint64
	Nonce           string
	Status          string
	RiskWarnings    []string
	ExpiresAt       time.Time
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
			RepositoryCount: item.RepositoryCount, ActiveOperationCount: item.ActiveOperationCount,
			CanRemove: item.CanRemove, RemovalBlockedBy: item.RemovalBlockedBy, FilesPreserved: item.FilesPreserved,
			Writable: item.Writable, CapacityKnown: item.CapacityKnown, TotalBytes: item.TotalBytes,
			AvailableBytes: item.AvailableBytes, Filesystem: item.Filesystem,
		})
	}
	return result, nil
}

func (a repositoryControlAdapter) ListPendingHostActions(ctx context.Context) ([]HostAction, error) {
	items, err := a.inner.ListPendingHostActions(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]HostAction, 0, len(items))
	for _, item := range items {
		result = append(result, hostAction(item))
	}
	return result, nil
}

func (a repositoryControlAdapter) SetHostActionExpectedVersion(ctx context.Context, actionID, nonce string, version uint64) (HostAction, error) {
	item, err := a.inner.SetHostActionExpectedVersion(ctx, actionID, nonce, version)
	if err != nil {
		return HostAction{}, err
	}
	return hostAction(item), nil
}

func (a repositoryControlAdapter) ExecuteHostAction(ctx context.Context, actionID, nonce, hostInstanceID, selectedPath string, riskConfirmation bool) (HostAction, error) {
	item, err := a.inner.ExecuteHostAction(ctx, actionID, nonce, hostInstanceID, selectedPath, riskConfirmation)
	if err != nil {
		return HostAction{}, err
	}
	return hostAction(item), nil
}

func (a repositoryControlAdapter) CancelHostAction(ctx context.Context, actionID string) (HostAction, error) {
	item, err := a.inner.CancelHostAction(ctx, actionID)
	if err != nil {
		return HostAction{}, err
	}
	return hostAction(item), nil
}

func hostAction(item app.HostActionInfo) HostAction {
	return HostAction{
		ID: item.ID, RequestID: item.RequestID, Kind: item.Kind, Actor: item.Actor,
		Purpose: item.Purpose, Name: item.Name, ExpectedVersion: item.ExpectedVersion,
		Nonce: item.Nonce, Status: item.Status, RiskWarnings: item.RiskWarnings, ExpiresAt: item.ExpiresAt,
	}
}

func adaptRepositoryControl(value app.RepositoryControl) StorageControl {
	if value == nil {
		return nil
	}
	return repositoryControlAdapter{inner: value}
}
