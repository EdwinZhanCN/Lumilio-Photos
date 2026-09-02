package queue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"server/internal/artifact"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/pipeline"
	"server/internal/storage"
	"server/internal/utils/imagesource"

	"github.com/google/uuid"
)

type MLImageLoader interface {
	LoadMLImage(ctx context.Context, assetID uuid.UUID, purpose imagesource.Purpose, preprocessVersion string) (*imagesource.MLImage, error)
}

var ErrDerivedAssetStale = ErrAssetWorkStale

type DBMLImageLoader struct {
	Reader MLImageReader
	Files  *storage.RepositoryFSFactory
}

type MLImageReader interface {
	AssetWorkReader
	GetThumbnailByAssetAndSize(context.Context, repo.GetThumbnailByAssetAndSizeParams) (repo.Thumbnail, error)
	GetRepository(context.Context, uuid.UUID) (repo.Repository, error)
}

func NewDBMLImageLoader(reader MLImageReader, files *storage.RepositoryFSFactory) *DBMLImageLoader {
	return &DBMLImageLoader{Reader: reader, Files: files}
}

func (l *DBMLImageLoader) ValidateAssetWork(ctx context.Context, assetID, expectedContentID uuid.UUID) error {
	_, err := validateCurrentAssetWork(ctx, l.Reader, assetID, expectedContentID)
	return err
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
	if l == nil || l.Reader == nil {
		return nil, fmt.Errorf("ml image loader unavailable")
	}
	if preprocessVersion != "" && preprocessVersion != MLPreprocessVersionV1 {
		return nil, fmt.Errorf("unsupported ml preprocess version: %s", preprocessVersion)
	}

	asset, err := validateCurrentAssetWork(ctx, l.Reader, assetID, uuid.Nil)
	if err != nil {
		return nil, err
	}
	if dbtypes.AssetType(asset.Type) != dbtypes.AssetTypePhoto {
		return nil, fmt.Errorf("asset %s is not a photo: %s", asset.AssetID.String(), asset.Type)
	}
	thumbnailSize := mlThumbnailSize(purpose)
	thumbnail, err := l.Reader.GetThumbnailByAssetAndSize(ctx, repo.GetThumbnailByAssetAndSizeParams{
		AssetID: assetID,
		Size:    thumbnailSize,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDerivedAssetNotReady
	}
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
	if thumbnail.RepositoryID == uuid.Nil {
		return nil, fmt.Errorf("%s thumbnail has no repository", thumbnailSize)
	}
	repository, err := l.Reader.GetRepository(ctx, thumbnail.RepositoryID)
	if err != nil {
		return nil, fmt.Errorf("get thumbnail repository: %w", err)
	}
	repositoryFS, err := l.Files.Open(repository)
	if err != nil {
		return nil, err
	}
	defer repositoryFS.Close()
	file, err := repositoryFS.OpenPrivate(thumbnailPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrDerivedAssetNotReady
	}
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
	expected, err := derivedThumbnailPath(asset.ContentID, size)
	if err != nil || thumbnailPath.String() != expected {
		return fmt.Errorf("%w: asset=%s size=%s", ErrDerivedAssetStale, asset.AssetID, size)
	}
	return nil
}

func derivedThumbnailPath(contentID uuid.UUID, size string) (string, error) {
	if contentID == uuid.Nil {
		return "", ErrDerivedAssetStale
	}
	p, err := (artifact.Identity{SourceFence: contentID.String(), Stage: "derivatives", PipelineVersion: pipeline.AssetPipelineVersion, Name: size + ".webp"}).Path()
	if err != nil {
		return "", err
	}
	return p.String(), nil
}
