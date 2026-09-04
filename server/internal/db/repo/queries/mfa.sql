-- name: GetUserMFAStatus :one
SELECT
  CASE WHEN totp.user_id IS NULL THEN 0 ELSE 1 END AS totp_enabled,
  COALESCE(passkeys.passkey_count, 0) AS passkey_count,
  COALESCE(recovery.recovery_codes_remaining, 0) AS recovery_codes_remaining,
  CAST(COALESCE((
    SELECT created_at
    FROM user_mfa_recovery_codes latest_recovery
    WHERE latest_recovery.user_id = u.user_id
    ORDER BY created_at DESC
    LIMIT 1
  ), 0) AS INTEGER) AS recovery_codes_generated_at
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
    COUNT(*) FILTER (WHERE user_mfa_recovery_codes.used_at IS NULL) AS recovery_codes_remaining
  FROM user_mfa_recovery_codes
  GROUP BY user_mfa_recovery_codes.user_id
) recovery ON recovery.user_id = u.user_id
WHERE u.user_id = ?1;

-- name: GetUserTOTPCredential :one
SELECT *
FROM user_mfa_totp_credentials
WHERE user_id = ?1;

-- name: GetPendingTOTPEnrollment :one
SELECT *
FROM pending_totp_enrollments
WHERE enrollment_id = ?1
  AND user_id = ?2
  AND auth_version = ?3
  AND consumed_at IS NULL
  AND expires_at > CAST(unixepoch('subsec') * 1000000 AS INTEGER);

-- name: CreatePendingTOTPEnrollment :one
INSERT INTO pending_totp_enrollments (
  enrollment_id,
  user_id,
  secret_ciphertext,
  auth_version,
  created_at,
  expires_at
)
VALUES (
  ?1,
  ?2,
  ?3,
  ?4,
  CAST(unixepoch('subsec') * 1000000 AS INTEGER),
  ?5
)
RETURNING *;

-- name: DeletePendingTOTPEnrollments :exec
DELETE FROM pending_totp_enrollments
WHERE user_id = ?1;

-- name: ConsumePendingTOTPEnrollment :execrows
UPDATE pending_totp_enrollments
SET consumed_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE enrollment_id = ?1
  AND user_id = ?2
  AND auth_version = ?3
  AND consumed_at IS NULL
  AND expires_at > CAST(unixepoch('subsec') * 1000000 AS INTEGER);

-- name: UpsertUserTOTPCredential :one
INSERT INTO user_mfa_totp_credentials (
  user_id,
  secret_ciphertext,
  enabled_at,
  created_at,
  updated_at,
  last_used_counter
)
VALUES (
    ?1,
    ?2,
    CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    ?3
)
ON CONFLICT (user_id) DO UPDATE
SET secret_ciphertext = EXCLUDED.secret_ciphertext,
    enabled_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    last_used_at = NULL,
    credential_version = user_mfa_totp_credentials.credential_version + 1,
    last_used_counter = EXCLUDED.last_used_counter
RETURNING *;

-- name: UseTOTPCode :execrows
UPDATE user_mfa_totp_credentials
SET last_used_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    last_used_counter = ?2
WHERE user_id = ?1
  AND last_used_counter < ?2;

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
