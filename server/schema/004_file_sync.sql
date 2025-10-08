-- File synchronization tables
CREATE TABLE file_records (
    id BIGSERIAL PRIMARY KEY,
    repository_id UUID NOT NULL REFERENCES repositories(repo_id) ON DELETE CASCADE,
    file_path TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    mod_time TIMESTAMP WITH TIME ZONE NOT NULL,
    content_hash CHAR(64),
    last_scanned TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    scan_generation BIGINT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(repository_id, file_path)
);

CREATE TABLE sync_operations (
    id BIGSERIAL PRIMARY KEY,
    repository_id UUID NOT NULL REFERENCES repositories(repo_id) ON DELETE CASCADE,
    operation_type TEXT NOT NULL CHECK (operation_type IN ('realtime', 'reconciliation', 'startup')),
    files_scanned INTEGER DEFAULT 0,
    files_added INTEGER DEFAULT 0,
    files_updated INTEGER DEFAULT 0,
    files_removed INTEGER DEFAULT 0,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE,
    duration_ms BIGINT,
    status TEXT DEFAULT 'running' CHECK (status IN ('running', 'completed', 'failed')),
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for file_records
CREATE INDEX idx_file_records_repo_id ON file_records(repository_id);
CREATE INDEX idx_file_records_repo_path ON file_records(repository_id, file_path);
CREATE INDEX idx_file_records_scan_gen ON file_records(scan_generation);
CREATE INDEX idx_file_records_mod_time ON file_records(repository_id, mod_time);
CREATE INDEX idx_file_records_hash ON file_records(content_hash) WHERE content_hash IS NOT NULL;

-- Indexes for sync_operations
CREATE INDEX idx_sync_operations_repo_id ON sync_operations(repository_id);
CREATE INDEX idx_sync_operations_start_time ON sync_operations(start_time DESC);
CREATE INDEX idx_sync_operations_status ON sync_operations(status);
