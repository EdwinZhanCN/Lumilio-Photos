-- name: GetCheckpoint :one
SELECT data FROM agent_checkpoints
WHERE id = ?1;

-- name: UpsertCheckpoint :exec
INSERT INTO agent_checkpoints (id, data, updated_at)
VALUES (sqlc.arg('id'), sqlc.arg('data'), sqlc.arg('updated_at'))
ON CONFLICT (id)
DO UPDATE SET
    data = EXCLUDED.data,
    updated_at = EXCLUDED.updated_at;

-- name: DeleteCheckpoint :exec
DELETE FROM agent_checkpoints
WHERE id = ?1;

-- name: UpsertAgentThread :one
INSERT INTO agent_threads (
    user_id, thread_id, checkpoint_key, mode, context_bindings,
    mention_bindings, policy_version, status, created_at, updated_at
)
VALUES (
    sqlc.arg('user_id'), sqlc.arg('thread_id'), sqlc.arg('checkpoint_key'),
    sqlc.arg('mode'), sqlc.arg('context_bindings'), sqlc.arg('mention_bindings'),
    sqlc.arg('policy_version'), 'active', sqlc.arg('created_at'), sqlc.arg('updated_at')
)
ON CONFLICT (user_id, thread_id) DO UPDATE SET
    mode = EXCLUDED.mode,
    context_bindings = EXCLUDED.context_bindings,
    mention_bindings = EXCLUDED.mention_bindings,
    policy_version = EXCLUDED.policy_version,
    checkpoint_key = EXCLUDED.checkpoint_key,
    updated_at = EXCLUDED.updated_at
WHERE agent_threads.active_run_id IS NULL
RETURNING *;

-- name: GetAgentThread :one
SELECT * FROM agent_threads
WHERE user_id = ?1 AND thread_id = ?2;

-- name: CreateAgentRun :one
INSERT INTO agent_runs (
    run_id, user_id, thread_id, status, started_at, created_at, updated_at
)
VALUES (
    sqlc.arg('run_id'), sqlc.arg('user_id'), sqlc.arg('thread_id'), 'running',
    sqlc.arg('started_at'), sqlc.arg('created_at'), sqlc.arg('updated_at')
)
RETURNING *;

-- name: SetAgentThreadActiveRun :exec
UPDATE agent_threads
SET active_run_id = sqlc.arg('run_id'),
    status = 'active',
    updated_at = sqlc.arg('updated_at')
WHERE user_id = sqlc.arg('user_id')
  AND thread_id = sqlc.arg('thread_id');

-- name: GetAgentRun :one
SELECT * FROM agent_runs
WHERE run_id = ?1 AND user_id = ?2 AND thread_id = ?3;

-- name: GetActiveAgentRun :one
SELECT r.*
FROM agent_runs r
JOIN agent_threads t
  ON t.user_id = r.user_id
 AND t.thread_id = r.thread_id
 AND t.active_run_id = r.run_id
WHERE r.user_id = ?1
  AND r.thread_id = ?2
  AND r.status IN ('running', 'cancel_requested', 'awaiting_confirmation');

-- name: RequestAgentRunCancel :one
UPDATE agent_runs
SET status = CASE WHEN status = 'running' THEN 'cancel_requested' ELSE status END,
    cancel_requested_at = COALESCE(cancel_requested_at, sqlc.arg('updated_at')),
    updated_at = sqlc.arg('updated_at')
WHERE run_id = sqlc.arg('run_id')
  AND user_id = sqlc.arg('user_id')
  AND thread_id = sqlc.arg('thread_id')
  AND status IN ('running', 'cancel_requested', 'awaiting_confirmation')
RETURNING *;

-- name: FinishAgentRun :exec
UPDATE agent_runs
SET status = sqlc.arg('status'),
    finished_at = CASE
        WHEN sqlc.arg('status') IN ('cancelled', 'completed', 'failed')
            THEN COALESCE(finished_at, sqlc.arg('updated_at'))
        ELSE finished_at
    END,
    updated_at = sqlc.arg('updated_at')
WHERE run_id = sqlc.arg('run_id')
  AND user_id = sqlc.arg('user_id')
  AND thread_id = sqlc.arg('thread_id')
  AND status NOT IN ('cancelled', 'completed', 'failed');

-- name: FinishAgentThread :exec
UPDATE agent_threads
SET status = CASE
        WHEN sqlc.arg('status') = 'awaiting_confirmation' THEN 'awaiting_confirmation'
        WHEN sqlc.arg('status') = 'cancelled' THEN 'cancelled'
        WHEN sqlc.arg('status') = 'failed' THEN 'failed'
        ELSE 'completed'
    END,
    active_run_id = CASE
        WHEN sqlc.arg('status') = 'awaiting_confirmation' THEN sqlc.arg('run_id')
        ELSE NULL
    END,
    updated_at = sqlc.arg('updated_at')
WHERE user_id = sqlc.arg('user_id')
  AND thread_id = sqlc.arg('thread_id')
  AND active_run_id = sqlc.arg('run_id');

-- name: ClearAwaitingAgentRun :exec
UPDATE agent_runs
SET status = 'completed',
    finished_at = COALESCE(finished_at, sqlc.arg('updated_at')),
    updated_at = sqlc.arg('updated_at')
WHERE run_id = sqlc.arg('run_id')
  AND user_id = sqlc.arg('user_id')
  AND thread_id = sqlc.arg('thread_id')
  AND status = 'awaiting_confirmation';

