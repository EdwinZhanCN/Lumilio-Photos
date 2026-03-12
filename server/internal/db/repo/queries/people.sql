-- name: CountPeopleScoped :one
SELECT COUNT(*)
FROM face_clusters fc
WHERE EXISTS (
    SELECT 1
    FROM face_cluster_members fcm
    JOIN face_items fi ON fi.id = fcm.face_id
    JOIN assets a ON a.asset_id = fi.asset_id
    WHERE fcm.cluster_id = fc.cluster_id
      AND a.is_deleted = false
      AND (
        sqlc.narg('repository_id')::uuid IS NULL
        OR a.repository_id = sqlc.narg('repository_id')
      )
      AND (
        sqlc.narg('owner_id')::integer IS NULL
        OR a.owner_id = sqlc.narg('owner_id')
      )
);

-- name: ListPeopleScoped :many
WITH page_people AS MATERIALIZED (
    SELECT
        fc.cluster_id,
        COUNT(DISTINCT fcm.face_id)::bigint AS member_count,
        COUNT(DISTINCT fi.asset_id)::bigint AS asset_count
    FROM face_clusters fc
    JOIN face_cluster_members fcm ON fcm.cluster_id = fc.cluster_id
    JOIN face_items fi ON fi.id = fcm.face_id
    JOIN assets a ON a.asset_id = fi.asset_id
    WHERE a.is_deleted = false
      AND (
        sqlc.narg('repository_id')::uuid IS NULL
        OR a.repository_id = sqlc.narg('repository_id')
      )
      AND (
        sqlc.narg('owner_id')::integer IS NULL
        OR a.owner_id = sqlc.narg('owner_id')
      )
    GROUP BY fc.cluster_id, fc.is_confirmed, fc.updated_at
    ORDER BY
        COALESCE(fc.is_confirmed, false) DESC,
        COUNT(DISTINCT fcm.face_id) DESC,
        fc.updated_at DESC,
        fc.cluster_id DESC
    LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset')
)
SELECT
    fc.cluster_id,
    fc.cluster_name,
    fc.is_confirmed,
    pp.member_count,
    pp.asset_count,
    COALESCE(rep.face_image_path, best.face_image_path) AS cover_face_image_path,
    COALESCE(rep.asset_id, best.asset_id)::uuid AS representative_asset_id,
    fc.created_at,
    fc.updated_at
FROM page_people pp
JOIN face_clusters fc ON fc.cluster_id = pp.cluster_id
LEFT JOIN LATERAL (
    SELECT fi.face_image_path, fi.asset_id
    FROM face_items fi
    JOIN assets a ON a.asset_id = fi.asset_id
    WHERE fi.id = fc.representative_face_id
      AND a.is_deleted = false
      AND (
        sqlc.narg('repository_id')::uuid IS NULL
        OR a.repository_id = sqlc.narg('repository_id')
      )
      AND (
        sqlc.narg('owner_id')::integer IS NULL
        OR a.owner_id = sqlc.narg('owner_id')
      )
    LIMIT 1
) rep ON true
LEFT JOIN LATERAL (
    SELECT fi.face_image_path, fi.asset_id
    FROM face_cluster_members fcm
    JOIN face_items fi ON fi.id = fcm.face_id
    JOIN assets a ON a.asset_id = fi.asset_id
    WHERE fcm.cluster_id = fc.cluster_id
      AND a.is_deleted = false
      AND (
        sqlc.narg('repository_id')::uuid IS NULL
        OR a.repository_id = sqlc.narg('repository_id')
      )
      AND (
        sqlc.narg('owner_id')::integer IS NULL
        OR a.owner_id = sqlc.narg('owner_id')
      )
    ORDER BY COALESCE(fi.is_primary, false) DESC, fi.confidence DESC, COALESCE(fi.face_size, 0) DESC, fi.id ASC
    LIMIT 1
) best ON true
ORDER BY
    COALESCE(fc.is_confirmed, false) DESC,
    pp.member_count DESC,
    fc.updated_at DESC,
    fc.cluster_id DESC;

-- name: GetPersonByIDScoped :one
SELECT
    fc.cluster_id,
    fc.cluster_name,
    fc.is_confirmed,
    scoped.member_count,
    scoped.asset_count,
    COALESCE(rep.face_image_path, best.face_image_path) AS cover_face_image_path,
    COALESCE(rep.asset_id, best.asset_id)::uuid AS representative_asset_id,
    fc.created_at,
    fc.updated_at
FROM face_clusters fc
JOIN LATERAL (
    SELECT
        COUNT(DISTINCT fcm.face_id)::bigint AS member_count,
        COUNT(DISTINCT fi.asset_id)::bigint AS asset_count
    FROM face_cluster_members fcm
    JOIN face_items fi ON fi.id = fcm.face_id
    JOIN assets a ON a.asset_id = fi.asset_id
    WHERE fcm.cluster_id = fc.cluster_id
      AND a.is_deleted = false
      AND (
        sqlc.narg('repository_id')::uuid IS NULL
        OR a.repository_id = sqlc.narg('repository_id')
      )
      AND (
        sqlc.narg('owner_id')::integer IS NULL
        OR a.owner_id = sqlc.narg('owner_id')
      )
) scoped ON scoped.member_count > 0
LEFT JOIN LATERAL (
    SELECT fi.face_image_path, fi.asset_id
    FROM face_items fi
    JOIN assets a ON a.asset_id = fi.asset_id
    WHERE fi.id = fc.representative_face_id
      AND a.is_deleted = false
      AND (
        sqlc.narg('repository_id')::uuid IS NULL
        OR a.repository_id = sqlc.narg('repository_id')
      )
      AND (
        sqlc.narg('owner_id')::integer IS NULL
        OR a.owner_id = sqlc.narg('owner_id')
      )
    LIMIT 1
) rep ON true
LEFT JOIN LATERAL (
    SELECT fi.face_image_path, fi.asset_id
    FROM face_cluster_members fcm
    JOIN face_items fi ON fi.id = fcm.face_id
    JOIN assets a ON a.asset_id = fi.asset_id
    WHERE fcm.cluster_id = fc.cluster_id
      AND a.is_deleted = false
      AND (
        sqlc.narg('repository_id')::uuid IS NULL
        OR a.repository_id = sqlc.narg('repository_id')
      )
      AND (
        sqlc.narg('owner_id')::integer IS NULL
        OR a.owner_id = sqlc.narg('owner_id')
      )
    ORDER BY COALESCE(fi.is_primary, false) DESC, fi.confidence DESC, COALESCE(fi.face_size, 0) DESC, fi.id ASC
    LIMIT 1
) best ON true
WHERE fc.cluster_id = sqlc.arg('cluster_id');

-- name: RenameFaceCluster :one
UPDATE face_clusters
SET
    cluster_name = sqlc.arg('cluster_name'),
    is_confirmed = true,
    updated_at = CURRENT_TIMESTAMP
WHERE cluster_id = sqlc.arg('cluster_id')
RETURNING *;

-- name: UpdateFaceClusterRepresentative :one
UPDATE face_clusters
SET
    representative_face_id = sqlc.narg('representative_face_id'),
    confidence_score = sqlc.narg('confidence_score'),
    updated_at = CURRENT_TIMESTAMP
WHERE cluster_id = sqlc.arg('cluster_id')
RETURNING *;
