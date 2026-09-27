package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"path"
	"strconv"
	"strings"

	"server/internal/db/vectorindex"
	migrations "server/migrations"

	"github.com/riverqueue/river/riverdriver/riversqlite"
	"github.com/riverqueue/river/rivermigrate"
)

// SchemaVersion is the catalog schema version stored in PRAGMA user_version.
// Version 1 is the v26.1.0-rc.1 compatibility baseline; 0 is reserved for an
// empty catalog. Later builds reach higher versions through forward steps and
// never edit a shipped baseline or step.
const SchemaVersion = 1

// baselineMigrationFile is the embedded catalog baseline (schema version 1).
// It is edited in place only until the rc.1 tag, and frozen afterwards.
const baselineMigrationFile = "000001_storage_baseline.up.sql"

// ErrPreReleaseCatalog marks a catalog created by a pre-release build. Such
// catalogs are rejected, never migrated.
var ErrPreReleaseCatalog = errors.New("catalog was created by a pre-release build of Lumilio Photos, whose data is not migrated")

func preReleaseCatalogError() error {
	return fmt.Errorf(
		"%w: move the SQLite catalog aside and restart Lumilio Photos to create a new one (media repositories and original files are not touched)",
		ErrPreReleaseCatalog,
	)
}

// UpgradeBackup snapshots a catalog before the first forward step runs and
// returns the snapshot path. It receives the catalog's read pool, the version
// found on disk, and the version the steps will reach. The db package cannot
// own backups (the backup package depends on it), so the runtime supplies it.
type UpgradeBackup func(ctx context.Context, source *sql.DB, from, to int) (string, error)

// catalogStep upgrades a catalog from Version-1 to Version. The runner applies
// it and stamps PRAGMA user_version = Version in one transaction.
type catalogStep struct {
	Version int
	Name    string
	Apply   func(ctx context.Context, tx *sql.Tx) error
}

// catalogStepsDir holds the embedded forward steps, named NNNN_<name>.sql
// where NNNN is the version the step produces (0002 is the first).
const catalogStepsDir = "steps"

// Migrate applies the catalog baseline and, for standalone package tests,
// River's schema on the same handle. Production calls MigrateCatalog for the
// catalog and migrates its independent QueueDB separately.
func (d *DB) Migrate(ctx context.Context) error {
	if err := d.MigrateCatalog(ctx, nil); err != nil {
		return err
	}
	return MigrateRiver(ctx, d.SQL)
}

// MigrateCatalog brings the catalog to SchemaVersion. An empty catalog gets the
// baseline and then every forward step, so fresh and upgraded catalogs run the
// same steps. An older catalog is first snapshotted through backup, then
// upgraded one step per transaction; a failed step rolls back its own
// transaction, leaves the catalog at the last completed version, and names the
// step and the snapshot. A nil backup refuses any upgrade.
func (d *DB) MigrateCatalog(ctx context.Context, backup UpgradeBackup) error {
	steps, err := loadCatalogSteps(migrations.FS, SchemaVersion)
	if err != nil {
		return err
	}
	return d.migrateCatalog(ctx, steps, SchemaVersion, backup)
}

func (d *DB) migrateCatalog(ctx context.Context, steps []catalogStep, current int, backup UpgradeBackup) error {
	if err := assertSchemaVersion(ctx, d.SQL, current); err != nil {
		return err
	}
	version, err := readSchemaVersion(ctx, d.SQL)
	if err != nil {
		return err
	}
	backupPath := ""
	switch {
	case version == 0:
		if err := applyCatalogBaseline(ctx, d.SQL); err != nil {
			return fmt.Errorf("create Lumilio schema baseline: %w", err)
		}
	case version < current:
		if backup == nil {
			return fmt.Errorf("catalog schema version %d needs an upgrade to %d, but no pre-upgrade backup is configured", version, current)
		}
		backupPath, err = backup(ctx, d.ReaderSQL, version, current)
		if err != nil {
			return fmt.Errorf("back up catalog before upgrading schema version %d to %d: %w", version, current, err)
		}
		log.Printf("Lumilio catalog pre-upgrade backup created: from=%d to=%d path=%s", version, current, backupPath)
	}
	if err := applyCatalogSteps(ctx, d.SQL, steps, backupPath); err != nil {
		return err
	}
	if err := vectorindex.Reconcile(ctx, d.Writer, d.ReaderSQL); err != nil {
		return fmt.Errorf("reconcile semantic Vec1 index: %w", err)
	}
	if err := d.Check(ctx); err != nil {
		return fmt.Errorf("post-migration integrity check: %w", err)
	}
	return nil
}

