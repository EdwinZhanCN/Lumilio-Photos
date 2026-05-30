package handler

import (
	"context"
	"fmt"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

func bioClipRuntimeAvailable(ctx context.Context, settingsService service.SettingsService, lumenService service.LumenService) (bool, error) {
	if settingsService == nil {
		return false, nil
	}

	mlConfig, err := settingsService.GetEffectiveMLConfig(ctx)
	if err != nil {
		return false, fmt.Errorf("load ML settings: %w", err)
	}
	if !mlConfig.BioCLIPEnabled {
		return false, nil
	}

	if lumenService == nil {
		return false, nil
	}
	return service.IsIndexingTaskRuntimeAvailable(lumenService, service.AssetIndexingTaskBioCLIP), nil
}

func enqueueBioClipAsset(ctx context.Context, queueClient *river.Client[pgx.Tx], asset repo.Asset) error {
	if queueClient == nil {
		return fmt.Errorf("queue client is not configured")
	}

	_, err := queueClient.Insert(ctx, jobs.ProcessBioClipArgs{
		AssetID:           asset.AssetID,
		PreprocessVersion: jobs.MLPreprocessVersionV1,
	}, &river.InsertOpts{Queue: "process_bioclip"})
	if err != nil {
		return fmt.Errorf("enqueue BioCLIP job: %w", err)
	}
	return nil
}

func shouldQueueBioClipForAlbumAsset(album repo.Album, asset repo.Asset) bool {
	return album.AlbumType == repo.AlbumTypeBio && asset.Type == string(dbtypes.AssetTypePhoto)
}
