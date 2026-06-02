-- name: GetCloudSyncCursor :one
SELECT cursor_value FROM cloud_sync_cursors
WHERE repository_id = $1 AND credential_id = $2 AND provider = $3;

-- name: UpsertCloudSyncCursor :exec
INSERT INTO cloud_sync_cursors (repository_id, credential_id, provider, cursor_value, updated_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (repository_id, credential_id, provider)
DO UPDATE SET cursor_value = $4, updated_at = now();

-- name: GetCloudSyncFile :one
SELECT etag, local_hash FROM cloud_sync_files
WHERE repository_id = $1 AND credential_id = $2 AND provider = $3 AND remote_key = $4;

-- name: MarkCloudSyncFile :exec
INSERT INTO cloud_sync_files (repository_id, credential_id, provider, remote_key, etag, local_hash, asset_id, synced_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
ON CONFLICT (repository_id, credential_id, provider, remote_key)
DO UPDATE SET etag = $5, local_hash = $6, asset_id = $7, synced_at = now();
