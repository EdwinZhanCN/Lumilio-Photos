package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// dbClient reads exact per-asset core-task timing directly from river_job,
// whose finalized_at is microsecond-precision (the API surface and the status
// JSONB updated_at are only RFC3339 second-precision). It is optional: when no
// DSN is supplied the benchmark falls back to API polling.
type dbClient struct {
	pool *pgxpool.Pool
}

func newDBClient(ctx context.Context, dsn string) (*dbClient, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open db pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &dbClient{pool: pool}, nil
}

func (d *dbClient) close() {
	if d != nil && d.pool != nil {
		d.pool.Close()
	}
}

// dbAssetTiming is the exact core-task timing for one asset, keyed by filename.
type dbAssetTiming struct {
	filename  string
	metaDone  *time.Time // finalized_at of the completed metadata_asset job
	thumbDone *time.Time // finalized_at of the completed thumbnail_asset job
	failed    bool       // a core job was discarded or cancelled
}

// coreTimings joins river_job to assets on the assetId arg (pgtype.UUID
// marshals to a canonical UUID string) and returns, per asset, the exact
// finalization time of each completed core task plus whether any core job was
// discarded/cancelled.
func (d *dbClient) coreTimings(ctx context.Context) ([]dbAssetTiming, error) {
	const query = `
SELECT
  a.original_filename,
  max(j.finalized_at) FILTER (WHERE j.queue = 'metadata_asset'  AND j.state = 'completed') AS meta_done,
  max(j.finalized_at) FILTER (WHERE j.queue = 'thumbnail_asset' AND j.state = 'completed') AS thumb_done,
  bool_or(j.state IN ('discarded', 'cancelled'))                                           AS failed
FROM river_job j
JOIN assets a ON a.asset_id = (j.args->>'assetId')::uuid
WHERE j.queue IN ('metadata_asset', 'thumbnail_asset')
GROUP BY a.original_filename
`
	rows, err := d.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query core timings: %w", err)
	}
	defer rows.Close()

	var out []dbAssetTiming
	for rows.Next() {
		var t dbAssetTiming
		var meta, thumb *time.Time
		if err := rows.Scan(&t.filename, &meta, &thumb, &t.failed); err != nil {
			return nil, err
		}
		t.metaDone, t.thumbDone = meta, thumb
		out = append(out, t)
	}
	return out, rows.Err()
}
