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
WITH matched_assets AS MATERIALIZED (
    SELECT fi.asset_id
    FROM face_cluster_members fcm
    JOIN face_items fi ON fi.id = fcm.face_id
    WHERE fcm.cluster_id = sqlc.arg('cluster_id')
    GROUP BY fi.asset_id
),
page_ids AS MATERIALIZED (
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
AND fi.confidence >= $1
ORDER BY fi.confidence DESC
LIMIT $2;

-- name: GetUnclusteredFacesInScope :many
SELECT fi.*
FROM face_items fi
JOIN assets a ON a.asset_id = fi.asset_id
LEFT JOIN face_cluster_members fcm ON fi.id = fcm.face_id
WHERE fcm.face_id IS NULL
  AND COALESCE(a.is_deleted, false) = false
  AND a.repository_id = sqlc.arg('repository_id')::uuid
  AND a.owner_id IS NOT DISTINCT FROM sqlc.narg('owner_id')::integer
  AND fi.embedding_model IS NOT DISTINCT FROM sqlc.narg('embedding_model')::text
  AND fi.embedding IS NOT NULL
  AND fi.confidence >= sqlc.arg('min_confidence')
  AND COALESCE(fi.face_size, 0) >= sqlc.arg('min_face_size')
ORDER BY fi.confidence DESC, COALESCE(fi.face_size, 0) DESC, fi.id ASC;

-- name: GetSimilarFaces :many
SELECT
    fi.*,
    CAST(1 - (fi.embedding <=> sqlc.arg('embedding_query')::vector) AS double precision) as similarity
FROM face_items fi
WHERE fi.id != sqlc.arg('id')
AND fi.embedding IS NOT NULL
AND 1 - (fi.embedding <=> sqlc.arg('embedding_query')::vector) >= sqlc.arg('min_similarity')::float8
ORDER BY similarity DESC
LIMIT sqlc.arg('limit');

-- name: GetIncrementalFaceNeighbors :many
SELECT
    fi.*,
    CAST(1 - (fi.embedding <=> sqlc.arg('embedding_query')::vector) AS double precision) AS similarity
FROM face_items fi
JOIN assets a ON a.asset_id = fi.asset_id
WHERE fi.id != sqlc.arg('id')
  AND COALESCE(a.is_deleted, false) = false
  AND a.repository_id = sqlc.arg('repository_id')::uuid
  AND a.owner_id IS NOT DISTINCT FROM sqlc.narg('owner_id')::integer
  AND fi.embedding_model IS NOT DISTINCT FROM sqlc.narg('embedding_model')::text
  AND fi.embedding IS NOT NULL
  AND fi.confidence >= sqlc.arg('min_confidence')
  AND COALESCE(fi.face_size, 0) >= sqlc.arg('min_face_size')
  AND 1 - (fi.embedding <=> sqlc.arg('embedding_query')::vector) >= sqlc.arg('min_similarity')::float8
ORDER BY similarity DESC, fi.confidence DESC, COALESCE(fi.face_size, 0) DESC, fi.id ASC
LIMIT sqlc.arg('limit');

-- name: CountIncrementalFaceNeighbors :one
SELECT COUNT(*)::bigint
FROM face_items fi
JOIN assets a ON a.asset_id = fi.asset_id
WHERE fi.id != sqlc.arg('id')
  AND COALESCE(a.is_deleted, false) = false
  AND a.repository_id = sqlc.arg('repository_id')::uuid
  AND a.owner_id IS NOT DISTINCT FROM sqlc.narg('owner_id')::integer
  AND fi.embedding_model IS NOT DISTINCT FROM sqlc.narg('embedding_model')::text
  AND fi.embedding IS NOT NULL
  AND fi.confidence >= sqlc.arg('min_confidence')
  AND COALESCE(fi.face_size, 0) >= sqlc.arg('min_face_size')
  AND 1 - (fi.embedding <=> sqlc.arg('embedding_query')::vector) >= sqlc.arg('min_similarity')::float8;

-- name: GetNearestAssignedFaceCluster :one
SELECT
    fcm.cluster_id,
    fi.id AS face_id,
    CAST(1 - (fi.embedding <=> sqlc.arg('embedding_query')::vector) AS double precision) AS similarity
FROM face_items fi
JOIN assets a ON a.asset_id = fi.asset_id
JOIN face_cluster_members fcm ON fcm.face_id = fi.id
WHERE fi.id != sqlc.arg('id')
  AND COALESCE(a.is_deleted, false) = false
  AND a.repository_id = sqlc.arg('repository_id')::uuid
  AND a.owner_id IS NOT DISTINCT FROM sqlc.narg('owner_id')::integer
  AND fi.embedding_model IS NOT DISTINCT FROM sqlc.narg('embedding_model')::text
  AND fi.embedding IS NOT NULL
  AND fi.confidence >= sqlc.arg('min_confidence')
  AND COALESCE(fi.face_size, 0) >= sqlc.arg('min_face_size')
  AND 1 - (fi.embedding <=> sqlc.arg('embedding_query')::vector) >= sqlc.arg('min_similarity')::float8
ORDER BY similarity DESC, fi.confidence DESC, COALESCE(fi.face_size, 0) DESC, fi.id ASC
LIMIT 1;

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
    embedding_model = $3
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

-- name: GetFaceClusteringCandidates :many
SELECT
    fi.*,
    a.repository_id,
    a.owner_id
FROM face_items fi
JOIN assets a ON a.asset_id = fi.asset_id
WHERE COALESCE(a.is_deleted, false) = false
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id')::uuid)
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id')::integer)
  AND fi.embedding IS NOT NULL
  AND fi.confidence >= sqlc.arg('min_confidence')
  AND COALESCE(fi.face_size, 0) >= sqlc.arg('min_face_size')
ORDER BY a.repository_id ASC, a.owner_id ASC NULLS FIRST, fi.embedding_model ASC NULLS FIRST, fi.confidence DESC, COALESCE(fi.face_size, 0) DESC, fi.id ASC;

-- name: GetFaceClusterAssignmentsForScope :many
SELECT
    fcm.face_id,
    fcm.cluster_id,
    fc.cluster_name,
    fc.is_confirmed
FROM face_cluster_members fcm
JOIN face_clusters fc ON fc.cluster_id = fcm.cluster_id
JOIN face_items fi ON fi.id = fcm.face_id
JOIN assets a ON a.asset_id = fi.asset_id
WHERE (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id')::uuid)
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id')::integer);

-- name: GetFaceClusterMembershipsByFaceIDs :many
SELECT face_id, cluster_id, similarity_score, confidence, is_manual
FROM face_cluster_members
WHERE face_id = ANY(sqlc.arg('face_ids')::integer[])
ORDER BY face_id ASC, confidence DESC, similarity_score DESC, id ASC;

-- name: AssignFaceClusterMemberExclusive :one
INSERT INTO face_cluster_members (cluster_id, face_id, similarity_score, confidence, is_manual)
VALUES (sqlc.arg('cluster_id'), sqlc.arg('face_id'), sqlc.arg('similarity_score'), sqlc.arg('confidence'), sqlc.narg('is_manual'))
ON CONFLICT (face_id)
DO UPDATE SET
    cluster_id = EXCLUDED.cluster_id,
    similarity_score = GREATEST(face_cluster_members.similarity_score, EXCLUDED.similarity_score),
    confidence = GREATEST(face_cluster_members.confidence, EXCLUDED.confidence),
    is_manual = COALESCE(face_cluster_members.is_manual, false) OR COALESCE(EXCLUDED.is_manual, false)
RETURNING *;

-- name: DeleteFaceClusterMembersForScope :exec
DELETE FROM face_cluster_members fcm
USING face_items fi, assets a
WHERE fcm.face_id = fi.id
  AND a.asset_id = fi.asset_id
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id')::uuid)
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id')::integer);

-- name: CopyFaceClusterMembersToCluster :exec
UPDATE face_cluster_members
SET cluster_id = sqlc.arg('target_cluster_id')
WHERE cluster_id = sqlc.arg('source_cluster_id');

-- name: DeleteFaceClusterMembersByCluster :exec
DELETE FROM face_cluster_members
WHERE cluster_id = $1;

-- name: DeleteEmptyUnconfirmedFaceClusters :exec
DELETE FROM face_clusters fc
WHERE COALESCE(fc.is_confirmed, false) = false
  AND NOT EXISTS (
      SELECT 1
      FROM face_cluster_members fcm
      WHERE fcm.cluster_id = fc.cluster_id
  );

-- name: DeleteEmptyFaceClusters :exec
DELETE FROM face_clusters fc
WHERE NOT EXISTS (
    SELECT 1
    FROM face_cluster_members fcm
    WHERE fcm.cluster_id = fc.cluster_id
);

-- name: MergeFaceClusters :exec
UPDATE face_cluster_members
SET cluster_id = $1
WHERE cluster_id = $2;

-- name: ListPersonFacesScoped :many
SELECT
    fi.id,
    fi.asset_id,
    fi.confidence,
    fi.is_primary,
    fi.face_image_path,
    COALESCE(fcm.is_manual, false) AS is_manual,
    a.original_filename,
    a.taken_time,
    a.upload_time
FROM face_cluster_members fcm
JOIN face_items fi ON fi.id = fcm.face_id
JOIN assets a ON a.asset_id = fi.asset_id
WHERE fcm.cluster_id = sqlc.arg('cluster_id')
  AND COALESCE(a.is_deleted, false) = false
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
ORDER BY COALESCE(fi.is_primary, false) DESC, fi.confidence DESC, COALESCE(fi.face_size, 0) DESC, fi.id ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountPersonFacesScoped :one
SELECT COUNT(*)::bigint
FROM face_cluster_members fcm
JOIN face_items fi ON fi.id = fcm.face_id
JOIN assets a ON a.asset_id = fi.asset_id
WHERE fcm.cluster_id = sqlc.arg('cluster_id')
  AND COALESCE(a.is_deleted, false) = false
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'));

-- name: GetPersonFaceScoped :one
SELECT
    fi.id,
    fi.asset_id,
    fi.confidence,
    fi.is_primary,
    fi.face_image_path,
    a.repository_id,
    a.owner_id
FROM face_cluster_members fcm
JOIN face_items fi ON fi.id = fcm.face_id
JOIN assets a ON a.asset_id = fi.asset_id
WHERE fcm.cluster_id = sqlc.arg('cluster_id')
  AND fi.id = sqlc.arg('face_id')
  AND COALESCE(a.is_deleted, false) = false
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'));

-- name: GetFaceForCorrectionScoped :one
SELECT
    fi.id,
    fi.asset_id,
    fi.confidence,
    fi.face_image_path,
    a.repository_id,
    a.owner_id
FROM face_items fi
JOIN assets a ON a.asset_id = fi.asset_id
WHERE fi.id = sqlc.arg('face_id')
  AND COALESCE(a.is_deleted, false) = false
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'));

-- name: GetManualFaceClusterMembershipsForScope :many
SELECT
    fcm.face_id,
    fcm.cluster_id,
    fcm.similarity_score,
    fcm.confidence
FROM face_cluster_members fcm
JOIN face_items fi ON fi.id = fcm.face_id
JOIN assets a ON a.asset_id = fi.asset_id
WHERE COALESCE(fcm.is_manual, false) = true
  AND COALESCE(a.is_deleted, false) = false
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id')::uuid)
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id')::integer);

-- name: MoveClusterMembersToClusterManual :exec
UPDATE face_cluster_members
SET cluster_id = sqlc.arg('target_cluster_id'),
    is_manual = true
WHERE cluster_id = sqlc.arg('source_cluster_id');

-- name: GetClusterMergeCandidates :many
WITH pair_scores AS (
    SELECT
        fc1.cluster_id,
        fc1.cluster_name AS name1,
        fc2.cluster_id AS other_cluster_id,
        fc2.cluster_name AS name2,
        AVG(1 - (fi1.embedding <=> fi2.embedding))::double precision AS avg_similarity
    FROM face_clusters fc1
    JOIN face_cluster_members fcm1 ON fcm1.cluster_id = fc1.cluster_id
    JOIN face_items fi1 ON fi1.id = fcm1.face_id
    JOIN face_clusters fc2 ON fc1.cluster_id < fc2.cluster_id
    JOIN face_cluster_members fcm2 ON fcm2.cluster_id = fc2.cluster_id
    JOIN face_items fi2 ON fi2.id = fcm2.face_id
    WHERE fi1.embedding IS NOT NULL
      AND fi2.embedding IS NOT NULL
      AND COALESCE(fc1.is_confirmed, false) = true
      AND COALESCE(fc2.is_confirmed, false) = true
    GROUP BY fc1.cluster_id, fc1.cluster_name, fc2.cluster_id, fc2.cluster_name
)
SELECT cluster_id, name1, other_cluster_id, name2, avg_similarity
FROM pair_scores
WHERE avg_similarity >= sqlc.arg('min_similarity')::float8
ORDER BY avg_similarity DESC
LIMIT sqlc.arg('limit');
