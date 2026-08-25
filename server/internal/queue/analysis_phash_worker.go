package queue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/service"
	"server/internal/storage"
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
	Files            *storage.RepositoryFSFactory
}

func (w *ProcessPHashWorker) Work(ctx context.Context, job *river.Job[ProcessPHashArgs]) error {
	asset, err := validateCurrentAssetWork(ctx, w.Queries, job.Args.AssetID, job.Args.ExpectedContentID)
	if errors.Is(err, ErrAssetWorkStale) {
		return nil
	}
	if err != nil {
		return err
	}
	thumbnail, err := w.Queries.GetThumbnailByAssetAndSize(ctx, repo.GetThumbnailByAssetAndSizeParams{
		AssetID: job.Args.AssetID,
		Size:    "small",
	})
	if errors.Is(err, sql.ErrNoRows) {
		return river.JobSnooze(time.Second)
	}
	if err != nil {
		return fmt.Errorf("get small thumbnail: %w", err)
	}

	thumbnailPath, err := storage.ParsePrivateRepositoryPath(thumbnail.StoragePath)
	if err != nil {
		return err
	}
	if err := validateThumbnailContent(asset, "small", thumbnailPath); err != nil {
		return err
	}
	if thumbnail.RepositoryID == uuid.Nil {
		return fmt.Errorf("small thumbnail has no repository")
	}
	repository, err := w.Queries.GetRepository(ctx, thumbnail.RepositoryID)
	if err != nil {
		return fmt.Errorf("get thumbnail repository: %w", err)
	}
	repositoryFS, err := w.Files.Open(repository)
	if err != nil {
		return err
	}
	defer repositoryFS.Close()
	file, err := repositoryFS.OpenPrivate(thumbnailPath)
	if errors.Is(err, os.ErrNotExist) {
		return river.JobSnooze(time.Second)
	}
	if err != nil {
		return fmt.Errorf("open small thumbnail: %w", err)
	}
	defer file.Close()

	hash, err := phash.ComputeFromReader(file)
	if err != nil {
		return err
	}

	vector := phash.ToVector(hash)
	if _, err := validateCurrentAssetWork(ctx, w.Queries, job.Args.AssetID, job.Args.ExpectedContentID); errors.Is(err, ErrAssetWorkStale) {
		return nil
	} else if err != nil {
		return err
	}

	if err := w.EmbeddingService.SaveEmbedding(ctx, job.Args.AssetID,
		service.EmbeddingTypePHash, phash.ModelDCTPHashV1, vector, true); err != nil {
		return fmt.Errorf("save phash embedding: %w", err)
	}

	return nil
}
