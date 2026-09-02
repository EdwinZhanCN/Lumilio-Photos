-- name: CreateOCRResult :one
INSERT INTO ocr_results (
    asset_id, model_id, total_count, processing_time_ms, created_at, updated_at
)
VALUES (
    ?1, ?2, ?3, ?4,
    CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    CAST(unixepoch('subsec') * 1000000 AS INTEGER)
)
RETURNING *;

-- name: GetOCRResultByAsset :one
SELECT * FROM ocr_results
WHERE asset_id = ?1;

-- name: DeleteOCRResultByAsset :exec
DELETE FROM ocr_results WHERE asset_id = ?1;

-- name: CreateOCRTextItem :one
INSERT INTO ocr_text_items (
    asset_id, text_content, confidence, bounding_box, text_length, area_pixels, created_at
)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, CAST(unixepoch('subsec') * 1000000 AS INTEGER))
RETURNING *;

-- name: GetOCRTextItemsByAsset :many
SELECT * FROM ocr_text_items
WHERE asset_id = ?1
ORDER BY confidence DESC, text_length DESC;

-- name: GetOCRTextItemsByAssetWithLimit :many
SELECT * FROM ocr_text_items
WHERE asset_id = ?1
ORDER BY confidence DESC, text_length DESC
LIMIT ?2;

-- name: DeleteOCRTextItemsByAsset :exec
DELETE FROM ocr_text_items WHERE asset_id = ?1;

-- name: BumpOCRIndexRevision :one
INSERT INTO ocr_index_metadata (
    asset_id, revision, updated_at
)
VALUES (
    ?1, 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)
)
ON CONFLICT (asset_id) DO UPDATE SET
    revision = ocr_index_metadata.revision + 1,
    updated_at = excluded.updated_at
RETURNING revision;

-- name: UpsertOCRIndexOutbox :exec
INSERT INTO ocr_index_outbox (
    asset_id, revision, updated_at
)
VALUES (
    ?1, ?2, CAST(unixepoch('subsec') * 1000000 AS INTEGER)
)
ON CONFLICT (asset_id) DO UPDATE SET
    revision = excluded.revision,
    updated_at = excluded.updated_at;

-- name: ListOCRIndexOutboxBatch :many
SELECT asset_id, revision
FROM ocr_index_outbox
ORDER BY updated_at, asset_id
LIMIT ?1;

-- name: AcknowledgeOCRIndexOutbox :execrows
DELETE FROM ocr_index_outbox
WHERE asset_id = ?1 AND revision = ?2;

-- name: ClearOCRIndexOutbox :exec
DELETE FROM ocr_index_outbox;

-- name: GetOCRDocumentsByAssetIDs :many
WITH filter_params AS (
    SELECT CAST(sqlc.arg('asset_ids') AS TEXT) AS asset_ids_json
)
SELECT
    a.asset_id,
    a.owner_id,
    COALESCE(CAST(occurrence.repository_id AS TEXT), '') AS repository_id,
    a.type AS asset_type,
    a.is_deleted,
    m.revision,
    ti.text_content
FROM ocr_results r
JOIN assets a ON a.asset_id = r.asset_id
LEFT JOIN (
    SELECT asset_id, MIN(repository_id) AS repository_id
    FROM active_asset_occurrences
    GROUP BY asset_id
) occurrence ON occurrence.asset_id = a.asset_id
JOIN ocr_index_metadata m ON m.asset_id = r.asset_id
LEFT JOIN ocr_text_items ti ON ti.asset_id = r.asset_id
WHERE r.asset_id IN (
    SELECT value FROM json_each((SELECT asset_ids_json FROM filter_params))
)
ORDER BY a.asset_id, ti.id;

-- name: GetOCRDocumentsForRebuild :many
WITH batch_assets AS (
    SELECT r.asset_id
    FROM ocr_results r
    WHERE r.asset_id > sqlc.arg('after_asset_id')
    ORDER BY r.asset_id
    LIMIT sqlc.arg('batch_size')
)
SELECT
    a.asset_id,
    a.owner_id,
    COALESCE(CAST(occurrence.repository_id AS TEXT), '') AS repository_id,
    a.type AS asset_type,
    a.is_deleted,
    m.revision,
    ti.text_content
FROM batch_assets b
JOIN ocr_results r ON r.asset_id = b.asset_id
JOIN assets a ON a.asset_id = r.asset_id
LEFT JOIN (
    SELECT asset_id, MIN(repository_id) AS repository_id
    FROM active_asset_occurrences
    GROUP BY asset_id
) occurrence ON occurrence.asset_id = a.asset_id
JOIN ocr_index_metadata m ON m.asset_id = r.asset_id
LEFT JOIN ocr_text_items ti ON ti.asset_id = r.asset_id
ORDER BY a.asset_id, ti.id;

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
WHERE asset_id = ?1;

-- name: GetHighConfidenceTextItems :many
SELECT * FROM ocr_text_items
WHERE confidence >= ?1
ORDER BY confidence DESC, text_length DESC
LIMIT ?2;

-- name: UpdateOCRResultStats :exec
UPDATE ocr_results
SET total_count = (
    SELECT COUNT(*) FROM ocr_text_items ti WHERE ti.asset_id = ?1
),
updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE asset_id = ?1;
