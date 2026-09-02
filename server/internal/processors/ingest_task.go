package processors

import (
	"context"
	"time"

	"go.uber.org/zap"

	"server/internal/db/repo"
	"server/internal/queue/jobs"
)

// IngestAsset converts an upload payload into an IngestSource and delegates to the
// SourceMaterializer for validation, staging→inbox commit, asset creation, and pipeline enqueuing.
// Audit logging is handled by the materializer.
func (ap *AssetProcessor) IngestAsset(ctx context.Context, task jobs.IngestAssetArgs) (*repo.Asset, error) {
	start := time.Now()
	defer func() {
		ap.logger.Debug("ingest_task",
			zap.String("commit_id", task.CommitID.String()),
			zap.Duration("duration", time.Since(start)),
		)
	}()
	return ap.materializer.MaterializeCommit(ctx, task.CommitID)
}
