-- name: CreateLifecycleOperation :one
INSERT INTO lifecycle_operations (
    operation_id,
    request_id,
    kind,
    payload_hash,
    payload,
    actor,
    actor_user_id,
    host_instance_id,
    target_type,
    target_id,
    phase,
    status,
    rollback_data,
    created_at,
    updated_at
) VALUES (
    ?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, 'prepared', 'running', ?11, ?12, ?12
)
RETURNING *;

-- name: GetLifecycleOperation :one
SELECT * FROM lifecycle_operations
WHERE operation_id = ?1;

-- name: GetLifecycleOperationByRequestID :one
SELECT * FROM lifecycle_operations
WHERE request_id = ?1;

-- name: ListIncompleteLifecycleOperations :many
SELECT * FROM lifecycle_operations
WHERE status = 'running'
ORDER BY created_at ASC, operation_id ASC;

-- name: UpdateLifecycleOperationPhase :one
UPDATE lifecycle_operations
SET
    phase = ?2,
    rollback_data = ?3,
    updated_at = ?4
WHERE operation_id = ?1
  AND status = 'running'
RETURNING *;

-- name: CompleteLifecycleOperation :one
UPDATE lifecycle_operations
SET
    phase = 'completed',
    status = 'completed',
    result = ?2,
    error = NULL,
    updated_at = ?3,
    completed_at = ?3
WHERE operation_id = ?1
  AND status = 'running'
RETURNING *;

-- name: FailLifecycleOperation :one
UPDATE lifecycle_operations
SET
    phase = ?2,
    status = ?3,
    error = ?4,
    rollback_data = ?5,
    updated_at = ?6,
    completed_at = ?6
WHERE operation_id = ?1
  AND status = 'running'
RETURNING *;
