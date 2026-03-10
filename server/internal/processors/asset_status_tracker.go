package processors

import (
	"context"
	"log"

	"server/internal/db/dbtypes"
	statusdb "server/internal/db/dbtypes/status"

	"github.com/jackc/pgx/v5/pgtype"
)

const (
	taskMetadata  = "metadata_asset"
	taskThumbnail = "thumbnail_asset"
	taskTranscode = "transcode_asset"
)

func trackedPipelineTasks(assetType dbtypes.AssetType) []string {
	switch assetType {
	case dbtypes.AssetTypePhoto:
		return []string{taskMetadata, taskThumbnail}
	case dbtypes.AssetTypeVideo:
		return []string{taskMetadata, taskThumbnail, taskTranscode}
	case dbtypes.AssetTypeAudio:
		return []string{taskMetadata, taskTranscode}
	default:
		return []string{taskMetadata}
	}
}

func buildTrackedProcessingStatus(assetType dbtypes.AssetType, message string) ([]byte, error) {
	status := statusdb.NewTrackedProcessingStatus(message, trackedPipelineTasks(assetType))
	return status.ToJSONB()
}

func (ap *AssetProcessor) runTrackedAssetTask(
	ctx context.Context,
	assetID pgtype.UUID,
	taskName string,
	startMessage string,
	successMessage string,
	fn func() error,
) error {
	ap.tryMutateAssetStatus(ctx, assetID, func(status *statusdb.AssetStatus) {
		status.MarkTaskProcessing(taskName, startMessage)
	})

	err := fn()
	if err != nil {
		ap.tryMutateAssetStatus(ctx, assetID, func(status *statusdb.AssetStatus) {
			status.MarkTaskFailed(taskName, err.Error(), err.Error())
		})
		return err
	}

	ap.tryMutateAssetStatus(ctx, assetID, func(status *statusdb.AssetStatus) {
		status.MarkTaskComplete(taskName, successMessage)
	})
	return nil
}

func (ap *AssetProcessor) tryMutateAssetStatus(
	ctx context.Context,
	assetID pgtype.UUID,
	mutate func(*statusdb.AssetStatus),
) {
	if err := ap.queries.MutateAssetStatus(ctx, assetID, func(current statusdb.AssetStatus) (statusdb.AssetStatus, error) {
		mutate(&current)
		return current, nil
	}); err != nil {
		log.Printf("Failed to mutate asset %s status: %v", assetID.String(), err)
	}
}
