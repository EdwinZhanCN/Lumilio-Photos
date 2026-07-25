-- name: GetCheckpoint :one
SELECT data FROM agent_checkpoints
WHERE id = $1;

-- name: UpsertCheckpoint :exec
INSERT INTO agent_checkpoints (id, data, updated_at)
VALUES ($1, $2, NOW())
ON CONFLICT (id)
DO UPDATE SET
    data = EXCLUDED.data,
    updated_at = NOW();

-- name: DeleteCheckpoint :exec
DELETE FROM agent_checkpoints
WHERE id = $1;

-- name: UpsertAgentThread :one
INSERT INTO agent_threads (
    user_id, thread_id, checkpoint_key, mode, context_bindings,
    mention_bindings, policy_version, status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'active')
ON CONFLICT (user_id, thread_id) DO UPDATE SET
    mode = EXCLUDED.mode,
    context_bindings = EXCLUDED.context_bindings,
    mention_bindings = EXCLUDED.mention_bindings,
    policy_version = EXCLUDED.policy_version,
    checkpoint_key = EXCLUDED.checkpoint_key,
    updated_at = NOW()
WHERE agent_threads.active_run_id IS NULL
RETURNING *;

-- name: GetAgentThread :one
SELECT * FROM agent_threads
WHERE user_id = $1 AND thread_id = $2;

-- name: CreateAgentRun :one
WITH created AS (
    INSERT INTO agent_runs (user_id, thread_id, status)
    VALUES ($1, $2, 'running')
    RETURNING *
)
UPDATE agent_threads t
SET active_run_id = created.run_id,
    status = 'active',
    updated_at = NOW()
FROM created
WHERE t.user_id = created.user_id
  AND t.thread_id = created.thread_id
RETURNING created.*;

-- name: GetAgentRun :one
SELECT * FROM agent_runs
WHERE run_id = $1 AND user_id = $2 AND thread_id = $3;

-- name: GetActiveAgentRun :one
SELECT r.*
FROM agent_runs r
JOIN agent_threads t
  ON t.user_id = r.user_id
 AND t.thread_id = r.thread_id
 AND t.active_run_id = r.run_id
WHERE r.user_id = $1
  AND r.thread_id = $2
  AND r.status = ANY (ARRAY['running', 'cancel_requested', 'awaiting_confirmation']);

-- name: RequestAgentRunCancel :one
UPDATE agent_runs
SET status = CASE WHEN status = 'running' THEN 'cancel_requested' ELSE status END,
    cancel_requested_at = COALESCE(cancel_requested_at, NOW()),
    updated_at = NOW()
WHERE run_id = $1
  AND user_id = $2
  AND thread_id = $3
  AND status = ANY (ARRAY['running', 'cancel_requested', 'awaiting_confirmation'])
RETURNING *;

-- name: FinishAgentRun :exec
WITH updated AS (
    UPDATE agent_runs r
    SET status = sqlc.arg('status'),
        finished_at = CASE
            WHEN sqlc.arg('status') = ANY (ARRAY['cancelled', 'completed', 'failed'])
                THEN COALESCE(r.finished_at, NOW())
            ELSE r.finished_at
        END,
        updated_at = NOW()
    WHERE r.run_id = sqlc.arg('run_id')
      AND r.user_id = sqlc.arg('user_id')
      AND r.thread_id = sqlc.arg('thread_id')
      AND r.status <> ALL (ARRAY['cancelled', 'completed', 'failed'])
    RETURNING r.user_id, r.thread_id, r.run_id, r.status
)
UPDATE agent_threads t
SET status = CASE
        WHEN updated.status = 'awaiting_confirmation' THEN 'awaiting_confirmation'
        WHEN updated.status = 'cancelled' THEN 'cancelled'
        WHEN updated.status = 'failed' THEN 'failed'
        ELSE 'completed'
    END,
    active_run_id = CASE
        WHEN updated.status = 'awaiting_confirmation' THEN updated.run_id
        ELSE NULL
    END,
    updated_at = NOW()
FROM updated
WHERE t.user_id = updated.user_id
  AND t.thread_id = updated.thread_id
  AND t.active_run_id = updated.run_id;

-- name: ClearAwaitingAgentRun :exec
UPDATE agent_runs
SET status = 'completed',
    finished_at = COALESCE(finished_at, NOW()),
    updated_at = NOW()
WHERE run_id = $1
  AND user_id = $2
  AND thread_id = $3
  AND status = 'awaiting_confirmation';

