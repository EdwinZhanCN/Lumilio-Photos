package processors

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/google/uuid"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/utils/exif"
	"server/internal/utils/file"

	"go.uber.org/zap"
)

// MetadataResult is the immutable output of EXIF/ffprobe extraction. It is
// passed to the commit coordinator; no catalog writer is carried through the
// compute boundary.
type MetadataResult struct {
	AssetID, SourceContentID uuid.UUID
	Metadata                 dbtypes.SpecificMetadata
	Common                   dbtypes.CommonMetadata
	ExifRaw                  dbtypes.JSON
	ComponentRelation        string
}

// ComputeMetadataTask handles EXIF/ffprobe metadata extraction only.
func (ap *AssetProcessor) ComputeMetadataTask(ctx context.Context, args MetadataArgs) (MetadataResult, error) {
	start := time.Now()
	defer func() {
		ap.logger.Debug("metadata_task",
			zap.String("asset_id", args.AssetID.String()),
			zap.Duration("duration", time.Since(start)),
		)
	}()
	source, err := ap.resolveCurrentAssetSource(ctx, args.AssetID, args.ExpectedContentID)
	if err != nil {
		if errors.Is(err, ErrAssetSourceStale) {
			return MetadataResult{}, nil
		}
		return MetadataResult{}, err
	}
	defer source.Close()
	asset := source.asset
	assetType := dbtypes.AssetType(asset.Type)
	original, err := source.files.OpenMedia(source.path)
	if err != nil {
		return MetadataResult{}, err
	}
	defer original.Close()

	switch assetType {
	case dbtypes.AssetTypePhoto:
		return ap.extractPhotoMetadata(ctx, asset, source.content.FileSize, original)
	case dbtypes.AssetTypeVideo:
		info, err := ap.getVideoInfo(source.localPath)
		if err != nil {
			return MetadataResult{}, err
		}
		return ap.extractVideoMetadata(ctx, asset, source.content.FileSize, original, info)
	case dbtypes.AssetTypeAudio:
		info, err := ap.getAudioInfo(source.localPath)
		if err != nil {
			return MetadataResult{}, err
		}
		return ap.extractAudioMetadata(ctx, asset, source.content.FileSize, original, info)
	default:
		return MetadataResult{}, fmt.Errorf("unsupported asset type for metadata: %s", assetType)
	}
}

// extractPhotoMetadata extracts EXIF metadata for photos.
func (ap *AssetProcessor) extractPhotoMetadata(ctx context.Context, asset *repo.Asset, fileSize int64, reader io.Reader) (MetadataResult, error) {
	// EXIF extraction
	exifCfg := ap.createEXIFConfig()
	extractor := exif.NewExtractor(exifCfg)
	defer extractor.Close()

	req := &exif.StreamingExtractRequest{
		Reader:    reader,
		AssetType: dbtypes.AssetTypePhoto,
		Filename:  asset.OriginalFilename,
		Size:      fileSize,
	}

	res, err := extractor.ExtractFromStream(ctx, req)
	if err != nil {
		return MetadataResult{}, fmt.Errorf("extract exif: %w", err)
	}
	// Defensive check: the extractor may store an error in the result even when
	// the top-level error is nil (e.g. exiftool timeout, corrupt output).
	if res.Error != nil {
		return MetadataResult{}, fmt.Errorf("extract exif: %w", res.Error)
	}

	// Update photo metadata
	meta, ok := res.Metadata.(*dbtypes.PhotoSpecificMetadata)
	if !ok {
		return MetadataResult{}, fmt.Errorf("unexpected metadata type for photo: %T", res.Metadata)
	}
	meta.IsRAW = file.IsRAWFile(asset.OriginalFilename)

	sm, err := dbtypes.MarshalMeta(meta)
	if err != nil {
		return MetadataResult{}, fmt.Errorf("marshal photo metadata: %w", err)
	}
	relation := repo.InitialMediaRelation(&file.ValidationResult{IsRAW: meta.IsRAW, MimeType: asset.MimeType}, asset.OriginalFilename)
	return MetadataResult{AssetID: asset.AssetID, SourceContentID: asset.ContentID, Metadata: sm, Common: res.Common, ExifRaw: dbtypes.JSON(res.Raw), ComponentRelation: string(relation)}, nil
}

func hasValidLocationGPS(latitude, longitude *float64) bool {
	if latitude == nil || longitude == nil {
		return false
	}
	lat := *latitude
	lng := *longitude
	return !math.IsNaN(lat) &&
		!math.IsInf(lat, 0) &&
		!math.IsNaN(lng) &&
		!math.IsInf(lng, 0) &&
		lat >= -90 && lat <= 90 &&
		lng >= -180 && lng <= 180
}

// loadAssetAndRepo chooses an active Location's repository for repository-
// private derived work. Asset itself has no repository ownership.
func (ap *AssetProcessor) loadAssetAndRepo(ctx context.Context, assetID uuid.UUID) (*repo.Asset, repo.Repository, error) {
	return ap.loadAssetAndRepoForContent(ctx, assetID, uuid.Nil)
}

func (ap *AssetProcessor) loadAssetAndRepoForContent(
	ctx context.Context,
	assetID uuid.UUID,
	expectedContentID uuid.UUID,
) (*repo.Asset, repo.Repository, error) {
	asset, err := ap.reader.GetAssetByID(ctx, assetID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.Repository{}, ErrAssetSourceStale
	}
	if err != nil {
		return nil, repo.Repository{}, fmt.Errorf("get asset: %w", err)
	}
	if expectedContentID != uuid.Nil && asset.ContentID != expectedContentID {
		return nil, repo.Repository{}, ErrAssetSourceStale
	}
	var repositoryID uuid.UUID
	if err := ap.readerDatabase.QueryRowContext(ctx, `
		SELECT repository_id
		FROM active_asset_occurrences
		WHERE asset_id = ?
		ORDER BY repository_id, node_id
		LIMIT 1`, assetID).Scan(&repositoryID); errors.Is(err, sql.ErrNoRows) {
		return nil, repo.Repository{}, ErrAssetSourceStale
	} else if err != nil {
		return nil, repo.Repository{}, fmt.Errorf("asset has no active Location: %w", err)
	}
	repository, err := ap.reader.GetRepository(ctx, repositoryID)
	if err != nil {
		return nil, repo.Repository{}, fmt.Errorf("get repository: %w", err)
	}
	return &asset, repository, nil
}
