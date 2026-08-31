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
	"strings"
	"time"

	"server/config"
	"server/internal/db/catalogtx"
	"server/platform/fsprivacy"
	"server/platform/sqliteuri"

	"github.com/mattn/go-sqlite3"
)

const queueApplicationID = 0x4c554d51 // "LUMQ"

// ErrQueueDatabaseCorrupt marks a queue file that cannot be trusted as River
// execution state. The catalog remains authoritative; callers may quarantine
// this file and rebuild it without touching catalog facts.
var ErrQueueDatabaseCorrupt = errors.New("SQLite queue database is corrupt or unrecognized")

// QueueDB owns River's execution-state SQLite file. It deliberately has no
// application query layer: catalog facts and work intent remain in DB, while
// this handle exposes only River's writer and query-only listener pools.
// QueueDB is opened and closed independently so queue traffic can overlap
// catalog writes without admitting work to the catalog writer.
type QueueDB struct {
	SQL       *sql.DB
	ReaderSQL *sql.DB
	Path      string

	transactionRecorder *catalogtx.Recorder
}

// OpenQueue opens the explicit queue_path from the complete runtime manifest.
// It uses the same verified SQLite pragma policy as the catalog but does not
// load Vec1, application schema, or catalog identity tables.
func OpenQueue(ctx context.Context, cfg config.DatabaseConfig, suppliedOptions ...OpenOption) (*QueueDB, error) {
	options := openOptions{}
	for _, option := range suppliedOptions {
		if option != nil {
			option(&options)
		}
	}
	path, err := normalizeQueuePath(cfg.QueuePath)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Path) != "" {
		catalogPath, catalogErr := normalizePath(cfg.Path)
		if catalogErr != nil {
			return nil, catalogErr
		}
		if filepath.Clean(path) == filepath.Clean(catalogPath) {
			return nil, errors.New("SQLite queue_path must differ from database.path")
		}
	}
	if err := ensurePrivateParent(path); err != nil {
		return nil, err
	}

	transactionRecorder := catalogtx.NewRecorder()
	transactionObserver := catalogtx.JoinObservers(
		transactionRecorder,
		options.transactionObserver,
		queueLogObserver{},
	)
	sqliteDriver := &sqlite3.SQLiteDriver{ConnectHook: configureSQLiteConnection}
	writerQuery := url.Values{
		"_busy_timeout": {"5000"},
		"_foreign_keys": {"on"},
		"_journal_mode": {"WAL"},
		"_synchronous":  {"NORMAL"},
	}
	writer := sql.OpenDB(catalogtx.NewConnector(
		sqliteDriver,
		sqliteuri.DSN(path, writerQuery),
		catalogtx.RoleWriter,
		transactionObserver,
	))
	writer.SetMaxOpenConns(1)
	writer.SetMaxIdleConns(1)
	writer.SetConnMaxLifetime(0)
	writer.SetConnMaxIdleTime(0)
	closeWriter := func(operation string, cause error) (*QueueDB, error) {
		_ = writer.Close()
		return nil, fmt.Errorf("%s SQLite queue database %s: %w", operation, safeLocation(path), cause)
	}
	if err := writer.PingContext(ctx); err != nil {
		return closeWriter("open", queueCorruptionError(err))
	}
	if err := verifyPragmas(ctx, writer); err != nil {
		return closeWriter("verify connection policy for", err)
	}
	if err := claimOrVerifyQueue(ctx, writer); err != nil {
		return closeWriter("verify identity of", err)
	}
	if err := validateIntegrity(ctx, writer); err != nil {
		return closeWriter("validate", fmt.Errorf("%w: %v", ErrQueueDatabaseCorrupt, err))
	}
	if err := fsprivacy.ApplyFileMode(path, fileMode); err != nil {
		return closeWriter("secure", err)
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
	closeBoth := func(operation string, cause error) (*QueueDB, error) {
		_ = reader.Close()
		return closeWriter(operation, cause)
	}
	if err := reader.PingContext(ctx); err != nil {
		return closeBoth("open query-only reader for", queueCorruptionError(err))
	}
	if err := verifyReaderPragmas(ctx, reader); err != nil {
		return closeBoth("verify query-only reader policy for", err)
	}

	log.Printf("SQLite queue database opened: location=%s writer_connections=1 reader_connections=4", safeLocation(path))
	return &QueueDB{SQL: writer, ReaderSQL: reader, Path: path, transactionRecorder: transactionRecorder}, nil
}

