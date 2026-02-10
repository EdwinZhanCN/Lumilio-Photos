package queue

import (
	"runtime"
	"server/config"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

func New(dbpool *pgxpool.Pool, workers *river.Workers, mlConfig config.MLConfig) (*river.Client[pgx.Tx], error) {
	queues := map[string]river.QueueConfig{
		"ingest_asset":    {MaxWorkers: 50},
		"discover_asset":  {MaxWorkers: 20},
		"metadata_asset":  {MaxWorkers: 20},
		"thumbnail_asset": {MaxWorkers: runtime.NumCPU() / 2},
		"transcode_asset": {MaxWorkers: 1},
		"retry_asset":     {MaxWorkers: 2},
	}

	if mlConfig.CLIPEnabled {
		queues["process_clip"] = river.QueueConfig{MaxWorkers: 2}
	}

	if mlConfig.OCREnabled {
		queues["process_ocr"] = river.QueueConfig{MaxWorkers: 3}
	}

	if mlConfig.CaptionEnabled {
		queues["process_caption"] = river.QueueConfig{MaxWorkers: 1}
	}

	if mlConfig.FaceEnabled {
		queues["process_face"] = river.QueueConfig{MaxWorkers: 2}
	}

	client, err := river.NewClient(riverpgxv5.New(dbpool), &river.Config{
		Schema:  "public",
		Queues:  queues,
		Workers: workers,
	})
	return client, err
}
