package app

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type countingStorageReconciler struct {
	storageLocations atomic.Int32
	repos            atomic.Int32
}

func (r *countingStorageReconciler) ReconcileStorageLocations(context.Context) error {
	r.storageLocations.Add(1)
	return nil
}

func (r *countingStorageReconciler) ReconcileAll(context.Context) error {
	r.repos.Add(1)
	return nil
}

func TestStorageReconcilerRunsWithoutManagePageTraffic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reconciler := &countingStorageReconciler{}
	startStorageReconciler(ctx, reconciler, time.Millisecond, nil)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if reconciler.storageLocations.Load() > 0 && reconciler.repos.Load() > 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("background reconciliation did not run: storage_locations=%d repos=%d", reconciler.storageLocations.Load(), reconciler.repos.Load())
}
