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
		// Detection quality depends on input resolution; medium (800px)
		// balances that against PP-OCR/SCRFD inference latency.
		return "medium"
	default:
		// Semantic/BioCLIP encoders consume 224x224 tensors, so the medium
		// thumbnail already carries ~3.5x the target resolution; decoding the
		// large (1920px) variant costs ~4x more CPU for no embedding gain.
		return "medium"
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
