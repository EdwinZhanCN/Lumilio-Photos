package domainoutbox

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"server/internal/db/catalogtx"
	"server/internal/pipeline"
)

// BackupScheduler publishes catalog-owned maintenance requests. River is only
// their disposable delivery controller and is never the periodic clock or the
// source of backup intent.
type BackupScheduler struct {
	writer   *catalogtx.Writer
	interval time.Duration
}

func NewBackupScheduler(writer *catalogtx.Writer, interval time.Duration) (*BackupScheduler, error) {
	if writer == nil || interval <= 0 {
		return nil, errors.New("backup scheduler requires a catalog writer and positive interval")
	}
	return &BackupScheduler{writer: writer, interval: interval}, nil
}

func (s *BackupScheduler) Request(ctx context.Context, force bool) (uuid.UUID, error) {
	if ctx == nil {
		return uuid.Nil, errors.New("backup scheduler context is nil")
	}
	receiptID := uuid.New()
	err := s.writer.Transact(ctx, catalogtx.OperationBackupRequest, nil, func(tx *sql.Tx) error {
		return pipeline.RequestBackupWithAdmissionTx(ctx, tx, receiptID, force, pipeline.AdmissionMaintenance)
	})
	return receiptID, err
}

func (s *BackupScheduler) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("backup scheduler context is nil")
	}
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		if _, err := s.Request(ctx, false); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			// A transient catalog lock or connection failure must not disable
			// future backup requests; the next bounded tick retries from the
			// catalog-owned receipt boundary.
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
