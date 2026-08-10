package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"

	"server/internal/db/dbtypes"
)

const (
	StorageRuntimeStateActive   = "active"
	StorageRuntimeStateDegraded = "degraded"

	StorageRuntimeReasonRecoveryRequired = "storage_recovery_required"
)

// StorageRuntimeStatus is a live availability fact. It is deliberately
// separate from bootstrap_phase, which becomes durable after setup completes.
type StorageRuntimeStatus struct {
	State  string
	Reason string
}

// StorageRuntimeStatus reconciles disk identity before deriving the global
// state. Ordinary repository failures are local; only the configured default
// Storage Location or its fixed primary child can degrade the instance.
func (rm *DefaultRepositoryManager) StorageRuntimeStatus(ctx context.Context) (StorageRuntimeStatus, error) {
	if err := rm.ReconcileRepositoryRoots(ctx); err != nil {
		return StorageRuntimeStatus{}, fmt.Errorf("reconcile storage locations: %w", err)
	}
	if err := rm.ReconcileAll(ctx); err != nil {
		return StorageRuntimeStatus{}, fmt.Errorf("reconcile repositories: %w", err)
	}

	defaultRoot, err := rm.queries.GetDefaultRepositoryRoot(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return degradedStorageRuntimeStatus(), nil
		}
		return StorageRuntimeStatus{}, fmt.Errorf("load default storage location: %w", err)
	}
	if defaultRoot.Status != dbtypes.RepositoryRootStatusActive {
		return degradedStorageRuntimeStatus(), nil
	}

	primary, err := rm.queries.GetPrimaryRepositoryRecord(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return degradedStorageRuntimeStatus(), nil
		}
		return StorageRuntimeStatus{}, fmt.Errorf("load primary repository: %w", err)
	}
	if primary.RootID != defaultRoot.RootID ||
		primary.Path != filepath.Join(defaultRoot.Path, "primary") ||
		primary.Reachability != dbtypes.RepositoryReachabilityActive {
		return degradedStorageRuntimeStatus(), nil
	}

	return StorageRuntimeStatus{State: StorageRuntimeStateActive}, nil
}

func degradedStorageRuntimeStatus() StorageRuntimeStatus {
	return StorageRuntimeStatus{
		State:  StorageRuntimeStateDegraded,
		Reason: StorageRuntimeReasonRecoveryRequired,
	}
}
