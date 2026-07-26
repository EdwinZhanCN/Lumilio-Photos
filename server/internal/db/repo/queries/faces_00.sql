-- name: CreateFaceResult :one
INSERT INTO face_results (asset_id, model_id, total_faces, processing_time_ms)
VALUES (?1, ?2, ?3, ?4)
RETURNING *;

-- name: GetFaceResultByAsset :one
SELECT * FROM face_results
WHERE asset_id = ?1;

-- name: DeleteFaceResultByAsset :exec
DELETE FROM face_results WHERE asset_id = ?1;

-- name: CreateFaceItem :one
INSERT INTO face_items (
    asset_id, face_id, bounding_box, confidence, age_group, gender,
    ethnicity, expression, face_size, face_image_path, embedding,
    embedding_model, is_primary, quality_score, blur_score, pose_angles
)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13, ?14, ?15, ?16)
RETURNING *;

-- name: GetFaceItemsByAsset :many
SELECT * FROM face_items
WHERE asset_id = ?1
ORDER BY is_primary DESC, confidence DESC;

-- name: GetFaceItemsByAssetWithLimit :many
SELECT * FROM face_items
WHERE asset_id = ?1
ORDER BY is_primary DESC, confidence DESC
LIMIT ?2;

-- name: GetFaceItemByID :one
SELECT * FROM face_items
WHERE id = ?1;

-- name: DeleteFaceItemsByAsset :exec
DELETE FROM face_items WHERE asset_id = ?1;

-- name: CreateFaceCluster :one
-- owner_id is the cluster's structural owner (rule: a cluster never spans
-- owners). NULL means the cluster groups ownerless assets and is admin-only.
INSERT INTO face_clusters (owner_id, cluster_name, representative_face_id, confidence_score, is_confirmed)
VALUES (sqlc.narg('owner_id'), sqlc.arg('cluster_name'), sqlc.arg('representative_face_id'), sqlc.arg('confidence_score'), sqlc.arg('is_confirmed'))
RETURNING *;

-- name: GetFaceClusterByID :one
SELECT * FROM face_clusters
WHERE cluster_id = ?1;

-- name: UpdateFaceCluster :one
UPDATE face_clusters
SET
    cluster_name = ?2,
    confidence_score = ?3,
    is_confirmed = ?4,
    updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE cluster_id = ?1
RETURNING *;

-- name: DeleteFaceCluster :exec
DELETE FROM face_clusters WHERE cluster_id = ?1;

-- name: GetFaceClusterByRepresentative :one
SELECT * FROM face_clusters
WHERE representative_face_id = ?1;

-- name: GetAllFaceClusters :many
SELECT * FROM face_clusters
ORDER BY is_confirmed DESC, member_count DESC;

-- name: GetConfirmedFaceClusters :many
SELECT * FROM face_clusters
WHERE is_confirmed = true
ORDER BY cluster_name ASC;

-- name: CreateFaceClusterMember :one
INSERT INTO face_cluster_members (cluster_id, face_id, similarity_score, confidence, is_manual)
VALUES (?1, ?2, ?3, ?4, ?5)
RETURNING *;

-- name: GetFaceClusterMembers :many
SELECT fi.*, fcm.similarity_score, fcm.confidence, fcm.is_manual
FROM face_cluster_members fcm
JOIN face_items fi ON fcm.face_id = fi.id
WHERE fcm.cluster_id = ?1
ORDER BY fcm.confidence DESC;

-- name: DeleteFaceClusterMember :exec
DELETE FROM face_cluster_members
WHERE cluster_id = ?1 AND face_id = ?2;

-- name: GetFaceClusterByFaceID :one
SELECT fc.* FROM face_clusters fc
JOIN face_cluster_members fcm ON fc.cluster_id = fcm.cluster_id
WHERE fcm.face_id = ?1;

-- name: SearchAssetsByFaceID :many
SELECT DISTINCT a.* FROM assets a
JOIN face_items fi ON a.asset_id = fi.asset_id
WHERE fi.face_id = ?1
ORDER BY a.upload_time DESC
LIMIT ?3 OFFSET ?2;

-- name: SearchAssetsByFaceCluster :many
WITH matched_assets AS (
    SELECT fi.asset_id
    FROM face_cluster_members fcm
    JOIN face_items fi ON fi.id = fcm.face_id
    WHERE fcm.cluster_id = sqlc.arg('cluster_id')
    GROUP BY fi.asset_id
),
page_ids AS (
    SELECT
        m.asset_id,
        a.upload_time
    FROM matched_assets m
    JOIN assets a ON a.asset_id = m.asset_id
    ORDER BY a.upload_time DESC, m.asset_id DESC
    LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset')
)
SELECT a.*
FROM page_ids p
JOIN assets a ON a.asset_id = p.asset_id
ORDER BY p.upload_time DESC, p.asset_id DESC;

-- name: GetUnclusteredFaces :many
SELECT fi.* FROM face_items fi
LEFT JOIN face_cluster_members fcm ON fi.id = fcm.face_id
WHERE fcm.face_id IS NULL
AND fi.confidence >= ?1
ORDER BY fi.confidence DESC
LIMIT ?2;
