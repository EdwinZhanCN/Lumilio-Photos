package storage

import "context"

// RepositoryManager is the storage port.
type RepositoryManager interface {
	ReconcileAll(context.Context) error
	List(context.Context) []string
}

// DefaultRepositoryManager implements RepositoryManager.
type DefaultRepositoryManager struct{}

// ReconcileAll writes the projection.
func (m *DefaultRepositoryManager) ReconcileAll(context.Context) error { return nil }

// List reads the projection.
func (m *DefaultRepositoryManager) List(context.Context) []string { return nil }
