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
	LoadMLImage(ctx context.Context, assetID pgtype.UUID, purpose imagesource.Purpose, preprocessVersion string) ([]byte, error)
}

type DBMLImageLoader struct {
	Queries *repo.Queries
}

func NewDBMLImageLoader(queries *repo.Queries) *DBMLImageLoader {
	return &DBMLImageLoader{Queries: queries}
}

func (l *DBMLImageLoader) LoadMLImage(ctx context.Context, assetID pgtype.UUID, purpose imagesource.Purpose, preprocessVersion string) ([]byte, error) {
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

	thumbnail, err := l.Queries.GetThumbnailByAssetAndSize(ctx, repo.GetThumbnailByAssetAndSizeParams{
		AssetID: assetID,
		Size:    "large",
	})
	if err != nil {
		return nil, fmt.Errorf("get large thumbnail: %w", err)
	}

	thumbnailPath := filepath.Join(repository.Path, filepath.FromSlash(thumbnail.StoragePath))
	file, err := os.Open(thumbnailPath)
	if err != nil {
		return nil, fmt.Errorf("open large thumbnail: %w", err)
	}
	defer file.Close()

	imageData, err := imagesource.ProcessMLImageFromReader(file, purpose)
	if err != nil {
		return nil, fmt.Errorf("process large thumbnail for ml: %w", err)
	}

	return imageData, nil
}
