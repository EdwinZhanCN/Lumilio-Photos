-- name: CreateFaceResult :one
INSERT INTO face_results (asset_id, model_id, total_faces, processing_time_ms)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetFaceResultByAsset :one
SELECT * FROM face_results
WHERE asset_id = $1;

-- name: DeleteFaceResultByAsset :exec
DELETE FROM face_results WHERE asset_id = $1;

-- name: CreateFaceItem :one
INSERT INTO face_items (
    asset_id, face_id, bounding_box, confidence, age_group, gender,
    ethnicity, expression, face_size, face_image_path, embedding,
    embedding_model, is_primary, quality_score, blur_score, pose_angles
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING *;

-- name: GetFaceItemsByAsset :many
SELECT * FROM face_items
WHERE asset_id = $1
ORDER BY is_primary DESC, confidence DESC;

-- name: GetFaceItemsByAssetWithLimit :many
SELECT * FROM face_items
WHERE asset_id = $1
ORDER BY is_primary DESC, confidence DESC
LIMIT $2;

-- name: GetFaceItemByID :one
SELECT * FROM face_items
WHERE id = $1;

-- name: DeleteFaceItemsByAsset :exec
DELETE FROM face_items WHERE asset_id = $1;

-- name: CreateFaceCluster :one
INSERT INTO face_clusters (cluster_name, representative_face_id, confidence_score, is_confirmed)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetFaceClusterByID :one
SELECT * FROM face_clusters
WHERE cluster_id = $1;

-- name: UpdateFaceCluster :one
UPDATE face_clusters
SET
    cluster_name = $2,
    confidence_score = $3,
    is_confirmed = $4,
    updated_at = CURRENT_TIMESTAMP
WHERE cluster_id = $1
RETURNING *;

-- name: DeleteFaceCluster :exec
DELETE FROM face_clusters WHERE cluster_id = $1;

-- name: GetFaceClusterByRepresentative :one
SELECT * FROM face_clusters
WHERE representative_face_id = $1;

-- name: GetAllFaceClusters :many
SELECT * FROM face_clusters
ORDER BY is_confirmed DESC, member_count DESC;

-- name: GetConfirmedFaceClusters :many
SELECT * FROM face_clusters
WHERE is_confirmed = true
ORDER BY cluster_name ASC;

-- name: CreateFaceClusterMember :one
INSERT INTO face_cluster_members (cluster_id, face_id, similarity_score, confidence, is_manual)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetFaceClusterMembers :many
SELECT fi.*, fcm.similarity_score, fcm.confidence, fcm.is_manual
FROM face_cluster_members fcm
JOIN face_items fi ON fcm.face_id = fi.id
WHERE fcm.cluster_id = $1
ORDER BY fcm.confidence DESC;

-- name: DeleteFaceClusterMember :exec
DELETE FROM face_cluster_members
WHERE cluster_id = $1 AND face_id = $2;

-- name: GetFaceClusterByFaceID :one
SELECT fc.* FROM face_clusters fc
JOIN face_cluster_members fcm ON fc.cluster_id = fcm.cluster_id
WHERE fcm.face_id = $1;

-- name: SearchAssetsByFaceID :many
SELECT DISTINCT a.* FROM assets a
JOIN face_items fi ON a.asset_id = fi.asset_id
WHERE fi.face_id = $1
ORDER BY a.upload_time DESC
LIMIT $3 OFFSET $2;

-- name: SearchAssetsByFaceCluster :many
SELECT DISTINCT a.* FROM assets a
JOIN face_items fi ON a.asset_id = fi.asset_id
JOIN face_cluster_members fcm ON fi.id = fcm.face_id
WHERE fcm.cluster_id = $1
ORDER BY a.upload_time DESC
LIMIT $3 OFFSET $2;

-- name: GetUnclusteredFaces :many
SELECT fi.* FROM face_items fi
LEFT JOIN face_cluster_members fcm ON fi.id = fcm.face_id
WHERE fcm.face_id IS NULL
AND fi.confidence >= $1
ORDER BY fi.confidence DESC
LIMIT $2;

-- name: GetSimilarFaces :many
SELECT
    fi.*,
    1 - (fi.embedding <=> $1::vector) as similarity
FROM face_items fi
WHERE fi.id != $2
AND fi.embedding IS NOT NULL
AND 1 - (fi.embedding <=> $1::vector) >= $3
ORDER BY similarity DESC
LIMIT $4;

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
WHERE asset_id = $1;

-- name: GetFaceDemographics :many
SELECT
    age_group,
    gender,
    ethnicity,
    COUNT(*) as count,
    AVG(confidence) as avg_confidence
FROM face_items
WHERE confidence >= $1
GROUP BY age_group, gender, ethnicity
ORDER BY count DESC;

-- name: GetTopFacesByQuality :many
SELECT * FROM face_items
WHERE quality_score >= $1
ORDER BY quality_score DESC, confidence DESC
LIMIT $2;

-- name: GetFacesByExpression :many
SELECT * FROM face_items
WHERE expression = $1
AND confidence >= $2
ORDER BY confidence DESC
LIMIT $3;

-- name: GetPrimaryFaces :many
SELECT * FROM face_items
WHERE is_primary = true
AND confidence >= $1
ORDER BY confidence DESC
LIMIT $2;

-- name: UpdateFaceResultStats :exec
UPDATE face_results
SET total_faces = (
    SELECT COUNT(*) FROM face_items fi WHERE fi.asset_id = $1
),
updated_at = CURRENT_TIMESTAMP
WHERE asset_id = $1;

-- name: UpdateFaceItemEmbedding :one
UPDATE face_items
SET
    embedding = $2,
    embedding_model = $3,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: GetFaceEmbeddingsForClustering :many
SELECT id, asset_id, face_id, embedding, confidence, quality_score
FROM face_items
WHERE embedding IS NOT NULL
AND confidence >= $1
AND quality_score >= $2
ORDER BY quality_score DESC, confidence DESC
LIMIT $3;

-- name: MergeFaceClusters :exec
UPDATE face_cluster_members
SET cluster_id = $1
WHERE cluster_id = $2;

-- name: GetClusterMergeCandidates :many
SELECT
    fc1.cluster_id,
    fc1.cluster_name as name1,
    fc2.cluster_id as other_cluster_id,
    fc2.cluster_name as name2,
    -- Calculate average similarity between cluster members
    (SELECT AVG(1 - (fi1.embedding <=> fi2.embedding))
     FROM face_cluster_members fcm1
     JOIN face_items fi1 ON fcm1.face_id = fi1.id
     JOIN face_cluster_members fcm2 ON fcm1.cluster_id = fc1.cluster_id
     JOIN face_items fi2 ON fcm2.face_id = fi2.id
     WHERE fcm2.cluster_id = fc2.cluster_id
     LIMIT 100) as avg_similarity
FROM face_clusters fc1
CROSS JOIN face_clusters fc2
WHERE fc1.cluster_id < fc2.cluster_id
AND fc1.is_confirmed = true
AND fc2.is_confirmed = true
HAVING AVG(1 - (fi1.embedding <=> fi2.embedding)) >= $1
ORDER BY avg_similarity DESC
LIMIT $2;