package processors

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/utils/exif"
	"server/internal/utils/file"

	"go.uber.org/zap"
)

// ProcessMetadataTask handles EXIF/ffprobe metadata extraction only.
func (ap *AssetProcessor) ProcessMetadataTask(ctx context.Context, args jobs.MetadataArgs) error {
	start := time.Now()
	defer func() {
		ap.logger.Debug("metadata_task",
			zap.String("asset_id", args.AssetID.String()),
			zap.Duration("duration", time.Since(start)),
		)
	}()
	source, err := ap.resolveCurrentAssetSource(ctx, args.AssetID, args.ObservationToken, args.ExpectedContentHash)
	if err != nil {
		if errors.Is(err, ErrAssetSourceStale) {
			return nil
		}
		return err
	}
	defer source.Close()
	asset := source.asset
	assetType := dbtypes.AssetType(asset.Type)
	original, err := source.files.OpenMedia(source.path)
	if err != nil {
		return err
	}
	defer original.Close()

	return ap.runTrackedAssetTask(
		ctx,
		args.AssetID,
		taskMetadata,
		"Extracting metadata",
		"Metadata extracted",
		func() error {
			switch assetType {
			case dbtypes.AssetTypePhoto:
				return ap.extractPhotoMetadata(ctx, asset, original)
			case dbtypes.AssetTypeVideo:
				info, err := ap.getVideoInfo(source.localPath)
				if err != nil {
					return err
				}
				return ap.extractVideoMetadata(ctx, asset, original, info)
			case dbtypes.AssetTypeAudio:
				info, err := ap.getAudioInfo(source.localPath)
				if err != nil {
					return err
				}
				return ap.extractAudioMetadata(ctx, asset, original, info)
			default:
				return fmt.Errorf("unsupported asset type for metadata: %s", assetType)
			}
		},
	)
}

// extractPhotoMetadata extracts EXIF metadata for photos.
func (ap *AssetProcessor) extractPhotoMetadata(ctx context.Context, asset *repo.Asset, reader io.Reader) error {
	// EXIF extraction
	exifCfg := ap.createEXIFConfig()
	extractor := exif.NewExtractor(exifCfg)
	defer extractor.Close()

	req := &exif.StreamingExtractRequest{
		Reader:    reader,
		AssetType: dbtypes.AssetTypePhoto,
		Filename:  asset.OriginalFilename,
		Size:      asset.FileSize,
	}

	res, err := extractor.ExtractFromStream(ctx, req)
	if err != nil {
		return fmt.Errorf("extract exif: %w", err)
	}
	// Defensive check: the extractor may store an error in the result even when
	// the top-level error is nil (e.g. exiftool timeout, corrupt output).
	if res.Error != nil {
		return fmt.Errorf("extract exif: %w", res.Error)
	}

	// Update photo metadata
	meta, ok := res.Metadata.(*dbtypes.PhotoSpecificMetadata)
	if !ok {
		return fmt.Errorf("unexpected metadata type for photo: %T", res.Metadata)
	}
	meta.IsRAW = file.IsRAWFile(asset.OriginalFilename)

	sm, err := dbtypes.MarshalMeta(meta)
	if err != nil {
		return fmt.Errorf("marshal photo metadata: %w", err)
	}
	if err := ap.assetService.UpdateAssetExtractedMetadata(ctx, asset.AssetID, sm, res.Common, res.Raw); err != nil {
		return fmt.Errorf("update asset metadata: %w", err)
	}

	ap.reconcileComponentRelation(ctx, asset, meta.IsRAW, asset.MimeType)

	if hasValidLocationGPS(res.Common.GPSLatitude, res.Common.GPSLongitude) {
		ap.enqueueLocationClusterRebuild(ctx, asset)
	}
	ap.enqueueLivePhotoMatcher(ctx, asset, meta.ContentIdentifier)
	ap.enqueueDetectStacks(ctx, asset)

	return nil
}

