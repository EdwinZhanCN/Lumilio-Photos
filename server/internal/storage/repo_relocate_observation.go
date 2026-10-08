package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"server/internal/db/dbtypes"
)

type catalogSQLExec interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// invalidateRepositoryObservationAfterRelocation re-anchors the scan index
// after a repository's registered path changes. It must run in the same
// catalog transaction as UpdateRepositoryPath. Entries are relative, so they
// stay; a relocated tree usually sits on another volume, where every file has
// a new change time and file ID, so those are forgotten and the next scan
// re-records them from an equal size and mtime instead of rehashing. Running
// scans stop, and one full scan is queued.
func invalidateRepositoryObservationAfterRelocation(
	ctx context.Context,
	db catalogSQLExec,
	repoID uuid.UUID,
	_ string,
	now dbtypes.Timestamp,
) error {
	repoIDStr := repoID.String()
	if _, err := db.ExecContext(ctx, `
		UPDATE repository_entries
		SET ctime_ns = NULL, file_id = NULL
		WHERE repository_id = ?
	`, repoIDStr); err != nil {
		return fmt.Errorf("forget relocated file identities: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE repository_scans
		SET status = 'cancelled', cancellation_requested = 1, finished_at = ?, updated_at = ?
		WHERE repository_id = ? AND status IN ('walking', 'sweeping')
	`, now, now, repoIDStr); err != nil {
		return fmt.Errorf("cancel running repository scans: %w", err)
	}
	result, err := db.ExecContext(ctx, `
		UPDATE repository_scans
		SET scope_path = '', not_before = NULL, updated_at = ?
		WHERE repository_id = ? AND status = 'queued'
	`, now, repoIDStr)
	if err != nil {
		return fmt.Errorf("widen queued repository scan: %w", err)
	}
	if rows, err := result.RowsAffected(); err != nil || rows > 0 {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO repository_scans (scan_id, repository_id, trigger, scope_path, status, requested_by, created_at, updated_at)
		VALUES (?, ?, 'manual', '', 'queued', 'storage_lifecycle', ?, ?)
	`, uuid.NewString(), repoIDStr, now, now); err != nil {
		return fmt.Errorf("queue relocated repository scan: %w", err)
	}
	return nil
}
