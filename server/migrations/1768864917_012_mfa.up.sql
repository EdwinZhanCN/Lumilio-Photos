CREATE TABLE user_mfa_totp_credentials (
    user_id INTEGER PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    secret_ciphertext BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    enabled_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMPTZ
);

CREATE TABLE user_mfa_recovery_codes (
    recovery_code_id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    code_hash VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    used_at TIMESTAMPTZ,
    UNIQUE (user_id, code_hash)
);

CREATE INDEX idx_user_mfa_recovery_codes_user_id
    ON user_mfa_recovery_codes(user_id);

CREATE INDEX idx_user_mfa_recovery_codes_unused
    ON user_mfa_recovery_codes(user_id)
    WHERE used_at IS NULL;
