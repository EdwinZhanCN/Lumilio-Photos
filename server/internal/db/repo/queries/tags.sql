-- name: CreateTag :one
INSERT INTO tags (tag_name, category, is_ai_generated)
VALUES (?1, ?2, ?3)
RETURNING *;

-- name: GetTagByID :one
SELECT * FROM tags WHERE tag_id = ?1;

-- name: GetTagByName :one
SELECT * FROM tags WHERE tag_name = ?1;

-- name: ListTags :many
SELECT * FROM tags
ORDER BY tag_name ASC
LIMIT ?1 OFFSET ?2;

-- name: SearchTagsByName :many
SELECT * FROM tags
WHERE sqlc.narg('query') IS NULL
   OR tag_name LIKE '%' || sqlc.narg('query') || '%'
ORDER BY tag_name ASC
LIMIT ?1;

-- name: GetTagsByCategory :many
SELECT * FROM tags
WHERE category = ?1
ORDER BY tag_name ASC;

-- name: UpdateTag :one
UPDATE tags
SET tag_name = ?2, category = ?3, is_ai_generated = ?4
WHERE tag_id = ?1
RETURNING *;

-- name: DeleteTag :exec
DELETE FROM tags WHERE tag_id = ?1;

-- name: GetTagSummaries :many
-- Browsable tag vocabulary with counts/cover, distinct from
-- SearchTagsByName (definition-only autocomplete). Groups by (tag_id,
-- source) because the same tag_id can carry manual assignments on some
-- assets and AI/system assignments on others.
WITH ranked AS (
  SELECT
    t.tag_id,
    t.tag_name,
    at.source,
    a.asset_id,
    a.taken_time,
    a.upload_time,
    ROW_NUMBER() OVER (
      PARTITION BY t.tag_id, at.source
      ORDER BY COALESCE(a.taken_time, a.upload_time) DESC, a.asset_id DESC
    ) AS cover_rank
  FROM asset_tags at
  JOIN tags t ON t.tag_id = at.tag_id
  JOIN assets a ON a.asset_id = at.asset_id
  WHERE a.is_deleted = false
    AND (sqlc.narg('owner_id') IS NULL OR a.owner_id = sqlc.narg('owner_id'))
    AND (sqlc.narg('repository_id') IS NULL OR a.repository_id = sqlc.narg('repository_id'))
    AND (sqlc.narg('source') IS NULL OR at.source = sqlc.narg('source'))
    AND (sqlc.narg('query') IS NULL OR t.tag_name LIKE '%' || sqlc.narg('query') || '%')
)
SELECT
  t.tag_id,
  t.tag_name,
  t.source,
  COUNT(DISTINCT a.asset_id) AS asset_count,
  MAX(CASE WHEN t.cover_rank = 1 THEN a.asset_id END) AS cover_asset_id,
  MAX(COALESCE(a.taken_time, a.upload_time)) AS last_used_at
FROM ranked AS t
JOIN assets a ON a.asset_id = t.asset_id
GROUP BY t.tag_id, t.tag_name, t.source
ORDER BY asset_count DESC, t.tag_name ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');
