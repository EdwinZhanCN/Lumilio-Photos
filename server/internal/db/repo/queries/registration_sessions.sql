-- name: CreateRegistrationSession :one
INSERT INTO registration_sessions (
  username,
  password_hash,
  role,
  webauthn_user_handle,
  created_at,
  expires_at,
  session_id
)
VALUES (
  ?1,
  ?2,
  ?3,
  ?4,
  CAST(unixepoch('subsec') * 1000000 AS INTEGER),
  ?5,
  sqlc.arg('session_id')
)
RETURNING *;

-- name: GetRegistrationSessionByID :one
SELECT *
FROM registration_sessions
WHERE session_id = ?1;

-- name: DeleteRegistrationSession :exec
DELETE FROM registration_sessions
WHERE session_id = ?1;

-- name: DeleteRegistrationSessionsByUsername :exec
DELETE FROM registration_sessions
WHERE username = ?1;

-- name: DeleteExpiredRegistrationSessions :exec
DELETE FROM registration_sessions
WHERE expires_at <= CAST(unixepoch('subsec') * 1000000 AS INTEGER);

-- name: UpdateRegistrationSessionTOTPSecret :one
UPDATE registration_sessions
SET totp_secret_ciphertext = ?2
WHERE session_id = ?1
RETURNING *;
