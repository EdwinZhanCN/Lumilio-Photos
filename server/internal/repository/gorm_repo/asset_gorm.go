package gorm_repo

import (
	"context"
	"server/internal/models"
	"server/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type gormAssetRepo struct {
	db *gorm.DB
}

// NewAssetRepository creates a new GORM-based asset repository
func NewAssetRepository(db *gorm.DB) repository.AssetRepository {
	return &gormAssetRepo{db: db}
}

func (r *gormAssetRepo) CreateAsset(ctx context.Context, asset *models.Asset) error {
	return r.db.WithContext(ctx).Create(asset).Error
}

func (r *gormAssetRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Asset, error) {
	var asset models.Asset
	err := r.db.WithContext(ctx).
		Preload("Thumbnails").
		Preload("Tags").
		Preload("Albums").
		Where("asset_id = ? AND is_deleted = ?", id, false).
		First(&asset).Error
	return &asset, err
}

func (r *gormAssetRepo) GetByType(ctx context.Context, assetType models.AssetType, limit, offset int) ([]*models.Asset, error) {
	var assets []*models.Asset
	err := r.db.WithContext(ctx).
		Where("type = ? AND is_deleted = ?", assetType, false).
		Limit(limit).
		Offset(offset).
		Order("upload_time DESC").
		Find(&assets).Error
	return assets, err
}

func (r *gormAssetRepo) GetByOwner(ctx context.Context, ownerID int, limit, offset int) ([]*models.Asset, error) {
	var assets []*models.Asset
	err := r.db.WithContext(ctx).
		Where("owner_id = ? AND is_deleted = ?", ownerID, false).
		Limit(limit).
		Offset(offset).
		Order("upload_time DESC").
		Find(&assets).Error
	return assets, err
}

func (r *gormAssetRepo) UpdateAsset(ctx context.Context, asset *models.Asset) error {
	return r.db.WithContext(ctx).Save(asset).Error
}

func (r *gormAssetRepo) DeleteAsset(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.Asset{}).
		Where("asset_id = ?", id).
		Updates(map[string]interface{}{
			"is_deleted": true,
			"deleted_at": gorm.Expr("CURRENT_TIMESTAMP"),
		}).Error
}

func (r *gormAssetRepo) AddAssetToAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error {
	return r.db.WithContext(ctx).Exec(
		"INSERT INTO album_assets (asset_id, album_id) VALUES (?, ?) ON CONFLICT DO NOTHING",
		assetID, albumID,
	).Error
}

func (r *gormAssetRepo) RemoveAssetFromAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error {
	return r.db.WithContext(ctx).Exec(
		"DELETE FROM album_assets WHERE asset_id = ? AND album_id = ?",
		assetID, albumID,
	).Error
}

func (r *gormAssetRepo) AddTagToAsset(ctx context.Context, assetID uuid.UUID, tagID int, confidence float32, source string) error {
	return r.db.WithContext(ctx).Exec(
		"INSERT INTO asset_tags (asset_id, tag_id, confidence, source) VALUES (?, ?, ?, ?) ON CONFLICT (asset_id, tag_id) DO UPDATE SET confidence = ?, source = ?",
		assetID, tagID, confidence, source, confidence, source,
	).Error
}

func (r *gormAssetRepo) RemoveTagFromAsset(ctx context.Context, assetID uuid.UUID, tagID int) error {
	return r.db.WithContext(ctx).Exec(
		"DELETE FROM asset_tags WHERE asset_id = ? AND tag_id = ?",
		assetID, tagID,
	).Error
}

func (r *gormAssetRepo) CreateThumbnail(ctx context.Context, thumbnail *models.Thumbnail) error {
	return r.db.WithContext(ctx).Create(thumbnail).Error
}

func (r *gormAssetRepo) UpdateAssetMetadata(ctx context.Context, assetID uuid.UUID, metadata models.SpecificMetadata) error {
	return r.db.WithContext(ctx).
		Model(&models.Asset{}).
		Where("asset_id = ?", assetID).
		Update("specific_metadata", metadata).Error
}

func (r *gormAssetRepo) SearchAssets(ctx context.Context, query string, assetType *models.AssetType, limit, offset int) ([]*models.Asset, error) {
	db := r.db.WithContext(ctx).Where("is_deleted = ?", false)

	if query != "" {
		db = db.Where("original_filename ILIKE ?", "%"+query+"%")
	}

	if assetType != nil {
		db = db.Where("type = ?", *assetType)
	}

	var assets []*models.Asset
	err := db.Limit(limit).
		Offset(offset).
		Order("upload_time DESC").
		Find(&assets).Error

	return assets, err
}

func (r *gormAssetRepo) GetAssetsByHash(ctx context.Context, hash string) ([]*models.Asset, error) {
	var assets []*models.Asset
	err := r.db.WithContext(ctx).
		Where("hash = ? AND is_deleted = ?", hash, false).
		Find(&assets).Error
	return assets, err
}
