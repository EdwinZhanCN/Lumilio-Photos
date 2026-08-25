CREATE TABLE repository_scan_runs_v2 (
    run_id TEXT PRIMARY KEY
        CHECK (run_id = lower(run_id) AND length(run_id) = 36),
    repository_id TEXT NOT NULL
        REFERENCES repositories(repo_id) ON DELETE CASCADE,
    requested_epoch INTEGER NOT NULL CHECK (requested_epoch > 0),
    mode TEXT NOT NULL
        CHECK (mode IN ('manual', 'periodic', 'watcher', 'recovery', 'migration')),
    requested_by TEXT,
    coalesced_count INTEGER NOT NULL DEFAULT 0 CHECK (coalesced_count >= 0),
    status TEXT NOT NULL
        CHECK (status IN (
            'queued', 'crawling', 'catching_up', 'finalizing',
            'completed', 'partial', 'failed', 'cancelled'
        )),
    created_at INTEGER NOT NULL,
    started_at INTEGER,
    finished_at INTEGER,
    cursor_start BLOB,
    cursor_end BLOB,
    cursor_target BLOB NOT NULL DEFAULT X'',
    volume_identity TEXT,
    directories_observed INTEGER NOT NULL DEFAULT 0 CHECK (directories_observed >= 0),
    files_observed INTEGER NOT NULL DEFAULT 0 CHECK (files_observed >= 0),
    bytes_queued INTEGER NOT NULL DEFAULT 0 CHECK (bytes_queued >= 0),
    bytes_hashed INTEGER NOT NULL DEFAULT 0 CHECK (bytes_hashed >= 0),
    authoritative_directories INTEGER NOT NULL DEFAULT 0 CHECK (authoritative_directories >= 0),
    error_directories INTEGER NOT NULL DEFAULT 0 CHECK (error_directories >= 0),
    outbox_depth INTEGER NOT NULL DEFAULT 0 CHECK (outbox_depth >= 0),
    partial_coverage INTEGER NOT NULL DEFAULT 0 CHECK (partial_coverage IN (0, 1)),
    cancellation_requested INTEGER NOT NULL DEFAULT 0 CHECK (cancellation_requested IN (0, 1)),
    force_full_verification INTEGER NOT NULL DEFAULT 0
        CHECK (force_full_verification IN (0, 1)),
    failure_code TEXT,
    failure_problem_type TEXT,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE UNIQUE INDEX repository_scan_runs_v2_one_active
    ON repository_scan_runs_v2 (repository_id)
    WHERE status IN ('queued', 'crawling', 'catching_up', 'finalizing');
CREATE INDEX idx_repository_scan_runs_v2_history
    ON repository_scan_runs_v2 (repository_id, created_at DESC, run_id);

CREATE TABLE repository_observation_state (
    repository_id TEXT PRIMARY KEY
        REFERENCES repositories(repo_id) ON DELETE CASCADE,
    desired_epoch INTEGER NOT NULL DEFAULT 0 CHECK (desired_epoch >= 0),
    applied_epoch INTEGER NOT NULL DEFAULT 0
        CHECK (applied_epoch >= 0 AND applied_epoch <= desired_epoch),
    next_revision INTEGER NOT NULL DEFAULT 1 CHECK (next_revision > 0),
    active_run_id TEXT
        REFERENCES repository_scan_runs_v2(run_id) ON DELETE SET NULL,
    controller_lease_id TEXT,
    controller_lease_expires_at INTEGER,
    adapter_kind TEXT NOT NULL DEFAULT 'periodic'
        CHECK (adapter_kind IN ('usn', 'rdcw', 'fsevents', 'inotify', 'periodic')),
    adapter_identity TEXT,
    volume_identity TEXT,
    volume_kind TEXT NOT NULL DEFAULT 'unknown'
        CHECK (volume_kind IN ('local', 'network', 'removable', 'unsupported', 'unknown')),
    path_case_mode TEXT NOT NULL DEFAULT 'sensitive'
        CHECK (path_case_mode IN ('sensitive', 'insensitive')),
    path_normalization TEXT NOT NULL DEFAULT 'unknown'
        CHECK (path_normalization IN ('none', 'nfc', 'nfd', 'unknown')),
    cursor_health TEXT NOT NULL DEFAULT 'unavailable'
        CHECK (cursor_health IN ('healthy', 'gap', 'overflow', 'unavailable')),
    full_verification_required INTEGER NOT NULL DEFAULT 1
        CHECK (full_verification_required IN (0, 1)),
    updated_at INTEGER NOT NULL
) STRICT;

CREATE INDEX idx_repository_observation_state_work
    ON repository_observation_state (desired_epoch, applied_epoch, controller_lease_expires_at)
    WHERE desired_epoch > applied_epoch OR full_verification_required = 1;

CREATE TABLE repository_nodes (
    node_id TEXT PRIMARY KEY
        CHECK (node_id = lower(node_id) AND length(node_id) = 36),
    repository_id TEXT NOT NULL
        REFERENCES repositories(repo_id) ON DELETE CASCADE,
    parent_node_id TEXT,
    name TEXT NOT NULL,
    name_key TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('directory', 'file', 'symlink')),
    lifecycle TEXT NOT NULL DEFAULT 'active'
        CHECK (lifecycle IN ('active', 'tombstoned')),
    native_identity_kind TEXT,
    native_identity_value TEXT,
    volume_identity TEXT,
    observation_revision INTEGER NOT NULL CHECK (observation_revision >= 0),
    stability_token TEXT,
    file_size INTEGER CHECK (file_size IS NULL OR file_size >= 0),
    modified_at_ns INTEGER,
    changed_at_ns INTEGER,
    last_seen_run_id TEXT
        REFERENCES repository_scan_runs_v2(run_id) ON DELETE SET NULL,
    last_authoritative_coverage_revision INTEGER NOT NULL DEFAULT 0
        CHECK (last_authoritative_coverage_revision >= 0),
    absence_first_observed_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE (repository_id, node_id),
    FOREIGN KEY (repository_id, parent_node_id)
        REFERENCES repository_nodes(repository_id, node_id)
        DEFERRABLE INITIALLY DEFERRED
) STRICT;

