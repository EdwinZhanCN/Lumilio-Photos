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
WHERE (sqlc.narg('repository_id') IS NULL OR EXISTS (
    SELECT 1 FROM active_asset_occurrences occurrence
    WHERE occurrence.asset_id = a.asset_id
      AND occurrence.repository_id = sqlc.narg('repository_id')
  ))
  AND (sqlc.narg('owner_id') IS NULL OR a.owner_id = sqlc.narg('owner_id'));

-- name: GetFaceClusterMembershipsByFaceIDs :many
SELECT face_id, cluster_id, similarity_score, confidence, is_manual
FROM face_cluster_members
WHERE face_id IN (sqlc.slice('face_ids'))
ORDER BY face_id ASC, confidence DESC, similarity_score DESC, id ASC;

-- name: AssignFaceClusterMemberExclusive :one
INSERT INTO face_cluster_members (
    cluster_id, face_id, similarity_score, confidence, is_manual, created_at
)
VALUES (
    sqlc.arg('cluster_id'),
    sqlc.arg('face_id'),
    sqlc.arg('similarity_score'),
    sqlc.arg('confidence'),
    sqlc.narg('is_manual'),
    CAST(unixepoch('subsec') * 1000000 AS INTEGER)
)
ON CONFLICT (face_id)
DO UPDATE SET
    cluster_id = EXCLUDED.cluster_id,
    similarity_score = max(face_cluster_members.similarity_score, EXCLUDED.similarity_score),
    confidence = max(face_cluster_members.confidence, EXCLUDED.confidence),
    is_manual = COALESCE(face_cluster_members.is_manual, false) OR COALESCE(EXCLUDED.is_manual, false)
RETURNING *;

-- name: DeleteFaceClusterMembersForScope :exec
DELETE FROM face_cluster_members
WHERE face_id IN (
    SELECT fi.id
    FROM face_items fi
    JOIN assets a ON a.asset_id = fi.asset_id
    WHERE (sqlc.narg('repository_id') IS NULL OR EXISTS (
      SELECT 1 FROM active_asset_occurrences occurrence
      WHERE occurrence.asset_id = a.asset_id
        AND occurrence.repository_id = sqlc.narg('repository_id')
    ))
      AND (sqlc.narg('owner_id') IS NULL OR a.owner_id = sqlc.narg('owner_id'))
);

-- name: CopyFaceClusterMembersToCluster :exec
UPDATE face_cluster_members
SET cluster_id = sqlc.arg('target_cluster_id')
WHERE cluster_id = sqlc.arg('source_cluster_id');

-- name: DeleteFaceClusterMembersByCluster :exec
DELETE FROM face_cluster_members
WHERE cluster_id = ?1;

-- name: DeleteEmptyUnconfirmedFaceClusters :exec
DELETE FROM face_clusters
WHERE COALESCE(face_clusters.is_confirmed, false) = false
  AND NOT EXISTS (
      SELECT 1
      FROM face_cluster_members fcm
      WHERE fcm.cluster_id = face_clusters.cluster_id
  );

-- name: DeleteEmptyFaceClusters :exec
DELETE FROM face_clusters
WHERE NOT EXISTS (
    SELECT 1
    FROM face_cluster_members fcm
    WHERE fcm.cluster_id = face_clusters.cluster_id
);

-- name: MergeFaceClusters :exec
UPDATE face_cluster_members
SET cluster_id = ?1
WHERE cluster_id = ?2;
