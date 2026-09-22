//go:build sqlite_fts5

package migrations_test

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"io/fs"
	"math"
	"regexp"
	"sort"
	"strings"
	"testing"

	"server/internal/db"
	"server/internal/db/sqlitespike"
	"server/migrations"
)

// The catalog schema is one standalone baseline. These counts are the complete
// inventory of the current schema; a future schema change edits the baseline
// in place and updates them deliberately.
const (
	baselineOrdinaryTables = 96
	baselineVirtualTables  = 5
	baselineViews          = 3
	baselineShadowTables   = 17
	baselineVec1Internals  = 5
	baselineIndexes        = 155
	baselineTriggers       = 41
)

// baselineVec1InternalTables are owned by the Vec1 virtual table module.
// PRAGMA table_list reports them as ordinary tables, but the module declares
// them without STRICT and the baseline cannot change that.
var baselineVec1InternalTables = []string{
	"search_embeddings_vec_base",
	"search_embeddings_vec_config",
	"search_embeddings_vec_idx",
	"search_embeddings_vec_meta",
	"search_embeddings_vec_model",
}

var (
	createTablePattern        = regexp.MustCompile(`(?m)^CREATE TABLE `)
	createVirtualTablePattern = regexp.MustCompile(`(?m)^CREATE VIRTUAL TABLE `)
	createViewPattern         = regexp.MustCompile(`(?m)^CREATE VIEW `)
	createIndexPattern        = regexp.MustCompile(`(?m)^CREATE (?:UNIQUE )?INDEX `)
	createTriggerPattern      = regexp.MustCompile(`(?m)^CREATE TRIGGER `)
)

// readEmbeddedBaseline refuses an embedded tree that is not exactly one
// standalone baseline file.
func readEmbeddedBaseline(t *testing.T) []byte {
	t.Helper()
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatalf("read embedded migrations: %v", err)
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	if len(files) != 1 || files[0] != "000001_storage_baseline.up.sql" {
		t.Fatalf("embedded SQL files = %v, want exactly [000001_storage_baseline.up.sql]", files)
	}
	body, err := migrations.FS.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	return body
}

func openBaselineCatalog(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	database, err := sqlitespike.Open(ctx, t.TempDir()+"/library.sqlite3")
	if err != nil {
		t.Fatalf("open SQLite: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if _, err := database.ExecContext(ctx, string(readEmbeddedBaseline(t))); err != nil {
		t.Fatalf("execute SQLite baseline: %v", err)
	}
	return database
}

func TestSQLiteBaselineCreatesCompleteStrictSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := openBaselineCatalog(t)

	var userVersion int
	if err := database.QueryRowContext(ctx, "PRAGMA user_version").Scan(&userVersion); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if int64(userVersion) != db.SchemaVersion {
		t.Fatalf("baseline user_version = %d, want runtime schema version %d", userVersion, db.SchemaVersion)
	}
	if stamp := fmt.Sprintf("PRAGMA user_version = %d;", db.SchemaVersion); !strings.HasSuffix(strings.TrimSpace(string(readEmbeddedBaseline(t))), stamp) {
		t.Fatalf("baseline does not end with %q", stamp)
	}

	assertBaselineInventory(t, database)
	assertBaselineFileInventory(t, readEmbeddedBaseline(t))
	assertBaselineIsLedgerFree(t, database)
	assertBaselineSeedsAndRejections(t, ctx, database)
	insertROEFixtures(t, ctx, database)
	insertVectorFixtures(t, ctx, database)
	insertFTSFixture(t, ctx, database)
	insertMusicFixtures(t, ctx, database)
	assertNoForeignKeyViolations(t, ctx, database)
}

