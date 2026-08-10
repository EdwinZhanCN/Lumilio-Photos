//go:build sqlite_fts5

package migrations_test

import (
	"context"
	"database/sql"
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"server/internal/db/sqlitespike"
	"server/migrations"
)

func TestSQLiteBaselineCreatesCompleteStrictSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database, err := sqlitespike.Open(ctx, t.TempDir()+"/library.sqlite3")
	if err != nil {
		t.Fatalf("open SQLite: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	baseline, err := migrations.FS.ReadFile("000003_vec1_baseline.up.sql")
	if err != nil {
		t.Fatalf("read SQLite baseline: %v", err)
	}
	if _, err := database.ExecContext(ctx, string(baseline)); err != nil {
		t.Fatalf("execute SQLite baseline: %v", err)
	}
	eventsMigration, err := migrations.FS.ReadFile("000004_media_semantic_events.up.sql")
	if err != nil {
		t.Fatalf("read Event migration: %v", err)
	}
	if _, err := database.ExecContext(ctx, string(eventsMigration)); err != nil {
		t.Fatalf("execute Event migration: %v", err)
	}

	for _, table := range applicationTables {
		assertStrictTable(t, database, table)
	}

	var userVersion int
	if err := database.QueryRowContext(ctx, "PRAGMA user_version").Scan(&userVersion); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if userVersion != 5 {
		t.Fatalf("baseline user_version = %d, want 5", userVersion)
	}

	var browseFactColumns int
	if err := database.QueryRowContext(ctx, `
		SELECT count(*) FROM pragma_table_info('media_item_browse_facts')
	`).Scan(&browseFactColumns); err != nil {
		t.Fatalf("inspect media_item_browse_facts: %v", err)
	}
	if browseFactColumns == 0 {
		t.Fatal("media_item_browse_facts view missing")
	}

	var seededClassifiers int
	if err := database.QueryRowContext(ctx, "SELECT count(*) FROM classifier_definitions").Scan(&seededClassifiers); err != nil {
		t.Fatalf("count classifier seeds: %v", err)
	}
	if seededClassifiers != 3 {
		t.Fatalf("classifier seed count = %d, want 3", seededClassifiers)
	}
	if _, err := database.ExecContext(ctx, `
		UPDATE system_state SET bootstrap_phase = 'catalog_ready' WHERE id = 1
	`); err != nil {
		t.Fatalf("persist pre-admin bootstrap phase: %v", err)
	}

	assertRejected(t, database, `
		INSERT INTO registration_sessions (
			session_id, username, password_hash, role, webauthn_user_handle,
			created_at, expires_at
		) VALUES ('NOT-A-UUID', 'bad', 'hash', 'admin', x'00', 1, 2)
	`)
	assertRejected(t, database, `
		INSERT INTO agent_pins (
			pin_id, user_id, plan, asset_ids, created_at, updated_at
		) VALUES ('00000000-0000-0000-0000-000000000000', 999, '{bad', '[]', 1, 1)
	`)

	insertVectorFixtures(t, ctx, database)
	insertFTSFixture(t, ctx, database)

	assertRejected(t, database, `
		INSERT INTO repositories (repo_id, name, path, created_at, updated_at)
		VALUES ('00000000-0000-0000-0000-000000000099', 'orphan', '/orphan', 1, 1)
	`)
	assertRejected(t, database, `
		UPDATE repositories SET reachability = 'error'
		WHERE repo_id = '00000000-0000-0000-0000-000000000002'
	`)
	assertRejected(t, database, `
		UPDATE repositories SET activity = 'active'
		WHERE repo_id = '00000000-0000-0000-0000-000000000002'
	`)
	assertRejected(t, database, `
		DELETE FROM repository_roots
		WHERE root_id = '00000000-0000-0000-0000-000000000001'
	`)

	var foreignKeyFailures int
	rows, err := database.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		t.Fatalf("foreign_key_check: %v", err)
	}
	for rows.Next() {
		foreignKeyFailures++
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close foreign_key_check: %v", err)
	}
	if foreignKeyFailures != 0 {
		t.Fatalf("foreign_key_check failures = %d", foreignKeyFailures)
	}
}

