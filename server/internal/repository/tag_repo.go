package repository

import (
	"context"
	"server/internal/models"
)

type TagRepository interface {
	GetByName(ctx context.Context, name string) (*models.Tag, error)
	Create(ctx context.Context, tag *models.Tag) error
}
