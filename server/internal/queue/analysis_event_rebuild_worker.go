package queue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"server/internal/event"
	"server/internal/queue/jobs"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type EventRebuildArgs = jobs.EventRebuildArgs
type ScheduleEventRebuildsArgs = jobs.ScheduleEventRebuildsArgs

type EventRebuildWorker struct {
	river.WorkerDefaults[EventRebuildArgs]
	DB      *sql.DB
	Service *event.Service
}

func (w *EventRebuildWorker) Timeout(*river.Job[EventRebuildArgs]) time.Duration {
	return 30 * time.Minute
}

func (w *EventRebuildWorker) Work(ctx context.Context, job *river.Job[EventRebuildArgs]) error {
	if w.DB == nil || w.Service == nil {
		return errors.New("Event rebuild worker not configured")
	}
	token := uuid.NewString()
	now := time.Now().UTC().UnixMicro()
	tx, err := w.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
INSERT INTO event_owner_state(owner_id,active_algorithm_version,initialized_at,revision,updated_at)
VALUES(?,?,?,0,?) ON CONFLICT(owner_id) DO NOTHING`,
		job.Args.OwnerID, event.AlgorithmVersion, now, now); err != nil {
		return err
	}
	var paused int
	var revision int64
	if err := tx.QueryRowContext(ctx, `
SELECT automatic_rebuild_paused,revision FROM event_owner_state WHERE owner_id=?`,
		job.Args.OwnerID).Scan(&paused, &revision); err != nil {
		return err
	}
	if paused != 0 {
		return event.ErrPaused
	}
	result, err := tx.ExecContext(ctx, `
UPDATE event_dirty_ranges SET claimed_at=?,claim_token=?
WHERE owner_id=? AND claim_token IS NULL`, now, token, job.Args.OwnerID)
	if err != nil {
		return err
	}
	claimed, _ := result.RowsAffected()
	if claimed == 0 {
		return tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE event_owner_state SET revision=revision+1,updated_at=? WHERE owner_id=?`,
		now, job.Args.OwnerID); err != nil {
		return err
	}
	revision++
	if err := tx.Commit(); err != nil {
		return err
	}

	computeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	leaseErrors := make(chan error, 1)
	go w.renewLease(computeCtx, job.Args.OwnerID, token, revision, leaseErrors)
	_, err = w.Service.RebuildClaimed(computeCtx, job.Args.OwnerID, revision)
	cancel()
	select {
	case leaseErr := <-leaseErrors:
		if leaseErr != nil {
			return fmt.Errorf("renew Event claim: %w", leaseErr)
		}
	default:
	}
	if err != nil {
		return fmt.Errorf("rebuild owner Events: %w", err)
	}
	// RebuildOwner advances revision exactly once. Only this worker's claim can
	// be removed; new factual ranges remain for the follower job.
	result, err = w.DB.ExecContext(ctx, `
DELETE FROM event_dirty_ranges WHERE owner_id=? AND claim_token=?`,
		job.Args.OwnerID, token)
	if err != nil {
		return err
	}
	deleted, _ := result.RowsAffected()
	if deleted != claimed {
		return event.ErrStaleRevision
	}
	return nil
}

func (w *EventRebuildWorker) renewLease(ctx context.Context, ownerID int32, token string, revision int64, result chan<- error) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now().UTC().UnixMicro()
			tx, err := w.DB.BeginTx(ctx, nil)
			if err != nil {
				result <- err
				return
			}
			var current int64
			err = tx.QueryRowContext(ctx, `SELECT revision FROM event_owner_state WHERE owner_id=?`, ownerID).Scan(&current)
			if err == nil && current != revision {
				err = event.ErrStaleRevision
			}
			if err == nil {
				var update sql.Result
				update, err = tx.ExecContext(ctx, `
UPDATE event_dirty_ranges SET claimed_at=? WHERE owner_id=? AND claim_token=?`,
					now, ownerID, token)
				if err == nil {
					var affected int64
					affected, _ = update.RowsAffected()
					if affected == 0 {
						err = event.ErrStaleRevision
					}
				}
			}
			if err == nil {
				err = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
			if err != nil {
				result <- err
				return
			}
		}
	}
}

type ScheduleEventRebuildsWorker struct {
	river.WorkerDefaults[ScheduleEventRebuildsArgs]
	DB *sql.DB
}

func (w *ScheduleEventRebuildsWorker) Work(ctx context.Context, _ *river.Job[ScheduleEventRebuildsArgs]) error {
	if w.DB == nil {
		return errors.New("Event rebuild scheduler not configured")
	}
	staleBefore := time.Now().UTC().Add(-15 * time.Minute).UnixMicro()
	if _, err := w.DB.ExecContext(ctx, `
UPDATE event_owner_state SET revision=revision+1,updated_at=?
WHERE owner_id IN (
 SELECT DISTINCT owner_id FROM event_dirty_ranges
 WHERE claimed_at IS NOT NULL AND claimed_at<?
)`, time.Now().UTC().UnixMicro(), staleBefore); err != nil {
		return err
	}
	if _, err := w.DB.ExecContext(ctx, `
UPDATE event_dirty_ranges SET claimed_at=NULL,claim_token=NULL
WHERE claimed_at IS NOT NULL AND claimed_at<?`, staleBefore); err != nil {
		return err
	}
	ownerIDs, err := pendingEventOwnerIDs(ctx, w.DB)
	if err != nil {
		return err
	}
	client, err := river.ClientFromContextSafely[*sql.Tx](ctx)
	if err != nil {
		return err
	}
	for _, ownerID := range ownerIDs {
		args := jobs.EventRebuildArgs{OwnerID: ownerID}
		opts := args.InsertOpts()
		if _, err := client.Insert(ctx, args, &opts); err != nil {
			return err
		}
	}
	return nil
}

func pendingEventOwnerIDs(ctx context.Context, db *sql.DB) ([]int32, error) {
	rows, err := db.QueryContext(ctx, `
SELECT DISTINCT owner_id FROM event_dirty_ranges WHERE claim_token IS NULL ORDER BY owner_id`)
	if err != nil {
		return nil, err
	}
	var ownerIDs []int32
	for rows.Next() {
		var ownerID int32
		if err := rows.Scan(&ownerID); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ownerIDs = append(ownerIDs, ownerID)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return ownerIDs, nil
}