func readSchemaVersion(ctx context.Context, database *sql.DB) (int, error) {
	var version int
	if err := database.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return version, nil
}

// assertSchemaVersion refuses a catalog this build cannot open or upgrade. An
// empty catalog passes so the baseline can claim it; versions 1..current pass
// so MigrateCatalog can upgrade them; a newer version is rejected rather than
// read. Catalog identity (application_id) is verified before this runs.
func assertSchemaVersion(ctx context.Context, database *sql.DB, current int) error {
	version, err := readSchemaVersion(ctx, database)
	if err != nil {
		return err
	}
	if version > current {
		return fmt.Errorf(
			"catalog schema version = %d is newer than this build supports (%d): it was written by a newer Lumilio Photos; upgrade Lumilio Photos to open it",
			version,
			current,
		)
	}
	if version >= 1 {
		return nil
	}
	if version < 0 {
		return fmt.Errorf("catalog schema version = %d is invalid", version)
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
			"catalog has %d user tables but PRAGMA user_version = 0: this catalog is incomplete or damaged; restore a backup, or move the SQLite catalog aside and restart Lumilio Photos (media repositories and original files are not touched)",
			userTables,
		)
	}
	return nil
}

// applyCatalogBaseline executes the embedded baseline on an empty catalog.
// The baseline stamps PRAGMA user_version = 1. A failure rolls the whole
// transaction back, leaving an empty catalog for a clean retry.
func applyCatalogBaseline(ctx context.Context, database *sql.DB) error {
	body, err := loadCatalogBaseline()
	if err != nil {
		return err
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
	log.Printf("Lumilio schema baseline applied: version=1 file=%s", baselineMigrationFile)
	return nil
}

// applyCatalogSteps runs every step above the catalog's version in order, each
// in its own transaction together with its user_version stamp.
func applyCatalogSteps(ctx context.Context, database *sql.DB, steps []catalogStep, backupPath string) error {
	for _, step := range steps {
		version, err := readSchemaVersion(ctx, database)
		if err != nil {
			return err
		}
		if step.Version <= version {
			continue
		}
		if err := applyCatalogStep(ctx, database, step); err != nil {
			failure := fmt.Errorf(
				"catalog upgrade step %d (%s) failed and was rolled back; the catalog remains at schema version %d: %w",
				step.Version,
				step.Name,
				version,
				err,
			)
			if backupPath != "" {
				failure = fmt.Errorf("%w (pre-upgrade backup: %s)", failure, backupPath)
			}
			return failure
		}
		log.Printf("Lumilio catalog upgrade step applied: version=%d name=%s", step.Version, step.Name)
	}
	return nil
}

func applyCatalogStep(ctx context.Context, database *sql.DB, step catalogStep) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if err := step.Apply(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", step.Version)); err != nil {
		return fmt.Errorf("stamp schema version: %w", err)
	}
	return tx.Commit()
}

// loadCatalogSteps reads the embedded SQL steps and requires them to be the
// contiguous sequence 2..current, so a step cannot be skipped and
// SchemaVersion cannot drift from the files that reach it.
func loadCatalogSteps(fsys fs.FS, current int) ([]catalogStep, error) {
	entries, err := fs.ReadDir(fsys, catalogStepsDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read embedded catalog steps: %w", err)
	}
	var steps []catalogStep
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		prefix, rest, ok := strings.Cut(strings.TrimSuffix(entry.Name(), ".sql"), "_")
		version, parseErr := strconv.Atoi(prefix)
		if !ok || parseErr != nil || len(prefix) != 4 || rest == "" {
			return nil, fmt.Errorf("catalog step %s must be named NNNN_<name>.sql", entry.Name())
		}
		body, err := fs.ReadFile(fsys, path.Join(catalogStepsDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read catalog step %s: %w", entry.Name(), err)
		}
		if strings.Contains(strings.ToLower(string(body)), "user_version") {
			return nil, fmt.Errorf("catalog step %s must not set user_version; the runner stamps it", entry.Name())
		}
		sqlText := string(body)
		steps = append(steps, catalogStep{
			Version: version,
			Name:    rest,
			Apply: func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, sqlText)
				return err
			},
		})
	}
	for index, step := range steps {
		if want := index + 2; step.Version != want {
			return nil, fmt.Errorf("catalog step versions must run 2..%d without gaps: found %d where %d belongs", current, step.Version, want)
		}
	}
	if last := len(steps) + 1; last != current {
		return nil, fmt.Errorf("catalog steps reach schema version %d, but SchemaVersion is %d", last, current)
	}
	return steps, nil
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