CREATE UNIQUE INDEX repository_nodes_one_active_root
    ON repository_nodes (repository_id)
    WHERE parent_node_id IS NULL AND lifecycle = 'active';
CREATE UNIQUE INDEX repository_nodes_one_active_child
    ON repository_nodes (repository_id, parent_node_id, name_key)
    WHERE lifecycle = 'active' AND parent_node_id IS NOT NULL;
CREATE INDEX idx_repository_nodes_children
    ON repository_nodes (repository_id, parent_node_id, lifecycle, node_id);
CREATE INDEX idx_repository_nodes_native_identity
    ON repository_nodes (repository_id, volume_identity, native_identity_kind, native_identity_value)
    WHERE lifecycle = 'active' AND native_identity_value IS NOT NULL;
CREATE INDEX idx_repository_nodes_run_coverage
    ON repository_nodes (repository_id, parent_node_id, last_seen_run_id)
    WHERE lifecycle = 'active';

CREATE TABLE repository_observations (
    observation_id TEXT PRIMARY KEY
        CHECK (observation_id = lower(observation_id) AND length(observation_id) = 36),
    repository_id TEXT NOT NULL
        REFERENCES repositories(repo_id) ON DELETE CASCADE,
    revision INTEGER NOT NULL CHECK (revision > 0),
    run_id TEXT
        REFERENCES repository_scan_runs_v2(run_id) ON DELETE SET NULL,
    source TEXT NOT NULL
        CHECK (source IN ('crawl', 'verifier', 'journal', 'watcher', 'upload', 'cloud', 'recovery')),
    source_event_key TEXT,
    source_cursor BLOB,
    path_hint TEXT,
    parent_node_id TEXT,
    name TEXT,
    name_key TEXT,
    entry_kind TEXT CHECK (entry_kind IS NULL OR entry_kind IN ('directory', 'file', 'symlink')),
    file_size INTEGER CHECK (file_size IS NULL OR file_size >= 0),
    modified_at_ns INTEGER,
    changed_at_ns INTEGER,
    native_identity_kind TEXT,
    native_identity_value TEXT,
    stability_token_before TEXT,
    stability_token_after TEXT,
    quick_fingerprint TEXT,
    quick_fingerprint_version TEXT,
    resolved_owner_id INTEGER REFERENCES users(user_id),
    mapped_node_id TEXT,
    processing_state TEXT NOT NULL DEFAULT 'pending'
        CHECK (processing_state IN ('pending', 'applied', 'superseded', 'retryable_error', 'terminal_unsupported')),
    failure_code TEXT,
    authoritative_child_set INTEGER NOT NULL DEFAULT 0
        CHECK (authoritative_child_set IN (0, 1)),
    created_at INTEGER NOT NULL,
    processed_at INTEGER,
    UNIQUE (repository_id, revision),
    FOREIGN KEY (repository_id, parent_node_id)
        REFERENCES repository_nodes(repository_id, node_id),
    FOREIGN KEY (repository_id, mapped_node_id)
        REFERENCES repository_nodes(repository_id, node_id)
) STRICT;

