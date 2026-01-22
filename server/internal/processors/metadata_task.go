package processors

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/utils/exif"
	"server/internal/utils/raw"
)

// ProcessMetadataTask handles EXIF/ffprobe metadata extraction only.
func (ap *AssetProcessor) ProcessMetadataTask(ctx context.Context, args jobs.MetadataArgs) error {
	asset, _, err := ap.loadAssetAndRepo(ctx, args.AssetID)
	if err != nil {
		return err
	}

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
		meta.IsRAW = raw.IsRAWFile(asset.OriginalFilename)

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
			_ = ap.assetService.UpdateAssetMetadata(ctx, asset.AssetID.Bytes, sm)
		}
	}

	return nil
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
