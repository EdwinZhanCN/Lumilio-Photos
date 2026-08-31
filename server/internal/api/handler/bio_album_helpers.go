package handler

import (
	"context"
	"database/sql"
	"fmt"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/pipeline"
	"server/internal/service"

	"github.com/google/uuid"
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

func requestBioClipAsset(ctx context.Context, writer *catalogtx.Writer, asset repo.Asset) error {
	if writer == nil {
		return fmt.Errorf("catalog writer is not configured")
	}
	err := writer.Transact(ctx, catalogtx.OperationAssetReprocess, nil, func(tx *sql.Tx) error {
		return pipeline.RequestAssetStagesTx(ctx, tx, asset.AssetID, asset.ContentID, []pipeline.Stage{pipeline.StageEnrich}, pipeline.AssetPipelineVersion, pipeline.AdmissionInteractive, uuid.New())
	})
	if err != nil {
		return fmt.Errorf("request asset enrichment: %w", err)
	}
	return nil
}

func shouldQueueBioClipForAlbumAsset(album repo.Album, asset repo.Asset) bool {
	return album.AlbumType == repo.AlbumTypeBio && asset.Type == string(dbtypes.AssetTypePhoto)
}
