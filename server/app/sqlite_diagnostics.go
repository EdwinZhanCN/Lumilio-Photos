package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"server/internal/db"
	"server/internal/db/catalogtx"
)

const sqliteRuntimeObservationFilename = "sqlite-runtime.json"

// sqliteRuntimeObservation is the bounded, latest-only operator diagnostic
// consumed by the host-side sqlitepressure controller. It is deliberately a
// file in the configured private log directory rather than a debug HTTP API.
type sqliteRuntimeObservation struct {
	ObservedAt         time.Time                    `json:"observed_at"`
	RuntimeElapsed     time.Duration                `json:"runtime_elapsed_ns"`
	Telemetry          db.TelemetrySnapshot         `json:"telemetry"`
	TelemetryInterval  catalogtx.Report             `json:"telemetry_interval"`
	IntervalStartedAt  time.Time                    `json:"interval_started_at"`
	IntervalError      string                       `json:"interval_error,omitempty"`
	WAL                db.WALState                  `json:"wal"`
	WALError           string                       `json:"wal_error,omitempty"`
	WriterWaitCount    int64                        `json:"writer_wait_count_delta"`
	WriterWaitDuration time.Duration                `json:"writer_wait_duration_delta_ns"`
	LastCheckpoint     *sqliteCheckpointObservation `json:"last_checkpoint,omitempty"`
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