func assertStrictTable(t *testing.T, database *sql.DB, table string) {
	t.Helper()

	rows, err := database.Query("PRAGMA table_list")
	if err != nil {
		t.Fatalf("table_list: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var schema, name, tableType string
		var columns, withoutRowID, strict int
		if err := rows.Scan(&schema, &name, &tableType, &columns, &withoutRowID, &strict); err != nil {
			t.Fatalf("scan table_list: %v", err)
		}
		if name == table {
			if strict != 1 {
				t.Fatalf("table %s strict = %d, want 1", table, strict)
			}
			return
		}
	}
	t.Fatalf("application table %s missing", table)
}

func assertRejected(t *testing.T, database *sql.DB, statement string) {
	t.Helper()
	if _, err := database.Exec(statement); err == nil {
		t.Fatalf("statement unexpectedly succeeded: %s", strings.TrimSpace(statement))
	}
}

func insertVectorFixtures(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()

	if _, err := database.ExecContext(ctx, `
		INSERT INTO users (
			user_id, username, password, created_at, updated_at, webauthn_user_handle
		) VALUES (1, 'owner', 'hash', 1, 1, x'01');
		INSERT INTO repository_roots (
			root_id, name, path, kind, created_at, updated_at
		) VALUES ('00000000-0000-0000-0000-000000000001', 'root', '/media', 'default', 1, 1);
		INSERT INTO repositories (
			repo_id, name, path, created_at, updated_at, default_owner_id, root_id
		) VALUES (
			'00000000-0000-0000-0000-000000000002', 'repo', '/media/repo',
			1, 1, 1, '00000000-0000-0000-0000-000000000001'
		);
		INSERT INTO assets (
			asset_id, owner_id, type, original_filename, mime_type, file_size,
			content_hash, upload_time, repository_id, updated_at
		) VALUES (
			'00000000-0000-0000-0000-000000000003', 1, 'PHOTO',
			'quiet sunrise.jpg', 'image/jpeg', 1, 'hash', 1,
			'00000000-0000-0000-0000-000000000002', 1
		);
		INSERT INTO embedding_spaces (
			id, embedding_type, model_id, dimensions, distance_metric, created_at, updated_at
		) VALUES (1, 'semantic', 'fixture', 768, 'l2', 1, 1);
	`); err != nil {
		t.Fatalf("insert vector prerequisites: %v", err)
	}

	vector := serializeVector(make([]float32, 768))
	if _, err := database.ExecContext(ctx, `
		INSERT INTO search_embeddings (
			asset_id, space_id, vector, model_id, created_at
		) VALUES (?, 1, ?, 'fixture', 1)
	`, "00000000-0000-0000-0000-000000000003", vector); err != nil {
		t.Fatalf("insert authoritative vector: %v", err)
	}
	var vectorRows int
	if err := database.QueryRowContext(ctx, "SELECT count(*) FROM search_embeddings_vec").Scan(&vectorRows); err != nil {
		t.Fatalf("count derived vectors: %v", err)
	}
	if vectorRows != 1 {
		t.Fatalf("derived vector count = %d, want 1", vectorRows)
	}

	var nearestRowID int64
	if err := database.QueryRowContext(ctx, `
		SELECT rowid
		FROM search_embeddings_vec(?, '{"k":1}')
		WHERE space_id = 1
		  AND owner_id = 1
		  AND is_deleted = 0
		  AND asset_type = 'PHOTO'
	`, vector).Scan(&nearestRowID); err != nil {
		t.Fatalf("query Vec1 fixture: %v", err)
	}
	if nearestRowID <= 0 {
		t.Fatalf("nearest Vec1 rowid = %d", nearestRowID)
	}

	if _, err := database.ExecContext(ctx, `
		UPDATE assets
		SET is_deleted = 1
		WHERE asset_id = '00000000-0000-0000-0000-000000000003'
	`); err != nil {
		t.Fatalf("update Vec1 metadata source: %v", err)
	}
	var visibleRows int
	if err := database.QueryRowContext(ctx, `
		SELECT count(*)
		FROM search_embeddings_vec(?, '{"k":1}')
		WHERE is_deleted = 0
	`, vector).Scan(&visibleRows); err != nil {
		t.Fatalf("query updated Vec1 metadata: %v", err)
	}
	if visibleRows != 0 {
		t.Fatalf("visible Vec1 rows after soft delete = %d, want 0", visibleRows)
	}
}

func serializeVector(vector []float32) []byte {
	blob := make([]byte, len(vector)*4)
	for index, value := range vector {
		binary.LittleEndian.PutUint32(blob[index*4:], math.Float32bits(value))
	}
	return blob
}

func insertFTSFixture(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()

	var assetID string
	if err := database.QueryRowContext(ctx, `
		SELECT assets.asset_id
		FROM asset_search_fts
		JOIN assets ON assets.rowid = asset_search_fts.rowid
		WHERE asset_search_fts MATCH 'sunrise'
	`).Scan(&assetID); err != nil {
		t.Fatalf("query asset FTS fixture: %v", err)
	}
	if assetID != "00000000-0000-0000-0000-000000000003" {
		t.Fatalf("asset FTS result = %s", assetID)
	}
}

var applicationTables = []string{
	"users",
	"registration_sessions",
	"settings",
	"user_mfa_recovery_codes",
	"user_mfa_totp_credentials",
	"user_webauthn_credentials",
	"refresh_tokens",
	"system_state",
	"repository_roots",
	"repositories",
	"repository_defaults",
	"lifecycle_operations",
	"host_actions",
	"assets",
	"repository_scan_runs",
	"tags",
	"asset_tags",
	"thumbnails",
	"albums",
	"album_assets",
	"media_items",
	"media_item_assets",
	"events",
	"event_media_items",
	"event_constraints",
	"event_redirects",
	"event_dirty_ranges",
	"event_owner_state",
	"asset_stacks",
	"asset_stack_members",
	"duplicate_groups",
	"duplicate_group_assets",
	"duplicate_group_edges",
	"location_clusters",
	"location_cluster_assets",
	"reverse_geocode_cache",
	"classifier_definitions",
	"embedding_spaces",
	"embeddings",
	"face_results",
	"face_items",
	"face_clusters",
	"face_cluster_members",
	"ocr_results",
	"ocr_text_items",
	"ocr_index_metadata",
	"ocr_index_outbox",
	"species_predictions",
	"asset_quality_scores",
	"search_embeddings",
	"semantic_vector_index_state",
	"agent_checkpoints",
	"agent_pins",
	"cloud_credentials",
	"cloud_import_runs",
	"cloud_sync_cursors",
	"cloud_sync_files",
	"repository_cloud_bindings",
	"share_links",
	"agent_threads",
	"agent_runs",
	"agent_refs",
	"agent_pending_effects",
}
