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
  AND (sqlc.narg('repository_id') IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id') IS NULL OR a.owner_id = sqlc.narg('owner_id'))
ORDER BY COALESCE(fi.is_primary, false) DESC, fi.confidence DESC, COALESCE(fi.face_size, 0) DESC, fi.id ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountPersonFacesScoped :one
SELECT COUNT(*)
FROM face_cluster_members fcm
JOIN face_items fi ON fi.id = fcm.face_id
JOIN assets a ON a.asset_id = fi.asset_id
WHERE fcm.cluster_id = sqlc.arg('cluster_id')
  AND COALESCE(a.is_deleted, false) = false
  AND (sqlc.narg('repository_id') IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id') IS NULL OR a.owner_id = sqlc.narg('owner_id'));

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
  AND (sqlc.narg('repository_id') IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id') IS NULL OR a.owner_id = sqlc.narg('owner_id'));

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
  AND (sqlc.narg('repository_id') IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id') IS NULL OR a.owner_id = sqlc.narg('owner_id'));

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
  AND (sqlc.narg('repository_id') IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('owner_id') IS NULL OR a.owner_id = sqlc.narg('owner_id'));

-- name: MoveClusterMembersToClusterManual :exec
UPDATE face_cluster_members
SET cluster_id = sqlc.arg('target_cluster_id'),
    is_manual = true
WHERE cluster_id = sqlc.arg('source_cluster_id');

-- name: GetClusterMergeCandidates :many
-- Merge suggestions never pair clusters of different owners; owner_id
-- optionally restricts candidates to one owner's clusters (nil = admin).
WITH pair_scores AS (
    SELECT
        fc1.cluster_id,
        fc1.cluster_name AS name1,
        fc2.cluster_id AS other_cluster_id,
        fc2.cluster_name AS name2,
        AVG(1 - vec1_cos_distance(fi1.embedding, fi2.embedding)) AS avg_similarity
    FROM face_clusters fc1
    JOIN face_cluster_members fcm1 ON fcm1.cluster_id = fc1.cluster_id
    JOIN face_items fi1 ON fi1.id = fcm1.face_id
    JOIN face_clusters fc2 ON fc1.cluster_id < fc2.cluster_id
        AND (
          fc1.owner_id = fc2.owner_id
          OR (fc1.owner_id IS NULL AND fc2.owner_id IS NULL)
        )
    JOIN face_cluster_members fcm2 ON fcm2.cluster_id = fc2.cluster_id
    JOIN face_items fi2 ON fi2.id = fcm2.face_id
    WHERE fi1.embedding IS NOT NULL
      AND fi2.embedding IS NOT NULL
      AND COALESCE(fc1.is_confirmed, false) = true
      AND COALESCE(fc2.is_confirmed, false) = true
      AND (sqlc.narg('owner_id') IS NULL OR fc1.owner_id = sqlc.narg('owner_id'))
    GROUP BY fc1.cluster_id, fc1.cluster_name, fc2.cluster_id, fc2.cluster_name
    HAVING AVG(1 - vec1_cos_distance(fi1.embedding, fi2.embedding)) >= sqlc.arg('min_similarity')
)
SELECT cluster_id, name1, other_cluster_id, name2, avg_similarity
FROM pair_scores
ORDER BY 5 DESC
LIMIT sqlc.arg('limit');
