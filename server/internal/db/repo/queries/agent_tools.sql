-- Queries backing the Phase 2 agent tools (producers, transformers,
-- observers). All ANY(asset_ids) queries operate on ref snapshots.

-- name: GetAssetIDsByPersonIDs :many
-- search_people producer: assets containing at least one of the given people
-- (union semantics; the agent intersects refs for "both people" requests).
SELECT a.asset_id
FROM assets a
WHERE a.is_deleted = false
  AND EXISTS (
    SELECT 1
    FROM face_items fi
    JOIN face_cluster_members fcm ON fcm.face_id = fi.id
    WHERE fi.asset_id = a.asset_id
      AND fcm.cluster_id = ANY(sqlc.arg('person_ids')::int[])
  )
ORDER BY COALESCE(a.taken_time, a.upload_time) DESC, a.asset_id DESC
LIMIT sqlc.arg('limit');

-- name: AgentLookupPeople :many
-- lookup_people entity resolver: named face clusters matching a name query.
SELECT
    fc.cluster_id,
    fc.cluster_name::text AS name,
    COUNT(DISTINCT fi.asset_id) AS asset_count
FROM face_clusters fc
JOIN face_cluster_members fcm ON fcm.cluster_id = fc.cluster_id
JOIN face_items fi ON fi.id = fcm.face_id
WHERE fc.cluster_name IS NOT NULL
  AND fc.cluster_name <> ''
  AND (sqlc.narg('name_query')::text IS NULL OR fc.cluster_name ILIKE '%' || sqlc.narg('name_query') || '%')
GROUP BY fc.cluster_id, fc.cluster_name
ORDER BY asset_count DESC
LIMIT sqlc.arg('limit');

-- name: RankAssetIDsByTime :many
-- rank(by=time) ascending; callers reverse for descending order.
SELECT asset_id
FROM assets
WHERE asset_id = ANY(sqlc.arg('asset_ids')::uuid[])
  AND is_deleted = false
ORDER BY COALESCE(taken_time, upload_time) ASC, asset_id ASC;

-- name: RankAssetIDsByQuality :many
-- rank(by=quality) ascending, using the featured-selector heuristic
-- (rating, liked, resolution); callers reverse for descending order.
SELECT asset_id
FROM assets a
WHERE asset_id = ANY(sqlc.arg('asset_ids')::uuid[])
  AND is_deleted = false
ORDER BY (
    0.45 * COALESCE(a.rating, 0)::float8 / 5.0
  + 0.20 * (CASE WHEN a.liked THEN 1.0 ELSE 0.0 END)
  + 0.35 * LEAST(COALESCE(a.width, 0)::float8 * COALESCE(a.height, 0)::float8 / 24000000.0, 1.0)
) ASC, asset_id ASC;

-- name: AgentPeekAssets :many
-- peek observer: minimal per-asset fields; snapshot order restored in Go.
SELECT asset_id, original_filename, type,
       COALESCE(taken_time, upload_time)::timestamptz AS captured_at,
       rating, liked
FROM assets
WHERE asset_id = ANY(sqlc.arg('asset_ids')::uuid[])
  AND is_deleted = false;

-- name: RankAssetIDsByUploadTime :many
-- "recently added" presentation order, ascending; callers reverse for newest first.
SELECT asset_id
FROM assets
WHERE asset_id = ANY(sqlc.arg('asset_ids')::uuid[])
  AND is_deleted = false
ORDER BY upload_time ASC, asset_id ASC;
