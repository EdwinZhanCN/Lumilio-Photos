package storage

import "context"

// RuntimeStatus answers a foreground read and must not reconcile.
func (m *DefaultRepositoryManager) RuntimeStatus(ctx context.Context) error {
	return m.ReconcileAll(ctx)
}
