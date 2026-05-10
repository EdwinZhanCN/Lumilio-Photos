package processors

import (
	"context"
	"fmt"
	"path/filepath"

	"server/config"
	"server/internal/db/dbtypes"
	"server/internal/queue/jobs"
)

// ProcessTranscodeTask handles video/audio transcoding.
func (ap *AssetProcessor) ProcessTranscodeTask(ctx context.Context, args jobs.TranscodeArgs) error {
	asset, repository, err := ap.loadAssetAndRepo(ctx, args.AssetID)
	if err != nil {
		return err
	}

	transcodeCfg := config.LoadTranscodeConfig()

	return ap.runTrackedAssetTask(
		ctx,
		args.AssetID,
		taskTranscode,
		"Transcoding asset",
		"Transcoding completed",
		func() error {
			fullPath := filepath.Join(args.RepoPath, args.StoragePath)
			switch args.AssetType {
			case dbtypes.AssetTypeVideo:
				info, err := ap.getVideoInfo(fullPath)
				if err != nil {
					return err
				}
				return ap.transcodeVideoSmart(ctx, repository.Path, asset, fullPath, info, transcodeCfg)
			case dbtypes.AssetTypeAudio:
				info, err := ap.getAudioInfo(fullPath)
				if err != nil {
					return err
				}
				return ap.transcodeAudioSmart(ctx, repository.Path, asset, fullPath, info)
			case dbtypes.AssetTypePhoto:
				// No transcode needed for photos
				return nil
			default:
				return fmt.Errorf("unsupported asset type for transcode: %s", args.AssetType)
			}
		},
	)
}
