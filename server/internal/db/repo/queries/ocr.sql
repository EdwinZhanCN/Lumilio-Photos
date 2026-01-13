-- name: CreateOCRResult :one
INSERT INTO ocr_results (asset_id, model_id, total_count, processing_time_ms)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetOCRResultByAsset :one
SELECT * FROM ocr_results
WHERE asset_id = $1;

-- name: DeleteOCRResultByAsset :exec
DELETE FROM ocr_results WHERE asset_id = $1;

-- name: CreateOCRTextItem :one
INSERT INTO ocr_text_items (asset_id, text_content, confidence, bounding_box, text_length, area_pixels)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetOCRTextItemsByAsset :many
SELECT * FROM ocr_text_items
WHERE asset_id = $1
ORDER BY confidence DESC, text_length DESC;

-- name: GetOCRTextItemsByAssetWithLimit :many
SELECT * FROM ocr_text_items
WHERE asset_id = $1
ORDER BY confidence DESC, text_length DESC
LIMIT $2;

-- name: DeleteOCRTextItemsByAsset :exec
DELETE FROM ocr_text_items WHERE asset_id = $1;

-- name: SearchAssetsByOCRText :many
SELECT DISTINCT a.* FROM assets a
JOIN ocr_results r ON a.asset_id = r.asset_id
JOIN ocr_text_items t ON r.asset_id = t.asset_id
WHERE to_tsvector('simple', t.text_content) @@ plainto_tsquery('simple', $1)
ORDER BY a.upload_time DESC
LIMIT $3 OFFSET $2;

-- name: SearchAssetsByOCRTextWithConfidence :many
SELECT DISTINCT a.* FROM assets a
JOIN ocr_results r ON a.asset_id = r.asset_id
JOIN ocr_text_items t ON r.asset_id = t.asset_id
WHERE to_tsvector('simple', t.text_content) @@ plainto_tsquery('simple', $1)
AND t.confidence >= $4
ORDER BY a.upload_time DESC
LIMIT $3 OFFSET $2;

-- name: GetOCRStatsByModel :many
SELECT
    model_id,
    COUNT(*) as total_assets,
    SUM(total_count) as total_text_items,
    AVG(total_count) as avg_items_per_asset,
    MIN(processing_time_ms) as min_processing_time,
    MAX(processing_time_ms) as max_processing_time,
    AVG(processing_time_ms) as avg_processing_time
FROM ocr_results
GROUP BY model_id
ORDER BY total_assets DESC;

-- name: GetOCRTextItemStatsByAsset :one
SELECT
    COUNT(*) as total_items,
    AVG(confidence) as avg_confidence,
    MIN(confidence) as min_confidence,
    MAX(confidence) as max_confidence,
    SUM(text_length) as total_text_length,
    AVG(text_length) as avg_text_length
FROM ocr_text_items
WHERE asset_id = $1;

-- name: GetHighConfidenceTextItems :many
SELECT * FROM ocr_text_items
WHERE confidence >= $1
ORDER BY confidence DESC, text_length DESC
LIMIT $2;

-- name: UpdateOCRResultStats :exec
UPDATE ocr_results
SET total_count = (
    SELECT COUNT(*) FROM ocr_text_items ti WHERE ti.asset_id = $1
),
updated_at = CURRENT_TIMESTAMP
WHERE asset_id = $1;