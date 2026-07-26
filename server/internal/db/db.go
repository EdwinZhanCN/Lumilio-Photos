package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"server/config"
	"server/internal/db/repo"

	sqlitevec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

const (
	applicationID = 0x4c554d49 // "LUMI"
	fileMode      = 0o600
	directoryMode = 0o700
)

var registerVectorExtension sync.Once

// DB is the single SQLite runtime boundary used by application queries, River,
// and short application transactions.
type DB struct {
	SQL     *sql.DB
	Queries *repo.Queries
	Path    string
}

// CatalogInfo is the independently verified identity and schema state of a
// SQLite catalog or backup snapshot.
type CatalogInfo struct {
	Path                     string
	LibraryID                string
	SQLiteVersion            string
	VectorVersion            string
	ApplicationMigration     int64
	RiverMigration           int64
	SizeBytes                int64
	QuickCheck               string
	ForeignKeyViolationCount int
}

// Open creates or opens the configured library catalog and applies the fixed
// connection policy. The one physical connection is intentionally never
// recycled because SQLite pragmas and statically registered extensions are
// connection-local.
func Open(ctx context.Context, cfg config.DatabaseConfig) (*DB, error) {
	path, err := normalizePath(cfg.Path)
	if err != nil {
		return nil, err
	}
	if err := ensurePrivateParent(path); err != nil {
		return nil, err
	}

	registerVectorExtension.Do(sqlitevec.Auto)

	dsn := (&url.URL{Scheme: "file", Path: path}).String()
	database, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open SQLite catalog %s: %w", safeLocation(path), err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	database.SetConnMaxLifetime(0)
	database.SetConnMaxIdleTime(0)

	closeOnError := func(operation string, cause error) (*DB, error) {
		_ = database.Close()
		return nil, fmt.Errorf("%s SQLite catalog %s: %w", operation, safeLocation(path), cause)
	}
	if err := database.PingContext(ctx); err != nil {
		return closeOnError("open", err)
	}
	if err := applyPragmas(ctx, database); err != nil {
		return closeOnError("configure", err)
	}
	if err := claimOrVerifyCatalog(ctx, database); err != nil {
		return closeOnError("verify identity of", err)
	}
	if err := validateIntegrity(ctx, database); err != nil {
		return closeOnError("validate", err)
	}
	if err := os.Chmod(path, fileMode); err != nil {
		return closeOnError("secure", err)
	}

	var sqliteVersion, vectorVersion string
	if err := database.QueryRowContext(ctx, "SELECT sqlite_version(), vec_version()").Scan(&sqliteVersion, &vectorVersion); err != nil {
		return closeOnError("probe versions for", err)
	}
	log.Printf(
		"SQLite catalog opened: location=%s sqlite=%s vector=%s writer_connections=1",
		safeLocation(path),
		sqliteVersion,
		vectorVersion,
	)

	return &DB{
		SQL:     database,
		Queries: repo.New(database),
		Path:    path,
	}, nil
}

// WithTx runs fn in a short write transaction shared by application queries
// and River InsertTx calls.
func (d *DB) WithTx(ctx context.Context, fn func(*sql.Tx, *repo.Queries) error) error {
	started := time.Now()
	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin SQLite transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := fn(tx, d.Queries.WithTx(tx)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit SQLite transaction after %s: %w", time.Since(started), err)
	}
	return nil
}

// Check validates the live catalog after migrations or before a destructive
// restore boundary.
func (d *DB) Check(ctx context.Context) error {
	if err := validateIntegrity(ctx, d.SQL); err != nil {
		return fmt.Errorf("validate SQLite catalog %s: %w", safeLocation(d.Path), err)
	}
	return nil
}

// InspectCatalog opens a catalog through an independent read-only connection
// and verifies that it is a healthy Lumilio SQLite database. Backup validation
// deliberately does not reuse the live application handle.
func InspectCatalog(ctx context.Context, path string) (CatalogInfo, error) {
	return inspectCatalog(ctx, path, false)
}

// InspectStandaloneCatalog validates a checkpointed catalog without creating
// SQLite WAL/SHM sidecars. It must only be used for immutable snapshots, never
// for an active WAL catalog whose newest pages may still live in the WAL.
func InspectStandaloneCatalog(ctx context.Context, path string) (CatalogInfo, error) {
	return inspectCatalog(ctx, path, true)
}