CREATE UNIQUE INDEX repository_observations_source_delivery
    ON repository_observations (repository_id, source, source_event_key)
    WHERE source_event_key IS NOT NULL;
CREATE INDEX idx_repository_observations_pending
    ON repository_observations (repository_id, processing_state, revision)
    WHERE processing_state IN ('pending', 'retryable_error');
CREATE INDEX idx_repository_observations_node_revision
    ON repository_observations (repository_id, mapped_node_id, revision DESC)
    WHERE mapped_node_id IS NOT NULL;

CREATE TABLE repository_scan_frontier (
    run_id TEXT NOT NULL
        REFERENCES repository_scan_runs_v2(run_id) ON DELETE CASCADE,
    directory_node_id TEXT NOT NULL
        REFERENCES repository_nodes(node_id) ON DELETE CASCADE,
    state TEXT NOT NULL DEFAULT 'pending'
        CHECK (state IN ('pending', 'leased', 'completed', 'error', 'absence')),
    purpose TEXT NOT NULL DEFAULT 'crawl'
        CHECK (purpose IN ('crawl', 'verify', 'absence')),
    lease_id TEXT,
    lease_expires_at INTEGER,
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    continuation_offset INTEGER NOT NULL DEFAULT 0 CHECK (continuation_offset >= 0),
    coverage_safe INTEGER NOT NULL DEFAULT 1 CHECK (coverage_safe IN (0, 1)),
    authoritative_child_set INTEGER NOT NULL DEFAULT 0
        CHECK (authoritative_child_set IN (0, 1)),
    absence_cursor TEXT NOT NULL DEFAULT '',
    absence_finalized INTEGER NOT NULL DEFAULT 0
        CHECK (absence_finalized IN (0, 1)),
    error_code TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (run_id, directory_node_id)
) STRICT;

CREATE INDEX idx_repository_scan_frontier_claim
    ON repository_scan_frontier (run_id, state, lease_expires_at, directory_node_id);

CREATE TABLE repository_change_cursors (
    repository_id TEXT NOT NULL
        REFERENCES repositories(repo_id) ON DELETE CASCADE,
    adapter_kind TEXT NOT NULL
        CHECK (adapter_kind IN ('usn', 'rdcw', 'fsevents', 'inotify', 'periodic')),
    cursor BLOB,
    volume_identity TEXT,
    journal_identity TEXT,
    status TEXT NOT NULL
        CHECK (status IN ('healthy', 'gap', 'overflow', 'unavailable')),
    applied_revision INTEGER NOT NULL DEFAULT 0 CHECK (applied_revision >= 0),
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (repository_id, adapter_kind)
) STRICT;

CREATE TABLE content_objects (
    content_id TEXT PRIMARY KEY
        CHECK (content_id = lower(content_id) AND length(content_id) = 36),
    hash_algorithm TEXT NOT NULL CHECK (hash_algorithm = 'blake3-v1'),
    full_hash TEXT NOT NULL CHECK (full_hash = lower(full_hash) AND length(full_hash) = 64),
    file_size INTEGER NOT NULL CHECK (file_size >= 0),
    created_at INTEGER NOT NULL,
    UNIQUE (hash_algorithm, full_hash, file_size)
) STRICT;

