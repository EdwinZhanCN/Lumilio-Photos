package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"server/internal/commit"
	"server/internal/db"
	"server/internal/db/catalogtx"
	"server/internal/execution"
)

const sqliteRuntimeObservationFilename = "sqlite-runtime.json"

// sqliteRuntimeObservation is the bounded, latest-only operator diagnostic.
// It is deliberately a file in the configured private log directory rather
// than a debug HTTP API.
type sqliteRuntimeObservation struct {
	ObservedAt              time.Time                    `json:"observed_at"`
	RuntimeElapsed          time.Duration                `json:"runtime_elapsed_ns"`
	Telemetry               db.TelemetrySnapshot         `json:"telemetry"`
	QueueTelemetry          db.QueueTelemetrySnapshot    `json:"queue_telemetry"`
	TelemetryInterval       catalogtx.Report             `json:"telemetry_interval"`
	IntervalStartedAt       time.Time                    `json:"interval_started_at"`
	IntervalError           string                       `json:"interval_error,omitempty"`
	WAL                     db.WALState                  `json:"wal"`
	QueueWAL                db.WALState                  `json:"queue_wal"`
	WALError                string                       `json:"wal_error,omitempty"`
	QueueWALError           string                       `json:"queue_wal_error,omitempty"`
	WriterWaitCount         int64                        `json:"writer_wait_count_delta"`
	WriterWaitDuration      time.Duration                `json:"writer_wait_duration_delta_ns"`
	QueueWriterWaitCount    int64                        `json:"queue_writer_wait_count_delta"`
	QueueWriterWaitDuration time.Duration                `json:"queue_writer_wait_duration_delta_ns"`
	LastCheckpoint          *sqliteCheckpointObservation `json:"last_checkpoint,omitempty"`
	LastQueueCheckpoint     *sqliteCheckpointObservation `json:"last_queue_checkpoint,omitempty"`
	CommitCoordinator       commit.Snapshot              `json:"commit_coordinator"`
	ExecutionGovernor       execution.Snapshot           `json:"execution_governor"`
	CatalogPipeline         catalogPipelineObservation   `json:"catalog_pipeline"`
	MacroQueue              macroQueueObservation        `json:"macro_queue"`
	CatalogPipelineError    string                       `json:"catalog_pipeline_error,omitempty"`
	MacroQueueError         string                       `json:"macro_queue_error,omitempty"`
}

// catalogPipelineObservation is a fixed-size view derived from catalog truth.
// Backlog and lag include terminal rows so operators can distinguish work that
// is waiting from work that requires intervention; runnable_backlog excludes
// those terminal rows. Throughput counts durable applied/completed catalog
// transitions in the current telemetry interval.
type catalogPipelineObservation struct {
	Backlog                     int64         `json:"backlog"`
	RunnableBacklog             int64         `json:"runnable_backlog"`
	TerminalBacklog             int64         `json:"terminal_backlog"`
	DesiredAppliedLag           int64         `json:"desired_applied_lag"`
	PendingOutbox               int64         `json:"pending_outbox"`
	OldestOutboxAge             time.Duration `json:"oldest_outbox_age_ns"`
	AppliedTransitions          int64         `json:"applied_transitions"`
	AppliedTransitionsPerSecond float64       `json:"applied_transitions_per_second"`
}

type macroQueueObservation struct {
	Available          int64         `json:"available"`
	Scheduled          int64         `json:"scheduled"`
	Running            int64         `json:"running"`
	Retryable          int64         `json:"retryable"`
	Remaining          int64         `json:"remaining"`
	Completed          int64         `json:"completed"`
	CompletedInterval  int64         `json:"completed_interval"`
	CompletedPerSecond float64       `json:"completed_per_second"`
	AverageLatency     time.Duration `json:"average_latency_ns"`
	AverageRuntime     time.Duration `json:"average_runtime_ns"`
	OldestRemainingAge time.Duration `json:"oldest_remaining_age_ns"`
}

