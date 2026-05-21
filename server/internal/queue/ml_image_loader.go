package queue

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/utils/imagesource"

	"github.com/jackc/pgx/v5/pgtype"
)

type MLImageLoader interface {
	LoadMLImage(ctx context.Context, assetID pgtype.UUID, purpose imagesource.Purpose, preprocessVersion string) (*imagesource.MLImage, error)
}

type DBMLImageLoader struct {
	Queries *repo.Queries
}

func NewDBMLImageLoader(queries *repo.Queries) *DBMLImageLoader {
	return &DBMLImageLoader{Queries: queries}
}

func mlThumbnailSize(purpose imagesource.Purpose) string {
	switch purpose {
	case imagesource.PurposeOCR, imagesource.PurposeFace:
		return "medium"
	default:
		return "large"
	}
}

func (l *DBMLImageLoader) LoadMLImage(ctx context.Context, assetID pgtype.UUID, purpose imagesource.Purpose, preprocessVersion string) (*imagesource.MLImage, error) {
	if l == nil || l.Queries == nil {
		return nil, fmt.Errorf("ml image loader unavailable")
	}
	if preprocessVersion != "" && preprocessVersion != jobs.MLPreprocessVersionV1 {
		return nil, fmt.Errorf("unsupported ml preprocess version: %s", preprocessVersion)
	}

	asset, err := l.Queries.GetAssetByID(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("get asset: %w", err)
	}
	if dbtypes.AssetType(asset.Type) != dbtypes.AssetTypePhoto {
		return nil, fmt.Errorf("asset %s is not a photo: %s", asset.AssetID.String(), asset.Type)
	}
	if !asset.RepositoryID.Valid {
		return nil, fmt.Errorf("asset %s has no repository", asset.AssetID.String())
	}

	repository, err := l.Queries.GetRepository(ctx, asset.RepositoryID)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	thumbnailSize := mlThumbnailSize(purpose)
	thumbnail, err := l.Queries.GetThumbnailByAssetAndSize(ctx, repo.GetThumbnailByAssetAndSizeParams{
		AssetID: assetID,
		Size:    thumbnailSize,
	})
	if err != nil {
		return nil, fmt.Errorf("get %s thumbnail: %w", thumbnailSize, err)
	}

	thumbnailPath := filepath.Join(repository.Path, filepath.FromSlash(thumbnail.StoragePath))
	file, err := os.Open(thumbnailPath)
	if err != nil {
		return nil, fmt.Errorf("open %s thumbnail: %w", thumbnailSize, err)
	}
	defer file.Close()

	imageData, err := imagesource.ProcessMLImageTensorFromReader(file, purpose)
	if err != nil {
		return nil, fmt.Errorf("process %s thumbnail for ml: %w", thumbnailSize, err)
	}

	return imageData, nil
}
