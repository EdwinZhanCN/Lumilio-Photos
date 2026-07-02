-- Folder browsing has no dedicated table: "folders" are derived from the
-- repository-relative prefix of assets.storage_path. All queries here treat
-- storage_path as relative (see assets_repository_id_storage_path_key) and
-- must never expose repositories.path (the absolute host path).

-- name: GetFolderChildSummaries :many
-- Lists immediate child folders of parent_path (recursive descendant
-- counts/covers). Excludes internal .lumilio paths and any asset that sits
-- directly in parent_path (files, not folders).
WITH scoped AS (
  SELECT
    a.asset_id,
    a.type,
    a.taken_time,
    a.upload_time,
    a.repository_id,
    CASE
      WHEN sqlc.arg('parent_path')::text = '' THEN a.storage_path
      ELSE substring(a.storage_path FROM length(sqlc.arg('parent_path')::text) + 2)
    END AS remainder
  FROM assets a
  WHERE a.is_deleted = false
    AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
    AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
    AND a.storage_path NOT LIKE '.lumilio/%'
    AND (
      sqlc.arg('parent_path')::text = ''
      OR a.storage_path LIKE sqlc.arg('parent_path')::text || '/%'
    )
),
child_folders AS (
  SELECT
    asset_id,
    type,
    taken_time,
    upload_time,
    repository_id,
    split_part(remainder, '/', 1) AS child_name
  FROM scoped
  WHERE remainder LIKE '%/%'
)
SELECT
  repository_id,
  child_name,
  COUNT(*)::bigint AS asset_count,
  COUNT(*) FILTER (WHERE type = 'PHOTO')::bigint AS photo_count,
  COUNT(*) FILTER (WHERE type = 'VIDEO')::bigint AS video_count,
  COUNT(*) FILTER (WHERE type = 'AUDIO')::bigint AS audio_count,
  MIN(COALESCE(taken_time, upload_time))::timestamptz AS date_start,
  MAX(COALESCE(taken_time, upload_time))::timestamptz AS date_end,
  (ARRAY_AGG(asset_id ORDER BY COALESCE(taken_time, upload_time) DESC))[1]::uuid AS cover_asset_id
FROM child_folders
GROUP BY repository_id, child_name
ORDER BY child_name ASC;

-- name: GetFolderSummary :one
-- Aggregate stats for one folder path (recursive descendants), used for the
-- folder detail header/hero.
SELECT
  COUNT(*)::bigint AS asset_count,
  COUNT(*) FILTER (WHERE a.type = 'PHOTO')::bigint AS photo_count,
  COUNT(*) FILTER (WHERE a.type = 'VIDEO')::bigint AS video_count,
  COUNT(*) FILTER (WHERE a.type = 'AUDIO')::bigint AS audio_count,
  MIN(COALESCE(a.taken_time, a.upload_time))::timestamptz AS date_start,
  MAX(COALESCE(a.taken_time, a.upload_time))::timestamptz AS date_end,
  (ARRAY_AGG(a.asset_id ORDER BY COALESCE(a.taken_time, a.upload_time) DESC))[1]::uuid AS cover_asset_id
FROM assets a
WHERE a.is_deleted = false
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
  AND a.repository_id = sqlc.arg('repository_id')
  AND a.storage_path NOT LIKE '.lumilio/%'
  AND (
    sqlc.arg('folder_path')::text = ''
    OR a.storage_path LIKE sqlc.arg('folder_path')::text || '/%'
  );
