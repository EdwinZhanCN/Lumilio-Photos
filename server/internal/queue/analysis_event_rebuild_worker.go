package queue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"server/internal/db/catalogtx"
	"server/internal/event"
	"server/internal/queue/jobs"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type EventRebuildArgs = jobs.EventRebuildArgs
type ScheduleEventRebuildsArgs = jobs.ScheduleEventRebuildsArgs

type EventRebuildWorker struct {
	river.WorkerDefaults[EventRebuildArgs]
	Writer  *catalogtx.Writer
	Service *event.Service
}

func (w *EventRebuildWorker) Timeout(*river.Job[EventRebuildArgs]) time.Duration {
	return 30 * time.Minute
}

func (w *EventRebuildWorker) Work(ctx context.Context, job *river.Job[EventRebuildArgs]) error {
	if w.Writer == nil || w.Service == nil {
		return errors.New("Event rebuild worker not configured")
	}
	now := time.Now().UTC().UnixMicro()
	token := uuid.NewString()
	var runID string
	var sourceRevision, publishedRevision int64
	tx, err := w.Writer.BeginTx(ctx, catalogtx.OperationEventRebuildClaim, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
INSERT INTO event_owner_state(owner_id,active_algorithm_version,initialized_at,revision,source_revision,published_revision,updated_at)
VALUES(?,?,?,0,0,0,?) ON CONFLICT(owner_id) DO NOTHING`,
		job.Args.OwnerID, event.AlgorithmVersion, now, now); err != nil {
		return err
	}
	var paused int
	if err := tx.QueryRowContext(ctx, `
SELECT automatic_rebuild_paused,source_revision,published_revision
FROM event_owner_state WHERE owner_id=?`, job.Args.OwnerID).
		Scan(&paused, &sourceRevision, &publishedRevision); err != nil {
		return err
	}
	if paused != 0 {
		return event.ErrPaused
	}
	if sourceRevision <= publishedRevision && !job.Args.Force {
		// An explicit rebuild may be requested even when facts are already
		// converged.  Close its run truthfully instead of leaving the UI in a
		// permanent queued state.
		_, _ = tx.ExecContext(ctx, `
UPDATE event_rebuild_runs SET state='succeeded',published_revision=?,finished_at=?
WHERE owner_id=? AND state='queued'`, publishedRevision, now, job.Args.OwnerID)
		return tx.Commit()
	}
	if err := tx.QueryRowContext(ctx, `
SELECT run_id FROM event_rebuild_runs
WHERE owner_id=? AND state='queued'
ORDER BY requested_at,run_id LIMIT 1`, job.Args.OwnerID).Scan(&runID); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		runID = uuid.NewString()
		if _, err := tx.ExecContext(ctx, `
INSERT INTO event_rebuild_runs(run_id,owner_id,state,requested_revision,requested_at)
VALUES(?,?, 'queued',?,?)`, runID, job.Args.OwnerID, sourceRevision, now); err != nil {
			// The statement above intentionally has no dependency on a client
			// request; automatic work always has an observable run.
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE event_owner_state SET rebuild_lease_token=?,rebuild_lease_expires_at=?,updated_at=?
WHERE owner_id=? AND (rebuild_lease_token IS NULL OR rebuild_lease_expires_at<?)`,
		token, now+int64((30*time.Minute)/time.Microsecond), now, job.Args.OwnerID, now); err != nil {
		return err
	}
	var leaseToken sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT rebuild_lease_token FROM event_owner_state WHERE owner_id=?`, job.Args.OwnerID).Scan(&leaseToken); err != nil {
		return err
	}
	if !leaseToken.Valid || leaseToken.String != token {
		return event.ErrStaleRevision
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE event_rebuild_runs SET state='running',started_at=? WHERE run_id=?`, now, runID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	computeCtx, cancel := context.WithCancel(ctx)
	leaseErrors := make(chan error, 1)
	go w.renewOwnerLease(computeCtx, job.Args.OwnerID, token, leaseErrors)
	preview, rebuildErr := w.Service.RebuildClaimed(computeCtx, job.Args.OwnerID, sourceRevision, token)
	cancel()
	select {
	case leaseErr := <-leaseErrors:
		if rebuildErr == nil {
			rebuildErr = leaseErr
		}
	default:
	}
	finish := time.Now().UTC().UnixMicro()
	finalize, finalizeErr := w.Writer.BeginTx(ctx, catalogtx.OperationEventRebuildFinalize, nil)
	if finalizeErr != nil {
		return finalizeErr
	}
	defer finalize.Rollback()
	if rebuildErr != nil {
		state := "failed"
		code := errorCode(rebuildErr)
		if errors.Is(rebuildErr, event.ErrStaleRevision) {
			state = "stale"
			code = "source_revision_changed"
		}
		_, _ = finalize.ExecContext(ctx, `UPDATE event_rebuild_runs SET state=?,finished_at=?,error_code=? WHERE run_id=?`, state, finish, code, runID)
		_, _ = finalize.ExecContext(ctx, `UPDATE event_owner_state SET rebuild_lease_token=NULL,rebuild_lease_expires_at=NULL,updated_at=? WHERE owner_id=? AND rebuild_lease_token=?`, finish, job.Args.OwnerID, token)
		if err := finalize.Commit(); err != nil {
			return err
		}
		if errors.Is(rebuildErr, event.ErrStaleRevision) {
			// River requires unique jobs to include the running state, so an
			// invalidation arriving during this run coalesces into this durable
			// row instead of inserting a follower. Snooze the same job and read
			// the new source revision on its next turn.
			return river.JobSnooze(0)
		}
		return fmt.Errorf("rebuild owner Events: %w", rebuildErr)
	}
	var currentSource int64
	if err := finalize.QueryRowContext(ctx, `SELECT source_revision FROM event_owner_state WHERE owner_id=?`, job.Args.OwnerID).Scan(&currentSource); err != nil {
		return err
	}
	state := "succeeded"
	if currentSource != sourceRevision {
		state = "stale"
	}
	if _, err := finalize.ExecContext(ctx, `
UPDATE event_rebuild_runs SET state=?,published_revision=?,finished_at=?,event_count=?,member_count=? WHERE run_id=?`,
		state, sourceRevision, finish, preview.Events, preview.Members, runID); err != nil {
		return err
	}
	if currentSource == sourceRevision {
		if _, err := finalize.ExecContext(ctx, `DELETE FROM event_dirty_ranges WHERE owner_id=?`, job.Args.OwnerID); err != nil {
			return err
		}
	}
	if _, err := finalize.ExecContext(ctx, `
UPDATE event_owner_state SET rebuild_lease_token=NULL,rebuild_lease_expires_at=NULL,updated_at=?
WHERE owner_id=? AND rebuild_lease_token=?`, finish, job.Args.OwnerID, token); err != nil {
		return err
	}
	if err := finalize.Commit(); err != nil {
		return err
	}
	if currentSource != sourceRevision {
		return river.JobSnooze(0)
	}
	return nil
}

