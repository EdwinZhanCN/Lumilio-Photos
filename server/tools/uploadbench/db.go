package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// dbClient reads exact per-asset core-task timing directly from river_job,
// whose finalized_at is millisecond-precision (the API surface and status
// updated_at values are only RFC3339 second-precision). It is optional: when no
// SQLite path is supplied the benchmark falls back to API polling.
type dbClient struct {
	database *sql.DB
}

func newDBClient(ctx context.Context, path string) (*dbClient, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve SQLite catalog path: %w", err)
	}
	location := &url.URL{Scheme: "file", Path: filepath.Clean(absolutePath)}
	query := location.Query()
	query.Set("mode", "ro")
	query.Set("_query_only", "1")
	location.RawQuery = query.Encode()

	database, err := sql.Open("sqlite3", location.String())
	if err != nil {
		return nil, fmt.Errorf("open SQLite catalog: %w", err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := database.PingContext(pingCtx); err != nil {
		database.Close()
		return nil, fmt.Errorf("ping SQLite catalog: %w", err)
	}
	return &dbClient{database: database}, nil
}

func (d *dbClient) close() {
	if d != nil && d.database != nil {
		_ = d.database.Close()
	}
}

// dbAssetTiming is the exact core-task timing for one asset, keyed by filename.
type dbAssetTiming struct {
	filename  string
	metaDone  *time.Time // finalized_at of the completed metadata_asset job
	thumbDone *time.Time // finalized_at of the completed thumbnail_asset job
	failed    bool       // a core job was discarded or cancelled
}

// coreTimings joins river_job to assets on the canonical UUID string stored in
// the assetId JSON argument. It returns the finalization time of each completed
// core task plus whether any core job was discarded or cancelled.
func (d *dbClient) coreTimings(ctx context.Context) ([]dbAssetTiming, error) {
	const query = `
SELECT
  a.original_filename,
  MAX(j.finalized_at) FILTER (WHERE j.queue = 'metadata_asset'  AND j.state = 'completed') AS meta_done,
  MAX(j.finalized_at) FILTER (WHERE j.queue = 'thumbnail_asset' AND j.state = 'completed') AS thumb_done,
  MAX(CASE WHEN j.state IN ('discarded', 'cancelled') THEN 1 ELSE 0 END)                    AS failed
FROM river_job j
JOIN assets a ON a.asset_id = json_extract(j.args, '$.assetId')
WHERE j.queue IN ('metadata_asset', 'thumbnail_asset')
GROUP BY a.original_filename
`
	rows, err := d.database.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query core timings: %w", err)
	}
	defer rows.Close()

	var out []dbAssetTiming
	for rows.Next() {
		var t dbAssetTiming
		var meta, thumb sql.NullString
		var failed int
		if err := rows.Scan(&t.filename, &meta, &thumb, &failed); err != nil {
			return nil, err
		}
		var err error
		if t.metaDone, err = parseRiverTime(meta); err != nil {
			return nil, fmt.Errorf("parse metadata completion for %q: %w", t.filename, err)
		}
		if t.thumbDone, err = parseRiverTime(thumb); err != nil {
			return nil, fmt.Errorf("parse thumbnail completion for %q: %w", t.filename, err)
		}
		t.failed = failed != 0
		out = append(out, t)
	}
	return out, rows.Err()
}

func parseRiverTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05.999999999-07:00",
		time.RFC3339Nano,
	} {
		if parsed, err := time.Parse(layout, value.String); err == nil {
			parsed = parsed.UTC()
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("unsupported River timestamp %q", value.String)
}
