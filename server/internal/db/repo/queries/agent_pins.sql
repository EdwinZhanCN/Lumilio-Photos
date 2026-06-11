-- name: CreateAgentPin :one
INSERT INTO agent_pins (user_id, title, widget, mode, plan, summary, asset_ids, truncated, layout_x, layout_y, layout_w, layout_h)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: ListAgentPins :many
SELECT * FROM agent_pins
WHERE user_id = $1
ORDER BY created_at ASC;

-- name: GetAgentPin :one
SELECT * FROM agent_pins
WHERE pin_id = $1 AND user_id = $2;

-- name: UpdateAgentPinLayout :exec
UPDATE agent_pins
SET layout_x = $3, layout_y = $4, layout_w = $5, layout_h = $6, updated_at = NOW()
WHERE pin_id = $1 AND user_id = $2;

-- name: DeleteAgentPin :exec
DELETE FROM agent_pins
WHERE pin_id = $1 AND user_id = $2;