-- name: UpsertAgentRef :exec
INSERT INTO agent_refs (
    user_id, thread_id, ref_id, sequence, plan, asset_ids,
    summary, truncated, created_at, last_accessed_at, expires_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (user_id, thread_id, ref_id) DO UPDATE SET
    plan = EXCLUDED.plan,
    asset_ids = EXCLUDED.asset_ids,
    summary = EXCLUDED.summary,
    truncated = EXCLUDED.truncated,
    last_accessed_at = EXCLUDED.last_accessed_at,
    expires_at = EXCLUDED.expires_at;

-- name: GetAgentRef :one
UPDATE agent_refs r
SET last_accessed_at = NOW(),
    expires_at = sqlc.arg('expires_at')
WHERE r.user_id = sqlc.arg('user_id')
  AND r.thread_id = sqlc.arg('thread_id')
  AND r.ref_id = sqlc.arg('ref_id')
  AND (
      r.expires_at > NOW()
      OR EXISTS (
          SELECT 1
          FROM agent_threads t
          WHERE t.user_id = r.user_id
            AND t.thread_id = r.thread_id
            AND t.status = ANY (ARRAY['active', 'awaiting_confirmation'])
      )
  )
RETURNING *;

-- name: ListAgentRefs :many
SELECT r.* FROM agent_refs r
WHERE r.user_id = $1
  AND r.thread_id = $2
  AND (
      r.expires_at > NOW()
      OR EXISTS (
          SELECT 1
          FROM agent_threads t
          WHERE t.user_id = r.user_id
            AND t.thread_id = r.thread_id
            AND t.status = ANY (ARRAY['active', 'awaiting_confirmation'])
      )
  )
ORDER BY r.sequence;

-- name: TrimAgentThreadRefs :many
WITH overflow AS (
    SELECT ref_id
    FROM agent_refs
    WHERE user_id = sqlc.arg('user_id')
      AND thread_id = sqlc.arg('thread_id')
    ORDER BY last_accessed_at DESC, sequence DESC
    OFFSET sqlc.arg('max_refs')
)
DELETE FROM agent_refs r
USING overflow
WHERE r.user_id = sqlc.arg('user_id')
  AND r.thread_id = sqlc.arg('thread_id')
  AND r.ref_id = overflow.ref_id
RETURNING r.ref_id;

-- name: DeleteAgentThreadRefs :exec
DELETE FROM agent_refs
WHERE user_id = $1 AND thread_id = $2;

-- name: ReleaseAgentThreadRefs :exec
UPDATE agent_refs
SET last_accessed_at = NOW(),
    expires_at = sqlc.arg('expires_at')
WHERE user_id = sqlc.arg('user_id')
  AND thread_id = sqlc.arg('thread_id');

-- name: DeleteExpiredAgentRefs :exec
DELETE FROM agent_refs r
WHERE r.expires_at <= NOW()
  AND NOT EXISTS (
      SELECT 1
      FROM agent_threads t
      WHERE t.user_id = r.user_id
        AND t.thread_id = r.thread_id
        AND t.status = ANY (ARRAY['active', 'awaiting_confirmation'])
  );

-- name: CreatePendingAgentEffect :one
INSERT INTO agent_pending_effects (
    effect_id, user_id, thread_id, initiating_run_id, tool_name,
    effect_class, policy_version, membership_snapshot, payload, target,
    idempotency_key
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (idempotency_key) DO UPDATE SET updated_at = NOW()
RETURNING *;

-- name: GetPendingAgentEffectForUpdate :one
SELECT * FROM agent_pending_effects
WHERE effect_id = $1 AND user_id = $2 AND thread_id = $3
FOR UPDATE;

-- name: UpdatePendingAgentEffect :exec
UPDATE agent_pending_effects
SET status = $4,
    receipt = $5,
    committed_at = CASE WHEN $4 = 'committed' THEN COALESCE(committed_at, NOW()) ELSE committed_at END,
    updated_at = NOW()
WHERE effect_id = $1 AND user_id = $2 AND thread_id = $3;

-- name: BindPendingAgentEffectExecutingRun :one
UPDATE agent_pending_effects e
SET executing_run_id = sqlc.arg('run_id'),
    updated_at = NOW()
FROM agent_runs r
WHERE e.effect_id = sqlc.arg('effect_id')
  AND e.user_id = sqlc.arg('user_id')
  AND e.thread_id = sqlc.arg('thread_id')
  AND r.run_id = sqlc.arg('run_id')
  AND r.user_id = e.user_id
  AND r.thread_id = e.thread_id
RETURNING e.effect_id;

-- name: AgentRunHasCommittedEffect :one
SELECT EXISTS (
    SELECT 1
    FROM agent_pending_effects
    WHERE user_id = sqlc.arg('user_id')
      AND thread_id = sqlc.arg('thread_id')
      AND executing_run_id = sqlc.arg('run_id')
      AND status = 'committed'
);

-- name: CancelPendingAgentEffects :exec
UPDATE agent_pending_effects
SET status = 'cancelled', updated_at = NOW()
WHERE user_id = $1 AND thread_id = $2 AND status = 'pending';

-- name: DeleteTerminalPendingAgentEffects :exec
DELETE FROM agent_pending_effects
WHERE user_id = $1
  AND thread_id = $2
  AND status <> 'committed';
