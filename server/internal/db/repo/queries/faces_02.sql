-- name: GetFaceStatsByModel :many
SELECT
    model_id,
    COUNT(*) as total_assets,
    SUM(total_faces) as total_faces,
    AVG(total_faces) as avg_faces_per_asset,
    MIN(processing_time_ms) as min_processing_time,
    MAX(processing_time_ms) as max_processing_time,
    AVG(processing_time_ms) as avg_processing_time
FROM face_results
GROUP BY model_id
ORDER BY total_assets DESC;

-- name: GetFaceStatsByAsset :one
SELECT
    COUNT(*) as total_faces,
    AVG(confidence) as avg_confidence,
    MIN(confidence) as min_confidence,
    MAX(confidence) as max_confidence,
    COUNT(CASE WHEN is_primary = true THEN 1 END) as primary_faces,
    AVG(quality_score) as avg_quality_score,
    AVG(face_size) as avg_face_size
FROM face_items
WHERE asset_id = ?1;

-- name: GetFaceDemographics :many
SELECT
    age_group,
    gender,
    ethnicity,
    COUNT(*) as count,
    AVG(confidence) as avg_confidence
FROM face_items
WHERE confidence >= ?1
GROUP BY age_group, gender, ethnicity
ORDER BY count DESC;

-- name: GetTopFacesByQuality :many
SELECT * FROM face_items
WHERE quality_score >= ?1
ORDER BY quality_score DESC, confidence DESC
LIMIT ?2;

-- name: GetFacesByExpression :many
SELECT * FROM face_items
WHERE expression = ?1
AND confidence >= ?2
ORDER BY confidence DESC
LIMIT ?3;

-- name: GetPrimaryFaces :many
SELECT * FROM face_items
WHERE is_primary = true
AND confidence >= ?1
ORDER BY confidence DESC
LIMIT ?2;

-- name: UpdateFaceResultStats :exec
UPDATE face_results
SET total_faces = (
    SELECT COUNT(*) FROM face_items fi WHERE fi.asset_id = ?1
),
updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE asset_id = ?1;

-- name: UpdateFaceItemEmbedding :one
UPDATE face_items
SET
    embedding = ?2,
    embedding_model = ?3
WHERE id = ?1
RETURNING *;

-- name: GetFaceEmbeddingsForClustering :many
SELECT id, asset_id, face_id, embedding, confidence, quality_score
FROM face_items
WHERE embedding IS NOT NULL
AND confidence >= ?1
AND quality_score >= ?2
ORDER BY quality_score DESC, confidence DESC
LIMIT ?3;

-- name: GetFaceClusteringCandidates :many
SELECT
    fi.*,
    a.repository_id,
    a.owner_id
FROM face_items fi
JOIN assets a ON a.asset_id = fi.asset_id
WHERE COALESCE(a.is_deleted, false) = false
  AND (sqlc.narg('repository_id') IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id') IS NULL OR a.owner_id = sqlc.narg('owner_id'))
  AND fi.embedding IS NOT NULL
  AND fi.confidence >= sqlc.arg('min_confidence')
  AND COALESCE(fi.face_size, 0) >= sqlc.arg('min_face_size')
ORDER BY a.repository_id ASC, a.owner_id ASC NULLS FIRST, fi.embedding_model ASC NULLS FIRST, fi.confidence DESC, COALESCE(fi.face_size, 0) DESC, fi.id ASC;
