CREATE TABLE settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),

    llm_agent_enabled BOOLEAN NOT NULL DEFAULT false,
    llm_provider TEXT NOT NULL DEFAULT 'ark',
    llm_model_name TEXT NOT NULL DEFAULT '',
    llm_base_url TEXT NOT NULL DEFAULT '',
    llm_api_key_ciphertext BYTEA,
    llm_api_key_configured BOOLEAN NOT NULL DEFAULT false,

    ml_auto TEXT NOT NULL DEFAULT 'disable'
        CHECK (ml_auto IN ('enable', 'disable')),
    ml_clip_enabled BOOLEAN NOT NULL DEFAULT false,
    ml_ocr_enabled BOOLEAN NOT NULL DEFAULT false,
    ml_caption_enabled BOOLEAN NOT NULL DEFAULT false,
    ml_face_enabled BOOLEAN NOT NULL DEFAULT false,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by INTEGER REFERENCES users(user_id)
);
