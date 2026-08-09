-- name: InsertLifecycleAuditEvent :one
INSERT INTO lifecycle_audit_events (
    event_id, occurred_at, actor, actor_user_id, host_instance_id, request_id,
    operation_id, action, target_type, target_id, source, confirmation_type,
    old_path, new_path, result, failure_stage, details
) VALUES (
    ?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13, ?14, ?15, ?16, ?17
)
RETURNING *;

-- name: ListLifecycleAuditEvents :many
SELECT * FROM lifecycle_audit_events
ORDER BY occurred_at DESC, event_id DESC
LIMIT ?1 OFFSET ?2;

-- name: ListLifecycleAuditEventsForTarget :many
SELECT * FROM lifecycle_audit_events
WHERE target_type = ?1 AND target_id = ?2
ORDER BY occurred_at DESC, event_id DESC
LIMIT ?3 OFFSET ?4;
