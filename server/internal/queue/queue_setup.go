package queue

import (
	"log/slog"
	"runtime"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

func clampWorkers(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func queueWorkerCountsForCPU(cpuCount int) (ingestWorkers int, thumbnailWorkers int, phashWorkers int) {
	if cpuCount < 1 {
		cpuCount = 1
	}

	// Favor user-visible thumbnail generation over lightweight ingest fan-out,
	// and keep pHash from contending with thumbnails during large imports.
	ingestWorkers = clampWorkers((cpuCount+1)/2, 2, 8)
	thumbnailWorkers = clampWorkers(cpuCount, 4, 12)
	phashWorkers = clampWorkers(cpuCount/4, 1, 4)

	if thumbnailWorkers < ingestWorkers {
		thumbnailWorkers = ingestWorkers
	}

	return ingestWorkers, thumbnailWorkers, phashWorkers
}

func queueWorkerCounts() (ingestWorkers int, thumbnailWorkers int, phashWorkers int) {
	return queueWorkerCountsForCPU(runtime.NumCPU())
}

func New(dbpool *pgxpool.Pool, workers *river.Workers, logger *slog.Logger) (*river.Client[pgx.Tx], error) {
	ingestWorkers, thumbnailWorkers, phashWorkers := queueWorkerCounts()

	queues := map[string]river.QueueConfig{
		"ingest_asset":              {MaxWorkers: ingestWorkers},
		"discover_asset":            {MaxWorkers: 20},
		"metadata_asset":            {MaxWorkers: 20},
		"thumbnail_asset":           {MaxWorkers: thumbnailWorkers},
		"transcode_asset":           {MaxWorkers: 1},
		"retry_asset":               {MaxWorkers: 2},
		"reindex_assets":            {MaxWorkers: 1},
		"rebuild_location_clusters": {MaxWorkers: 1},
		"scan_repository":           {MaxWorkers: 1},
		"detect_stacks":             {MaxWorkers: 1},
		"match_live_photo":          {MaxWorkers: 2},
		"process_clip":              {MaxWorkers: 2},
		"process_bioclip":           {MaxWorkers: 2},
		"process_ocr":               {MaxWorkers: 3},
		"process_face":              {MaxWorkers: 2},
		"process_phash":             {MaxWorkers: phashWorkers},
	}

	client, err := river.NewClient(riverpgxv5.New(dbpool), &river.Config{
		Schema:  "public",
		Queues:  queues,
		Workers: workers,
		Logger:  logger,
	})
	return client, err
}