func inspectCatalog(ctx context.Context, path string, immutable bool) (CatalogInfo, error) {
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return CatalogInfo{}, fmt.Errorf("resolve SQLite catalog %q: %w", path, err)
	}
	cleanPath = filepath.Clean(cleanPath)

	registerVectorExtension.Do(sqlitevec.Auto)
	location := &url.URL{Scheme: "file", Path: cleanPath}
	query := location.Query()
	query.Set("mode", "ro")
	if immutable {
		query.Set("immutable", "1")
	}
	location.RawQuery = query.Encode()

	database, err := sql.Open("sqlite3", location.String())
	if err != nil {
		return CatalogInfo{}, fmt.Errorf("open SQLite catalog snapshot %s: %w", safeLocation(cleanPath), err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	defer database.Close()

	var catalogApplicationID int
	if err := database.QueryRowContext(ctx, "PRAGMA application_id").Scan(&catalogApplicationID); err != nil {
		return CatalogInfo{}, fmt.Errorf("read SQLite application_id: %w", err)
	}
	if catalogApplicationID != applicationID {
		return CatalogInfo{}, fmt.Errorf("SQLite application_id = %#x, want %#x", catalogApplicationID, applicationID)
	}

	info := CatalogInfo{Path: cleanPath}
	if err := database.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&info.QuickCheck); err != nil {
		return CatalogInfo{}, fmt.Errorf("run SQLite quick_check: %w", err)
	}
	if info.QuickCheck != "ok" {
		return CatalogInfo{}, fmt.Errorf("SQLite quick_check failed: %s", info.QuickCheck)
	}

	rows, err := database.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return CatalogInfo{}, fmt.Errorf("run SQLite foreign_key_check: %w", err)
	}
	for rows.Next() {
		info.ForeignKeyViolationCount++
	}
	if err := rows.Close(); err != nil {
		return CatalogInfo{}, fmt.Errorf("close SQLite foreign_key_check rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return CatalogInfo{}, fmt.Errorf("iterate SQLite foreign_key_check: %w", err)
	}
	if info.ForeignKeyViolationCount != 0 {
		return CatalogInfo{}, fmt.Errorf("SQLite foreign_key_check found %d violations", info.ForeignKeyViolationCount)
	}

	if err := database.QueryRowContext(ctx, "SELECT sqlite_version(), vec_version()").Scan(&info.SQLiteVersion, &info.VectorVersion); err != nil {
		return CatalogInfo{}, fmt.Errorf("probe SQLite snapshot versions: %w", err)
	}
	if err := database.QueryRowContext(ctx, "SELECT library_id FROM system_state WHERE id = 1").Scan(&info.LibraryID); err != nil {
		return CatalogInfo{}, fmt.Errorf("read SQLite library identity: %w", err)
	}
	if err := database.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM lumilio_schema_migrations").Scan(&info.ApplicationMigration); err != nil {
		return CatalogInfo{}, fmt.Errorf("read application migration version: %w", err)
	}
	if err := database.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM river_migration WHERE line = 'main'").Scan(&info.RiverMigration); err != nil {
		return CatalogInfo{}, fmt.Errorf("read River migration version: %w", err)
	}
	fileInfo, err := os.Stat(cleanPath)
	if err != nil {
		return CatalogInfo{}, fmt.Errorf("stat SQLite catalog snapshot: %w", err)
	}
	info.SizeBytes = fileInfo.Size()
	return info, nil
}

// Close performs bounded maintenance after HTTP and River have drained, then
// closes the sole connection.
func (d *DB) Close(ctx context.Context) error {
	if d == nil || d.SQL == nil {
		return nil
	}

	var maintenanceErr error
	if _, err := d.SQL.ExecContext(ctx, "PRAGMA optimize"); err != nil {
		maintenanceErr = errors.Join(maintenanceErr, fmt.Errorf("optimize SQLite catalog: %w", err))
	}
	var busy, logPages, checkpointed int
	if err := d.SQL.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logPages, &checkpointed); err != nil {
		maintenanceErr = errors.Join(maintenanceErr, fmt.Errorf("checkpoint SQLite catalog: %w", err))
	} else if busy != 0 {
		maintenanceErr = errors.Join(maintenanceErr, fmt.Errorf(
			"checkpoint SQLite catalog remained busy: log_pages=%d checkpointed=%d",
			logPages,
			checkpointed,
		))
	}
	if err := d.SQL.Close(); err != nil {
		maintenanceErr = errors.Join(maintenanceErr, fmt.Errorf("close SQLite catalog: %w", err))
	}
	if maintenanceErr == nil {
		log.Printf("SQLite catalog closed: location=%s", safeLocation(d.Path))
	}
	return maintenanceErr
}

