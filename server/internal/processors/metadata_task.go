package processors

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

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

	// Update photo metadata
	if meta, ok := res.Metadata.(*dbtypes.PhotoSpecificMetadata); ok {
		meta.IsRAW = file.IsRAWFile(asset.OriginalFilename)

		// Parse dimensions and update asset
		// The dimensions in meta.Dimensions are already corrected by orientation
		re := regexp.MustCompile(`(\d+)\D+(\d+)`)
		if matches := re.FindStringSubmatch(meta.Dimensions); len(matches) == 3 {
			width, _ := strconv.ParseInt(matches[1], 10, 32)
			height, _ := strconv.ParseInt(matches[2], 10, 32)
			_ = ap.assetService.UpdateAssetDimensions(ctx, asset.AssetID.Bytes, int32(width), int32(height))
		}

		sm, err := dbtypes.MarshalMeta(meta)
		if err == nil {
			_ = ap.assetService.UpdateAssetMetadataWithExifRaw(ctx, asset.AssetID.Bytes, sm, res.Raw)
			if hasValidLocationGPS(meta.GPSLatitude, meta.GPSLongitude) {
				ap.enqueueLocationClusterRebuild(ctx, asset)
			}
			ap.enqueueDetectStacks(ctx, asset)
		}
	}

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