func readCatalogPipelineObservation(ctx context.Context, reader *sql.DB, startedAt, endedAt time.Time) (catalogPipelineObservation, error) {
	var snapshot catalogPipelineObservation
	if reader == nil {
		return snapshot, errors.New("catalog diagnostics reader is nil")
	}
	startedMicros := startedAt.UTC().UnixMicro()
	endedMicros := endedAt.UTC().UnixMicro()
	var oldestOutbox sql.NullInt64
	err := reader.QueryRowContext(ctx, `
WITH pipeline_state(pending, runnable, terminal, lag, applied_in_interval) AS (
  SELECT desired_version > applied_version,
         desired_version > applied_version AND terminal_error IS NULL,
         desired_version > applied_version AND terminal_error IS NOT NULL,
         max(desired_version - applied_version, 0),
         applied_version >= desired_version AND updated_at >= ?1 AND updated_at < ?2
  FROM asset_pipeline_state
  UNION ALL
  SELECT desired_epoch > applied_epoch OR full_verification_required = 1,
         (desired_epoch > applied_epoch OR full_verification_required = 1) AND terminal_error IS NULL,
         (desired_epoch > applied_epoch OR full_verification_required = 1) AND terminal_error IS NOT NULL,
         max(desired_epoch - applied_epoch, full_verification_required),
         desired_epoch > 0 AND applied_epoch >= desired_epoch AND full_verification_required = 0 AND updated_at >= ?1 AND updated_at < ?2
  FROM repository_observation_state
  UNION ALL
  SELECT source_revision > applied_revision,
         source_revision > applied_revision AND terminal_error IS NULL,
         source_revision > applied_revision AND terminal_error IS NOT NULL,
         max(source_revision - applied_revision, 0),
         applied_revision >= source_revision AND updated_at >= ?1 AND updated_at < ?2
  FROM event_projection_pipeline_state
  UNION ALL
  SELECT source_revision > published_revision,
         source_revision > published_revision AND terminal_error IS NULL,
         source_revision > published_revision AND terminal_error IS NOT NULL,
         max(source_revision - published_revision, 0),
         published_revision >= source_revision AND updated_at >= ?1 AND updated_at < ?2
  FROM location_projection_state
  UNION ALL
  SELECT projection_version > applied_revision,
         projection_version > applied_revision AND terminal_error IS NULL,
         projection_version > applied_revision AND terminal_error IS NOT NULL,
         max(projection_version - applied_revision, 0),
         applied_revision >= projection_version AND updated_at >= ?1 AND updated_at < ?2
  FROM location_resolution_pipeline_state
  UNION ALL
  SELECT projection_version > applied_revision,
         projection_version > applied_revision AND terminal_error IS NULL,
         projection_version > applied_revision AND terminal_error IS NOT NULL,
         max(projection_version - applied_revision, 0),
         applied_revision >= projection_version AND updated_at >= ?1 AND updated_at < ?2
  FROM ocr_projection_pipeline_state
  UNION ALL
  SELECT requested_revision > applied_revision,
         requested_revision > applied_revision,
         0,
         max(requested_revision - applied_revision, 0),
         applied_revision >= requested_revision AND updated_at >= ?1 AND updated_at < ?2
  FROM asset_reindex_requests
  UNION ALL
  SELECT state != 'completed',
         state = 'pending',
         state = 'failed',
         max(desired_version - applied_version, 0),
         state = 'completed' AND updated_at >= ?1 AND updated_at < ?2
  FROM catalog_operation_receipts
)
SELECT
  coalesce(sum(pending), 0),
  coalesce(sum(runnable), 0),
  coalesce(sum(terminal), 0),
  coalesce(sum(lag), 0),
  coalesce(sum(applied_in_interval), 0),
  (SELECT count(*) FROM domain_outbox WHERE delivered_at IS NULL),
  (SELECT min(created_at) FROM domain_outbox WHERE delivered_at IS NULL)
FROM pipeline_state`, startedMicros, endedMicros).Scan(
		&snapshot.Backlog,
		&snapshot.RunnableBacklog,
		&snapshot.TerminalBacklog,
		&snapshot.DesiredAppliedLag,
		&snapshot.AppliedTransitions,
		&snapshot.PendingOutbox,
		&oldestOutbox,
	)
	if err != nil {
		return catalogPipelineObservation{}, fmt.Errorf("read catalog pipeline diagnostics: %w", err)
	}
	interval := endedAt.Sub(startedAt)
	if interval > 0 {
		snapshot.AppliedTransitionsPerSecond = float64(snapshot.AppliedTransitions) / interval.Seconds()
	}
	if oldestOutbox.Valid {
		snapshot.OldestOutboxAge = endedAt.Sub(time.UnixMicro(oldestOutbox.Int64))
		if snapshot.OldestOutboxAge < 0 {
			snapshot.OldestOutboxAge = 0
		}
	}
	return snapshot, nil
}

