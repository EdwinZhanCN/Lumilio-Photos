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
	"runtime"
	"strings"
	"sync"
	"time"

	"server/config"
	"server/internal/db/catalogtx"
	"server/internal/db/repo"
	"server/internal/db/vec1ext"
	"server/platform/fsprivacy"
	"server/platform/sqliteuri"

	"github.com/mattn/go-sqlite3"
)

const (
	applicationID = 0x4c554d49 // "LUMI"
	fileMode      = 0o600
	directoryMode = 0o700
)

var registerVec1Extension sync.Once

const sqliteWriteTransactionBudget = 25 * time.Millisecond

// DB owns the SQLite catalog's explicit connection roles. SQL is the sole
// writer pool shared by application writes and River. ReaderSQL is a bounded,
// query-only WAL pool for foreground reads and controller snapshots.
type DB struct {
	SQL           *sql.DB
	Queries       *repo.Queries
	ReaderSQL     *sql.DB
	ReaderQueries *repo.Queries
	Writer        *catalogtx.Writer
	Reader        *catalogtx.Reader
	Path          string

	transactionRecorder *catalogtx.Recorder
}

type openOptions struct {
	transactionObserver catalogtx.Observer
}

// OpenOption customizes internal catalog diagnostics without changing the
// schema-versioned runtime manifest.
type OpenOption func(*openOptions)

// WithTransactionObserver adds a process-local observer to the always-on,
// bounded HDR recorder. It is intended for tests and host-side harness wiring;
// it does not publish an HTTP debug surface.
func WithTransactionObserver(observer catalogtx.Observer) OpenOption {
	return func(options *openOptions) {
		options.transactionObserver = observer
	}
}

// CatalogInfo is the independently verified identity and schema state of a
// SQLite catalog or backup snapshot.
type CatalogInfo struct {
	Path                     string
	LibraryID                string
	SQLiteVersion            string
	Vec1Version              string
	ApplicationMigration     int64
	RiverMigration           int64
	SizeBytes                int64
	QuickCheck               string
	ForeignKeyViolationCount int
}

// CheckpointResult describes one explicit PASSIVE WAL checkpoint. PASSIVE is
// intentionally non-blocking with respect to active readers; a later pass can
// finish pages pinned by an older read snapshot.
type CheckpointResult struct {
	Busy         int
	LogPages     int
	Checkpointed int
	Duration     time.Duration
}

// WALState is a cheap filesystem observation used to avoid repeating a
// completed PASSIVE checkpoint merely because SQLite retained the allocated
// WAL file for reuse.
type WALState struct {
	SizeBytes  int64
	ModifiedAt time.Time
}

