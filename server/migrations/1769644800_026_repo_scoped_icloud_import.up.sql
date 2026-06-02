-- 026_repo_scoped_icloud_import: repo-scoped iCloud credentials and import runs

CREATE TABLE cloud_credentials (
    credential_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider TEXT NOT NULL,
    display_name TEXT NOT NULL,
    account_identifier_hash TEXT NOT NULL,
    masked_account TEXT NOT NULL,
    domain TEXT NOT NULL DEFAULT 'com',
    status TEXT NOT NULL DEFAULT 'connected',
    cookie_dir TEXT NOT NULL,
    created_by_user_id INTEGER REFERENCES users(user_id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, account_identifier_hash, domain)
);

CREATE INDEX idx_cloud_credentials_provider_status
    ON cloud_credentials(provider, status);

CREATE TABLE cloud_import_runs (
    run_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repository_id UUID NOT NULL REFERENCES repositories(repo_id) ON DELETE CASCADE,
    credential_id UUID NOT NULL REFERENCES cloud_credentials(credential_id) ON DELETE RESTRICT,
    provider TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    total_seen BIGINT NOT NULL DEFAULT 0,
    downloaded_count BIGINT NOT NULL DEFAULT 0,
    imported_count BIGINT NOT NULL DEFAULT 0,
    skipped_count BIGINT NOT NULL DEFAULT 0,
    failed_count BIGINT NOT NULL DEFAULT 0,
    error TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_cloud_import_runs_repository_created
    ON cloud_import_runs(repository_id, created_at DESC);
CREATE INDEX idx_cloud_import_runs_credential_created
    ON cloud_import_runs(credential_id, created_at DESC);
CREATE INDEX idx_cloud_import_runs_status
    ON cloud_import_runs(status);

CREATE TABLE repository_cloud_bindings (
    repository_id UUID NOT NULL REFERENCES repositories(repo_id) ON DELETE CASCADE,
    credential_id UUID NOT NULL REFERENCES cloud_credentials(credential_id) ON DELETE RESTRICT,
    provider TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_import_run_id UUID REFERENCES cloud_import_runs(run_id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (repository_id, provider)
);

CREATE INDEX idx_repository_cloud_bindings_credential
    ON repository_cloud_bindings(credential_id);

-- The old tables used repository+provider as their identity. The new model
-- intentionally starts fresh and scopes sync state by credential.
DELETE FROM cloud_sync_cursors;
DELETE FROM cloud_sync_files;

ALTER TABLE cloud_sync_cursors
    ADD COLUMN credential_id UUID REFERENCES cloud_credentials(credential_id) ON DELETE CASCADE;
ALTER TABLE cloud_sync_cursors
    DROP CONSTRAINT cloud_sync_cursors_pkey;
ALTER TABLE cloud_sync_cursors
    ALTER COLUMN credential_id SET NOT NULL;
ALTER TABLE cloud_sync_cursors
    ADD PRIMARY KEY (repository_id, credential_id, provider);

ALTER TABLE cloud_sync_files
    ADD COLUMN credential_id UUID REFERENCES cloud_credentials(credential_id) ON DELETE CASCADE;
ALTER TABLE cloud_sync_files
    DROP CONSTRAINT cloud_sync_files_pkey;
ALTER TABLE cloud_sync_files
    ALTER COLUMN credential_id SET NOT NULL;
ALTER TABLE cloud_sync_files
    ADD PRIMARY KEY (repository_id, credential_id, provider, remote_key);
