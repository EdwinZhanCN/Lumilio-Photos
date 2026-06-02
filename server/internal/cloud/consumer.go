package cloud

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"server/internal/sourcing"
)

// CloudSyncConsumer reads IngestSource candidates from a CloudImportSource
// and feeds them through the SourceMaterializer for ingestion into the
// local repository.
type CloudSyncConsumer struct {
	source       sourcing.AssetSource
	materializer *sourcing.SourceMaterializer
	state        SyncStateStore
	onProgress   func(delta ImportProgressDelta)
	logger       *zap.Logger
}

// NewCloudSyncConsumer creates a consumer for a cloud import source.
func NewCloudSyncConsumer(
	source sourcing.AssetSource,
	materializer *sourcing.SourceMaterializer,
	state SyncStateStore,
	onProgress func(delta ImportProgressDelta),
	logger *zap.Logger,
) *CloudSyncConsumer {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &CloudSyncConsumer{
		source:       source,
		materializer: materializer,
		state:        state,
		onProgress:   onProgress,
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
			c.progress(ImportProgressDelta{Failed: 1})
			continue
		}

		provider := candidate.Metadata["provider"].(ProviderKind)
		remoteKey := candidate.Metadata["remote_key"].(string)
		etag := candidate.Metadata["remote_etag"].(string)

		// Record the synced etag so subsequent runs skip this remote file via
		// IsFileSynced. We do this for both freshly ingested assets and content
		// that the materializer deduped away (asset == nil); otherwise deduped
		// remote keys are never recorded and get re-downloaded on every run.
		var assetUUID uuid.UUID
		if asset != nil {
			assetUUID = uuid.UUID(asset.AssetID.Bytes)
		}
		if err := c.state.MarkFileSynced(ctx, candidate.RepositoryID, provider, remoteKey, etag, assetUUID); err != nil {
			c.logger.Warn("failed to mark cloud file as synced",
				zap.String("remote_key", remoteKey),
				zap.String("asset_id", assetUUID.String()),
				zap.Error(err),
			)
		}

		if asset != nil {
			c.progress(ImportProgressDelta{Imported: 1})
			c.logger.Info("cloud asset ingested",
				zap.String("asset_id", assetUUID.String()),
				zap.String("remote_key", remoteKey),
			)
		} else {
			// Downloaded but deduplicated (already present/unchanged).
			c.progress(ImportProgressDelta{Skipped: 1})
			c.logger.Debug("cloud asset deduplicated",
				zap.String("remote_key", remoteKey),
			)
		}
	}

	return nil
}

func (c *CloudSyncConsumer) progress(delta ImportProgressDelta) {
	if c.onProgress != nil {
		c.onProgress(delta)
	}
}
