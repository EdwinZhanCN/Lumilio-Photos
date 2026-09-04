-- A resumed execution receives its own durable run id before Eino accepts the
-- checkpoint. Prepared resumes are deliberately excluded from the one-active-
-- run invariant until the old awaiting run is completed in the same
-- transaction that activates the replacement.
ALTER TABLE agent_runs
    ADD COLUMN activation_state TEXT NOT NULL DEFAULT 'active'
        CHECK (activation_state IN ('prepared_resume', 'active', 'terminal'));

UPDATE agent_runs
SET activation_state = 'terminal'
WHERE status IN ('cancelled', 'completed', 'failed');

DROP INDEX idx_agent_runs_one_active_thread;
CREATE UNIQUE INDEX idx_agent_runs_one_active_thread ON agent_runs (user_id, thread_id)
    WHERE activation_state = 'active'
      AND status IN ('running', 'cancel_requested', 'awaiting_confirmation');
