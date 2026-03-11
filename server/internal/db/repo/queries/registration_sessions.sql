-- name: CreateRegistrationSession :one
INSERT INTO registration_sessions (
  username,
  password_hash,
  role,
  webauthn_user_handle,
  expires_at
)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetRegistrationSessionByID :one
SELECT *
FROM registration_sessions
WHERE session_id = $1;

-- name: DeleteRegistrationSession :exec
DELETE FROM registration_sessions
WHERE session_id = $1;

-- name: DeleteRegistrationSessionsByUsername :exec
DELETE FROM registration_sessions
WHERE username = $1;

-- name: DeleteExpiredRegistrationSessions :exec
DELETE FROM registration_sessions
WHERE expires_at <= CURRENT_TIMESTAMP;

-- name: UpdateRegistrationSessionTOTPSecret :one
UPDATE registration_sessions
SET totp_secret_ciphertext = $2
WHERE session_id = $1
RETURNING *;
