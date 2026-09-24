//go:build sqlite_fts5

package processing

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/config"
	"server/internal/db"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// fixture is a migrated Catalog with River tables in the same file, so one
// reader pool serves both sides of the read model.
type fixture struct {
	t        *testing.T
	ctx      context.Context
	database *db.DB
	next     int
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	directory := t.TempDir()
	require.NoError(t, os.Chmod(directory, 0o700))
	database, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(directory, "processing.sqlite3")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })
	require.NoError(t, database.Migrate(ctx))
	f := &fixture{t: t, ctx: ctx, database: database}
	f.exec(`INSERT INTO users(user_id,username,password,created_at,updated_at,webauthn_user_handle) VALUES(1,'owner','unused',1,1,X'01')`)
	return f
}

func (f *fixture) exec(query string, args ...any) {
	f.t.Helper()
	_, err := f.database.SQL.ExecContext(f.ctx, query, args...)
	require.NoError(f.t, err)
}

func (f *fixture) reader() *Reader {
	return NewReader(f.database.ReaderSQL, f.database.ReaderSQL)
}

type assetRow struct {
	id      uuid.UUID
	content uuid.UUID
}

// asset inserts one photo; deleted assets must never be counted.
func (f *fixture) asset(deleted bool) assetRow {
	f.t.Helper()
	f.next++
	row := assetRow{id: uuid.New(), content: uuid.New()}
	f.exec(`INSERT INTO content_objects(content_id,hash_algorithm,full_hash,file_size,created_at) VALUES(?,'blake3-v1',?,1,1)`, row.content.String(), fmt.Sprintf("%064x", f.next))
	f.exec(`INSERT INTO assets(asset_id,owner_id,content_id,type,original_filename,mime_type,upload_time,updated_at,is_deleted) VALUES(?,1,?,'PHOTO',?,'image/jpeg',1,1,?)`,
		row.id.String(), row.content.String(), fmt.Sprintf("IMG_%04d.jpg", f.next), deleted)
	return row
}

// stage sets one pipeline row. terminal non-empty marks a terminal failure.
func (f *fixture) stage(asset assetRow, stage string, desired, applied int, terminal string) {
	f.t.Helper()
	var terminalValue any
	if terminal != "" {
		terminalValue = terminal
	}
	f.exec(`INSERT INTO asset_pipeline_state(asset_id,source_content_id,stage,pipeline_version,desired_version,applied_version,priority,terminal_error,updated_at) VALUES(?,?,?,'asset-v1',?,?,3,?,?)`,
		asset.id.String(), asset.content.String(), stage, desired, applied, terminalValue, time.Now().UTC().UnixMicro())
}

func (f *fixture) retryLater(asset assetRow, stage string, desired int) {
	f.t.Helper()
	f.exec(`INSERT INTO asset_pipeline_failures VALUES(?,?,?,'asset-v1',?,2,?,'processing_retry_pending',1)`,
		asset.id.String(), stage, asset.content.String(), desired, time.Now().Add(time.Hour).UnixMicro())
}

func (f *fixture) receipt(kind, subject, state string, updated time.Time) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	var terminal any
	if state == "failed" {
		terminal = "fixture_failure"
	}
	f.exec(`INSERT INTO catalog_operation_receipts(receipt_id,kind,subject_id,desired_version,state,terminal_error,created_at,updated_at) VALUES(?,?,?,1,?,?,?,?)`,
		id.String(), kind, subject, state, terminal, updated.UnixMicro(), updated.UnixMicro())
	return id
}

func (f *fixture) delivery(kind, projectionKind, state string) {
	f.t.Helper()
	args := "{}"
	if projectionKind != "" {
		args = fmt.Sprintf(`{"projectionKind":%q}`, projectionKind)
	}
	var finalized any
	if state == "completed" || state == "discarded" || state == "cancelled" {
		finalized = time.Now().UTC().Format("2006-01-02 15:04:05")
	}
	f.exec(`INSERT INTO river_job(args,kind,max_attempts,queue,state,attempted_at,finalized_at) VALUES(jsonb(?),?,8,'catalog_macro',?,datetime('now'),?)`, args, kind, state, finalized)
}

func stageByID(t *testing.T, summary Summary, id StageID) StageSummary {
	t.Helper()
	for _, stage := range summary.Stages {
		if stage.ID == id {
			return stage
		}
	}
	t.Fatalf("stage %s missing", id)
	return StageSummary{}
}

var _ = sql.ErrNoRows
