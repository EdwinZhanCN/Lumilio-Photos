-- name: CreateAuthSecurityVerification :one
INSERT INTO auth_security_verifications (
  verification_id,
  token_hash,
  user_id,
  auth_version,
  purpose,
  created_at,
  expires_at
)
VALUES (
  ?1,
  ?2,
  ?3,
  ?4,
  ?5,
  CAST(unixepoch('subsec') * 1000000 AS INTEGER),
  ?6
)
RETURNING *;

-- name: GetAuthSecurityVerification :one
SELECT *
FROM auth_security_verifications
WHERE token_hash = ?1
  AND user_id = ?2
  AND auth_version = ?3
  AND purpose = ?4
  AND consumed_at IS NULL
  AND expires_at > CAST(unixepoch('subsec') * 1000000 AS INTEGER);

-- name: ConsumeAuthSecurityVerification :execrows
UPDATE auth_security_verifications
SET consumed_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE verification_id = ?1
  AND user_id = ?2
  AND auth_version = ?3
  AND purpose = ?4
  AND consumed_at IS NULL
  AND expires_at > CAST(unixepoch('subsec') * 1000000 AS INTEGER);

-- name: DeleteExpiredAuthSecurityVerifications :exec
DELETE FROM auth_security_verifications
WHERE expires_at <= CAST(unixepoch('subsec') * 1000000 AS INTEGER)
   OR consumed_at IS NOT NULL;
