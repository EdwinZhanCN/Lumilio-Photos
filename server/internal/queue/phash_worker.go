package queue

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"path/filepath"
	"strings"

	"github.com/corona10/goimagehash"
	"github.com/riverqueue/river"

	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/service"
	"server/internal/utils/imagesource"
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
	if asset.StoragePath == nil || strings.TrimSpace(*asset.StoragePath) == "" {
		return fmt.Errorf("asset %s has no storage path", asset.AssetID.String())
	}
	if !asset.RepositoryID.Valid {
		return fmt.Errorf("asset %s has no repository", asset.AssetID.String())
	}

	repository, err := w.Queries.GetRepository(ctx, asset.RepositoryID)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	fullPath := filepath.Join(repository.Path, filepath.FromSlash(*asset.StoragePath))

	reader, err := imagesource.OpenPhoto(ctx, fullPath, asset.OriginalFilename)
	if err != nil {
		return fmt.Errorf("open photo: %w", err)
	}
	defer reader.Close()

	img, _, err := image.Decode(reader)
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}

	phash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return fmt.Errorf("compute perceptual hash: %w", err)
	}

	vector := phashToVector(phash)

	// Convert asset_id to pgtype.UUID (GetAssetByID returns it directly)
	if err := w.EmbeddingService.SaveEmbedding(ctx, job.Args.AssetID,
		service.EmbeddingTypePHash, "dct-phash-v1", vector, true); err != nil {
		return fmt.Errorf("save phash embedding: %w", err)
	}

	return nil
}

// phashToVector converts a 64-bit perceptual hash into a 64-element float32 vector
// suitable for pgvector storage and HNSW similarity search.
func phashToVector(h *goimagehash.ImageHash) []float32 {
	hashBits := h.GetHash()
	vector := make([]float32, 64)
	for i := range 64 {
		if (hashBits>>i)&1 == 1 {
			vector[i] = 1.0
		} else {
			vector[i] = 0.0
		}
	}
	return vector
}
