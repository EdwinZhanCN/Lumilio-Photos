package gorm_repo

import (
	"context"
	"server/internal/models"
	"server/internal/repository"

	"gorm.io/gorm"
)

type gormTagRepo struct {
	db *gorm.DB
}

func NewTagRepository(db *gorm.DB) repository.TagRepository {
	return &gormTagRepo{db: db}
}

func (r *gormTagRepo) GetByName(ctx context.Context, name string) (*models.Tag, error) {
	var tag models.Tag
	err := r.db.WithContext(ctx).Where("tag_name = ?", name).First(&tag).Error
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

func (r *gormTagRepo) Create(ctx context.Context, tag *models.Tag) error {
	return r.db.WithContext(ctx).Create(tag).Error
}