func readMacroQueueObservation(ctx context.Context, reader *sql.DB, startedAt, endedAt time.Time) (macroQueueObservation, error) {
	var snapshot macroQueueObservation
	if reader == nil {
		return snapshot, errors.New("macro queue diagnostics reader is nil")
	}
	var averageLatency, averageRuntime sql.NullFloat64
	var oldestRemaining sql.NullInt64
	err := reader.QueryRowContext(ctx, `
SELECT
  count(*) FILTER (WHERE state = 'available'),
  count(*) FILTER (WHERE state = 'scheduled'),
  count(*) FILTER (WHERE state = 'running'),
  count(*) FILTER (WHERE state = 'retryable'),
  count(*) FILTER (WHERE state IN ('available','scheduled','running','retryable')),
  count(*) FILTER (WHERE state = 'completed'),
  count(*) FILTER (WHERE state = 'completed' AND finalized_at >= ?1 AND finalized_at < ?2),
  avg((julianday(finalized_at) - julianday(created_at)) * 86400000000000.0)
    FILTER (WHERE finalized_at IS NOT NULL),
  avg((julianday(finalized_at) - julianday(attempted_at)) * 86400000000000.0)
    FILTER (WHERE finalized_at IS NOT NULL AND attempted_at IS NOT NULL),
  CAST(unixepoch(min(created_at) FILTER (WHERE state IN ('available','scheduled','running','retryable')), 'subsec') * 1000000 AS INTEGER)
FROM river_job
WHERE queue = 'catalog_macro'`, startedAt.UTC(), endedAt.UTC()).Scan(
		&snapshot.Available,
		&snapshot.Scheduled,
		&snapshot.Running,
		&snapshot.Retryable,
		&snapshot.Remaining,
		&snapshot.Completed,
		&snapshot.CompletedInterval,
		&averageLatency,
		&averageRuntime,
		&oldestRemaining,
	)
	if err != nil {
		return macroQueueObservation{}, fmt.Errorf("read macro queue diagnostics: %w", err)
	}
	interval := endedAt.Sub(startedAt)
	if interval > 0 {
		snapshot.CompletedPerSecond = float64(snapshot.CompletedInterval) / interval.Seconds()
	}
	if averageLatency.Valid && averageLatency.Float64 > 0 {
		snapshot.AverageLatency = time.Duration(averageLatency.Float64)
	}
	if averageRuntime.Valid && averageRuntime.Float64 > 0 {
		snapshot.AverageRuntime = time.Duration(averageRuntime.Float64)
	}
	if oldestRemaining.Valid {
		snapshot.OldestRemainingAge = endedAt.Sub(time.UnixMicro(oldestRemaining.Int64))
		if snapshot.OldestRemainingAge < 0 {
			snapshot.OldestRemainingAge = 0
		}
	}
	return snapshot, nil
}

type sqliteCheckpointObservation struct {
	ObservedAt time.Time           `json:"observed_at"`
	WALBefore  db.WALState         `json:"wal_before"`
	Result     db.CheckpointResult `json:"result"`
	Duration   time.Duration       `json:"duration_ns"`
	Error      string              `json:"error,omitempty"`
}

func boundedDiagnosticError(err error) string {
	if err == nil {
		return ""
	}
	const limit = 512
	message := strings.TrimSpace(err.Error())
	if len(message) <= limit {
		return message
	}
	return message[:limit]
}

func writeSQLiteRuntimeObservation(logDir string, observation sqliteRuntimeObservation) (returnErr error) {
	if strings.TrimSpace(logDir) == "" {
		return errors.New("SQLite diagnostics log directory is empty")
	}
	payload, err := json.Marshal(observation)
	if err != nil {
		return fmt.Errorf("encode SQLite runtime diagnostics: %w", err)
	}
	payload = append(payload, '\n')

	path := filepath.Join(logDir, sqliteRuntimeObservationFilename)
	temporary, err := os.CreateTemp(logDir, ".sqlite-runtime-*.tmp")
	if err != nil {
		return fmt.Errorf("create SQLite runtime diagnostics: %w", err)
	}
	temporaryPath := temporary.Name()
	closed := false
	defer func() {
		if !closed {
			if closeErr := temporary.Close(); closeErr != nil && returnErr == nil {
				returnErr = fmt.Errorf("close SQLite runtime diagnostics: %w", closeErr)
			}
		}
		if removeErr := os.Remove(temporaryPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) && returnErr == nil {
			returnErr = fmt.Errorf("remove temporary SQLite runtime diagnostics: %w", removeErr)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("secure SQLite runtime diagnostics: %w", err)
	}
	if _, err := temporary.Write(payload); err != nil {
		return fmt.Errorf("write SQLite runtime diagnostics: %w", err)
	}
	closed = true
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close SQLite runtime diagnostics before publish: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish SQLite runtime diagnostics: %w", err)
	}
	return nil
}
