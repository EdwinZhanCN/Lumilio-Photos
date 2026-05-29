-- name: GetCloudSyncCursor :one
SELECT cursor_value FROM cloud_sync_cursors
WHERE repository_id = $1 AND provider = $2;

-- name: UpsertCloudSyncCursor :exec
INSERT INTO cloud_sync_cursors (repository_id, provider, cursor_value, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (repository_id, provider)
DO UPDATE SET cursor_value = $3, updated_at = now();

-- name: GetCloudSyncFile :one
SELECT etag, local_hash FROM cloud_sync_files
WHERE repository_id = $1 AND provider = $2 AND remote_key = $3;

-- name: GetAssetIDByCloudFile :one
SELECT asset_id FROM cloud_sync_files
WHERE repository_id = $1 AND provider = $2 AND remote_key = $3;

-- name: MarkCloudSyncFile :exec
INSERT INTO cloud_sync_files (repository_id, provider, remote_key, etag, local_hash, asset_id, synced_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (repository_id, provider, remote_key)
DO UPDATE SET etag = $4, local_hash = $5, asset_id = $6, synced_at = now();
