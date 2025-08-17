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