func errorCode(err error) string {
	switch {
	case errors.Is(err, event.ErrPaused):
		return "rebuild_paused"
	case errors.Is(err, event.ErrWouldBeEmpty):
		return "invalid_empty_event"
	case errors.Is(err, event.ErrConstraintConflict):
		return "correction_conflict"
	default:
		return "rebuild_failed"
	}
}

func (w *EventRebuildWorker) renewOwnerLease(ctx context.Context, ownerID int32, token string, result chan<- error) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now().UTC().UnixMicro()
			tx, err := w.Writer.BeginTx(ctx, catalogtx.OperationEventRebuildLeaseRenew, nil)
			if err != nil {
				result <- err
				return
			}
			if err == nil {
				var update sql.Result
				update, err = tx.ExecContext(ctx, `
UPDATE event_owner_state SET rebuild_lease_expires_at=?,updated_at=?
WHERE owner_id=? AND rebuild_lease_token=?`,
					now+int64((30*time.Minute)/time.Microsecond), now, ownerID, token)
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
	DB     *sql.DB
	Writer *catalogtx.Writer
	ReadDB *sql.DB
}

func (w *ScheduleEventRebuildsWorker) Work(ctx context.Context, _ *river.Job[ScheduleEventRebuildsArgs]) error {
	if w.DB == nil {
		return errors.New("Event rebuild scheduler not configured")
	}
	writer := w.Writer
	if writer == nil {
		writer = catalogtx.NewWriter(w.DB, nil)
	}
	now := time.Now().UTC().UnixMicro()
	if _, err := writer.ExecContext(ctx, catalogtx.OperationEventSchedulerLeaseCleanup, `
UPDATE event_owner_state SET rebuild_lease_token=NULL,rebuild_lease_expires_at=NULL,updated_at=?
WHERE rebuild_lease_expires_at IS NOT NULL AND rebuild_lease_expires_at<?`, now, now); err != nil {
		return err
	}
	readDB := w.ReadDB
	if readDB == nil {
		readDB = w.DB
	}
	ownerIDs, err := pendingEventOwnerIDsConverged(ctx, readDB)
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

func pendingEventOwnerIDsConverged(ctx context.Context, db *sql.DB) ([]int32, error) {
	rows, err := db.QueryContext(ctx, `
SELECT owner_id FROM event_owner_state
WHERE automatic_rebuild_paused=0 AND source_revision>published_revision
ORDER BY owner_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ownerIDs []int32
	for rows.Next() {
		var ownerID int32
		if err := rows.Scan(&ownerID); err != nil {
			return nil, err
		}
		ownerIDs = append(ownerIDs, ownerID)
	}
	return ownerIDs, rows.Err()
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
