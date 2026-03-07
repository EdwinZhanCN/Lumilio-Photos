-- name: GetSettings :one
SELECT * FROM settings
WHERE id = 1;

-- name: UpsertSettings :one
INSERT INTO settings (
    id,
    llm_agent_enabled,
    llm_provider,
    llm_model_name,
    llm_base_url,
    llm_api_key_ciphertext,
    llm_api_key_configured,
    ml_auto,
    ml_clip_enabled,
    ml_ocr_enabled,
    ml_caption_enabled,
    ml_face_enabled,
    updated_by
)
VALUES (
    1,
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11,
    $12
)
ON CONFLICT (id) DO UPDATE SET
    llm_agent_enabled = EXCLUDED.llm_agent_enabled,
    llm_provider = EXCLUDED.llm_provider,
    llm_model_name = EXCLUDED.llm_model_name,
    llm_base_url = EXCLUDED.llm_base_url,
    llm_api_key_ciphertext = EXCLUDED.llm_api_key_ciphertext,
    llm_api_key_configured = EXCLUDED.llm_api_key_configured,
    ml_auto = EXCLUDED.ml_auto,
    ml_clip_enabled = EXCLUDED.ml_clip_enabled,
    ml_ocr_enabled = EXCLUDED.ml_ocr_enabled,
    ml_caption_enabled = EXCLUDED.ml_caption_enabled,
    ml_face_enabled = EXCLUDED.ml_face_enabled,
    updated_at = NOW(),
    updated_by = EXCLUDED.updated_by
RETURNING *;