// OpenQueueWithRecovery quarantines only an explicitly unrecognized/corrupt
// existing queue file. A missing queue is created normally; permission and
// configuration errors are returned without moving user data.
func OpenQueueWithRecovery(ctx context.Context, cfg config.DatabaseConfig, suppliedOptions ...OpenOption) (*QueueDB, string, error) {
	queueDB, err := OpenQueue(ctx, cfg, suppliedOptions...)
	if err == nil {
		return queueDB, "", nil
	}
	if !errors.Is(err, ErrQueueDatabaseCorrupt) {
		return nil, "", err
	}
	path, pathErr := normalizeQueuePath(cfg.QueuePath)
	if pathErr != nil {
		return nil, "", err
	}
	if _, statErr := os.Stat(path); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return nil, "", err
		}
		return nil, "", fmt.Errorf("inspect corrupt SQLite queue %s: %w", safeLocation(path), statErr)
	}
	suffix := fmt.Sprintf(".quarantine-%d", time.Now().UTC().UnixNano())
	quarantinePath := path + suffix
	if renameErr := os.Rename(path, quarantinePath); renameErr != nil {
		return nil, "", fmt.Errorf("quarantine corrupt SQLite queue %s: %w", safeLocation(path), renameErr)
	}
	for _, sidecar := range []string{"-wal", "-shm"} {
		if renameErr := renameIfExists(path+sidecar, quarantinePath+sidecar); renameErr != nil {
			return nil, quarantinePath, renameErr
		}
	}
	queueDB, retryErr := OpenQueue(ctx, cfg, suppliedOptions...)
	if retryErr != nil {
		return nil, quarantinePath, fmt.Errorf("reopen rebuilt SQLite queue after quarantine: %w", retryErr)
	}
	return queueDB, quarantinePath, nil
}

func renameIfExists(source, target string) error {
	if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if err := os.Rename(source, target); err != nil {
		return fmt.Errorf("quarantine SQLite sidecar %s: %w", safeLocation(source), err)
	}
	return nil
}

func normalizeQueuePath(value string) (string, error) {
	if strings.TrimSpace(value) == "" || value == ":memory:" {
		return "", errors.New("SQLite database.queue_path must be a persistent filesystem path")
	}
	path, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("resolve SQLite database.queue_path %q: %w", value, err)
	}
	return filepath.Clean(path), nil
}

func queueCorruptionError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "not a database") || strings.Contains(message, "malformed") || strings.Contains(message, "encrypted") {
		return fmt.Errorf("%w: %v", ErrQueueDatabaseCorrupt, err)
	}
	return err
}

func claimOrVerifyQueue(ctx context.Context, database *sql.DB) error {
	var got int
	if err := database.QueryRowContext(ctx, "PRAGMA application_id").Scan(&got); err != nil {
		return fmt.Errorf("read queue application_id: %w", queueCorruptionError(err))
	}
	if got == queueApplicationID {
		return nil
	}
	if got != 0 {
		return fmt.Errorf("%w: queue application_id = %#x, want Lumilio queue %#x", ErrQueueDatabaseCorrupt, got, queueApplicationID)
	}
	var userTables int
	if err := database.QueryRowContext(ctx, `
		SELECT count(*) FROM sqlite_schema
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
	`).Scan(&userTables); err != nil {
		return fmt.Errorf("inspect unclaimed queue database: %w", err)
	}
	if userTables != 0 {
		return fmt.Errorf("%w: unrecognized SQLite queue database has %d user tables and no Lumilio queue application_id", ErrQueueDatabaseCorrupt, userTables)
	}
	if _, err := database.ExecContext(ctx, fmt.Sprintf("PRAGMA application_id = %d", queueApplicationID)); err != nil {
		return fmt.Errorf("set queue application_id: %w", err)
	}
	return nil
}

