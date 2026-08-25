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

// StorageRuntimeStatus derives the global state from the latest reconciled
// catalog projection. Startup and the background storage reconciler own disk
// inspection and projection writes; this foreground status read never waits for
// SQLite's sole writer. Ordinary repository failures are local; only the
// configured default Storage Location or its fixed primary child can degrade
// the instance.
func (rm *DefaultRepositoryManager) StorageRuntimeStatus(ctx context.Context) (StorageRuntimeStatus, error) {
	defaultRoot, err := rm.readerQueries.GetDefaultRepositoryRoot(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return degradedStorageRuntimeStatus(), nil
		}
		return StorageRuntimeStatus{}, fmt.Errorf("load default storage location: %w", err)
	}
	if defaultRoot.Status != dbtypes.RepositoryRootStatusActive {
		return degradedStorageRuntimeStatus(), nil
	}

	primary, err := rm.readerQueries.GetPrimaryRepositoryRecord(ctx)
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
