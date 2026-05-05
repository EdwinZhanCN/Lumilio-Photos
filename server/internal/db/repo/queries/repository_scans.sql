-- name: CreateRepositoryScanRun :one
INSERT INTO repository_scan_runs (
    scan_id,
    repository_id,
    mode,
    requested_by,
    status,
    started_at
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetRepositoryScanRun :one
SELECT * FROM repository_scan_runs
WHERE scan_id = $1;

-- name: GetLatestRepositoryScanRun :one
SELECT * FROM repository_scan_runs
WHERE repository_id = $1
ORDER BY started_at DESC
LIMIT 1;

-- name: ListRepositoryScanRuns :many
SELECT * FROM repository_scan_runs
WHERE repository_id = $1
ORDER BY started_at DESC
LIMIT $2 OFFSET $3;

-- name: CountRunningRepositoryScanRuns :one
SELECT COUNT(*) FROM repository_scan_runs
WHERE repository_id = $1
  AND status = 'running'
  AND scan_id <> $2;

-- name: CompleteRepositoryScanRun :one
UPDATE repository_scan_runs
SET
    status = 'completed',
    finished_at = $2,
    discovered_count = $3,
    updated_count = $4,
    deleted_count = $5,
    skipped_count = $6,
    error = NULL
WHERE scan_id = $1
RETURNING *;

-- name: FailRepositoryScanRun :one
UPDATE repository_scan_runs
SET
    status = 'failed',
    finished_at = $2,
    discovered_count = $3,
    updated_count = $4,
    deleted_count = $5,
    skipped_count = $6,
    error = $7
WHERE scan_id = $1
RETURNING *;

-- name: CancelRepositoryScanRun :one
UPDATE repository_scan_runs
SET
    status = 'cancelled',
    finished_at = $2,
    error = $3
WHERE scan_id = $1
RETURNING *;
