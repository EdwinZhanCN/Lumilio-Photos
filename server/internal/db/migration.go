package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"strings"

	"server/internal/db/vectorindex"
	migrations "server/migrations"

	"github.com/riverqueue/river/riverdriver/riversqlite"
	"github.com/riverqueue/river/rivermigrate"
)

// SchemaVersion is the single catalog schema discriminator stored in
// PRAGMA user_version. It is written once by the baseline and never
// incremented in place: a catalog carrying any other value is rejected rather
// than upgraded.
const SchemaVersion = 9

// baselineMigrationFile is the one embedded catalog baseline. It is edited in
// place when the schema changes. There is deliberately no migration sequence,
// checksum ledger, or historical generation selection.
const baselineMigrationFile = "000001_storage_baseline.up.sql"

// Migrate applies the catalog baseline and, for standalone package tests,
// River's schema on the same handle. Production calls MigrateCatalog for the
// catalog and migrates its independent QueueDB separately.
func (d *DB) Migrate(ctx context.Context) error {
	if err := d.MigrateCatalog(ctx); err != nil {
		return err
	}
	return MigrateRiver(ctx, d.SQL)
}

// MigrateCatalog creates the catalog from the single baseline when it is empty
// and verifies the schema version when it is not. The baseline runs in one
// transaction, so a failed first start leaves no partial schema and a restart
// is idempotent.
func (d *DB) MigrateCatalog(ctx context.Context) error {
	if err := assertSchemaVersion(ctx, d.SQL); err != nil {
		return err
	}
	if err := applyCatalogBaseline(ctx, d.SQL); err != nil {
		return fmt.Errorf("create Lumilio schema baseline: %w", err)
	}
	if err := vectorindex.Reconcile(ctx, d.Writer, d.ReaderSQL); err != nil {
		return fmt.Errorf("reconcile semantic Vec1 index: %w", err)
	}
	if err := d.Check(ctx); err != nil {
		return fmt.Errorf("post-migration integrity check: %w", err)
	}
	return nil
}

// assertSchemaVersion refuses to start against a catalog that is not the
// current schema version. An empty catalog passes so the baseline can claim
// it; a catalog with any user table must already carry SchemaVersion.
func assertSchemaVersion(ctx context.Context, database *sql.DB) error {
	var version int
	if err := database.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version == SchemaVersion {
		return nil
	}
	if version != 0 {
		return fmt.Errorf(
			"catalog schema version = %d, want %d: this catalog belongs to an incompatible schema; delete the SQLite catalog and restart Lumilio Photos (media repositories and original files are not deleted)",
			version,
			SchemaVersion,
		)
	}

	var userTables int
	if err := database.QueryRowContext(ctx, `
		SELECT count(*)
		FROM sqlite_schema
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
	`).Scan(&userTables); err != nil {
		return fmt.Errorf("inspect unversioned catalog: %w", err)
	}
	if userTables != 0 {
		return fmt.Errorf(
			"catalog has %d user tables but PRAGMA user_version = 0, want %d: this catalog belongs to an incompatible schema; delete the SQLite catalog and restart Lumilio Photos (media repositories and original files are not deleted)",
			userTables,
			SchemaVersion,
		)
	}
	return nil
}

// applyCatalogBaseline executes the single embedded baseline exactly once. The
// baseline stamps PRAGMA user_version, so re-running on a current catalog is a
// no-op. A failure rolls the whole transaction back, leaving an empty catalog
// for a clean retry.
func applyCatalogBaseline(ctx context.Context, database *sql.DB) error {
	body, err := loadCatalogBaseline()
	if err != nil {
		return err
	}

	var version int
	if err := database.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version == SchemaVersion {
		return nil
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin baseline transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if _, err := tx.ExecContext(ctx, string(body)); err != nil {
		return fmt.Errorf("execute %s: %w", baselineMigrationFile, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit %s: %w", baselineMigrationFile, err)
	}
	log.Printf("Lumilio schema baseline applied: version=%d file=%s", SchemaVersion, baselineMigrationFile)
	return nil
}

// loadCatalogBaseline loads the one embedded catalog baseline and rejects an
// embedded tree that carries a second migration file.
func loadCatalogBaseline() ([]byte, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			files = append(files, entry.Name())
		}
	}
	if len(files) != 1 || files[0] != baselineMigrationFile {
		return nil, fmt.Errorf("catalog baseline must be exactly %s, found %v", baselineMigrationFile, files)
	}
	body, err := migrations.FS.ReadFile(baselineMigrationFile)
	if err != nil {
		return nil, fmt.Errorf("read catalog baseline %s: %w", baselineMigrationFile, err)
	}
	return body, nil
}

// MigrateRiver applies only River's execution schema to the supplied queue
// database. The catalog and queue files intentionally have independent
// migration ledgers and lifecycle owners.
func MigrateRiver(ctx context.Context, database *sql.DB) error {
	migrator, err := rivermigrate.New(riversqlite.New(database), nil)
	if err != nil {
		return fmt.Errorf("initialize River migrator: %w", err)
	}
	result, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return fmt.Errorf("apply River migrations: %w", err)
	}
	for _, version := range result.Versions {
		log.Printf(
			"River migration applied: version=%d name=%s duration=%s",
			version.Version,
			version.Name,
			version.Duration,
		)
	}
	return nil
}
