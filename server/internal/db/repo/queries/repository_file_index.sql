-- name: GetRepositoryFileIndexEntry :one
SELECT * FROM repository_file_index
WHERE repository_id = sqlc.arg('repository_id')
  AND storage_path = sqlc.arg('storage_path');

-- name: ListRepositoryFileIndex :many
SELECT * FROM repository_file_index
WHERE repository_id = sqlc.arg('repository_id')
ORDER BY storage_path ASC;

-- name: UpsertRepositoryFileObservation :one
INSERT INTO repository_file_index (
    repository_id,
    storage_path,
    asset_id,
    entry_kind,
    file_size,
    modified_at_ns,
    changed_at_ns,
    file_identity_kind,
    file_identity_value,
    observation_token,
    quick_fingerprint,
    quick_fingerprint_version,
    content_hash,
    state,
    first_seen_scan_id,
    last_seen_scan_id,
    missing_since_scan_id,
    missing_confirmations,
    ambiguity_group,
    reconciliation_reason,
    last_inspection_error,
    updated_at
) VALUES (
    sqlc.arg('repository_id'),
    sqlc.arg('storage_path'),
    sqlc.narg('asset_id'),
    sqlc.arg('entry_kind'),
    sqlc.arg('file_size'),
    sqlc.arg('modified_at_ns'),
    sqlc.narg('changed_at_ns'),
    sqlc.narg('file_identity_kind'),
    sqlc.narg('file_identity_value'),
    sqlc.arg('observation_token'),
    sqlc.narg('quick_fingerprint'),
    sqlc.narg('quick_fingerprint_version'),
    sqlc.narg('content_hash'),
    sqlc.arg('state'),
    sqlc.narg('first_seen_scan_id'),
    sqlc.narg('last_seen_scan_id'),
    sqlc.narg('missing_since_scan_id'),
    sqlc.arg('missing_confirmations'),
    sqlc.narg('ambiguity_group'),
    sqlc.narg('reconciliation_reason'),
    sqlc.narg('last_inspection_error'),
    sqlc.arg('updated_at')
)
ON CONFLICT (repository_id, storage_path) DO UPDATE SET
    asset_id = COALESCE(excluded.asset_id, repository_file_index.asset_id),
    entry_kind = excluded.entry_kind,
    file_size = excluded.file_size,
    modified_at_ns = excluded.modified_at_ns,
    changed_at_ns = excluded.changed_at_ns,
    file_identity_kind = excluded.file_identity_kind,
    file_identity_value = excluded.file_identity_value,
    observation_token = excluded.observation_token,
    quick_fingerprint = COALESCE(excluded.quick_fingerprint, repository_file_index.quick_fingerprint),
    quick_fingerprint_version = COALESCE(excluded.quick_fingerprint_version, repository_file_index.quick_fingerprint_version),
    content_hash = COALESCE(excluded.content_hash, repository_file_index.content_hash),
    state = excluded.state,
    last_seen_scan_id = COALESCE(excluded.last_seen_scan_id, repository_file_index.last_seen_scan_id),
    missing_since_scan_id = excluded.missing_since_scan_id,
    missing_confirmations = excluded.missing_confirmations,
    ambiguity_group = excluded.ambiguity_group,
    reconciliation_reason = excluded.reconciliation_reason,
    last_inspection_error = excluded.last_inspection_error,
    updated_at = excluded.updated_at
RETURNING *;

-- name: UpdateRepositoryFileIndexState :one
UPDATE repository_file_index
SET
    state = sqlc.arg('state'),
    missing_since_scan_id = sqlc.narg('missing_since_scan_id'),
    missing_confirmations = sqlc.arg('missing_confirmations'),
    ambiguity_group = sqlc.narg('ambiguity_group'),
    reconciliation_reason = sqlc.narg('reconciliation_reason'),
    last_inspection_error = sqlc.narg('last_inspection_error'),
    updated_at = sqlc.arg('updated_at')
WHERE repository_id = sqlc.arg('repository_id')
  AND storage_path = sqlc.arg('storage_path')
RETURNING *;

-- name: BindRepositoryFileIndexAsset :one
UPDATE repository_file_index
SET
    asset_id = sqlc.arg('asset_id'),
    state = 'present',
    missing_since_scan_id = NULL,
    missing_confirmations = 0,
    ambiguity_group = NULL,
    reconciliation_reason = NULL,
    last_inspection_error = NULL,
    updated_at = sqlc.arg('updated_at')
WHERE repository_id = sqlc.arg('repository_id')
  AND storage_path = sqlc.arg('storage_path')
RETURNING *;

-- name: DeleteRepositoryFileIndexEntry :exec
DELETE FROM repository_file_index
WHERE repository_id = sqlc.arg('repository_id')
  AND storage_path = sqlc.arg('storage_path');

-- name: ResetRepositoryFileIndex :exec
DELETE FROM repository_file_index
WHERE repository_id = sqlc.arg('repository_id');

-- name: ListRecoverableIngestClaims :many
SELECT
    asset_id,
    storage_path,
    CAST(COALESCE(json_extract(status, '$.ingest.staging_path'), '') AS TEXT) AS staging_path
FROM assets
WHERE repository_id = sqlc.arg('repository_id')
  AND storage_path IS NOT NULL
  AND json_extract(status, '$.ingest.recoverable') = 1
  AND json_extract(status, '$.ingest.phase') IN ('prepared', 'commit_failed', 'conflict')
ORDER BY storage_path ASC;
