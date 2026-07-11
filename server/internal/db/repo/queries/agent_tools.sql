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
-- rank(by=quality) ascending, using the aesthetic score from the SigLIP MLP
-- head when available, falling back to the legacy heuristic (rating, liked,
-- resolution) for unscored assets. Callers reverse for descending order.
SELECT a.asset_id
FROM assets a
LEFT JOIN asset_quality_scores aqs ON aqs.asset_id = a.asset_id
WHERE a.asset_id = ANY(sqlc.arg('asset_ids')::uuid[])
  AND a.is_deleted = false
ORDER BY COALESCE(
    aqs.score,
    1.0 + 0.45 * COALESCE(a.rating, 0)::float8 / 5.0
        + 0.20 * (CASE WHEN a.liked THEN 1.0 ELSE 0.0 END)
        + 0.35 * LEAST(COALESCE(a.width, 0)::float8 * COALESCE(a.height, 0)::float8 / 24000000.0, 1.0)
) ASC, a.asset_id ASC;

-- name: AgentLookupAlbums :many
-- lookup_albums entity resolver: albums matching a title query.
SELECT
    al.album_id,
    al.album_name::text AS title,
    COUNT(DISTINCT aa.asset_id) AS asset_count
FROM albums al
LEFT JOIN album_assets aa ON aa.album_id = al.album_id
LEFT JOIN assets a ON a.asset_id = aa.asset_id AND a.is_deleted = false
WHERE al.user_id = sqlc.arg('user_id')
  AND (sqlc.narg('title_query')::text IS NULL OR al.album_name ILIKE '%' || sqlc.narg('title_query') || '%')
GROUP BY al.album_id, al.album_name
ORDER BY asset_count DESC
LIMIT sqlc.arg('limit');

-- name: AgentInspectAssets :many
-- inspect observer: per-asset EXIF facets for small refs.
SELECT asset_id, type, specific_metadata
FROM assets
WHERE asset_id = ANY(sqlc.arg('asset_ids')::uuid[])
  AND is_deleted = false;

-- name: AgentPeekAssets :many
-- peek observer: minimal per-asset fields plus place + people; snapshot order
-- restored in Go. place/people are correlated subqueries so each asset stays a
-- single row (no fan-out from the cluster joins).
SELECT
    a.asset_id,
    a.original_filename,
    a.type,
    COALESCE(a.taken_time, a.upload_time)::timestamptz AS captured_at,
    a.rating,
    a.liked,
    COALESCE((
        SELECT COALESCE(lc.label, lc.city, lc.region, lc.country)
        FROM location_cluster_assets lca
        JOIN location_clusters lc ON lc.cluster_id = lca.cluster_id
        WHERE lca.asset_id = a.asset_id
          AND COALESCE(lc.label, lc.city, lc.region, lc.country) IS NOT NULL
        LIMIT 1
    ), '')::text AS place,
    (
        SELECT array_agg(DISTINCT fc.cluster_name)
        FROM face_items fi
        JOIN face_cluster_members fcm ON fcm.face_id = fi.id
        JOIN face_clusters fc ON fc.cluster_id = fcm.cluster_id
        WHERE fi.asset_id = a.asset_id
          AND fc.cluster_name IS NOT NULL
          AND fc.cluster_name <> ''
    )::text[] AS people
FROM assets a
WHERE a.asset_id = ANY(sqlc.arg('asset_ids')::uuid[])
  AND a.is_deleted = false;

-- name: AgentCapturedTimes :many
-- Capture times for a set of assets, for the sample tool's distribution
-- summary. Order is irrelevant; bucketing happens in Go.
SELECT COALESCE(taken_time, upload_time)::timestamptz AS captured_at
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

-- name: AgentAssetAestheticScores :many
-- Per-asset SigLIP aesthetic scores for a ref snapshot. Unscored assets are
-- omitted; callers that filter by quality percentile drop them.
SELECT asset_id, score
FROM asset_quality_scores
WHERE asset_id = ANY(sqlc.arg('asset_ids')::uuid[]);