-- name: UpsertAgentRef :exec
INSERT INTO agent_refs (
    user_id, thread_id, ref_id, sequence, plan, asset_ids,
    summary, truncated, created_at, last_accessed_at, expires_at
)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11)
ON CONFLICT (user_id, thread_id, ref_id) DO UPDATE SET
    plan = EXCLUDED.plan,
    asset_ids = EXCLUDED.asset_ids,
    summary = EXCLUDED.summary,
    truncated = EXCLUDED.truncated,
    last_accessed_at = EXCLUDED.last_accessed_at,
    expires_at = EXCLUDED.expires_at;

-- name: GetAgentRef :one
UPDATE agent_refs
SET last_accessed_at = sqlc.arg('now'),
    expires_at = sqlc.arg('expires_at')
WHERE agent_refs.user_id = sqlc.arg('user_id')
  AND agent_refs.thread_id = sqlc.arg('thread_id')
  AND agent_refs.ref_id = sqlc.arg('ref_id')
  AND (
      expires_at > sqlc.arg('now')
      OR EXISTS (
          SELECT 1
          FROM agent_threads t
          WHERE t.user_id = agent_refs.user_id
            AND t.thread_id = agent_refs.thread_id
            AND t.status IN ('active', 'awaiting_confirmation')
      )
  )
RETURNING *;

-- name: ListAgentRefs :many
SELECT r.* FROM agent_refs r
WHERE r.user_id = ?1
  AND r.thread_id = ?2
  AND (
      r.expires_at > sqlc.arg('now')
      OR EXISTS (
          SELECT 1
          FROM agent_threads t
          WHERE t.user_id = r.user_id
            AND t.thread_id = r.thread_id
            AND t.status IN ('active', 'awaiting_confirmation')
      )
  )
ORDER BY r.sequence;

-- name: TrimAgentThreadRefs :many
DELETE FROM agent_refs
WHERE rowid IN (
    SELECT candidate.rowid
    FROM agent_refs AS candidate
    WHERE candidate.user_id = sqlc.arg('user_id')
      AND candidate.thread_id = sqlc.arg('thread_id')
    ORDER BY candidate.last_accessed_at DESC, candidate.sequence DESC
    LIMIT -1
    OFFSET sqlc.arg('max_refs')
)
RETURNING ref_id;

-- name: DeleteAgentThreadRefs :exec
DELETE FROM agent_refs
WHERE user_id = ?1 AND thread_id = ?2;

-- name: ReleaseAgentThreadRefs :exec
UPDATE agent_refs
SET last_accessed_at = sqlc.arg('now'),
    expires_at = sqlc.arg('expires_at')
WHERE user_id = sqlc.arg('user_id')
  AND thread_id = sqlc.arg('thread_id');

-- name: DeleteExpiredAgentRefs :exec
DELETE FROM agent_refs
WHERE expires_at <= sqlc.arg('now')
  AND NOT EXISTS (
      SELECT 1
      FROM agent_threads t
      WHERE t.user_id = agent_refs.user_id
        AND t.thread_id = agent_refs.thread_id
        AND t.status IN ('active', 'awaiting_confirmation')
  );

-- name: CreatePendingAgentEffect :one
INSERT INTO agent_pending_effects (
    effect_id, user_id, thread_id, initiating_run_id, tool_name,
    effect_class, policy_version, membership_snapshot, payload, target,
    idempotency_key, created_at, updated_at
)
VALUES (
    sqlc.arg('effect_id'), sqlc.arg('user_id'), sqlc.arg('thread_id'),
    sqlc.arg('initiating_run_id'), sqlc.arg('tool_name'), sqlc.arg('effect_class'),
    sqlc.arg('policy_version'), sqlc.arg('membership_snapshot'), sqlc.arg('payload'),
    sqlc.arg('target'), sqlc.arg('idempotency_key'), sqlc.arg('created_at'),
    sqlc.arg('updated_at')
)
ON CONFLICT (idempotency_key) DO UPDATE SET updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: GetPendingAgentEffectForUpdate :one
SELECT * FROM agent_pending_effects
WHERE effect_id = ?1 AND user_id = ?2 AND thread_id = ?3
;

-- name: UpdatePendingAgentEffect :exec
UPDATE agent_pending_effects
SET status = ?4,
    receipt = ?5,
    committed_at = CASE
        WHEN ?4 = 'committed' THEN COALESCE(committed_at, sqlc.arg('updated_at'))
        ELSE committed_at
    END,
    updated_at = sqlc.arg('updated_at')
WHERE effect_id = ?1 AND user_id = ?2 AND thread_id = ?3;

-- name: BindPendingAgentEffectExecutingRun :one
UPDATE agent_pending_effects
SET executing_run_id = sqlc.arg('run_id'),
    updated_at = sqlc.arg('updated_at')
WHERE agent_pending_effects.effect_id = sqlc.arg('effect_id')
  AND agent_pending_effects.user_id = sqlc.arg('user_id')
  AND agent_pending_effects.thread_id = sqlc.arg('thread_id')
  AND EXISTS (
      SELECT 1
      FROM agent_runs r
      WHERE r.run_id = sqlc.arg('run_id')
        AND r.user_id = agent_pending_effects.user_id
        AND r.thread_id = agent_pending_effects.thread_id
  )
RETURNING effect_id;

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
SET status = 'cancelled', updated_at = sqlc.arg('updated_at')
WHERE user_id = ?1 AND thread_id = ?2 AND status = 'pending';

-- name: DeleteTerminalPendingAgentEffects :exec
DELETE FROM agent_pending_effects
WHERE user_id = ?1
  AND thread_id = ?2
  AND status <> 'committed';
