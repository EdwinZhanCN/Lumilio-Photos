package storage

import "context"

// RuntimeStatus answers a foreground read from the cached projection.
func (m *DefaultRepositoryManager) RuntimeStatus(ctx context.Context) []string {
	return m.List(ctx)
}