// Migrate applies only River's schema to the queue database.
func (q *QueueDB) Migrate(ctx context.Context) error {
	if q == nil || q.SQL == nil {
		return errors.New("queue database is not open")
	}
	return MigrateRiver(ctx, q.SQL)
}

// TelemetrySnapshot exposes queue-specific database/sql and transaction stats
// without mixing them into the catalog telemetry stream.
type QueueTelemetrySnapshot struct {
	ObservedAt time.Time
	Writer     sql.DBStats
	Reader     sql.DBStats
	Catalog    catalogtx.Report
}

// PassiveCheckpoint and InspectWAL mirror the catalog maintenance surface but
// operate only on QueueDB's writer and WAL file. Queue checkpoints never use
// the catalog writer connection.
func (q *QueueDB) PassiveCheckpoint(ctx context.Context) (CheckpointResult, error) {
	if q == nil || q.SQL == nil {
		return CheckpointResult{}, errors.New("queue database is not open")
	}
	started := time.Now()
	var result CheckpointResult
	if err := q.SQL.QueryRowContext(ctx, "PRAGMA wal_checkpoint(PASSIVE)").Scan(
		&result.Busy, &result.LogPages, &result.Checkpointed,
	); err != nil {
		return CheckpointResult{}, fmt.Errorf("passive SQLite queue WAL checkpoint: %w", err)
	}
	result.Duration = time.Since(started)
	return result, nil
}

func (q *QueueDB) InspectWAL() (WALState, error) {
	if q == nil || strings.TrimSpace(q.Path) == "" {
		return WALState{}, errors.New("queue database is not open")
	}
	info, err := os.Stat(q.Path + "-wal")
	if errors.Is(err, os.ErrNotExist) {
		return WALState{}, nil
	}
	if err != nil {
		return WALState{}, fmt.Errorf("inspect SQLite queue WAL: %w", err)
	}
	return WALState{SizeBytes: info.Size(), ModifiedAt: info.ModTime()}, nil
}

func (q *QueueDB) TelemetrySnapshot() QueueTelemetrySnapshot {
	result := QueueTelemetrySnapshot{ObservedAt: time.Now().UTC()}
	if q == nil {
		return result
	}
	if q.SQL != nil {
		result.Writer = q.SQL.Stats()
	}
	if q.ReaderSQL != nil {
		result.Reader = q.ReaderSQL.Stats()
	}
	if q.transactionRecorder != nil {
		result.Catalog = q.transactionRecorder.Report()
	}
	return result
}

// Close releases the queue readers before checkpointing and closing the sole
// queue writer. River must already be stopped by the caller.
func (q *QueueDB) Close(ctx context.Context) error {
	if q == nil || q.SQL == nil {
		return nil
	}
	var closeErr error
	if q.ReaderSQL != nil {
		if err := q.ReaderSQL.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("close SQLite queue reader pool: %w", err))
		}
	}
	if _, err := q.SQL.ExecContext(ctx, "PRAGMA optimize"); err != nil {
		closeErr = errors.Join(closeErr, fmt.Errorf("optimize SQLite queue database: %w", err))
	}
	var busy, logPages, checkpointed int
	if err := q.SQL.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logPages, &checkpointed); err != nil {
		closeErr = errors.Join(closeErr, fmt.Errorf("checkpoint SQLite queue database: %w", err))
	} else if busy != 0 {
		closeErr = errors.Join(closeErr, fmt.Errorf("checkpoint SQLite queue database remained busy: log_pages=%d checkpointed=%d", logPages, checkpointed))
	}
	if err := q.SQL.Close(); err != nil {
		closeErr = errors.Join(closeErr, fmt.Errorf("close SQLite queue database: %w", err))
	}
	return closeErr
}

type queueLogObserver struct{}

func (queueLogObserver) ObserveTransaction(sample catalogtx.TransactionSample) {
	if sample.Role == catalogtx.RoleWriter && sample.Total-sample.Admission > sqliteWriteTransactionBudget {
		log.Printf("SQLite queue write transaction exceeded hold budget: operation=%s hold=%s", sample.OperationName, sample.Total-sample.Admission)
	}
}

func (queueLogObserver) ObserveStatement(catalogtx.StatementSample) {}
func (queueLogObserver) ObserveRows(catalogtx.RowsEvent)            {}