CREATE TABLE assets_v2 (
    asset_id TEXT PRIMARY KEY
        CHECK (asset_id = lower(asset_id) AND length(asset_id) = 36),
    owner_id INTEGER NOT NULL REFERENCES users(user_id),
    content_id TEXT NOT NULL REFERENCES content_objects(content_id),
    type TEXT NOT NULL CHECK (type IN ('PHOTO', 'VIDEO', 'AUDIO')),
    original_filename TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    width INTEGER,
    height INTEGER,
    duration REAL,
    upload_time INTEGER NOT NULL,
    taken_time INTEGER,
    capture_offset_minutes INTEGER
        CHECK (capture_offset_minutes IS NULL OR capture_offset_minutes BETWEEN -840 AND 840),
    is_deleted INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0, 1)),
    deleted_at INTEGER,
    specific_metadata TEXT CHECK (specific_metadata IS NULL OR json_valid(specific_metadata)),
    rating INTEGER,
    liked INTEGER NOT NULL DEFAULT 0 CHECK (liked IN (0, 1)),
    status TEXT NOT NULL DEFAULT '{"state":"processing","message":"Pending processing"}'
        CHECK (json_valid(status)),
    updated_at INTEGER NOT NULL,
    gps_latitude REAL CHECK (gps_latitude IS NULL OR gps_latitude BETWEEN -90 AND 90),
    gps_longitude REAL CHECK (gps_longitude IS NULL OR gps_longitude BETWEEN -180 AND 180),
    gps_geohash_5 TEXT,
    gps_geohash_7 TEXT,
    exif_raw TEXT CHECK (exif_raw IS NULL OR json_valid(exif_raw)),
    UNIQUE (owner_id, content_id)
) STRICT;

CREATE INDEX idx_assets_v2_taken_time ON assets_v2 (taken_time DESC, asset_id);
CREATE INDEX idx_assets_v2_owner_deleted ON assets_v2 (owner_id, is_deleted, asset_id);

CREATE TABLE asset_locations (
    location_id TEXT PRIMARY KEY
        CHECK (location_id = lower(location_id) AND length(location_id) = 36),
    node_id TEXT NOT NULL
        REFERENCES repository_nodes(node_id) ON DELETE CASCADE,
    asset_id TEXT NOT NULL
        REFERENCES assets_v2(asset_id) ON DELETE CASCADE,
    bound_observation_revision INTEGER NOT NULL CHECK (bound_observation_revision > 0),
    unbound_observation_revision INTEGER
        CHECK (unbound_observation_revision IS NULL OR unbound_observation_revision > bound_observation_revision),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE UNIQUE INDEX asset_locations_one_active_node
    ON asset_locations (node_id)
    WHERE unbound_observation_revision IS NULL;
CREATE INDEX idx_asset_locations_active_asset
    ON asset_locations (asset_id, node_id)
    WHERE unbound_observation_revision IS NULL;

CREATE TABLE repository_outbox (
    outbox_id TEXT PRIMARY KEY
        CHECK (outbox_id = lower(outbox_id) AND length(outbox_id) = 36),
    repository_id TEXT NOT NULL
        REFERENCES repositories(repo_id) ON DELETE CASCADE,
    effect_key TEXT NOT NULL UNIQUE,
    effect_kind TEXT NOT NULL
        CHECK (effect_kind IN ('hash', 'bind', 'process_asset', 'controller')),
    entity_id TEXT NOT NULL,
    expected_revision INTEGER NOT NULL CHECK (expected_revision >= 0),
    payload TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(payload)),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'delivering', 'delivered', 'dead')),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    lease_id TEXT,
    lease_expires_at INTEGER,
    last_failure_code TEXT,
    created_at INTEGER NOT NULL,
    delivered_at INTEGER,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE INDEX idx_repository_outbox_drain
    ON repository_outbox (effect_kind, status, lease_expires_at, created_at, outbox_id)
    WHERE status IN ('pending', 'delivering');
CREATE INDEX idx_repository_outbox_pending_repository
    ON repository_outbox (repository_id, status)
    WHERE status IN ('pending', 'delivering');
CREATE INDEX idx_repository_outbox_entity
    ON repository_outbox (repository_id, entity_id, expected_revision);
