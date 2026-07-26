-- name: CreateAgentPin :one
INSERT INTO agent_pins (
    user_id, title, widget, mode, plan, summary, asset_ids, truncated,
    layout_x, layout_y, layout_w, layout_h, created_at, updated_at, pin_id
)
VALUES (
    ?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12,
    CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    sqlc.arg('pin_id')
)
RETURNING *;

-- name: ListAgentPins :many
SELECT * FROM agent_pins
WHERE user_id = ?1
ORDER BY created_at ASC;

-- name: GetAgentPin :one
SELECT * FROM agent_pins
WHERE pin_id = ?1 AND user_id = ?2;

-- name: TouchAgentPinLiveRefresh :exec
UPDATE agent_pins
SET last_successful_refresh_at = ?3,
    updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE pin_id = ?1 AND user_id = ?2;

-- name: UpdateAgentPinLayout :exec
UPDATE agent_pins
SET layout_x = ?3, layout_y = ?4, layout_w = ?5, layout_h = ?6,
    updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE pin_id = ?1 AND user_id = ?2;

-- name: UpdateAgentPinTitle :exec
UPDATE agent_pins
SET title = ?3, updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE pin_id = ?1 AND user_id = ?2;

-- name: UpdateAgentPinWidget :exec
UPDATE agent_pins
SET widget = ?3, updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE pin_id = ?1 AND user_id = ?2;

-- name: DeleteAgentPin :exec
DELETE FROM agent_pins
WHERE pin_id = ?1 AND user_id = ?2;
