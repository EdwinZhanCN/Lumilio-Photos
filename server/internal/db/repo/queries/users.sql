-- name: CreateUser :one
INSERT INTO users (username, password, display_name, role, webauthn_user_handle)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: CountActiveUsersByRole :one
SELECT COUNT(*)
FROM users
WHERE role = $1
  AND is_active = true;

-- name: GetUserByID :one
SELECT * FROM users WHERE user_id = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: UpdateUser :one
UPDATE users
SET username = $2, updated_at = CURRENT_TIMESTAMP, last_login = $3
WHERE user_id = $1
RETURNING *;

-- name: UpdateUserLastLogin :exec
UPDATE users
SET last_login = $2, updated_at = CURRENT_TIMESTAMP
WHERE user_id = $1;

-- name: UpdateUserProfile :one
UPDATE users
SET display_name = $2,
    avatar_url = $3,
    updated_at = CURRENT_TIMESTAMP
WHERE user_id = $1
RETURNING *;

-- name: AdminUpdateUser :one
UPDATE users
SET username = $2,
    display_name = $3,
    avatar_url = $4,
    role = $5,
    is_active = $6,
    updated_at = CURRENT_TIMESTAMP
WHERE user_id = $1
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users
SET password = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE user_id = $1;

-- name: DeleteUser :exec
DELETE FROM users WHERE user_id = $1;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListUsersWithStats :many
SELECT
  u.*,
  COALESCE(asset_counts.asset_count, 0)::bigint AS asset_count,
  COALESCE(album_counts.album_count, 0)::bigint AS album_count
FROM users u
LEFT JOIN (
  SELECT owner_id AS user_id, COUNT(*) AS asset_count
  FROM assets
  WHERE owner_id IS NOT NULL
    AND is_deleted = false
  GROUP BY owner_id
) asset_counts ON asset_counts.user_id = u.user_id
LEFT JOIN (
  SELECT user_id, COUNT(*) AS album_count
  FROM albums
  GROUP BY user_id
) album_counts ON album_counts.user_id = u.user_id
ORDER BY u.created_at DESC, u.user_id DESC
LIMIT $1 OFFSET $2;

-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, token, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetRefreshTokenByToken :one
SELECT * FROM refresh_tokens
WHERE token = $1 AND is_revoked = false;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens SET is_revoked = true WHERE token_id = $1;

-- name: RevokeUserRefreshTokens :exec
UPDATE refresh_tokens
SET is_revoked = true
WHERE user_id = $1
  AND is_revoked = false;
