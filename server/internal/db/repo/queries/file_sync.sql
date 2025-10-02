-- File Records Queries

-- name: CreateFileRecord :one
INSERT INTO file_records (
    repository_id,
    file_path,
    file_size,
    mod_time,
    content_hash,
    last_scanned,
    scan_generation
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetFileRecord :one
SELECT * FROM file_records
WHERE repository_id = $1 AND file_path = $2;

-- name: ListFileRecords :many
SELECT * FROM file_records
WHERE repository_id = $1
ORDER BY file_path;

-- name: ListFileRecordsByGeneration :many
SELECT * FROM file_records
WHERE repository_id = $1 AND scan_generation = $2
ORDER BY file_path;

-- name: UpdateFileRecord :one
UPDATE file_records
SET
    file_size = $3,
    mod_time = $4,
    content_hash = $5,
    last_scanned = $6,
    scan_generation = $7,
    updated_at = NOW()
WHERE repository_id = $1 AND file_path = $2
RETURNING *;

-- name: UpsertFileRecord :one
INSERT INTO file_records (
    repository_id,
    file_path,
    file_size,
    mod_time,
    content_hash,
    last_scanned,
    scan_generation
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (repository_id, file_path)
DO UPDATE SET
    file_size = EXCLUDED.file_size,
    mod_time = EXCLUDED.mod_time,
    content_hash = EXCLUDED.content_hash,
    last_scanned = EXCLUDED.last_scanned,
    scan_generation = EXCLUDED.scan_generation,
    updated_at = NOW()
RETURNING *;

-- name: DeleteFileRecord :exec
DELETE FROM file_records
WHERE repository_id = $1 AND file_path = $2;

-- name: DeleteOrphanedFileRecords :execrows
DELETE FROM file_records
WHERE repository_id = $1 AND scan_generation < $2;

-- name: DeleteAllFileRecordsForRepo :exec
DELETE FROM file_records
WHERE repository_id = $1;

-- name: GetFileRecordCount :one
SELECT COUNT(*) FROM file_records
WHERE repository_id = $1;

-- name: GetFileRecordsByHash :many
SELECT * FROM file_records
WHERE content_hash = $1
ORDER BY repository_id, file_path;

-- Sync Operations Queries

-- name: CreateSyncOperation :one
INSERT INTO sync_operations (
    repository_id,
    operation_type,
    files_scanned,
    files_added,
    files_updated,
    files_removed,
    start_time,
    end_time,
    duration_ms,
    status,
    error_message
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
) RETURNING *;

-- name: GetSyncOperation :one
SELECT * FROM sync_operations
WHERE id = $1;

-- name: ListSyncOperations :many
SELECT * FROM sync_operations
WHERE repository_id = $1
ORDER BY start_time DESC
LIMIT $2;

-- name: ListSyncOperationsByType :many
SELECT * FROM sync_operations
WHERE repository_id = $1 AND operation_type = $2
ORDER BY start_time DESC
LIMIT $3;

-- name: GetLatestSyncOperation :one
SELECT * FROM sync_operations
WHERE repository_id = $1
ORDER BY start_time DESC
LIMIT 1;

-- name: GetLatestSyncOperationByType :one
SELECT * FROM sync_operations
WHERE repository_id = $1 AND operation_type = $2
ORDER BY start_time DESC
LIMIT 1;

-- name: UpdateSyncOperation :one
UPDATE sync_operations
SET
    files_scanned = $2,
    files_added = $3,
    files_updated = $4,
    files_removed = $5,
    end_time = $6,
    duration_ms = $7,
    status = $8,
    error_message = $9
WHERE id = $1
RETURNING *;

-- name: GetRunningSyncOperations :many
SELECT * FROM sync_operations
WHERE repository_id = $1 AND status = 'running'
ORDER BY start_time DESC;

-- name: GetFailedSyncOperations :many
SELECT * FROM sync_operations
WHERE repository_id = $1 AND status = 'failed'
ORDER BY start_time DESC
LIMIT $2;

-- name: CountSyncOperationsByStatus :one
SELECT COUNT(*) FROM sync_operations
WHERE repository_id = $1 AND status = $2;

-- name: GetSyncStatistics :one
SELECT
    COUNT(*) as total_operations,
    SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed_count,
    SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_count,
    SUM(files_scanned) as total_files_scanned,
    SUM(files_added) as total_files_added,
    SUM(files_updated) as total_files_updated,
    SUM(files_removed) as total_files_removed,
    AVG(duration_ms) as avg_duration_ms
FROM sync_operations
WHERE repository_id = $1 AND status = 'completed';

-- name: DeleteOldSyncOperations :exec
DELETE FROM sync_operations
WHERE repository_id = $1
    AND created_at < $2
    AND status != 'running';
