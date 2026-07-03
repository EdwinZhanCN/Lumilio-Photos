-- name: CreateShareLink :one
INSERT INTO share_links (owner_id, token_hash, title, description, source_kind, source_ref, asset_ids, asset_count, allow_download, include_originals, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListShareLinksByOwner :many
SELECT * FROM share_links WHERE owner_id = $1 ORDER BY created_at DESC;

-- name: GetShareLinkByID :one
SELECT * FROM share_links WHERE share_id = $1 AND owner_id = $2;

-- name: UpdateShareLinkSettings :one
UPDATE share_links
SET title = $3, description = $4, allow_download = $5, include_originals = $6, updated_at = CURRENT_TIMESTAMP
WHERE share_id = $1 AND owner_id = $2
RETURNING *;

-- name: ExtendShareLinkExpiry :one
UPDATE share_links
SET expires_at = $3, updated_at = CURRENT_TIMESTAMP
WHERE share_id = $1 AND owner_id = $2
RETURNING *;

-- name: RevokeShareLink :one
UPDATE share_links
SET status = 'revoked', revoked_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE share_id = $1 AND owner_id = $2
RETURNING *;

-- name: DeleteShareLink :execrows
DELETE FROM share_links
WHERE share_id = $1 AND owner_id = $2 AND (status = 'revoked' OR expires_at < CURRENT_TIMESTAMP);

-- name: GetActiveShareLinkByTokenHash :one
SELECT * FROM share_links
WHERE token_hash = $1 AND status = 'active' AND expires_at > CURRENT_TIMESTAMP;

-- name: IncrementShareLinkView :exec
UPDATE share_links
SET view_count = view_count + 1, last_viewed_at = CURRENT_TIMESTAMP
WHERE share_id = $1;
