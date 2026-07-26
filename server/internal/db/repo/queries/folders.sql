-- Folder browsing has no dedicated table: "folders" are derived from the
-- repository-relative prefix of assets.storage_path. All queries here treat
-- storage_path as relative (see assets_repository_id_storage_path_key) and
-- must never expose repositories.path (the absolute host path).

-- name: GetFolderChildSummaries :many
-- Lists immediate child folders of parent_path (recursive descendant
-- counts/covers). Excludes internal .lumilio paths, app-managed inbox
-- uploads, and any asset that sits directly in parent_path (files, not
-- folders).
WITH scoped AS (
  SELECT
    a.asset_id,
    a.type,
    a.taken_time,
    a.upload_time,
    a.repository_id,
    CASE
      WHEN sqlc.arg('parent_path') = '' THEN a.storage_path
      ELSE substr(a.storage_path, length(sqlc.arg('parent_path')) + 2)
    END AS remainder
  FROM assets a
  WHERE a.is_deleted = false
    AND (sqlc.narg('owner_id') IS NULL OR a.owner_id = sqlc.narg('owner_id'))
    AND (sqlc.narg('repository_id') IS NULL OR a.repository_id = sqlc.narg('repository_id'))
    AND a.storage_path NOT LIKE '.lumilio/%'
    AND a.storage_path NOT LIKE 'inbox/%'
    AND (
      sqlc.arg('parent_path') = ''
      OR a.storage_path LIKE sqlc.arg('parent_path') || '/%'
    )
),
child_folders AS (
  SELECT
    asset_id,
    type,
    taken_time,
    upload_time,
    repository_id,
    substr(remainder, 1, instr(remainder, '/') - 1) AS child_name
  FROM scoped
  WHERE remainder LIKE '%/%'
),
ranked AS (
  SELECT
    child_folders.*,
    ROW_NUMBER() OVER (
      PARTITION BY repository_id, child_name
      ORDER BY COALESCE(taken_time, upload_time) DESC, asset_id DESC
    ) AS cover_rank
  FROM child_folders
)
SELECT
  repository_id,
  child_name,
  COUNT(*) AS asset_count,
  COUNT(*) FILTER (WHERE type = 'PHOTO') AS photo_count,
  COUNT(*) FILTER (WHERE type = 'VIDEO') AS video_count,
  COUNT(*) FILTER (WHERE type = 'AUDIO') AS audio_count,
  MIN(COALESCE(taken_time, upload_time)) AS date_start,
  MAX(COALESCE(taken_time, upload_time)) AS date_end,
  MAX(CASE WHEN cover_rank = 1 THEN asset_id END) AS cover_asset_id
FROM ranked
GROUP BY repository_id, child_name
ORDER BY child_name ASC;

-- name: GetFolderSummary :one
-- Aggregate stats for one folder path (recursive descendants).
WITH scoped AS (
  SELECT
    a.*,
    ROW_NUMBER() OVER (
      ORDER BY COALESCE(a.taken_time, a.upload_time) DESC, a.asset_id DESC
    ) AS cover_rank
  FROM assets a
  WHERE a.is_deleted = false
    AND (sqlc.narg('owner_id') IS NULL OR a.owner_id = sqlc.narg('owner_id'))
    AND a.repository_id = sqlc.arg('repository_id')
    AND a.storage_path NOT LIKE '.lumilio/%'
    AND a.storage_path NOT LIKE 'inbox/%'
    AND (
      sqlc.arg('folder_path') = ''
      OR a.storage_path LIKE sqlc.arg('folder_path') || '/%'
    )
)
SELECT
  COUNT(*) AS asset_count,
  COUNT(*) FILTER (WHERE type = 'PHOTO') AS photo_count,
  COUNT(*) FILTER (WHERE type = 'VIDEO') AS video_count,
  COUNT(*) FILTER (WHERE type = 'AUDIO') AS audio_count,
  MIN(COALESCE(taken_time, upload_time)) AS date_start,
  MAX(COALESCE(taken_time, upload_time)) AS date_end,
  MAX(CASE WHEN cover_rank = 1 THEN asset_id END) AS cover_asset_id
FROM scoped;