// Open creates or opens the configured library catalog and applies the fixed
// policy to every physical connection. DSN parameters own pragmas supported by
// go-sqlite3; the driver hook owns the remaining connection-local settings.
func Open(ctx context.Context, cfg config.DatabaseConfig, suppliedOptions ...OpenOption) (*DB, error) {
	options := openOptions{}
	for _, option := range suppliedOptions {
		if option != nil {
			option(&options)
		}
	}
	transactionRecorder := catalogtx.NewRecorder()
	transactionObserver := catalogtx.JoinObservers(
		transactionRecorder,
		options.transactionObserver,
		catalogLogObserver{},
	)
	path, err := normalizePath(cfg.Path)
	if err != nil {
		return nil, err
	}
	if err := ensurePrivateParent(path); err != nil {
		return nil, err
	}

	registerVec1Extension.Do(vec1ext.Auto)
	sqliteDriver := &sqlite3.SQLiteDriver{ConnectHook: configureSQLiteConnection}

	writerQuery := url.Values{
		"_busy_timeout": {"5000"},
		"_foreign_keys": {"on"},
		"_journal_mode": {"WAL"},
		"_synchronous":  {"NORMAL"},
	}
	writerDSN := sqliteuri.DSN(path, writerQuery)
	database := sql.OpenDB(catalogtx.NewConnector(sqliteDriver, writerDSN, catalogtx.RoleWriter, transactionObserver))
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
	if err := verifyPragmas(ctx, database); err != nil {
		return closeOnError("verify connection policy for", err)
	}
	if err := claimOrVerifyCatalog(ctx, database); err != nil {
		return closeOnError("verify identity of", err)
	}
	if err := assertSchemaGeneration(ctx, database); err != nil {
		return closeOnError("verify schema generation of", err)
	}
	if err := validateIntegrity(ctx, database); err != nil {
		return closeOnError("validate", err)
	}
	if err := fsprivacy.ApplyFileMode(path, fileMode); err != nil {
		return closeOnError("secure", err)
	}

	readerQuery := url.Values{
		"mode":          {"ro"},
		"_busy_timeout": {"5000"},
		"_foreign_keys": {"on"},
		"_query_only":   {"on"},
		"_synchronous":  {"NORMAL"},
	}
	reader := sql.OpenDB(catalogtx.NewConnector(
		sqliteDriver,
		sqliteuri.DSN(path, readerQuery),
		catalogtx.RoleReader,
		transactionObserver,
	))
	reader.SetMaxOpenConns(4)
	reader.SetMaxIdleConns(4)
	reader.SetConnMaxLifetime(0)
	reader.SetConnMaxIdleTime(0)
	closeBothOnError := func(operation string, cause error) (*DB, error) {
		_ = reader.Close()
		return closeOnError(operation, cause)
	}
	if err := reader.PingContext(ctx); err != nil {
		return closeBothOnError("open query-only reader for", err)
	}
	if err := verifyReaderPragmas(ctx, reader); err != nil {
		return closeBothOnError("verify query-only reader policy for", err)
	}

	var sqliteVersion, vec1Info string
	if err := database.QueryRowContext(ctx, "SELECT sqlite_version(), vec1_info()").Scan(&sqliteVersion, &vec1Info); err != nil {
		return closeBothOnError("probe versions for", err)
	}
	if _, err := parseVec1Version(vec1Info); err != nil {
		return closeBothOnError("probe Vec1 version for", err)
	}
	log.Printf(
		"SQLite catalog opened: location=%s sqlite=%s vec1=%s writer_connections=1 reader_connections=4",
		safeLocation(path),
		sqliteVersion,
		vec1Info,
	)
	writerCapability := catalogtx.NewWriter(database, transactionObserver)
	readerCapability := catalogtx.NewReader(reader, transactionObserver)

	return &DB{
		SQL:           database,
		Queries:       repo.New(newQueryRouter(database, reader, writerCapability, readerCapability)),
		ReaderSQL:     reader,
		ReaderQueries: repo.New(reader),
		Writer:        writerCapability,
		Reader:        readerCapability,
		Path:          path,

		transactionRecorder: transactionRecorder,
	}, nil
}

// WithTx runs fn in one named, measured write transaction shared by
// application queries and River InsertTx calls.
func (d *DB) WithTx(
	ctx context.Context,
	operation catalogtx.Operation,
	fn func(*sql.Tx, *repo.Queries) error,
) error {
	if d == nil || d.Writer == nil {
		return catalogtx.ErrNilPool
	}
	return d.Writer.Transact(ctx, operation, nil, func(tx *sql.Tx) error {
		return fn(tx, d.Queries.WithTx(tx))
	})
}

// TransactionReport returns a bounded in-memory HDR summary. It contains only
// compile-time operation names and durations, never entity identifiers.
func (d *DB) TransactionReport() catalogtx.Report {
	if d == nil || d.transactionRecorder == nil {
		return catalogtx.Report{}
	}
	return d.transactionRecorder.Report()
}

// TransactionIntervalReport atomically rotates an exact mergeable HDR
// interval while preserving TransactionReport's process-lifetime totals.
func (d *DB) TransactionIntervalReport() (catalogtx.Report, error) {
	if d == nil || d.transactionRecorder == nil {
		return catalogtx.Report{}, nil
	}
	return d.transactionRecorder.IntervalReport()
}

func logSlowCatalogTransaction(sample catalogtx.TransactionSample) {
	hold := sample.Total - sample.Admission
	if sample.Role != catalogtx.RoleWriter || hold <= sqliteWriteTransactionBudget {
		return
	}
	log.Printf(
		"SQLite write transaction exceeded hold budget: operation=%s admission=%s hold=%s body=%s commit=%s total=%s outcome=%s cancellation=%s budget=%s",
		sample.OperationName,
		sample.Admission,
		hold,
		sample.Body,
		sample.Commit,
		sample.Total,
		sample.Outcome,
		sample.Cancellation,
		sqliteWriteTransactionBudget,
	)
}

type catalogLogObserver struct{}

func (catalogLogObserver) ObserveTransaction(sample catalogtx.TransactionSample) {
	logSlowCatalogTransaction(sample)
}

