package repository

import (
	"context"
	"server/internal/models"

	"github.com/google/uuid"
)

// AssetRepository defines the interface for asset-related database operations
type AssetRepository interface {
	CreateAsset(ctx context.Context, asset *models.Asset) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Asset, error)
	GetByIDWithOptions(ctx context.Context, id uuid.UUID, includeThumbnails, includeTags, includeAlbums bool) (*models.Asset, error)
	GetByType(ctx context.Context, assetType models.AssetType, limit, offset int) ([]*models.Asset, error)
	GetByOwner(ctx context.Context, ownerID int, limit, offset int) ([]*models.Asset, error)
	UpdateAsset(ctx context.Context, asset *models.Asset) error
	DeleteAsset(ctx context.Context, id uuid.UUID) error
	AddAssetToAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error
	RemoveAssetFromAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error
	AddTagToAsset(ctx context.Context, assetID uuid.UUID, tagID int, confidence float32, source string) error
	RemoveTagFromAsset(ctx context.Context, assetID uuid.UUID, tagID int) error
	CreateThumbnail(ctx context.Context, thumbnail *models.Thumbnail) error
	UpdateAssetMetadata(ctx context.Context, assetID uuid.UUID, metadata models.SpecificMetadata) error
	SearchAssets(ctx context.Context, query string, assetType *models.AssetType, limit, offset int) ([]*models.Asset, error)
	GetAssetsByHash(ctx context.Context, hash string) ([]*models.Asset, error)
	GetThumbnailByID(ctx context.Context, thumbnailID int) (*models.Thumbnail, error)
	GetThumbnailByAssetIDAndSize(ctx context.Context, assetID uuid.UUID, size string) (*models.Thumbnail, error)
	SaveBioAtlas(ctx context.Context, assetID uuid.UUID, predictions []*models.SpeciesPrediction) error
}
