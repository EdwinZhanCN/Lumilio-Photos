package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"server/internal/db/dbtypes"
)

type catalogSQLExec interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// invalidateRepositoryObservationAfterRelocation fences stale directory observation
// and scan work after a repository's registered path changes. It must run in the
// same catalog transaction as UpdateRepositoryPath.
func invalidateRepositoryObservationAfterRelocation(
	ctx context.Context,
	db catalogSQLExec,
	repoID uuid.UUID,
	newRepositoryPath string,
	now dbtypes.Timestamp,
) error {
	repoIDStr := repoID.String()
	if _, err := db.ExecContext(ctx, `DELETE FROM repository_change_cursors WHERE repository_id = ?`, repoIDStr); err != nil {
		return fmt.Errorf("clear repository change cursors: %w", err)
	}

	var nextRevision int64
	err := db.QueryRowContext(ctx, `
		SELECT next_revision FROM repository_observation_state WHERE repository_id = ?
	`, repoIDStr).Scan(&nextRevision)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read observation revision fence: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
		UPDATE repository_nodes
		SET observation_revision = CASE
			WHEN observation_revision < ? THEN ?
			ELSE observation_revision
		END,
		updated_at = ?
		WHERE repository_id = ?
	`, nextRevision, nextRevision, now, repoIDStr); err != nil {
		return fmt.Errorf("bump repository node observation revisions: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
		UPDATE repository_scan_runs
		SET status = 'cancelled',
		    cancellation_requested = 1,
		    finished_at = ?,
		    updated_at = ?
		WHERE repository_id = ?
		  AND status IN ('queued', 'crawling', 'catching_up', 'finalizing')
	`, now, now, repoIDStr); err != nil {
		return fmt.Errorf("cancel active repository scan runs: %w", err)
	}

	pathInfo := InspectStoragePathReadOnly(newRepositoryPath)
	var volumeIdentity any
	if pathInfo.MountID != "" {
		volumeIdentity = pathInfo.MountID
	}
	fencedNextRevision := nextRevision + 1
	if _, err := db.ExecContext(ctx, `
		UPDATE repository_observation_state
		SET cursor_health = 'unavailable',
		    full_verification_required = 1,
		    adapter_identity = NULL,
		    volume_identity = ?,
		    next_revision = ?,
		    active_run_id = NULL,
		    controller_lease_id = NULL,
		    controller_lease_expires_at = NULL,
		    updated_at = ?
		WHERE repository_id = ?
	`, volumeIdentity, fencedNextRevision, now, repoIDStr); err != nil {
		return fmt.Errorf("fence repository observation state: %w", err)
	}
	return nil
}
