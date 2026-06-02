-- name: CreateCloudCredential :one
INSERT INTO cloud_credentials (
    credential_id,
    provider,
    display_name,
    identity_hash,
    masked_identity,
    status,
    public_config,
    secret_ciphertext,
    artifact_dir,
    created_by_user_id,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now(), now()
) RETURNING *;

-- name: ListCloudCredentials :many
SELECT * FROM cloud_credentials
ORDER BY created_at DESC;

-- name: GetCloudCredential :one
SELECT * FROM cloud_credentials
WHERE credential_id = $1;

-- name: GetCloudCredentialByIdentity :one
SELECT * FROM cloud_credentials
WHERE provider = $1 AND identity_hash = $2;

-- name: UpdateCloudCredentialStatus :one
UPDATE cloud_credentials
SET status = $2, updated_at = now()
WHERE credential_id = $1
RETURNING *;

-- name: UpdateCloudCredentialAuthState :one
UPDATE cloud_credentials
SET status = $2,
    public_config = $3,
    secret_ciphertext = $4,
    artifact_dir = $5,
    updated_at = now()
WHERE credential_id = $1
RETURNING *;

-- name: DeleteCloudCredential :exec
DELETE FROM cloud_credentials
WHERE credential_id = $1;

-- name: CountRepositoryCloudBindingsByCredential :one
SELECT COUNT(*) FROM repository_cloud_bindings
WHERE credential_id = $1 AND enabled = true;

-- name: UpsertRepositoryCloudBinding :one
INSERT INTO repository_cloud_bindings (
    repository_id,
    credential_id,
    provider,
    enabled,
    last_import_run_id,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, true, NULL, now(), now()
)
ON CONFLICT (repository_id, provider)
DO UPDATE SET
    credential_id = $2,
    enabled = true,
    updated_at = now()
RETURNING *;

-- name: GetRepositoryCloudBinding :one
SELECT * FROM repository_cloud_bindings
WHERE repository_id = $1 AND provider = $2;

-- name: GetActiveRepositoryCloudBinding :one
SELECT * FROM repository_cloud_bindings
WHERE repository_id = $1 AND enabled = true
ORDER BY created_at DESC
LIMIT 1;

-- name: ListRepositoryCloudBindings :many
SELECT * FROM repository_cloud_bindings
WHERE repository_id = $1
ORDER BY created_at DESC;

-- name: UpdateRepositoryCloudBindingLastRun :one
UPDATE repository_cloud_bindings
SET last_import_run_id = $3, updated_at = now()
WHERE repository_id = $1 AND provider = $2
RETURNING *;

-- name: CreateCloudImportRun :one
INSERT INTO cloud_import_runs (
    run_id,
    repository_id,
    credential_id,
    provider,
    status,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, now(), now()
) RETURNING *;

-- name: GetCloudImportRun :one
SELECT * FROM cloud_import_runs
WHERE run_id = $1;

-- name: ListCloudImportRunsForRepository :many
SELECT * FROM cloud_import_runs
WHERE repository_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: MarkCloudImportRunStarted :one
UPDATE cloud_import_runs
SET status = 'running', started_at = now(), updated_at = now()
WHERE run_id = $1
RETURNING *;

-- name: IncrementCloudImportRunCounts :one
UPDATE cloud_import_runs
SET
    total_seen = total_seen + $2,
    downloaded_count = downloaded_count + $3,
    imported_count = imported_count + $4,
    skipped_count = skipped_count + $5,
    failed_count = failed_count + $6,
    updated_at = now()
WHERE run_id = $1
RETURNING *;

-- name: FinishCloudImportRun :one
UPDATE cloud_import_runs
SET status = $2, error = $3, finished_at = now(), updated_at = now()
WHERE run_id = $1
RETURNING *;

-- name: MarkStaleCloudImportRunsInterrupted :exec
UPDATE cloud_import_runs
SET status = 'interrupted', finished_at = now(), updated_at = now()
WHERE status IN ('queued', 'running');