func (catalogLogObserver) ObserveStatement(sample catalogtx.StatementSample) {
	hold := sample.Total - sample.Admission
	if sample.Outcome == catalogtx.StatementOutcomeFailed {
		log.Printf(
			"SQLite statement failed: operation=%s query_name=%s query_fingerprint=%s role=%s admission=%s hold=%s total=%s cancellation=%s",
			sample.OperationName,
			sample.QueryName,
			sample.QueryFingerprint,
			sample.Role,
			sample.Admission,
			hold,
			sample.Total,
			sample.Cancellation,
		)
	}
	if sample.Role != catalogtx.RoleWriter || hold <= sqliteWriteTransactionBudget {
		return
	}
	log.Printf(
		"SQLite writer statement exceeded hold budget: operation=%s admission=%s hold=%s execution=%s rows_lifetime=%s total=%s outcome=%s cancellation=%s budget=%s",
		sample.OperationName,
		sample.Admission,
		hold,
		sample.Execution,
		sample.RowsLifetime,
		sample.Total,
		sample.Outcome,
		sample.Cancellation,
		sqliteWriteTransactionBudget,
	)
}

func (catalogLogObserver) ObserveRows(catalogtx.RowsEvent) {}

// TelemetrySnapshot aligns bounded catalog histograms with database/sql's
// aggregate wait counters. Per-operation p99 comes from Catalog; DBStats is a
// reconciliation signal and is never divided into a synthetic percentile.
type TelemetrySnapshot struct {
	ObservedAt time.Time        `json:"observed_at"`
	Writer     sql.DBStats      `json:"writer"`
	Reader     sql.DBStats      `json:"reader"`
	Catalog    catalogtx.Report `json:"catalog"`
}

func (d *DB) TelemetrySnapshot() TelemetrySnapshot {
	result := TelemetrySnapshot{ObservedAt: time.Now().UTC(), Catalog: d.TransactionReport()}
	if d == nil {
		return result
	}
	if d.SQL != nil {
		result.Writer = d.SQL.Stats()
	}
	if d.ReaderSQL != nil {
		result.Reader = d.ReaderSQL.Stats()
	}
	return result
}

// PassiveCheckpoint performs the runtime WAL maintenance that is deliberately
// disabled on foreground commits. It uses the same single writer pool as all
// other writes, so checkpoint work is serialized rather than racing a second
// write-capable connection.
func (d *DB) PassiveCheckpoint(ctx context.Context) (CheckpointResult, error) {
	started := time.Now()
	var result CheckpointResult
	if err := d.Writer.QueryRowContext(ctx, catalogtx.OperationSQLitePassiveCheckpoint, "PRAGMA wal_checkpoint(PASSIVE)").Scan(
		&result.Busy,
		&result.LogPages,
		&result.Checkpointed,
	); err != nil {
		return CheckpointResult{}, fmt.Errorf("passive SQLite WAL checkpoint: %w", err)
	}
	result.Duration = time.Since(started)
	return result, nil
}

// InspectWAL returns the currently allocated live WAL file version. A missing
// WAL means the catalog has no accumulated frames and is reported as zero.
func (d *DB) InspectWAL() (WALState, error) {
	info, err := os.Stat(d.Path + "-wal")
	if errors.Is(err, os.ErrNotExist) {
		return WALState{}, nil
	}
	if err != nil {
		return WALState{}, fmt.Errorf("inspect SQLite WAL: %w", err)
	}
	return WALState{SizeBytes: info.Size(), ModifiedAt: info.ModTime()}, nil
}

