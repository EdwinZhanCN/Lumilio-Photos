-- Keep the epoch of a force request separate from the sticky requirement so
-- ordinary watcher hints do not inherit a full crawl that already completed.
ALTER TABLE repository_observation_state ADD COLUMN full_verification_requested_epoch INTEGER NOT NULL DEFAULT 0;
UPDATE repository_observation_state
SET full_verification_requested_epoch = desired_epoch
WHERE full_verification_required = 1;
ALTER TABLE repository_scan_runs ADD COLUMN full_verification_performed INTEGER NOT NULL DEFAULT 0 CHECK (full_verification_performed IN (0,1));

CREATE INDEX idx_repository_scan_runs_full_verification
 ON repository_scan_runs(repository_id, finished_at DESC)
 WHERE status = 'completed' AND full_verification_performed = 1;

-- Failure attempts and cooldowns survive replacement of the disposable QueueDB.
-- Identity columns fence this bounded row to the requested asset generation.
CREATE TABLE asset_pipeline_failures (
    asset_id TEXT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    stage TEXT NOT NULL CHECK (stage IN ('analyze','derivatives','transcode','enrich')),
    source_content_id TEXT NOT NULL,
    pipeline_version TEXT NOT NULL,
    desired_version INTEGER NOT NULL CHECK (desired_version > 0),
    failure_count INTEGER NOT NULL CHECK (failure_count > 0),
    retry_after INTEGER NOT NULL,
    failure_code TEXT NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (asset_id, stage)
) STRICT;
