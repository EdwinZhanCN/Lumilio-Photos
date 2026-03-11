-- name: ListUserWebAuthnCredentials :many
SELECT *
FROM user_webauthn_credentials
WHERE user_id = $1
ORDER BY created_at ASC, user_webauthn_credential_id ASC;

-- name: ListUserWebAuthnCredentialSummaries :many
SELECT
  user_webauthn_credential_id,
  user_id,
  transports,
  created_at,
  last_used_at
FROM user_webauthn_credentials
WHERE user_id = $1
ORDER BY created_at ASC, user_webauthn_credential_id ASC;

-- name: CreateUserWebAuthnCredential :one
INSERT INTO user_webauthn_credentials (
  credential_id,
  user_id,
  public_key,
  sign_count,
  transports,
  attestation_type,
  aaguid,
  user_present,
  user_verified,
  backup_eligible,
  backup_state
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: UpdateUserWebAuthnCredentialUsage :one
UPDATE user_webauthn_credentials
SET sign_count = $3,
    transports = $4,
    user_present = $5,
    user_verified = $6,
    backup_eligible = $7,
    backup_state = $8,
    last_used_at = CURRENT_TIMESTAMP
WHERE user_id = $1
  AND credential_id = $2
RETURNING *;

-- name: DeleteUserWebAuthnCredential :execrows
DELETE FROM user_webauthn_credentials
WHERE user_id = $1
  AND user_webauthn_credential_id = $2;

-- name: DeleteUserWebAuthnCredentials :exec
DELETE FROM user_webauthn_credentials
WHERE user_id = $1;
