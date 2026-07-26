-- name: GetNearestAssignedFaceCluster :one
-- Cluster attachment is owner-scoped only: a face may join a cluster whose
-- members live in a different repository, same owner.
SELECT
    fcm.cluster_id,
    fi.id AS face_id,
    CAST(1.0 - vec_distance_cosine(fi.embedding, sqlc.arg('embedding_query')) AS REAL) AS similarity
FROM face_items fi
JOIN assets a ON a.asset_id = fi.asset_id
JOIN face_cluster_members fcm ON fcm.face_id = fi.id
WHERE fi.id != sqlc.arg('id')
  AND COALESCE(a.is_deleted, false) = false
  AND COALESCE(a.owner_id, -1) = COALESCE(sqlc.narg('owner_id'), -1)
  AND COALESCE(fi.embedding_model, '') = COALESCE(sqlc.narg('embedding_model'), '')
  AND fi.embedding IS NOT NULL
  AND fi.confidence >= sqlc.arg('min_confidence')
  AND COALESCE(fi.face_size, 0) >= sqlc.arg('min_face_size')
  AND 1.0 - vec_distance_cosine(fi.embedding, sqlc.arg('embedding_query'))
      >= CAST(sqlc.arg('min_similarity') AS REAL)
ORDER BY similarity DESC, fi.confidence DESC, COALESCE(fi.face_size, 0) DESC, fi.id ASC
LIMIT 1;
