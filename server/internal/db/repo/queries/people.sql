-- name: CountPeopleScoped :one
SELECT COUNT(*)
FROM face_clusters fc
WHERE (sqlc.arg('include_hidden') OR COALESCE(fc.is_hidden, false) = false)
  AND (
    sqlc.narg('owner_id') IS NULL
    OR fc.owner_id = sqlc.narg('owner_id')
  )
  AND EXISTS (
    SELECT 1
    FROM face_cluster_members fcm
    JOIN face_items fi ON fi.id = fcm.face_id
    JOIN assets a ON a.asset_id = fi.asset_id
    WHERE fcm.cluster_id = fc.cluster_id
      AND a.is_deleted = false
      AND (
        sqlc.narg('owner_id') IS NULL
        OR a.owner_id = sqlc.narg('owner_id')
      )
      AND (
        sqlc.narg('repository_id') IS NULL
        OR EXISTS (
          SELECT 1 FROM active_asset_occurrences occurrence
          WHERE occurrence.asset_id = a.asset_id
            AND occurrence.repository_id = sqlc.narg('repository_id')
        )
      )
);

-- name: ListPeopleScoped :many
WITH page_people AS (
    SELECT
        fc.cluster_id,
        COUNT(DISTINCT fcm.face_id) AS member_count,
        COUNT(DISTINCT fi.asset_id) AS asset_count
    FROM face_clusters fc
    JOIN face_cluster_members fcm ON fcm.cluster_id = fc.cluster_id
    JOIN face_items fi ON fi.id = fcm.face_id
    JOIN assets a ON a.asset_id = fi.asset_id
    WHERE a.is_deleted = false
      AND (sqlc.arg('include_hidden') OR COALESCE(fc.is_hidden, false) = false)
      AND (
        sqlc.narg('owner_id') IS NULL
        OR fc.owner_id = sqlc.narg('owner_id')
      )
      AND (
        sqlc.narg('repository_id') IS NULL
        OR EXISTS (
          SELECT 1 FROM active_asset_occurrences occurrence
          WHERE occurrence.asset_id = a.asset_id
            AND occurrence.repository_id = sqlc.narg('repository_id')
        )
      )
    GROUP BY fc.cluster_id, fc.is_confirmed, fc.updated_at
    ORDER BY
        COALESCE(fc.is_confirmed, false) DESC,
        COUNT(DISTINCT fcm.face_id) DESC,
        fc.updated_at DESC,
        fc.cluster_id DESC
    LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset')
),
representative_faces AS (
    SELECT fi.id, fi.face_image_path, fi.asset_id, fi.repository_id
    FROM face_items fi
    JOIN assets a ON a.asset_id = fi.asset_id
    WHERE a.is_deleted = false
      AND (sqlc.narg('owner_id') IS NULL OR a.owner_id = sqlc.narg('owner_id'))
      AND (sqlc.narg('repository_id') IS NULL OR EXISTS (
        SELECT 1 FROM active_asset_occurrences occurrence
        WHERE occurrence.asset_id = a.asset_id
          AND occurrence.repository_id = sqlc.narg('repository_id')
      ))
),
ranked_faces AS (
    SELECT
        fcm.cluster_id,
        fi.face_image_path,
        fi.asset_id,
        fi.repository_id,
        ROW_NUMBER() OVER (
            PARTITION BY fcm.cluster_id
            ORDER BY COALESCE(fi.is_primary, false) DESC,
                     fi.confidence DESC,
                     COALESCE(fi.face_size, 0) DESC,
                     fi.id
        ) AS face_rank
    FROM face_cluster_members fcm
    JOIN face_items fi ON fi.id = fcm.face_id
    JOIN assets a ON a.asset_id = fi.asset_id
    WHERE a.is_deleted = false
      AND (sqlc.narg('owner_id') IS NULL OR a.owner_id = sqlc.narg('owner_id'))
      AND (sqlc.narg('repository_id') IS NULL OR EXISTS (
        SELECT 1 FROM active_asset_occurrences occurrence
        WHERE occurrence.asset_id = a.asset_id
          AND occurrence.repository_id = sqlc.narg('repository_id')
      ))
)
SELECT
    fc.cluster_id,
    fc.cluster_name,
    fc.is_confirmed,
    COALESCE(fc.is_hidden, false) AS is_hidden,
    fc.hidden_at,
    pp.member_count,
    pp.asset_count,
    COALESCE(rep.face_image_path, best.face_image_path) AS cover_face_image_path,
    COALESCE(rep.asset_id, best.asset_id) AS representative_asset_id,
    COALESCE(rep.repository_id, best.repository_id) AS cover_repository_id,
    fc.created_at,
    fc.updated_at
