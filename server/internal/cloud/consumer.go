package cloud

import (
	"context"
	"fmt"

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

// Run materializes candidates synchronously so the source cannot advance its
// pagination cursor ahead of the repository and SQLite commit.
func (c *CloudSyncConsumer) Run(ctx context.Context) error {
	return c.source.ForEach(ctx, func(candidate sourcing.IngestSource) error {
		// Materialize: staging → inbox → asset record → pipeline
		asset, err := c.materializer.MaterializeStaged(ctx, candidate)
		if err != nil {
			remoteKey, _ := candidate.Metadata["remote_key"].(string)
			c.logger.Error("materialize cloud asset failed",
				zap.String("remote_key", remoteKey),
				zap.String("filename", candidate.OriginalFilename),
				zap.Error(err),
			)
			c.progress(ImportProgressDelta{Failed: 1})
			return fmt.Errorf("materialize cloud asset: %w", err)
		}

		provider, providerOK := candidate.Metadata["provider"].(ProviderKind)
		remoteKey, keyOK := candidate.Metadata["remote_key"].(string)
		etag, etagOK := candidate.Metadata["remote_etag"].(string)
		if !providerOK || !keyOK || !etagOK {
			return fmt.Errorf("cloud candidate metadata is incomplete")
		}

		// Record the synced etag so subsequent runs skip this remote file via
		// IsFileSynced. We do this for both freshly ingested assets and content
		// that the materializer deduped away (asset == nil); otherwise deduped
		// remote keys are never recorded and get re-downloaded on every run.
		var assetUUID uuid.UUID
		if asset != nil {
			assetUUID = asset.AssetID
		}
		if err := c.state.MarkFileSynced(ctx, candidate.RepositoryID, provider, remoteKey, etag, assetUUID); err != nil {
			c.logger.Error("failed to mark cloud file as synced",
				zap.String("remote_key", remoteKey),
				zap.String("asset_id", assetUUID.String()),
				zap.Error(err),
			)
			return fmt.Errorf("mark cloud file synced: %w", err)
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
		return nil
	})
}

func (c *CloudSyncConsumer) progress(delta ImportProgressDelta) {
	if c.onProgress != nil {
		c.onProgress(delta)
	}
}
