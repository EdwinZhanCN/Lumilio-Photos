package handler

import (
	"context"

	"fx/storage"
)

// Handler serves requests.
type Handler struct{ manager storage.RepositoryManager }

// List reads the projection.
func (h *Handler) List(ctx context.Context) []string { return h.manager.List(ctx) }