// assertBaselineFileInventory pins the authored file itself to the complete
// object inventory, so deleting an object from the baseline cannot pass merely
// because the database still opens.
func assertBaselineFileInventory(t *testing.T, baseline []byte) {
	t.Helper()
	checks := []struct {
		name    string
		pattern *regexp.Regexp
		want    int
	}{
		{"ordinary table declarations", createTablePattern, baselineOrdinaryTables},
		{"virtual table declarations", createVirtualTablePattern, baselineVirtualTables},
		{"view declarations", createViewPattern, baselineViews},
		{"index declarations", createIndexPattern, baselineIndexes},
		{"trigger declarations", createTriggerPattern, baselineTriggers},
	}
	for _, check := range checks {
		if got := len(check.pattern.FindAll(baseline, -1)); got != check.want {
			t.Errorf("baseline %s = %d, want %d", check.name, got, check.want)
		}
	}
}

// assertBaselineInventory derives the schema inventory from pragma_table_list
// instead of a hand-maintained table list: every ordinary table must be
// STRICT, and the only non-STRICT rows may be the Vec1 virtual table's
// module-owned internals.
func assertBaselineInventory(t *testing.T, database *sql.DB) {
	t.Helper()

	rows, err := database.Query(`
		SELECT name, type, strict FROM pragma_table_list WHERE schema = 'main'
	`)
	if err != nil {
		t.Fatalf("table_list: %v", err)
	}
	defer rows.Close()

	var ordinary, virtual, views, shadow, system, vec1Internals []string
	nonStrict := map[string]bool{}
	inventory := map[string]string{}
	for rows.Next() {
		var name, tableType string
		var strict int
		if err := rows.Scan(&name, &tableType, &strict); err != nil {
			t.Fatalf("scan table_list: %v", err)
		}
		inventory[name] = tableType
		switch {
		case strings.HasPrefix(name, "sqlite_"):
			system = append(system, name)
		case tableType == "view":
			views = append(views, name)
		case tableType == "virtual":
			virtual = append(virtual, name)
		case tableType == "shadow":
			shadow = append(shadow, name)
		case tableType == "table" && strings.HasPrefix(name, "search_embeddings_vec_"):
			vec1Internals = append(vec1Internals, name)
			if strict != 1 {
				nonStrict[name] = true
			}
		case tableType == "table":
			ordinary = append(ordinary, name)
			if strict != 1 {
				nonStrict[name] = true
			}
		default:
			t.Fatalf("unexpected table_list type %q for %s", tableType, name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table_list: %v", err)
	}

	if len(system) == 0 {
		t.Fatal("table_list reported no SQLite system tables")
	}
	if len(vec1Internals) != baselineVec1Internals {
		t.Fatalf("Vec1 internal tables = %d, want %d: %v", len(vec1Internals), baselineVec1Internals, vec1Internals)
	}
	if len(ordinary) != baselineOrdinaryTables {
		t.Fatalf("ordinary tables = %d, want %d: %v", len(ordinary), baselineOrdinaryTables, ordinary)
	}
	if len(virtual) != baselineVirtualTables {
		t.Fatalf("virtual tables = %d, want %d: %v", len(virtual), baselineVirtualTables, virtual)
	}
	if len(views) != baselineViews {
		t.Fatalf("views = %d, want %d: %v", len(views), baselineViews, views)
	}
	if len(shadow) != baselineShadowTables {
		t.Fatalf("shadow tables = %d, want %d: %v", len(shadow), baselineShadowTables, shadow)
	}

	var gotNonStrict []string
	for name := range nonStrict {
		gotNonStrict = append(gotNonStrict, name)
	}
	sort.Strings(gotNonStrict)
	if strings.Join(gotNonStrict, ",") != strings.Join(baselineVec1InternalTables, ",") {
		t.Fatalf("non-STRICT ordinary tables = %v, want only Vec1 internals %v", gotNonStrict, baselineVec1InternalTables)
	}
	for _, name := range ordinary {
		if nonStrict[name] {
			continue
		}
		if inventory[name] != "table" {
			t.Fatalf("%s reported type %q", name, inventory[name])
		}
	}
	if nonStrict["music_track_lyrics"] {
		t.Fatal("music_track_lyrics must be STRICT")
	}
	if inventory["music_track_lyrics"] != "table" {
		t.Fatalf("music_track_lyrics type = %q, want table", inventory["music_track_lyrics"])
	}
	if inventory["search_embeddings_vec"] != "virtual" {
		t.Fatalf("search_embeddings_vec type = %q, want virtual", inventory["search_embeddings_vec"])
	}

	required := []string{
		// Authentication and authorization.
		"users", "refresh_tokens", "pending_totp_enrollments", "user_mfa_totp_credentials",
		"user_mfa_recovery_codes", "user_webauthn_credentials", "auth_security_verifications",
		"share_links", "agent_pins", "agent_threads", "agent_runs", "agent_refs", "agent_pending_effects",
		// Repository observation engine and exact-content identity.
		"content_objects", "assets", "asset_locations", "repository_nodes",
		"repository_observations", "repository_change_cursors", "repository_scan_runs",
		"repository_scan_frontier", "repository_observation_state", "repository_staging_commits",
		"location_projection_state", "location_projection_receipt_scopes",
		// Async processing and pipeline state.
		"asset_pipeline_state", "asset_pipeline_receipt_stages", "asset_pipeline_failures",
		"catalog_operation_receipts", "catalog_backup_requests", "asset_reindex_requests",
		"event_projection_pipeline_state", "ocr_projection_pipeline_state",
		"location_resolution_pipeline_state", "lifecycle_audit_events", "lifecycle_operations",
		// Music library.
		"music_albums", "music_artists", "music_album_artists", "music_tracks",
		"music_track_artists", "music_track_overrides", "music_track_lyrics",
		"music_playlists", "music_playlist_entries", "music_playback_sessions", "music_playback_entries",
		// Media, events, and search projections.
		"media_items", "media_item_assets", "albums", "tags", "asset_tags", "thumbnails",
		"events", "event_media_items", "event_owner_state", "event_dirty_ranges",
		"duplicate_groups", "asset_stacks", "embedding_spaces", "search_embeddings",
		"semantic_vector_index_state", "classifier_definitions", "settings", "system_state",
	}
	for _, name := range required {
		if _, ok := inventory[name]; !ok {
			t.Errorf("required application table %s is missing", name)
		}
	}

	for _, name := range []string{
		"asset_search_fts", "location_search_fts", "music_search_fts", "species_search_fts",
		"active_asset_occurrences", "active_asset_occurrence_paths", "media_item_browse_facts",
	} {
		if _, ok := inventory[name]; !ok {
			t.Errorf("required view or virtual table %s is missing", name)
		}
	}
}

// assertBaselineIsLedgerFree proves the historical migration machinery is
// gone: no ledger table, no sequence/checksum columns, and no superseded
// intermediate tables.
func assertBaselineIsLedgerFree(t *testing.T, database *sql.DB) {
	t.Helper()
	for _, name := range []string{
		"lumilio_schema_migrations",
		"registration_sessions",
		"repository_file_index",
		"events_converged",
		"assets_v2",
		"repository_scan_runs_v2",
		"thumbnails_before_music",
		"ocr_search_fts",
	} {
		var count int
		if err := database.QueryRow(`
			SELECT count(*) FROM sqlite_schema WHERE name = ?
		`, name).Scan(&count); err != nil {
			t.Fatalf("inspect historical residue %s: %v", name, err)
		}
		if count != 0 {
			t.Errorf("historical schema object %s still exists", name)
		}
	}
}

func assertBaselineSeedsAndRejections(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()

	var provider, endpoint, language, userAgent string
	var revision int64
	if err := database.QueryRowContext(ctx, `
		SELECT geocoding_provider, geocoding_nominatim_endpoint,
		       geocoding_language, geocoding_user_agent, geocoding_revision
		FROM settings WHERE id = 1
	`).Scan(&provider, &endpoint, &language, &userAgent, &revision); err != nil {
		t.Fatalf("read geocoding defaults: %v", err)
	}
	if provider != "disabled" || endpoint != "https://nominatim.openstreetmap.org/reverse" ||
		language != "en" || userAgent != "Lumilio-Photos/1.0" || revision != 1 {
		t.Fatalf("unexpected geocoding defaults: provider=%q endpoint=%q language=%q user_agent=%q revision=%d", provider, endpoint, language, userAgent, revision)
	}
	assertRejected(t, database, `UPDATE settings SET geocoding_provider = 'other' WHERE id = 1`)
	assertRejected(t, database, `UPDATE settings SET geocoding_revision = 0 WHERE id = 1`)
	assertRejected(t, database, `UPDATE settings SET geocoding_user_agent = 'bad' || char(9) || 'agent' WHERE id = 1`)
	assertRejected(t, database, `UPDATE settings SET geocoding_nominatim_endpoint = replace(printf('%2049s', 'x'), ' ', 'x') WHERE id = 1`)
	assertRejected(t, database, `UPDATE settings SET geocoding_nominatim_endpoint = 'https://user:pass@example.test/reverse' WHERE id = 1`)
	assertRejected(t, database, `UPDATE settings SET geocoding_nominatim_endpoint = 'https:///reverse' WHERE id = 1`)
	assertRejected(t, database, `UPDATE settings SET geocoding_nominatim_endpoint = 'https://example.test/reverse#fragment' WHERE id = 1`)

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
	var seededDefaults int
	if err := database.QueryRowContext(ctx, "SELECT count(*) FROM repository_defaults WHERE id = 1").Scan(&seededDefaults); err != nil {
		t.Fatalf("count repository defaults: %v", err)
	}
	if seededDefaults != 1 {
		t.Fatalf("repository_defaults seed count = %d, want 1", seededDefaults)
	}
	var seededIndexState int
	if err := database.QueryRowContext(ctx, "SELECT count(*) FROM semantic_vector_index_state WHERE id = 1").Scan(&seededIndexState); err != nil {
		t.Fatalf("count semantic index state: %v", err)
	}
	if seededIndexState != 1 {
		t.Fatalf("semantic_vector_index_state seed count = %d, want 1", seededIndexState)
	}
	var libraryID string
	if err := database.QueryRowContext(ctx, "SELECT library_id FROM system_state WHERE id = 1").Scan(&libraryID); err != nil {
		t.Fatalf("read system_state seed: %v", err)
	}
	if len(libraryID) != 32 {
		t.Fatalf("seeded library_id length = %d, want 32", len(libraryID))
	}
	if _, err := database.ExecContext(ctx, `
		UPDATE system_state SET bootstrap_phase = 'catalog_ready' WHERE id = 1
	`); err != nil {
		t.Fatalf("persist pre-admin bootstrap phase: %v", err)
	}

	assertRejected(t, database, `
		INSERT INTO pending_totp_enrollments (
			enrollment_id, user_id, secret_ciphertext, auth_version,
			created_at, expires_at
		) VALUES ('NOT-A-UUID', 999, x'00', 0, 1, 2)
	`)
	assertRejected(t, database, `
		INSERT INTO agent_pins (
			pin_id, user_id, plan, asset_ids, created_at, updated_at
		) VALUES ('00000000-0000-0000-0000-000000000000', 999, '{bad', '[]', 1, 1)
	`)
	assertRejected(t, database, `
		INSERT INTO repositories (repo_id, name, path, created_at, updated_at)
		VALUES ('00000000-0000-0000-0000-000000000099', 'orphan', '/orphan', 1, 1)
	`)
}

func insertROEFixtures(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()

	if _, err := database.ExecContext(ctx, `
		INSERT INTO users (
			user_id, username, password, created_at, updated_at, webauthn_user_handle
		) VALUES (1, 'owner', 'hash', 1, 1, x'01');
		INSERT INTO storage_locations (
			storage_location_id, name, path, kind, created_at, updated_at
		) VALUES ('00000000-0000-0000-0000-000000000001', 'root', '/media', 'default', 1, 1);
		INSERT INTO repositories (
			repo_id, name, path, created_at, updated_at, default_owner_id, storage_location_id
		) VALUES (
			'00000000-0000-0000-0000-000000000002', 'repo', '/media/repo',
			1, 1, 1, '00000000-0000-0000-0000-000000000001'
		);
		INSERT INTO content_objects (
			content_id, hash_algorithm, full_hash, file_size, created_at
		) VALUES (
			'00000000-0000-0000-0000-000000000010', 'blake3-v1',
			'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 1024, 1
		);
		INSERT INTO assets (
			asset_id, owner_id, content_id, type, original_filename, mime_type,
			upload_time, updated_at
		) VALUES (
			'00000000-0000-0000-0000-000000000003', 1,
			'00000000-0000-0000-0000-000000000010', 'PHOTO',
			'quiet sunrise.jpg', 'image/jpeg', 1, 1
		);
		INSERT INTO repository_nodes (
			node_id, repository_id, parent_node_id, name, name_key, kind,
			observation_revision, created_at, updated_at
		) VALUES (
			'00000000-0000-0000-0000-000000000020',
			'00000000-0000-0000-0000-000000000002', NULL,
			'photos', 'photos', 'directory', 1, 1, 1
		);
		INSERT INTO repository_nodes (
			node_id, repository_id, parent_node_id, name, name_key, kind,
			observation_revision, created_at, updated_at
		) VALUES (
			'00000000-0000-0000-0000-000000000021',
			'00000000-0000-0000-0000-000000000002',
			'00000000-0000-0000-0000-000000000020',
			'quiet sunrise.jpg', 'quiet sunrise.jpg', 'file', 1, 1, 1
		);
		INSERT INTO asset_locations (
			location_id, node_id, asset_id, bound_observation_revision, created_at, updated_at
		) VALUES (
			'00000000-0000-0000-0000-000000000022',
			'00000000-0000-0000-0000-000000000021',
			'00000000-0000-0000-0000-000000000003', 1, 1, 1
		);
	`); err != nil {
		t.Fatalf("insert ROE fixtures: %v", err)
	}

	var occurrences int
	if err := database.QueryRowContext(ctx, `
		SELECT count(*)
		FROM active_asset_occurrences
		WHERE asset_id = '00000000-0000-0000-0000-000000000003'
		  AND repository_id = '00000000-0000-0000-0000-000000000002'
		  AND node_id = '00000000-0000-0000-0000-000000000021'
	`).Scan(&occurrences); err != nil {
		t.Fatalf("query active_asset_occurrences: %v", err)
	}
	if occurrences != 1 {
		t.Fatalf("active occurrence count = %d, want 1", occurrences)
	}
	var relativePath string
	if err := database.QueryRowContext(ctx, `
		SELECT relative_path
		FROM active_asset_occurrence_paths
		WHERE asset_id = '00000000-0000-0000-0000-000000000003'
	`).Scan(&relativePath); err != nil {
		t.Fatalf("query active_asset_occurrence_paths: %v", err)
	}
	if relativePath != "quiet sunrise.jpg" {
		t.Fatalf("relative_path = %q, want quiet sunrise.jpg", relativePath)
	}

	assertRejected(t, database, `
		UPDATE repositories SET reachability = 'error'
		WHERE repo_id = '00000000-0000-0000-0000-000000000002'
	`)
	assertRejected(t, database, `
		UPDATE repositories SET activity = 'active'
		WHERE repo_id = '00000000-0000-0000-0000-000000000002'
	`)
	assertRejected(t, database, `
		DELETE FROM storage_locations
		WHERE storage_location_id = '00000000-0000-0000-0000-000000000001'
	`)
}

func insertVectorFixtures(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()

	if _, err := database.ExecContext(ctx, `
		INSERT INTO embedding_spaces (
			id, embedding_type, model_id, dimensions, distance_metric, created_at, updated_at
		) VALUES (1, 'semantic', 'fixture', 768, 'l2', 1, 1);
	`); err != nil {
		t.Fatalf("insert embedding space: %v", err)
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

// insertMusicFixtures proves the nontrivial music tables work through their
// triggers and that music_track_lyrics participates in the STRICT schema.
func insertMusicFixtures(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()

	const trackID = "00000000-0000-0000-0000-000000000030"
	if _, err := database.ExecContext(ctx, `
		INSERT INTO content_objects (
			content_id, hash_algorithm, full_hash, file_size, created_at
		) VALUES (
			'00000000-0000-0000-0000-000000000031', 'blake3-v1',
			'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', 2048, 1
		);
		INSERT INTO assets (
			asset_id, owner_id, content_id, type, original_filename, mime_type,
			upload_time, updated_at
		) VALUES (
			?, 1, '00000000-0000-0000-0000-000000000031', 'AUDIO',
			'morning song.flac', 'audio/flac', 1, 1
		);
	`, trackID); err != nil {
		t.Fatalf("insert audio asset: %v", err)
	}

	var title string
	if err := database.QueryRowContext(ctx, `
		SELECT title FROM music_tracks WHERE track_id = ?
	`, trackID).Scan(&title); err != nil {
		t.Fatalf("music asset trigger did not create a track: %v", err)
	}
	if title != "morning song.flac" {
		t.Fatalf("seeded track title = %q", title)
	}

	var ftsTrack string
	if err := database.QueryRowContext(ctx, `
		SELECT track_id FROM music_search_fts WHERE music_search_fts MATCH 'morning'
	`).Scan(&ftsTrack); err != nil {
		t.Fatalf("query music FTS fixture: %v", err)
	}
	if ftsTrack != trackID {
		t.Fatalf("music FTS result = %s, want %s", ftsTrack, trackID)
	}

	if _, err := database.ExecContext(ctx, `
		INSERT INTO music_track_lyrics (track_id, content, revision)
		VALUES (?, 'first line', 1)
	`, trackID); err != nil {
		t.Fatalf("insert music lyrics: %v", err)
	}
	var lyricsRevision int
	if err := database.QueryRowContext(ctx, `
		SELECT revision FROM music_track_lyrics WHERE track_id = ?
	`, trackID).Scan(&lyricsRevision); err != nil {
		t.Fatalf("read music lyrics: %v", err)
	}
	if lyricsRevision != 1 {
		t.Fatalf("lyrics revision = %d, want 1", lyricsRevision)
	}
	assertRejected(t, database, `
		UPDATE music_track_lyrics SET revision = 0 WHERE track_id = '00000000-0000-0000-0000-000000000030'
	`)
	assertRejected(t, database, `
		INSERT INTO music_track_lyrics (track_id, content, revision)
		VALUES ('00000000-0000-0000-0000-000000000030', 'duplicate', 1)
	`)
}

func assertNoForeignKeyViolations(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()
	var failures int
	rows, err := database.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		t.Fatalf("foreign_key_check: %v", err)
	}
	for rows.Next() {
		failures++
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close foreign_key_check: %v", err)
	}
	if failures != 0 {
		t.Fatalf("foreign_key_check failures = %d", failures)
	}
}

func assertRejected(t *testing.T, database *sql.DB, statement string) {
	t.Helper()
	if _, err := database.Exec(statement); err == nil {
		t.Fatalf("statement unexpectedly succeeded: %s", strings.TrimSpace(statement))
	}
}
