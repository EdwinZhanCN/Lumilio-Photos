-- name: CreateCaption :one
INSERT INTO captions (asset_id, model_id, description, summary, confidence, tokens_generated, processing_time_ms, prompt_used, finish_reason)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetCaptionByAsset :one
SELECT * FROM captions
WHERE asset_id = $1;

-- name: DeleteCaptionByAsset :exec
DELETE FROM captions WHERE asset_id = $1;

-- name: UpdateCaption :one
UPDATE captions
SET
    model_id = $2,
    description = $3,
    summary = $4,
    confidence = $5,
    tokens_generated = $6,
    processing_time_ms = $7,
    prompt_used = $8,
    finish_reason = $9,
    updated_at = CURRENT_TIMESTAMP
WHERE asset_id = $1
RETURNING *;

-- name: SearchAssetsByCaption :many
SELECT DISTINCT a.* FROM assets a
JOIN captions d ON a.asset_id = d.asset_id
WHERE to_tsvector('english', d.description) @@ plainto_tsquery('english', $1)
ORDER BY a.upload_time DESC
LIMIT $3 OFFSET $2;

-- name: SearchAssetsByCaptionWithConfidence :many
SELECT DISTINCT a.* FROM assets a
JOIN captions d ON a.asset_id = d.asset_id
WHERE to_tsvector('english', d.description) @@ plainto_tsquery('english', $1)
AND d.confidence >= $4
ORDER BY a.upload_time DESC
LIMIT $3 OFFSET $2;

-- name: SearchAssetsByCaptionSummary :many
SELECT DISTINCT a.* FROM assets a
JOIN captions d ON a.asset_id = d.asset_id
WHERE to_tsvector('english', d.summary) @@ plainto_tsquery('english', $1)
AND d.summary IS NOT NULL
ORDER BY a.upload_time DESC
LIMIT $3 OFFSET $2;

-- name: GetCaptionStatsByModel :many
SELECT
    model_id,
    COUNT(*) as total_descriptions,
    AVG(tokens_generated) as avg_tokens,
    MIN(tokens_generated) as min_tokens,
    MAX(tokens_generated) as max_tokens,
    AVG(processing_time_ms) as avg_processing_time,
    MIN(processing_time_ms) as min_processing_time,
    MAX(processing_time_ms) as max_processing_time,
    AVG(confidence) as avg_confidence
FROM captions
GROUP BY model_id
ORDER BY total_descriptions DESC;

-- name: GetTopCaptionsByTokens :many
SELECT * FROM captions
ORDER BY tokens_generated DESC
LIMIT $1;

-- name: GetCaptionsByModel :many
SELECT * FROM captions
WHERE model_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetLongCaptions :many
SELECT * FROM captions
WHERE LENGTH(description) > sqlc.arg(min_length)::int
ORDER BY LENGTH(description) DESC
LIMIT sqlc.arg(row_limit);

-- name: UpdateCaptionStats :exec
UPDATE captions
SET
    tokens_generated = (
        SELECT LENGTH(description) / 4.0 -- Approximate token count (rough estimate: 1 token ≈ 4 characters)
        WHERE asset_id = $1
    )
WHERE asset_id = $1;
