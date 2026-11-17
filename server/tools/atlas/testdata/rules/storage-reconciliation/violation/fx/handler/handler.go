package handler

import (
	"context"

	"fx/storage"
)

// Handler serves requests.
type Handler struct{ manager storage.RepositoryManager }

// List reconciles on a foreground read.
func (h *Handler) List(ctx context.Context) []string {
	_ = h.manager.ReconcileAll(ctx)
	return h.manager.List(ctx)
}
