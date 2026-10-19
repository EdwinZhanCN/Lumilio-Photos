package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

// StorageRuntimeStatus reports whether bootstrap anchors exist in the catalog.
// After setup completes, individual Repository reachability and Storage Location
// health projections do not degrade the whole instance; those facts stay local
// to each Repository and Location summary.
func (rm *DefaultRepositoryManager) StorageRuntimeStatus(ctx context.Context) (StorageRuntimeStatus, error) {
	defaultLocation, err := rm.readerQueries.GetDefaultStorageLocation(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return degradedStorageRuntimeStatus(), nil
		}
		return StorageRuntimeStatus{}, fmt.Errorf("load default storage location: %w", err)
	}
	primary, err := rm.readerQueries.GetPrimaryRepositoryRecord(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return degradedStorageRuntimeStatus(), nil
		}
		return StorageRuntimeStatus{}, fmt.Errorf("load primary repository: %w", err)
	}
	// Both anchors are registered. An anchor that reconciliation found
	// offline or in error keeps the instance ready (setup is durable) but
	// the runtime is degraded until the default tree is back or recovered;
	// a maintenance barrier is a transient command state, not a fault.
	if defaultLocation.Status == dbtypes.StorageLocationStatusOffline || defaultLocation.Status == dbtypes.StorageLocationStatusError ||
		primary.Reachability == dbtypes.RepositoryReachabilityOffline {
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
