-- name: CreateTag :one
INSERT INTO tags (tag_name, category, is_ai_generated)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetTagByID :one
SELECT * FROM tags WHERE tag_id = $1;

-- name: GetTagByName :one
SELECT * FROM tags WHERE tag_name = $1;

-- name: ListTags :many
SELECT * FROM tags
ORDER BY tag_name ASC
LIMIT $1 OFFSET $2;

-- name: SearchTagsByName :many
SELECT * FROM tags
WHERE sqlc.narg('query')::text IS NULL
   OR tag_name ILIKE '%' || sqlc.narg('query')::text || '%'
ORDER BY tag_name ASC
LIMIT $1;

-- name: GetTagsByCategory :many
SELECT * FROM tags
WHERE category = $1
ORDER BY tag_name ASC;

-- name: UpdateTag :one
UPDATE tags
SET tag_name = $2, category = $3, is_ai_generated = $4
WHERE tag_id = $1
RETURNING *;

-- name: DeleteTag :exec
DELETE FROM tags WHERE tag_id = $1;

-- name: GetTagSummaries :many
-- Browsable tag vocabulary with counts/cover, distinct from
-- SearchTagsByName (definition-only autocomplete). Groups by (tag_id,
-- source) because the same tag_id can carry manual assignments on some
-- assets and AI/system assignments on others.
SELECT
  t.tag_id,
  t.tag_name,
  at.source,
  COUNT(DISTINCT a.asset_id)::bigint AS asset_count,
  (ARRAY_AGG(a.asset_id ORDER BY COALESCE(a.taken_time, a.upload_time) DESC))[1]::uuid AS cover_asset_id,
  MAX(COALESCE(a.taken_time, a.upload_time))::timestamptz AS last_used_at
FROM asset_tags at
JOIN tags t ON t.tag_id = at.tag_id
JOIN assets a ON a.asset_id = at.asset_id
WHERE a.is_deleted = false
  AND (sqlc.narg('owner_id')::integer IS NULL OR a.owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('repository_id')::uuid IS NULL OR a.repository_id = sqlc.narg('repository_id'))
  AND (sqlc.narg('source')::text IS NULL OR at.source = sqlc.narg('source'))
  AND (sqlc.narg('query')::text IS NULL OR t.tag_name ILIKE '%' || sqlc.narg('query')::text || '%')
GROUP BY t.tag_id, t.tag_name, at.source
ORDER BY asset_count DESC, t.tag_name ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');
