package repository

import (
	"context"
	"server/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PhotoRepository interface {
	CreatePhoto(ctx context.Context, photo *models.Photo) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Photo, error)
	UpdatePhoto(ctx context.Context, photo *models.Photo) error
	AddPhotoToAlbum(ctx context.Context, photoID uuid.UUID, albumID int) error
	RemovePhotoFromAlbum(ctx context.Context, photoID uuid.UUID, albumID int) error
	AddTagToPhoto(ctx context.Context, photoID uuid.UUID, tagID int, confidence float32, source string) error
	RemoveTagFromPhoto(ctx context.Context, photoID uuid.UUID, tagID int) error
	CreateThumbnail(ctx context.Context, thumbnail *models.Thumbnail) error
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

func (r *gormPhotoRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Photo, error) {
	var photo models.Photo
	err := r.db.WithContext(ctx).Where("photo_id = ?", id).First(&photo).Error
	return &photo, err
}

func (r *gormPhotoRepo) UpdatePhoto(ctx context.Context, photo *models.Photo) error {
	return r.db.WithContext(ctx).Save(photo).Error
}

func (r *gormPhotoRepo) AddPhotoToAlbum(ctx context.Context, photoID uuid.UUID, albumID int) error {
	return r.db.WithContext(ctx).Exec("INSERT INTO album_photos (photo_id, album_id) VALUES (?, ?)", photoID, albumID).Error
}

func (r *gormPhotoRepo) RemovePhotoFromAlbum(ctx context.Context, photoID uuid.UUID, albumID int) error {
	return r.db.WithContext(ctx).Exec("DELETE FROM album_photos WHERE photo_id = ? AND album_id = ?", photoID, albumID).Error
}

func (r *gormPhotoRepo) AddTagToPhoto(ctx context.Context, photoID uuid.UUID, tagID int, confidence float32, source string) error {
	return r.db.WithContext(ctx).Exec(
		"INSERT INTO photo_tags (photo_id, tag_id, confidence, source) VALUES (?, ?, ?, ?)",
		photoID, tagID, confidence, source,
	).Error
}

func (r *gormPhotoRepo) RemoveTagFromPhoto(ctx context.Context, photoID uuid.UUID, tagID int) error {
	return r.db.WithContext(ctx).Exec("DELETE FROM photo_tags WHERE photo_id = ? AND tag_id = ?", photoID, tagID).Error
}

func (r *gormPhotoRepo) CreateThumbnail(ctx context.Context, thumbnail *models.Thumbnail) error {
	return r.db.WithContext(ctx).Create(thumbnail).Error
}
