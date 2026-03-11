DROP INDEX IF EXISTS idx_user_webauthn_credentials_user_id;
DROP TABLE IF EXISTS user_webauthn_credentials;

DROP INDEX IF EXISTS idx_registration_sessions_expires_at;
DROP TABLE IF EXISTS registration_sessions;

ALTER TABLE users
    ADD COLUMN email VARCHAR(100);

CREATE UNIQUE INDEX idx_users_email
    ON users(email)
    WHERE email IS NOT NULL;

DROP INDEX IF EXISTS idx_users_webauthn_user_handle;

ALTER TABLE users
    DROP COLUMN webauthn_user_handle;
