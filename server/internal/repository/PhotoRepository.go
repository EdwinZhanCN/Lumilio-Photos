package repository

import (
	"context"
	"gorm.io/gorm"
	"server/internal/models"
)

type PhotoRepository interface {
	CreatePhoto(ctx context.Context, photo *models.Photo) error
	GetByID(ctx context.Context, id string) (*models.Photo, error)
	// 其他数据操作方法...
}

type gormPhotoRepo struct {
	db *gorm.DB
}

func NewPhotoRepository(db *gorm.DB) PhotoRepository {
	return &gormPhotoRepo{db: db}
}

func (r *gormPhotoRepo) CreatePhoto(ctx context.Context, photo *models.Photo) error {
	return r.db.WithContext(ctx).Create(photo).Error
}

func (r *gormPhotoRepo) GetByID(ctx context.Context, id string) (*models.Photo, error) {
	var photo models.Photo
	err := r.db.WithContext(ctx).Where("photo_id = ?", id).First(&photo).Error
	return &photo, err
}