// reconcileComponentRelation re-derives the media item component relation from
// metadata-confirmed facts after extraction. Relations assigned by stack or
// live-photo matching (live_photo_*, edited_version) are never overwritten,
// and the SQL is a no-op when the stored relation already matches, so metadata
// retries stay idempotent.
func (ap *AssetProcessor) reconcileComponentRelation(ctx context.Context, asset *repo.Asset, isRAW bool, mimeType string) {
	if ap == nil || ap.queries == nil || asset == nil {
		return
	}
	relation := repo.InitialMediaRelation(&file.ValidationResult{IsRAW: isRAW, MimeType: mimeType}, asset.OriginalFilename)
	if err := ap.queries.ReconcileMediaItemComponentRelation(ctx, repo.ReconcileMediaItemComponentRelationParams{
		AssetID:  asset.AssetID,
		Relation: string(relation),
	}); err != nil && ap.logger != nil {
		ap.logger.Warn("failed to reconcile media item component relation",
			zap.String("asset_id", asset.AssetID.String()),
			zap.String("relation", string(relation)),
			zap.Error(err),
		)
	}
}

func (ap *AssetProcessor) enqueueLocationClusterRebuild(ctx context.Context, asset *repo.Asset) {
	if ap == nil || ap.queueClient == nil || asset == nil || !asset.RepositoryID.Valid {
		return
	}
	repositoryID := asset.RepositoryID.UUID.String()
	args := jobs.RebuildLocationClustersArgs{
		RepositoryID: &repositoryID,
		OwnerID:      asset.OwnerID,
	}
	opts := args.InsertOpts()
	opts.Queue = "rebuild_location_clusters"
	if _, err := ap.queueClient.Insert(ctx, args, &opts); err != nil && ap.logger != nil {
		ap.logger.Warn("failed to enqueue location cluster rebuild", zap.Error(err))
	}
}

func (ap *AssetProcessor) enqueueDetectStacks(ctx context.Context, asset *repo.Asset) {
	if ap == nil || ap.queueClient == nil || asset == nil || !asset.RepositoryID.Valid {
		return
	}

	repositoryID := asset.RepositoryID.UUID.String()
	args := jobs.DetectStacksArgs{
		RepositoryID: repositoryID,
	}
	opts := args.InsertOpts()
	opts.Queue = "detect_stacks"

	if _, err := ap.queueClient.Insert(ctx, args, &opts); err != nil && ap.logger != nil {
		ap.logger.Warn("failed to enqueue detect stacks after metadata extraction",
			zap.String("repository_id", repositoryID),
			zap.Error(err),
		)
	}
}

func (ap *AssetProcessor) enqueueLivePhotoMatcher(ctx context.Context, asset *repo.Asset, contentIdentifier string) {
	if ap == nil || ap.queueClient == nil || asset == nil || asset.AssetID == uuid.Nil {
		return
	}
	if strings.TrimSpace(contentIdentifier) == "" {
		return
	}

	args := jobs.LivePhotoMatchArgs{AssetID: asset.AssetID}
	opts := args.InsertOpts()
	opts.Queue = "match_live_photo"

	if _, err := ap.queueClient.Insert(ctx, args, &opts); err != nil && ap.logger != nil {
		ap.logger.Warn("failed to enqueue live photo matcher after metadata extraction",
			zap.String("asset_id", asset.AssetID.String()),
			zap.Error(err),
		)
	}
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

// loadAssetAndRepo loads asset and repository by asset ID.
func (ap *AssetProcessor) loadAssetAndRepo(ctx context.Context, assetID uuid.UUID) (*repo.Asset, repo.Repository, error) {
	asset, err := ap.queries.GetAssetByID(ctx, assetID)
	if err != nil {
		return nil, repo.Repository{}, fmt.Errorf("get asset: %w", err)
	}
	if !asset.RepositoryID.Valid {
		return nil, repo.Repository{}, fmt.Errorf("asset has no repository")
	}
	repository, err := ap.queries.GetRepository(ctx, asset.RepositoryID.UUID)
	if err != nil {
		return nil, repo.Repository{}, fmt.Errorf("get repository: %w", err)
	}
	return &asset, repository, nil
}
