-- name: GetUnclusteredFacesInScope :many
SELECT fi.*
FROM face_items fi
JOIN assets a ON a.asset_id = fi.asset_id
LEFT JOIN face_cluster_members fcm ON fi.id = fcm.face_id
WHERE fcm.face_id IS NULL
  AND COALESCE(a.is_deleted, false) = false
  AND (sqlc.narg('repository_id') IS NULL OR EXISTS (
    SELECT 1 FROM active_asset_occurrences occurrence
    WHERE occurrence.asset_id = a.asset_id
      AND occurrence.repository_id = sqlc.narg('repository_id')
  ))
  AND COALESCE(a.owner_id, -1) = COALESCE(sqlc.narg('owner_id'), -1)
  AND COALESCE(fi.embedding_model, '') = COALESCE(sqlc.narg('embedding_model'), '')
  AND fi.embedding IS NOT NULL
  AND fi.confidence >= sqlc.arg('min_confidence')
  AND COALESCE(fi.face_size, 0) >= sqlc.arg('min_face_size')
ORDER BY fi.confidence DESC, COALESCE(fi.face_size, 0) DESC, fi.id ASC;
