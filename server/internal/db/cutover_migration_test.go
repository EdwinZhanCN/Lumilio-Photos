package db

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"server/config"
)

func TestDestructiveCutoverRequiresSuccessfulPreflightAndPurgesLegacyJobs(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(secureTempDir(t), "cutover.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(context.Background()) })
	migrateLegacyCatalogThrough(t, ctx, database, 10)
	seedLegacyCutoverCatalog(t, ctx, database)

	err = database.Migrate(ctx)
	if !errors.Is(err, ErrDestructiveMigrationPreflightRequired) {
		t.Fatalf("migration without preflight error = %v", err)
	}
	assertLegacyCutoverState(t, ctx, database)

	preflightFailure := errors.New("backup destination unavailable")
	err = database.MigrateWithPreflight(ctx, func(_ context.Context, plan DestructiveMigrationPlan) error {
		if plan.FromApplicationMigration != 10 || plan.ToApplicationMigration != 12 {
			t.Fatalf("preflight plan = %+v", plan)
		}
		assertLegacyCutoverState(t, ctx, database)
		return preflightFailure
	})
	if !errors.Is(err, preflightFailure) {
		t.Fatalf("failed preflight migration error = %v", err)
	}
	assertLegacyCutoverState(t, ctx, database)

	preflightCalls := 0
	err = database.MigrateWithPreflight(ctx, func(_ context.Context, plan DestructiveMigrationPlan) error {
		preflightCalls++
		assertLegacyCutoverState(t, ctx, database)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if preflightCalls != 1 {
		t.Fatalf("preflight calls = %d, want 1", preflightCalls)
	}

	var applicationVersion, assets, jobs, migrationRuns, controllerEffects int
	if err := database.SQL.QueryRowContext(ctx, `
		SELECT
		  (SELECT max(version) FROM lumilio_schema_migrations),
		  (SELECT count(*) FROM assets),
		  (SELECT count(*) FROM river_job),
		  (SELECT count(*) FROM repository_scan_runs WHERE mode = 'migration'),
		  (SELECT count(*) FROM repository_outbox WHERE effect_kind = 'controller')
	`).Scan(
		&applicationVersion,
		&assets,
		&jobs,
		&migrationRuns,
		&controllerEffects,
	); err != nil {
		t.Fatal(err)
	}
	if applicationVersion != currentApplicationMigration || assets != 0 || jobs != 0 || migrationRuns != 1 || controllerEffects != 1 {
		t.Fatalf(
			"cutover state: version=%d assets=%d jobs=%d runs=%d effects=%d",
			applicationVersion,
			assets,
			jobs,
			migrationRuns,
			controllerEffects,
		)
	}
	if columnExists(t, ctx, database, "assets", "storage_path") {
		t.Fatal("cutover Asset table retained path identity")
	}

	all, err := loadEmbeddedMigrations()
	if err != nil {
		t.Fatal(err)
	}
	current, err := currentGenerationMigrations(all)
	if err != nil {
		t.Fatal(err)
	}
	var oldBinaryMigrations []embeddedMigration
	for _, migration := range current {
		if migration.version <= 10 {
			oldBinaryMigrations = append(oldBinaryMigrations, migration)
		}
	}
	applied, err := readAppliedMigrations(ctx, database.SQL)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateAppliedMigrations(oldBinaryMigrations, applied); err == nil ||
		!strings.Contains(err.Error(), "unknown future migration") {
		t.Fatalf("old-binary schema gate error = %v", err)
	}
}

func TestCutoverMigrationGroupRollsBackSchemaAndCatalogOnLateFailure(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(secureTempDir(t), "cutover-rollback.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(context.Background()) })
	migrateLegacyCatalogThrough(t, ctx, database, 10)
	seedLegacyCutoverCatalog(t, ctx, database)

	all, err := loadEmbeddedMigrations()
	if err != nil {
		t.Fatal(err)
	}
	var group []embeddedMigration
	for _, migration := range all {
		if migration.version == 11 || migration.version == 12 {
			group = append(group, migration)
		}
	}
	if len(group) != 2 {
		t.Fatalf("cutover group length = %d, want 2", len(group))
	}
	group[1].sql += "\nINSERT INTO table_that_does_not_exist DEFAULT VALUES;\n"
	if err := applyMigrationGroup(ctx, database.SQL, group); err == nil {
		t.Fatal("late cutover failure unexpectedly committed")
	}
	assertLegacyCutoverState(t, ctx, database)
}

func migrateLegacyCatalogThrough(t *testing.T, ctx context.Context, database *DB, version int64) {
	t.Helper()
	if err := migrateRiver(ctx, database.SQL); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `
		CREATE TABLE lumilio_schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			checksum TEXT NOT NULL CHECK (length(checksum) = 64),
			applied_at INTEGER NOT NULL
		) STRICT
	`); err != nil {
		t.Fatal(err)
	}
	all, err := loadEmbeddedMigrations()
	if err != nil {
		t.Fatal(err)
	}
	current, err := currentGenerationMigrations(all)
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range current {
		if migration.version > version {
			break
		}
		if err := applyMigration(ctx, database.SQL, migration); err != nil {
			t.Fatalf("apply legacy migration %d: %v", migration.version, err)
		}
	}
}

func seedLegacyCutoverCatalog(t *testing.T, ctx context.Context, database *DB) {
	t.Helper()
	rootID := "11111111-1111-4111-8111-111111111111"
	repositoryID := "22222222-2222-4222-8222-222222222222"
	assetID := "33333333-3333-4333-8333-333333333333"
	if _, err := database.SQL.ExecContext(ctx, `
		INSERT INTO users (
			user_id, username, password, created_at, updated_at,
			display_name, role, webauthn_user_handle
		) VALUES (1, 'owner', 'unused', 1, 1, 'Owner', 'admin', X'01');
		INSERT INTO repository_roots (
			root_id, name, path, kind, status, created_at, updated_at
		) VALUES (?, 'Root', '/legacy-root', 'external', 'active', 1, 1);
		INSERT INTO repositories (
			repo_id, name, path, config, reachability, activity,
			created_at, updated_at, default_owner_id, role, root_id
		) VALUES (?, 'Repository', '/legacy-root/repository', NULL, 'active', 'idle', 1, 1, 1, 'regular', ?);
		INSERT INTO assets (
			asset_id, owner_id, type, original_filename, storage_path, mime_type,
			file_size, content_hash, upload_time, repository_id, updated_at
		) VALUES (?, 1, 'PHOTO', 'keep.jpg', 'keep.jpg', 'image/jpeg', 4, ?, 1, ?, 1);
		INSERT INTO river_job (kind, args, state)
		VALUES ('scan_repository', jsonb('{"repositoryId":"legacy"}'), 'available');
	`, rootID, repositoryID, rootID, assetID, strings.Repeat("a", 64), repositoryID); err != nil {
		t.Fatal(err)
	}
}

func assertLegacyCutoverState(t *testing.T, ctx context.Context, database *DB) {
	t.Helper()
	var version, assets, jobs, v2Tables int
	if err := database.SQL.QueryRowContext(ctx, `
		SELECT
		  (SELECT max(version) FROM lumilio_schema_migrations),
		  (SELECT count(*) FROM assets WHERE storage_path = 'keep.jpg'),
		  (SELECT count(*) FROM river_job WHERE kind = 'scan_repository'),
		  (SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = 'repository_nodes')
	`).Scan(&version, &assets, &jobs, &v2Tables); err != nil {
		t.Fatal(err)
	}
	if version != 10 || assets != 1 || jobs != 1 || v2Tables != 0 {
		t.Fatalf(
			"legacy state changed: version=%d assets=%d jobs=%d v2_tables=%d",
			version,
			assets,
			jobs,
			v2Tables,
		)
	}
}

func columnExists(t *testing.T, ctx context.Context, database *DB, table, column string) bool {
	t.Helper()
	rows, err := database.SQL.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%q)", table))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var ordinal, notNull, primaryKey int
		var name, kind string
		var defaultValue any
		if err := rows.Scan(&ordinal, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return false
}