func normalizePath(value string) (string, error) {
	if value == "" || value == ":memory:" {
		return "", errors.New("SQLite database.path must be a persistent filesystem path")
	}
	path, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("resolve SQLite database.path %q: %w", value, err)
	}
	return filepath.Clean(path), nil
}

func ensurePrivateParent(path string) error {
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(parent, directoryMode); err != nil {
			return fmt.Errorf("create SQLite parent directory %s: %w", safeLocation(parent), err)
		}
	case err != nil:
		return fmt.Errorf("inspect SQLite parent directory %s: %w", safeLocation(parent), err)
	case !info.IsDir():
		return fmt.Errorf("SQLite parent path %s is not a directory", safeLocation(parent))
	case info.Mode().Perm()&0o077 != 0:
		return fmt.Errorf("SQLite parent directory %s must not be accessible by group or others", safeLocation(parent))
	}
	return nil
}

func applyPragmas(ctx context.Context, database *sql.DB) error {
	statements := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA temp_store = MEMORY",
		"PRAGMA wal_autocheckpoint = 1000",
	}
	for _, statement := range statements {
		if _, err := database.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply %q: %w", statement, err)
		}
	}

	var journalMode string
	if err := database.QueryRowContext(ctx, "PRAGMA journal_mode = WAL").Scan(&journalMode); err != nil {
		return fmt.Errorf("enable WAL: %w", err)
	}
	if journalMode != "wal" {
		return fmt.Errorf("journal_mode = %q, want wal", journalMode)
	}

	checks := []struct {
		name string
		want int
	}{
		{name: "foreign_keys", want: 1},
		{name: "synchronous", want: 1},
		{name: "busy_timeout", want: 5000},
		{name: "temp_store", want: 2},
		{name: "wal_autocheckpoint", want: 1000},
	}
	for _, check := range checks {
		var got int
		if err := database.QueryRowContext(ctx, "PRAGMA "+check.name).Scan(&got); err != nil {
			return fmt.Errorf("read PRAGMA %s: %w", check.name, err)
		}
		if got != check.want {
			return fmt.Errorf("PRAGMA %s = %d, want %d", check.name, got, check.want)
		}
	}
	return nil
}

func claimOrVerifyCatalog(ctx context.Context, database *sql.DB) error {
	var got int
	if err := database.QueryRowContext(ctx, "PRAGMA application_id").Scan(&got); err != nil {
		return fmt.Errorf("read application_id: %w", err)
	}
	if got == applicationID {
		return nil
	}
	if got != 0 {
		return fmt.Errorf("application_id = %#x, want Lumilio %#x", got, applicationID)
	}

	var userTables int
	if err := database.QueryRowContext(ctx, `
		SELECT count(*)
		FROM sqlite_schema
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
	`).Scan(&userTables); err != nil {
		return fmt.Errorf("inspect unclaimed catalog: %w", err)
	}
	if userTables != 0 {
		return fmt.Errorf("unrecognized SQLite catalog has %d user tables and no Lumilio application_id", userTables)
	}
	if _, err := database.ExecContext(ctx, fmt.Sprintf("PRAGMA application_id = %d", applicationID)); err != nil {
		return fmt.Errorf("set application_id: %w", err)
	}
	return nil
}

func validateIntegrity(ctx context.Context, database *sql.DB) error {
	var quickCheck string
	if err := database.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&quickCheck); err != nil {
		return fmt.Errorf("quick_check: %w", err)
	}
	if quickCheck != "ok" {
		return fmt.Errorf("quick_check: %s", quickCheck)
	}

	rows, err := database.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("foreign_key_check: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		var table, parent string
		var rowID, foreignKeyID any
		if err := rows.Scan(&table, &rowID, &parent, &foreignKeyID); err != nil {
			return fmt.Errorf("scan foreign_key_check result: %w", err)
		}
		return fmt.Errorf(
			"foreign_key_check: table=%s rowid=%v parent=%s foreign_key=%v",
			table,
			rowID,
			parent,
			foreignKeyID,
		)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate foreign_key_check: %w", err)
	}
	return nil
}

func safeLocation(path string) string {
	return filepath.Clean(path)
}
