package cloud

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"server/internal/service"
	"server/internal/sourcing"
)

// CloudSyncConsumer reads IngestSource candidates from a CloudImportSource
// and feeds them through the SourceMaterializer for ingestion into the
// local repository.
type CloudSyncConsumer struct {
	source       sourcing.AssetSource
	materializer *sourcing.SourceMaterializer
	assetService service.AssetService
	state        SyncStateStore
	logger       *zap.Logger
}

// NewCloudSyncConsumer creates a consumer for a cloud import source.
func NewCloudSyncConsumer(
	source sourcing.AssetSource,
	materializer *sourcing.SourceMaterializer,
	assetService service.AssetService,
	state SyncStateStore,
	logger *zap.Logger,
) *CloudSyncConsumer {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &CloudSyncConsumer{
		source:       source,
		materializer: materializer,
		assetService: assetService,
		state:        state,
		logger:       logger.With(zap.String("component", "cloud_sync_consumer")),
	}
}

// Run starts the discovery → materialize loop.  It blocks until discovery
// completes or ctx is cancelled.
func (c *CloudSyncConsumer) Run(ctx context.Context) error {
	ch, err := c.source.Discover(ctx)
	if err != nil {
		return fmt.Errorf("start cloud discovery: %w", err)
	}

	for candidate := range ch {
		// Handle remote deletes (one-way sync tombstone)
		if action, _ := candidate.Metadata["action"].(string); action == "delete" {
			if err := c.handleRemoteDelete(ctx, candidate); err != nil {
				c.logger.Error("failed to handle remote delete",
					zap.String("remote_key", candidate.SourcePath),
					zap.Error(err),
				)
			}
			continue
		}

		// Materialize: staging → inbox → asset record → pipeline
		asset, err := c.materializer.Materialize(ctx, candidate)
		if err != nil {
			c.logger.Error("materialize cloud asset failed",
				zap.String("remote_key", candidate.Metadata["remote_key"].(string)),
				zap.String("filename", candidate.OriginalFilename),
				zap.Error(err),
			)
			// Clean up staging file on failure
			os.Remove(candidate.SourcePath)
			continue
		}

		// Mark as synced with asset_id for future tombstone reconciliation.
		// Only mark after successful materialize so failed imports are retried.
		if asset != nil {
			assetUUID := uuid.UUID(asset.AssetID.Bytes)
			provider := candidate.Metadata["provider"].(ProviderKind)
			remoteKey := candidate.Metadata["remote_key"].(string)
			etag := candidate.Metadata["remote_etag"].(string)

			if err := c.state.MarkFileSynced(ctx, candidate.RepositoryID, provider, remoteKey, etag, assetUUID); err != nil {
				c.logger.Warn("failed to mark cloud file as synced",
					zap.String("remote_key", remoteKey),
					zap.String("asset_id", assetUUID.String()),
					zap.Error(err),
				)
			}

			c.logger.Info("cloud asset ingested",
				zap.String("asset_id", assetUUID.String()),
				zap.String("remote_key", remoteKey),
			)
		}
	}

	return nil
}

// handleRemoteDelete soft-deletes the local asset mapped to a remote file
// that has been deleted in the cloud.
func (c *CloudSyncConsumer) handleRemoteDelete(ctx context.Context, candidate sourcing.IngestSource) error {
	provider, _ := candidate.Metadata["provider"].(ProviderKind)
	remoteKey := candidate.SourcePath

	assetID, err := c.state.GetAssetIDByRemoteKey(ctx, candidate.RepositoryID, provider, remoteKey)
	if err != nil {
		return fmt.Errorf("lookup asset id: %w", err)
	}
	if assetID == uuid.Nil {
		// Never synced, nothing to delete
		return nil
	}

	// Soft-delete via AssetService (moves file to trash + marks deleted)
	err = c.assetService.DeleteAsset(ctx, assetID)
	if err != nil {
		return fmt.Errorf("soft delete asset %s: %w", assetID.String(), err)
	}

	c.logger.Info("remote delete synced",
		zap.String("asset_id", assetID.String()),
		zap.String("remote_key", remoteKey),
	)
	return nil
}
