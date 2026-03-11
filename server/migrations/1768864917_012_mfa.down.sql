DROP INDEX IF EXISTS idx_user_mfa_recovery_codes_unused;
DROP INDEX IF EXISTS idx_user_mfa_recovery_codes_user_id;

DROP TABLE IF EXISTS user_mfa_recovery_codes;
DROP TABLE IF EXISTS user_mfa_totp_credentials;