FROM page_people pp
JOIN face_clusters fc ON fc.cluster_id = pp.cluster_id
LEFT JOIN representative_faces rep ON rep.id = fc.representative_face_id
LEFT JOIN ranked_faces best ON best.cluster_id = fc.cluster_id AND best.face_rank = 1
ORDER BY
    COALESCE(fc.is_confirmed, false) DESC,
    pp.member_count DESC,
    fc.updated_at DESC,
    fc.cluster_id DESC;

-- name: GetPersonByIDScoped :one
WITH scoped AS (
    SELECT
        fcm.cluster_id,
        COUNT(DISTINCT fcm.face_id) AS member_count,
        COUNT(DISTINCT fi.asset_id) AS asset_count
    FROM face_cluster_members fcm
    JOIN face_items fi ON fi.id = fcm.face_id
    JOIN assets a ON a.asset_id = fi.asset_id
    WHERE a.is_deleted = false
      AND (sqlc.narg('repository_id') IS NULL OR EXISTS (
        SELECT 1 FROM active_asset_occurrences occurrence
        WHERE occurrence.asset_id = a.asset_id
          AND occurrence.repository_id = sqlc.narg('repository_id')
      ))
    GROUP BY fcm.cluster_id
),
representative_faces AS (
    SELECT fi.id, fi.face_image_path, fi.asset_id, fi.repository_id
    FROM face_items fi
    JOIN assets a ON a.asset_id = fi.asset_id
    WHERE a.is_deleted = false
      AND (sqlc.narg('repository_id') IS NULL OR EXISTS (
        SELECT 1 FROM active_asset_occurrences occurrence
        WHERE occurrence.asset_id = a.asset_id
          AND occurrence.repository_id = sqlc.narg('repository_id')
      ))
),
ranked_faces AS (
    SELECT
        fcm.cluster_id,
        fi.face_image_path,
        fi.asset_id,
        fi.repository_id,
        ROW_NUMBER() OVER (
            PARTITION BY fcm.cluster_id
            ORDER BY COALESCE(fi.is_primary, false) DESC,
                     fi.confidence DESC,
                     COALESCE(fi.face_size, 0) DESC,
                     fi.id
        ) AS face_rank
    FROM face_cluster_members fcm
    JOIN face_items fi ON fi.id = fcm.face_id
    JOIN assets a ON a.asset_id = fi.asset_id
    WHERE a.is_deleted = false
      AND (sqlc.narg('repository_id') IS NULL OR EXISTS (
        SELECT 1 FROM active_asset_occurrences occurrence
        WHERE occurrence.asset_id = a.asset_id
          AND occurrence.repository_id = sqlc.narg('repository_id')
      ))
)
SELECT
    fc.cluster_id,
    fc.cluster_name,
    fc.is_confirmed,
    COALESCE(fc.is_hidden, false) AS is_hidden,
    fc.hidden_at,
    scoped.member_count,
    scoped.asset_count,
    COALESCE(rep.face_image_path, best.face_image_path) AS cover_face_image_path,
    COALESCE(rep.asset_id, best.asset_id) AS representative_asset_id,
    COALESCE(rep.repository_id, best.repository_id) AS cover_repository_id,
    fc.created_at,
    fc.updated_at
FROM face_clusters fc
JOIN scoped ON scoped.cluster_id = fc.cluster_id AND scoped.member_count > 0
LEFT JOIN representative_faces rep ON rep.id = fc.representative_face_id
LEFT JOIN ranked_faces best ON best.cluster_id = fc.cluster_id AND best.face_rank = 1
WHERE fc.cluster_id = sqlc.arg('cluster_id')
  AND (
    sqlc.narg('owner_id') IS NULL
    OR fc.owner_id = sqlc.narg('owner_id')
  );

-- name: RenameFaceCluster :one
UPDATE face_clusters
SET
    cluster_name = sqlc.arg('cluster_name'),
    is_confirmed = true,
    updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE cluster_id = sqlc.arg('cluster_id')
RETURNING *;

-- name: UpdateFaceClusterRepresentative :one
UPDATE face_clusters
SET
    representative_face_id = sqlc.narg('representative_face_id'),
    confidence_score = sqlc.narg('confidence_score'),
    updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE cluster_id = sqlc.arg('cluster_id')
RETURNING *;

-- name: SetFaceClusterHidden :one
UPDATE face_clusters
SET
    is_hidden = sqlc.arg('is_hidden'),
    hidden_at = CASE
        WHEN sqlc.arg('is_hidden') THEN CAST(unixepoch('subsec') * 1000000 AS INTEGER)
        ELSE NULL
    END,
    updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE cluster_id = sqlc.arg('cluster_id')
RETURNING *;
