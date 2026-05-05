CREATE TABLE IF NOT EXISTS repository_scan_runs (
    scan_id          UUID PRIMARY KEY,
    repository_id    UUID NOT NULL REFERENCES repositories(repo_id) ON DELETE CASCADE,
    mode             TEXT NOT NULL CHECK (mode IN ('periodic', 'manual')),
    requested_by     TEXT,
    status           TEXT NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled')),
    started_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at      TIMESTAMPTZ,
    discovered_count BIGINT NOT NULL DEFAULT 0,
    updated_count    BIGINT NOT NULL DEFAULT 0,
    deleted_count    BIGINT NOT NULL DEFAULT 0,
    skipped_count    BIGINT NOT NULL DEFAULT 0,
    error            TEXT
);

CREATE INDEX IF NOT EXISTS idx_repository_scan_runs_repo_started
    ON repository_scan_runs(repository_id, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_repository_scan_runs_running
    ON repository_scan_runs(repository_id)
    WHERE status = 'running';
