package pipeline

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"

	"server/internal/workqos"
)

func TestAssetPipelineKeepsMostUrgentPendingQoS(t *testing.T) {
	database, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "pipeline-qos.sqlite3")+"?_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })
	for _, statement := range []string{
		`CREATE TABLE assets(asset_id TEXT PRIMARY KEY,status TEXT,updated_at INTEGER)`,
		`CREATE TABLE catalog_operation_receipts(receipt_id TEXT PRIMARY KEY,kind TEXT)`,
		`CREATE TABLE asset_pipeline_state(asset_id TEXT,source_content_id TEXT,stage TEXT,pipeline_version TEXT,desired_version INTEGER,applied_version INTEGER,priority INTEGER,terminal_error TEXT,updated_at INTEGER,PRIMARY KEY(asset_id,stage))`,
		`CREATE TABLE asset_pipeline_receipt_stages(receipt_id TEXT,asset_id TEXT,stage TEXT,desired_version INTEGER,PRIMARY KEY(receipt_id,asset_id,stage))`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	assetID, contentID := uuid.New(), uuid.New()
	if _, err := database.Exec(`INSERT INTO assets(asset_id,status,updated_at) VALUES(?, '{}', 0)`, assetID.String()); err != nil {
		t.Fatal(err)
	}
	request := func(qos workqos.Class) {
		t.Helper()
		tx, err := database.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := RequestAssetStagesTx(context.Background(), tx, assetID, contentID, []Stage{StageEnrich}, AssetPipelineVersion, qos, uuid.New()); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}

	request(workqos.Background)
	request(workqos.Interactive)
	request(workqos.Maintenance)

	var priority int
	var desired uint64
	if err := database.QueryRow(`SELECT priority,desired_version FROM asset_pipeline_state WHERE asset_id=? AND stage='enrich'`, assetID.String()).Scan(&priority, &desired); err != nil {
		t.Fatal(err)
	}
	wantPriority, _ := workqos.Interactive.Priority()
	if priority != wantPriority || desired != 3 {
		t.Fatalf("pending asset pipeline priority/version = %d/%d, want %d/3", priority, desired, wantPriority)
	}
}
