package processors

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

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
			zap.String("type", string(args.AssetType)),
			zap.Duration("duration", time.Since(start)),
		)
	}()
	asset, _, err := ap.loadAssetAndRepo(ctx, args.AssetID)
	if err != nil {
		return err
	}

	return ap.runTrackedAssetTask(
		ctx,
		args.AssetID,
		taskMetadata,
		"Extracting metadata",
		"Metadata extracted",
		func() error {
			fullPath := filepath.Join(args.RepoPath, args.StoragePath)
			switch args.AssetType {
			case dbtypes.AssetTypePhoto:
				return ap.extractPhotoMetadata(ctx, asset, fullPath)
			case dbtypes.AssetTypeVideo:
				info, err := ap.getVideoInfo(fullPath)
				if err != nil {
					return err
				}
				return ap.extractVideoMetadata(ctx, asset, fullPath, info)
			case dbtypes.AssetTypeAudio:
				info, err := ap.getAudioInfo(fullPath)
				if err != nil {
					return err
				}
				return ap.extractAudioMetadata(ctx, asset, fullPath, info)
			default:
				return fmt.Errorf("unsupported asset type for metadata: %s", args.AssetType)
			}
		},
	)
}

// extractPhotoMetadata extracts EXIF metadata for photos.
func (ap *AssetProcessor) extractPhotoMetadata(ctx context.Context, asset *repo.Asset, fullPath string) error {
	// EXIF extraction
	exifCfg := ap.createEXIFConfig()
	extractor := exif.NewExtractor(exifCfg)
	defer extractor.Close()

	f, err := os.Open(fullPath)
	if err != nil {
		return fmt.Errorf("open photo for exif: %w", err)
	}
	defer f.Close()

	req := &exif.StreamingExtractRequest{
		Reader:    f,
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

	// Parse dimensions and update asset
	// The dimensions in meta.Dimensions are already corrected by orientation
	re := regexp.MustCompile(`(\d+)\D+(\d+)`)
	if matches := re.FindStringSubmatch(meta.Dimensions); len(matches) == 3 {
		width, _ := strconv.ParseInt(matches[1], 10, 32)
		height, _ := strconv.ParseInt(matches[2], 10, 32)
		if err := ap.assetService.UpdateAssetDimensions(ctx, asset.AssetID.Bytes, int32(width), int32(height)); err != nil {
			return fmt.Errorf("update asset dimensions: %w", err)
		}
	}

	sm, err := dbtypes.MarshalMeta(meta)
	if err != nil {
		return fmt.Errorf("marshal photo metadata: %w", err)
	}
	if err := ap.assetService.UpdateAssetMetadataWithExifRaw(ctx, asset.AssetID.Bytes, sm, res.Raw); err != nil {
		return fmt.Errorf("update asset metadata: %w", err)
	}

	if hasValidLocationGPS(meta.GPSLatitude, meta.GPSLongitude) {
		ap.enqueueLocationClusterRebuild(ctx, asset)
	}
	ap.enqueueDetectStacks(ctx, asset)
	ap.enqueueLivePhotoMatcher(ctx, asset, meta.ContentIdentifier)

	return nil
}

func (ap *AssetProcessor) enqueueLocationClusterRebuild(ctx context.Context, asset *repo.Asset) {
	if ap == nil || ap.queueClient == nil || asset == nil || !asset.RepositoryID.Valid {
		return
	}
	repositoryID := asset.RepositoryID.String()
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

	repositoryID := uuid.UUID(asset.RepositoryID.Bytes).String()
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
	if ap == nil || ap.queueClient == nil || asset == nil || !asset.AssetID.Valid {
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
			zap.String("asset_id", uuid.UUID(asset.AssetID.Bytes).String()),
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
func (ap *AssetProcessor) loadAssetAndRepo(ctx context.Context, assetID pgtype.UUID) (*repo.Asset, repo.Repository, error) {
	asset, err := ap.queries.GetAssetByID(ctx, assetID)
	if err != nil {
		return nil, repo.Repository{}, fmt.Errorf("get asset: %w", err)
	}
	repository, err := ap.queries.GetRepository(ctx, asset.RepositoryID)
	if err != nil {
		return nil, repo.Repository{}, fmt.Errorf("get repository: %w", err)
	}
	return &asset, repository, nil
}
