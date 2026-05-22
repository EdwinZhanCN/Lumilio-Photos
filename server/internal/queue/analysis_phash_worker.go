package queue

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/riverqueue/river"

	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/service"
	"server/internal/utils/phash"
)

// ProcessPHashArgs is the job payload alias to avoid import cycles.
type ProcessPHashArgs = jobs.ProcessPHashArgs

// ProcessPHashWorker computes a perceptual hash (pHash) for photo duplicate detection.
// Unlike ML workers, pHash is pure image processing — no external services needed.
type ProcessPHashWorker struct {
	river.WorkerDefaults[ProcessPHashArgs]

	Queries          *repo.Queries
	EmbeddingService service.EmbeddingService
}

func (w *ProcessPHashWorker) Work(ctx context.Context, job *river.Job[ProcessPHashArgs]) error {
	asset, err := w.Queries.GetAssetByID(ctx, job.Args.AssetID)
	if err != nil {
		return fmt.Errorf("get asset: %w", err)
	}
	if !asset.RepositoryID.Valid {
		return fmt.Errorf("asset %s has no repository", asset.AssetID.String())
	}

	repository, err := w.Queries.GetRepository(ctx, asset.RepositoryID)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	thumbnail, err := w.Queries.GetThumbnailByAssetAndSize(ctx, repo.GetThumbnailByAssetAndSizeParams{
		AssetID: job.Args.AssetID,
		Size:    "small",
	})
	if err != nil {
		return fmt.Errorf("get small thumbnail: %w", err)
	}

	thumbnailPath := filepath.Join(repository.Path, filepath.FromSlash(thumbnail.StoragePath))
	file, err := os.Open(thumbnailPath)
	if err != nil {
		return fmt.Errorf("open small thumbnail: %w", err)
	}
	defer file.Close()

	hash, err := phash.ComputeFromReader(file)
	if err != nil {
		return err
	}

	vector := phash.ToVector(hash)

	// Convert asset_id to pgtype.UUID (GetAssetByID returns it directly)
	if err := w.EmbeddingService.SaveEmbedding(ctx, job.Args.AssetID,
		service.EmbeddingTypePHash, phash.ModelDCTPHashV1, vector, true); err != nil {
		return fmt.Errorf("save phash embedding: %w", err)
	}

	return nil
}
