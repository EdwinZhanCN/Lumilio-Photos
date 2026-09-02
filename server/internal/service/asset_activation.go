package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/pipeline"
	"server/internal/workqos"
)

// ApplyAssetActivationTx publishes the logical media item and requests the
// fenced asset pipeline for an immutable repository publication. The caller
// owns the transaction; this is intentionally usable by the commit coordinator
// and by the source-materialization recovery boundary.
func ApplyAssetActivationTx(ctx context.Context, tx *sql.Tx, queries *repo.Queries, repositoryID, nodeID, assetID, contentID uuid.UUID) error {
	if tx == nil || queries == nil || repositoryID == uuid.Nil || nodeID == uuid.Nil || assetID == uuid.Nil || contentID == uuid.Nil {
		return errors.New("asset activation transaction is incomplete")
	}
	asset, err := queries.GetAssetByIDAny(ctx, assetID)
	if errors.Is(err, sql.ErrNoRows) || asset.IsDeleted {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load materialized asset: %w", err)
	}
	if asset.ContentID != contentID {
		return nil
	}
	node, err := queries.GetRepositoryNode(ctx, repo.GetRepositoryNodeParams{RepositoryID: repositoryID, NodeID: nodeID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("load materialized repository node: %w", err)
	}
	if node.Lifecycle != "active" {
		return nil
	}
	if node.RepositoryID != repositoryID {
		return nil
	}
	location, err := queries.GetActiveAssetLocationByNode(ctx, nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("load active asset location: %w", err)
	}
	if location.AssetID != asset.AssetID {
		return nil
	}
	if _, err := queries.GetMediaItemByAssetID(ctx, asset.AssetID); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		createdAt := dbtypes.NewTimestamp(time.Now().UTC())
		mediaItemID := uuid.New()
		if err := queries.CreateMediaItemForAsset(ctx, repo.CreateMediaItemForAssetParams{
			MediaItemID:  mediaItemID,
			OwnerID:      asset.OwnerID,
			RepositoryID: uuid.NullUUID{UUID: repositoryID, Valid: true},
			MediaKind:    strings.ToLower(asset.Type),
			AssetID:      uuid.NullUUID{UUID: asset.AssetID, Valid: true},
			CreatedAt:    createdAt,
		}); err != nil {
			return err
		}
		if err := queries.AttachAssetToMediaItem(ctx, repo.AttachAssetToMediaItemParams{
			AssetID: asset.AssetID, MediaItemID: mediaItemID, Relation: "original", CreatedAt: createdAt,
		}); err != nil {
			return err
		}
	}

	stages := []pipeline.Stage{pipeline.StageAnalyze, pipeline.StageDerivatives, pipeline.StageEnrich}
	if dbtypes.AssetType(asset.Type) == dbtypes.AssetTypeVideo || dbtypes.AssetType(asset.Type) == dbtypes.AssetTypeAudio {
		stages = append(stages, pipeline.StageTranscode)
	}
	missing := make([]pipeline.Stage, 0, len(stages))
	reset := false
	for _, stage := range stages {
		var fence, version string
		if err := tx.QueryRowContext(ctx, `SELECT source_content_id,pipeline_version FROM asset_pipeline_state WHERE asset_id=? AND stage=?`, asset.AssetID.String(), string(stage)).Scan(&fence, &version); errors.Is(err, sql.ErrNoRows) {
			missing = append(missing, stage)
		} else if err != nil {
			return err
		} else if fence != asset.ContentID.String() || version != pipeline.AssetPipelineVersion {
			reset = true
		}
	}
	if reset {
		return pipeline.RequestAssetStagesTx(ctx, tx, asset.AssetID, asset.ContentID, stages,
			pipeline.AssetPipelineVersion, workqos.Background, nodeID)
	}
	if len(missing) == 0 {
		return nil
	}
	return pipeline.RequestAssetStagesTx(ctx, tx, asset.AssetID, asset.ContentID, missing,
		pipeline.AssetPipelineVersion, workqos.Background, nodeID)
}
