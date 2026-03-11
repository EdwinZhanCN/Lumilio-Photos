ALTER TABLE users
    ADD COLUMN webauthn_user_handle BYTEA;

UPDATE users
SET webauthn_user_handle = decode(md5(user_id::text || ':' || username || ':' || created_at::text), 'hex')
WHERE webauthn_user_handle IS NULL;

ALTER TABLE users
    ALTER COLUMN webauthn_user_handle SET NOT NULL;

CREATE UNIQUE INDEX idx_users_webauthn_user_handle
    ON users(webauthn_user_handle);

ALTER TABLE users
    DROP COLUMN email;

CREATE TABLE registration_sessions (
    session_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(20) NOT NULL,
    webauthn_user_handle BYTEA NOT NULL,
    totp_secret_ciphertext BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_registration_sessions_expires_at
    ON registration_sessions(expires_at);

CREATE TABLE user_webauthn_credentials (
    user_webauthn_credential_id SERIAL PRIMARY KEY,
    credential_id BYTEA NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    public_key BYTEA NOT NULL,
    sign_count BIGINT NOT NULL DEFAULT 0,
    transports JSONB NOT NULL DEFAULT '[]'::jsonb,
    attestation_type VARCHAR(50) NOT NULL DEFAULT 'none',
    aaguid BYTEA,
    user_present BOOLEAN NOT NULL DEFAULT false,
    user_verified BOOLEAN NOT NULL DEFAULT false,
    backup_eligible BOOLEAN NOT NULL DEFAULT false,
    backup_state BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMPTZ
);

CREATE INDEX idx_user_webauthn_credentials_user_id
    ON user_webauthn_credentials(user_id);
