package queue

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// New River Client, add your queue here.
func New(dbpool *pgxpool.Pool, workers *river.Workers) (*river.Client[pgx.Tx], error) {
	// TODO: Config MaxWorkers Dynamically
	client, err := river.NewClient(riverpgxv5.New(dbpool), &river.Config{
		Schema: "public",
		Queues: map[string]river.QueueConfig{
			"process_asset": {MaxWorkers: 5},
			"process_clip":  {MaxWorkers: 1},
		},
		Workers: workers,
	})
	return client, err
}
