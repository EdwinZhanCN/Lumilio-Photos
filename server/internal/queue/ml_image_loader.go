package queue

import (
	"context"
	"errors"
	"fmt"
	"path"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/storage"
	"server/internal/utils/imagesource"

	"github.com/google/uuid"
)

type MLImageLoader interface {
	LoadMLImage(ctx context.Context, assetID uuid.UUID, purpose imagesource.Purpose, preprocessVersion string) (*imagesource.MLImage, error)
}

var ErrDerivedAssetStale = errors.New("derived asset does not match current content")

type DBMLImageLoader struct {
	Queries *repo.Queries
	Files   *storage.RepositoryFSFactory
}

func NewDBMLImageLoader(queries *repo.Queries, files *storage.RepositoryFSFactory) *DBMLImageLoader {
	return &DBMLImageLoader{Queries: queries, Files: files}
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

func (l *DBMLImageLoader) LoadMLImage(ctx context.Context, assetID uuid.UUID, purpose imagesource.Purpose, preprocessVersion string) (*imagesource.MLImage, error) {
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

	repository, err := l.Queries.GetRepository(ctx, asset.RepositoryID.UUID)
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

	thumbnailPath, err := storage.ParsePrivateRepositoryPath(thumbnail.StoragePath)
	if err != nil {
		return nil, err
	}
	if err := validateThumbnailContent(asset, thumbnailSize, thumbnailPath); err != nil {
		return nil, err
	}
	repositoryFS, err := l.Files.Open(repository)
	if err != nil {
		return nil, err
	}
	defer repositoryFS.Close()
	file, err := repositoryFS.OpenPrivate(thumbnailPath)
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

func validateThumbnailContent(asset repo.Asset, size string, thumbnailPath storage.RepositoryPath) error {
	expected := fmt.Sprintf("%s_%s.webp", asset.ContentHash, size)
	if asset.ContentHash == "" || path.Base(thumbnailPath.String()) != expected {
		return fmt.Errorf("%w: asset=%s size=%s", ErrDerivedAssetStale, asset.AssetID, size)
	}
	return nil
}
