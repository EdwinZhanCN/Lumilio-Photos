-- 025_cloud_sync: cursor and file tracking for cloud provider imports

CREATE TABLE cloud_sync_cursors (
    repository_id UUID NOT NULL REFERENCES repositories(repo_id) ON DELETE CASCADE,
    provider      TEXT NOT NULL,
    cursor_value  TEXT NOT NULL DEFAULT '',
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (repository_id, provider)
);

CREATE TABLE cloud_sync_files (
    repository_id UUID NOT NULL REFERENCES repositories(repo_id) ON DELETE CASCADE,
    provider      TEXT NOT NULL,
    remote_key    TEXT NOT NULL,
    etag          TEXT NOT NULL DEFAULT '',
    local_hash    TEXT NOT NULL DEFAULT '',
    asset_id      UUID,
    synced_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (repository_id, provider, remote_key)
);
