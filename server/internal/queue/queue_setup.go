package queue

import (
	"runtime"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

func New(dbpool *pgxpool.Pool, workers *river.Workers) (*river.Client[pgx.Tx], error) {
	queues := map[string]river.QueueConfig{
		"ingest_asset":    {MaxWorkers: 50},
		"discover_asset":  {MaxWorkers: 20},
		"metadata_asset":  {MaxWorkers: 20},
		"thumbnail_asset": {MaxWorkers: runtime.NumCPU() / 2},
		"transcode_asset": {MaxWorkers: 1},
		"retry_asset":     {MaxWorkers: 2},
		"reindex_assets":  {MaxWorkers: 1},
		"process_clip":    {MaxWorkers: 2},
		"process_ocr":     {MaxWorkers: 3},
		"process_caption": {MaxWorkers: 1},
		"process_face":    {MaxWorkers: 2},
	}

	client, err := river.NewClient(riverpgxv5.New(dbpool), &river.Config{
		Schema:  "public",
		Queues:  queues,
		Workers: workers,
	})
	return client, err
}
