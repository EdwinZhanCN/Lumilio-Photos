package queue

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

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
	if asset.StoragePath == nil || strings.TrimSpace(*asset.StoragePath) == "" {
		return nil, fmt.Errorf("asset %s has no storage path", asset.AssetID.String())
	}
	if !asset.RepositoryID.Valid {
		return nil, fmt.Errorf("asset %s has no repository", asset.AssetID.String())
	}

	repository, err := l.Queries.GetRepository(ctx, asset.RepositoryID)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	fullPath := filepath.Join(repository.Path, filepath.FromSlash(*asset.StoragePath))
	imageData, err := imagesource.ProcessMLImage(ctx, fullPath, asset.OriginalFilename, purpose)
	if err != nil {
		return nil, fmt.Errorf("process ml image source: %w", err)
	}
	return imageData, nil
}
