-- name: GetUserMFAStatus :one
SELECT
  (totp.user_id IS NOT NULL) AS totp_enabled,
  COALESCE(passkeys.passkey_count, 0) AS passkey_count,
  COALESCE(recovery.recovery_codes_remaining, 0) AS recovery_codes_remaining,
  recovery.recovery_codes_generated_at
FROM users u
LEFT JOIN user_mfa_totp_credentials totp ON totp.user_id = u.user_id
LEFT JOIN (
  SELECT
    user_webauthn_credentials.user_id,
    COUNT(*) AS passkey_count
  FROM user_webauthn_credentials
  GROUP BY user_webauthn_credentials.user_id
) passkeys ON passkeys.user_id = u.user_id
LEFT JOIN (
  SELECT
    user_mfa_recovery_codes.user_id,
    COUNT(*) FILTER (WHERE user_mfa_recovery_codes.used_at IS NULL) AS recovery_codes_remaining,
    MAX(user_mfa_recovery_codes.created_at) AS recovery_codes_generated_at
  FROM user_mfa_recovery_codes
  GROUP BY user_mfa_recovery_codes.user_id
) recovery ON recovery.user_id = u.user_id
WHERE u.user_id = ?1;

-- name: GetUserTOTPCredential :one
SELECT *
FROM user_mfa_totp_credentials
WHERE user_id = ?1;

-- name: UpsertUserTOTPCredential :one
INSERT INTO user_mfa_totp_credentials (
  user_id,
  secret_ciphertext,
  enabled_at,
  created_at,
  updated_at
)
VALUES (
    ?1,
    ?2,
    CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    CAST(unixepoch('subsec') * 1000000 AS INTEGER)
)
ON CONFLICT (user_id) DO UPDATE
SET secret_ciphertext = EXCLUDED.secret_ciphertext,
    enabled_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    last_used_at = NULL
RETURNING *;

-- name: UpdateUserTOTPLastUsed :exec
UPDATE user_mfa_totp_credentials
SET last_used_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE user_id = ?1;

-- name: DeleteUserTOTPCredential :exec
DELETE FROM user_mfa_totp_credentials
WHERE user_id = ?1;

-- name: DeleteUserRecoveryCodes :exec
DELETE FROM user_mfa_recovery_codes
WHERE user_id = ?1;

-- name: CreateUserRecoveryCode :exec
INSERT INTO user_mfa_recovery_codes (user_id, code_hash, created_at)
VALUES (?1, ?2, CAST(unixepoch('subsec') * 1000000 AS INTEGER));

-- name: UseRecoveryCode :one
UPDATE user_mfa_recovery_codes
SET used_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE user_id = ?1
  AND code_hash = ?2
  AND used_at IS NULL
RETURNING recovery_code_id;
