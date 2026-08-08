ALTER TABLE repository_scan_runs
    ADD COLUMN moved_count INTEGER NOT NULL DEFAULT 0 CHECK (moved_count >= 0);
ALTER TABLE repository_scan_runs
    ADD COLUMN deferred_count INTEGER NOT NULL DEFAULT 0 CHECK (deferred_count >= 0);
ALTER TABLE repository_scan_runs
    ADD COLUMN ambiguous_count INTEGER NOT NULL DEFAULT 0 CHECK (ambiguous_count >= 0);
ALTER TABLE repository_scan_runs
    ADD COLUMN authoritative INTEGER NOT NULL DEFAULT 0 CHECK (authoritative IN (0, 1));
ALTER TABLE repository_scan_runs
    ADD COLUMN partial_reason TEXT;

CREATE TABLE repository_file_index (
    repository_id TEXT NOT NULL
        REFERENCES repositories(repo_id) ON DELETE CASCADE,
    storage_path TEXT NOT NULL,
    asset_id TEXT
        REFERENCES assets(asset_id) ON DELETE SET NULL,
    entry_kind TEXT NOT NULL
        CHECK (entry_kind IN ('regular', 'symlink')),
    file_size INTEGER NOT NULL CHECK (file_size >= 0),
    modified_at_ns INTEGER NOT NULL,
    changed_at_ns INTEGER,
    file_identity_kind TEXT,
    file_identity_value TEXT,
    observation_token TEXT NOT NULL,
    quick_fingerprint TEXT,
    quick_fingerprint_version TEXT,
    content_hash TEXT,
    state TEXT NOT NULL
        CHECK (state IN ('present', 'missing', 'ambiguous', 'deferred')),
    first_seen_scan_id TEXT
        REFERENCES repository_scan_runs(scan_id) ON DELETE CASCADE,
    last_seen_scan_id TEXT
        REFERENCES repository_scan_runs(scan_id) ON DELETE CASCADE,
    missing_since_scan_id TEXT
        REFERENCES repository_scan_runs(scan_id) ON DELETE SET NULL,
    missing_confirmations INTEGER NOT NULL DEFAULT 0
        CHECK (missing_confirmations >= 0 AND missing_confirmations <= 2),
    ambiguity_group TEXT,
    reconciliation_reason TEXT,
    last_inspection_error TEXT,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (repository_id, storage_path)
) STRICT;

CREATE INDEX idx_repository_file_index_asset
    ON repository_file_index (asset_id)
    WHERE asset_id IS NOT NULL;
CREATE INDEX idx_repository_file_index_generation
    ON repository_file_index (repository_id, last_seen_scan_id);
CREATE INDEX idx_repository_file_index_missing
    ON repository_file_index (repository_id, state, missing_confirmations)
    WHERE state IN ('missing', 'ambiguous', 'deferred');
CREATE INDEX idx_repository_file_index_size_hash
    ON repository_file_index (repository_id, file_size, content_hash)
    WHERE content_hash IS NOT NULL;
CREATE INDEX idx_repository_file_index_identity
    ON repository_file_index (repository_id, file_identity_kind, file_identity_value)
    WHERE file_identity_value IS NOT NULL;