// WALSize returns the allocated WAL bytes for callers that do not need its
// file version.
func (d *DB) WALSize() (int64, error) {
	state, err := d.InspectWAL()
	return state.SizeBytes, err
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

	registerVec1Extension.Do(vec1ext.Auto)
	query := make(url.Values)
	query.Set("mode", "ro")
	if immutable {
		query.Set("immutable", "1")
	}

	database, err := sql.Open("sqlite3", sqliteuri.DSN(cleanPath, query))
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

	var vec1Info string
	if err := database.QueryRowContext(ctx, "SELECT sqlite_version(), vec1_info()").Scan(&info.SQLiteVersion, &vec1Info); err != nil {
		return CatalogInfo{}, fmt.Errorf("probe SQLite snapshot versions: %w", err)
	}
	info.Vec1Version, err = parseVec1Version(vec1Info)
	if err != nil {
		return CatalogInfo{}, fmt.Errorf("probe SQLite snapshot Vec1 version: %w", err)
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

func parseVec1Version(info string) (string, error) {
	const prefix = "version "
	if !strings.HasPrefix(info, prefix) {
		return "", fmt.Errorf("unexpected vec1_info() value %q", info)
	}
	version, _, _ := strings.Cut(strings.TrimPrefix(info, prefix), " ")
	if version == "" {
		return "", fmt.Errorf("unexpected vec1_info() value %q", info)
	}
	return version, nil
}

// Close releases readers before bounded writer maintenance so no lingering
// read snapshot can hold WAL checkpoint progress.
func (d *DB) Close(ctx context.Context) error {
	if d == nil || d.SQL == nil {
		return nil
	}

	var maintenanceErr error
	if d.ReaderSQL != nil {
		if err := d.ReaderSQL.Close(); err != nil {
			maintenanceErr = errors.Join(maintenanceErr, fmt.Errorf("close SQLite reader pool: %w", err))
		}
	}
	if _, err := d.Writer.ExecContext(ctx, catalogtx.OperationSQLiteOptimize, "PRAGMA optimize"); err != nil {
		maintenanceErr = errors.Join(maintenanceErr, fmt.Errorf("optimize SQLite catalog: %w", err))
	}
	var busy, logPages, checkpointed int
	if err := d.Writer.QueryRowContext(ctx, catalogtx.OperationSQLiteTruncateCheckpoint, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logPages, &checkpointed); err != nil {
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

// SQLiteDriverConnection unwraps catalog observation and returns the native
// connection required by SQLite-specific APIs such as Online Backup and
// sqlite3_stmt_readonly.
func SQLiteDriverConnection(connection any) (*sqlite3.SQLiteConn, bool) {
	native, ok := catalogtx.UnwrapDriverConnection(connection).(*sqlite3.SQLiteConn)
	return native, ok
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
	created := false
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(parent, directoryMode); err != nil {
			return fmt.Errorf("create SQLite parent directory %s: %w", safeLocation(parent), err)
		}
		created = true
	case err != nil:
		return fmt.Errorf("inspect SQLite parent directory %s: %w", safeLocation(parent), err)
	case !info.IsDir():
		return fmt.Errorf("SQLite parent path %s is not a directory", safeLocation(parent))
	}

	if created || runtime.GOOS == "windows" {
		if err := fsprivacy.ApplyDirectoryMode(parent, directoryMode); err != nil {
			return fmt.Errorf("protect SQLite parent directory %s: %w", safeLocation(parent), err)
		}
		if runtime.GOOS == "windows" {
			private, err := fsprivacy.IsPrivate(parent)
			if err != nil {
				return fmt.Errorf("verify SQLite parent directory %s: %w", safeLocation(parent), err)
			}
			if !private {
				return fmt.Errorf("SQLite parent directory %s does not have a protected DACL", safeLocation(parent))
			}
		}
		return nil
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("SQLite parent directory %s must not be accessible by group or others", safeLocation(parent))
	}
	return nil
}

func configureSQLiteConnection(connection *sqlite3.SQLiteConn) error {
	statements := []string{
		"PRAGMA temp_store = MEMORY",
		// Runtime maintenance owns checkpoint timing. SQLite's default
		// auto-checkpoint otherwise charges an arbitrary foreground commit
		// for all checkpoint work when it crosses the page threshold.
		"PRAGMA wal_autocheckpoint = 0",
	}
	for _, statement := range statements {
		if _, err := connection.Exec(statement, nil); err != nil {
			return fmt.Errorf("apply %q: %w", statement, err)
		}
	}
	return nil
}

func verifyPragmas(ctx context.Context, database *sql.DB) error {
	var journalMode string
	if err := database.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		return fmt.Errorf("read journal_mode: %w", err)
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
		{name: "wal_autocheckpoint", want: 0},
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

func verifyReaderPragmas(ctx context.Context, database *sql.DB) error {
	if err := verifyPragmas(ctx, database); err != nil {
		return err
	}
	var queryOnly int
	if err := database.QueryRowContext(ctx, "PRAGMA query_only").Scan(&queryOnly); err != nil {
		return fmt.Errorf("read query_only: %w", err)
	}
	if queryOnly != 1 {
		return fmt.Errorf("PRAGMA query_only = %d, want 1", queryOnly)
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
