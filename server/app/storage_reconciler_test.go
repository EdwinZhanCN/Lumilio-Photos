package app

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type countingStorageReconciler struct {
	roots atomic.Int32
	repos atomic.Int32
}

func (r *countingStorageReconciler) ReconcileRepositoryRoots(context.Context) error {
	r.roots.Add(1)
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
		if reconciler.roots.Load() > 0 && reconciler.repos.Load() > 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("background reconciliation did not run: roots=%d repos=%d", reconciler.roots.Load(), reconciler.repos.Load())
}
