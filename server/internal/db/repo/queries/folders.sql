-- Folder collection views are projections of present repository entries.
-- Repository-relative paths belong to entries and are never stored on Asset.

-- name: GetFolderChildSummaries :many
-- Lists immediate child folders of parent_path and computes recursive
-- descendant counts/covers from active Locations.
WITH scoped AS (
  SELECT
    asset.asset_id,
    asset.type,
    asset.taken_time,
    asset.upload_time,
    node_path.repository_id,
    CASE
      WHEN sqlc.arg('parent_path') = '' THEN node_path.relative_path
      ELSE substr(node_path.relative_path, length(sqlc.arg('parent_path')) + 2)
    END AS remainder
  FROM assets asset
  JOIN active_asset_occurrence_paths node_path
    ON node_path.asset_id = asset.asset_id
  WHERE asset.lifecycle_state = 'active'
    AND (sqlc.narg('owner_id') IS NULL OR asset.owner_id = sqlc.narg('owner_id'))
    AND (sqlc.narg('repository_id') IS NULL OR node_path.repository_id = sqlc.narg('repository_id'))
    AND node_path.relative_path NOT LIKE '.lumilio/%'
    AND node_path.relative_path NOT LIKE 'inbox/%'
    AND (
      sqlc.arg('parent_path') = ''
      OR node_path.relative_path LIKE sqlc.arg('parent_path') || '/%'
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
  COUNT(DISTINCT asset_id) AS asset_count,
  COUNT(DISTINCT asset_id) FILTER (WHERE type = 'PHOTO') AS photo_count,
  COUNT(DISTINCT asset_id) FILTER (WHERE type = 'VIDEO') AS video_count,
  COUNT(DISTINCT asset_id) FILTER (WHERE type = 'AUDIO') AS audio_count,
  MIN(COALESCE(taken_time, upload_time)) AS date_start,
  MAX(COALESCE(taken_time, upload_time)) AS date_end,
  MAX(CASE WHEN cover_rank = 1 THEN asset_id END) AS cover_asset_id
FROM ranked
GROUP BY repository_id, child_name
ORDER BY child_name ASC;

-- name: GetFolderSummary :one
-- Aggregate stats for one graph-derived folder path and all descendants.
WITH scoped AS (
  SELECT
    asset.asset_id,
    asset.type,
    asset.taken_time,
    asset.upload_time,
    ROW_NUMBER() OVER (
      ORDER BY COALESCE(asset.taken_time, asset.upload_time) DESC, asset.asset_id DESC
    ) AS cover_rank
  FROM assets asset
  JOIN active_asset_occurrence_paths node_path
    ON node_path.asset_id = asset.asset_id
   AND node_path.repository_id = sqlc.arg('repository_id')
  WHERE asset.lifecycle_state = 'active'
    AND (sqlc.narg('owner_id') IS NULL OR asset.owner_id = sqlc.narg('owner_id'))
    AND node_path.relative_path NOT LIKE '.lumilio/%'
    AND node_path.relative_path NOT LIKE 'inbox/%'
    AND (
      sqlc.arg('folder_path') = ''
      OR node_path.relative_path LIKE sqlc.arg('folder_path') || '/%'
    )
)
SELECT
  COUNT(DISTINCT asset_id) AS asset_count,
  COUNT(DISTINCT asset_id) FILTER (WHERE type = 'PHOTO') AS photo_count,
  COUNT(DISTINCT asset_id) FILTER (WHERE type = 'VIDEO') AS video_count,
  COUNT(DISTINCT asset_id) FILTER (WHERE type = 'AUDIO') AS audio_count,
  MIN(COALESCE(taken_time, upload_time)) AS date_start,
  MAX(COALESCE(taken_time, upload_time)) AS date_end,
  MAX(CASE WHEN cover_rank = 1 THEN asset_id END) AS cover_asset_id
FROM scoped;
