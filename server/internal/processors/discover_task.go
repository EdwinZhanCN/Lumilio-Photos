package processors

import (
	"context"
	"fmt"

	"server/internal/queue/jobs"
)

// ProcessDiscoveredAsset delegates one generation-bound file-index candidate
// to the SourceMaterializer. Deletion and move decisions are owned by Scanner.
func (ap *AssetProcessor) ProcessDiscoveredAsset(ctx context.Context, args jobs.DiscoverAssetArgs) error {
	if ap == nil || ap.materializer == nil {
		return fmt.Errorf("discovery materializer unavailable")
	}
	_, err := ap.materializer.MaterializeDiscovered(
		ctx,
		args.RepositoryID,
		args.StoragePath,
		args.ScanID,
		args.ObservationToken,
	)
	return err
}
