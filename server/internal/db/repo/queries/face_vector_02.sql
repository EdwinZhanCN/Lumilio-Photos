-- name: CountIncrementalFaceNeighbors :one
-- DBSCAN core check runs over the whole owner scope: clusters span
-- repositories, so neighbors are never repository-filtered.
SELECT COUNT(*)
FROM face_items fi
JOIN assets a ON a.asset_id = fi.asset_id
WHERE fi.id != sqlc.arg('id')
  AND COALESCE(a.is_deleted, false) = false
  AND COALESCE(a.owner_id, -1) = COALESCE(sqlc.narg('owner_id'), -1)
  AND COALESCE(fi.embedding_model, '') = COALESCE(sqlc.narg('embedding_model'), '')
  AND fi.embedding IS NOT NULL
  AND fi.confidence >= sqlc.arg('min_confidence')
  AND COALESCE(fi.face_size, 0) >= sqlc.arg('min_face_size')
  AND 1.0 - vec1_cos_distance(fi.embedding, sqlc.arg('embedding_query'))
      >= CAST(sqlc.arg('min_similarity') AS REAL);
