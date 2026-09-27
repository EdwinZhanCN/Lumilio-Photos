-- Lumilio Photos catalog baseline.
--
-- This is the standalone schema for catalog schema version 1, the
-- v26.1.0-rc.1 compatibility baseline. Until the rc.1 tag it is edited in
-- place; from the tag on it is frozen, and later schema changes are numbered
-- forward steps. PRAGMA user_version is the only version discriminator; there
-- is no checksum ledger.
--
-- Every ordinary table is STRICT. FTS5 and Vec1 virtual tables own their
-- shadow/internal tables, which the modules create and this file therefore
-- does not declare. This schema never stores or modifies original media.

-- ===== TABLES (96) =====

CREATE TABLE "assets" (
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

CREATE TABLE "events" (
    event_id TEXT PRIMARY KEY
        CHECK (event_id = lower(event_id) AND length(event_id) = 36),
    owner_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'redirected', 'retired')),
    start_at INTEGER NOT NULL,
    end_at INTEGER NOT NULL,
    timezone TEXT,
    generated_title TEXT,
    title_override TEXT,
    generated_cover_media_item_id TEXT
        REFERENCES media_items(media_item_id) ON DELETE SET NULL,
    cover_override_media_item_id TEXT
        REFERENCES media_items(media_item_id) ON DELETE SET NULL,
    is_hidden INTEGER NOT NULL DEFAULT 0 CHECK (is_hidden IN (0, 1)),
    algorithm_version TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    CHECK (start_at <= end_at),
    UNIQUE (event_id, owner_id)
) STRICT;

CREATE TABLE "repository_scan_runs" (
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
, full_verification_performed INTEGER NOT NULL DEFAULT 0 CHECK (full_verification_performed IN (0,1))) STRICT;

CREATE TABLE agent_checkpoints (
    id TEXT PRIMARY KEY,
    data BLOB NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE agent_pending_effects (
    effect_id TEXT PRIMARY KEY CHECK (effect_id = lower(effect_id) AND length(effect_id) = 36),
    user_id INTEGER NOT NULL,
    thread_id TEXT NOT NULL,
    initiating_run_id TEXT NOT NULL REFERENCES agent_runs(run_id) ON DELETE CASCADE,
    executing_run_id TEXT REFERENCES agent_runs(run_id) ON DELETE SET NULL,
    tool_name TEXT NOT NULL,
    effect_class TEXT NOT NULL,
    policy_version INTEGER NOT NULL,
    membership_snapshot TEXT NOT NULL DEFAULT '[]'
        CHECK (json_valid(membership_snapshot) AND json_type(membership_snapshot) = 'array'),
    payload TEXT NOT NULL CHECK (json_valid(payload)),
    target TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(target)),
    idempotency_key TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'committed', 'rejected', 'cancelled', 'failed')),
    receipt TEXT CHECK (receipt IS NULL OR json_valid(receipt)),
    created_at INTEGER NOT NULL,
    committed_at INTEGER,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (user_id, thread_id) REFERENCES agent_threads(user_id, thread_id) ON DELETE CASCADE
) STRICT;

CREATE TABLE agent_pins (
    pin_id TEXT PRIMARY KEY CHECK (pin_id = lower(pin_id) AND length(pin_id) = 36),
    user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    title TEXT NOT NULL DEFAULT '',
    widget TEXT NOT NULL DEFAULT 'cover_card',
    mode TEXT NOT NULL DEFAULT 'frozen' CHECK (mode IN ('frozen', 'live')),
    plan TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(plan)),
    summary TEXT NOT NULL DEFAULT '',
    asset_ids TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(asset_ids) AND json_type(asset_ids) = 'array'),
    truncated INTEGER NOT NULL DEFAULT 0 CHECK (truncated IN (0, 1)),
    layout_x INTEGER NOT NULL DEFAULT 0,
    layout_y INTEGER NOT NULL DEFAULT 0,
    layout_w INTEGER NOT NULL DEFAULT 4,
    layout_h INTEGER NOT NULL DEFAULT 4,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    last_successful_refresh_at INTEGER
) STRICT;

CREATE TABLE agent_refs (
    user_id INTEGER NOT NULL,
    thread_id TEXT NOT NULL,
    ref_id TEXT NOT NULL,
    sequence INTEGER NOT NULL,
    plan TEXT NOT NULL CHECK (json_valid(plan)),
    asset_ids TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(asset_ids) AND json_type(asset_ids) = 'array'),
    summary TEXT NOT NULL DEFAULT '',
    truncated INTEGER NOT NULL DEFAULT 0 CHECK (truncated IN (0, 1)),
    created_at INTEGER NOT NULL,
    last_accessed_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    PRIMARY KEY (user_id, thread_id, ref_id),
    FOREIGN KEY (user_id, thread_id) REFERENCES agent_threads(user_id, thread_id) ON DELETE CASCADE
) STRICT;

CREATE TABLE agent_runs (
    run_id TEXT PRIMARY KEY CHECK (run_id = lower(run_id) AND length(run_id) = 36),
    user_id INTEGER NOT NULL,
    thread_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'cancel_requested', 'awaiting_confirmation', 'cancelled', 'completed', 'failed')),
    cancel_requested_at INTEGER,
    started_at INTEGER NOT NULL,
    finished_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL, activation_state TEXT NOT NULL DEFAULT 'active'
        CHECK (activation_state IN ('prepared_resume', 'active', 'terminal')),
    FOREIGN KEY (user_id, thread_id) REFERENCES agent_threads(user_id, thread_id) ON DELETE CASCADE
) STRICT;

CREATE TABLE agent_threads (
    user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    thread_id TEXT NOT NULL,
    checkpoint_key TEXT NOT NULL UNIQUE,
    mode TEXT NOT NULL CHECK (mode IN ('free', 'review', 'organize', 'analyze', 'curate')),
    context_bindings TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(context_bindings) AND json_type(context_bindings) = 'array'),
    mention_bindings TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(mention_bindings) AND json_type(mention_bindings) = 'array'),
    policy_version INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'awaiting_confirmation', 'completed', 'cancelled', 'failed')),
    active_run_id TEXT REFERENCES agent_runs(run_id) ON DELETE SET NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (user_id, thread_id)
) STRICT;

CREATE TABLE album_assets (
    album_id INTEGER NOT NULL REFERENCES albums(album_id) ON DELETE CASCADE,
    asset_id TEXT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    added_time INTEGER NOT NULL,
    PRIMARY KEY (album_id, asset_id)
) STRICT;

CREATE TABLE albums (
    album_id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(user_id),
    album_name TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    description TEXT,
    cover_asset_id TEXT REFERENCES assets(asset_id) ON DELETE SET NULL,
    album_type TEXT NOT NULL DEFAULT 'default' CHECK (album_type IN ('default', 'smart', 'bio'))
) STRICT;

CREATE TABLE asset_locations (
    location_id TEXT PRIMARY KEY
        CHECK (location_id = lower(location_id) AND length(location_id) = 36),
    node_id TEXT NOT NULL
        REFERENCES repository_nodes(node_id) ON DELETE CASCADE,
    asset_id TEXT NOT NULL
        REFERENCES "assets"(asset_id) ON DELETE CASCADE,
    bound_observation_revision INTEGER NOT NULL CHECK (bound_observation_revision > 0),
    unbound_observation_revision INTEGER
        CHECK (unbound_observation_revision IS NULL OR unbound_observation_revision > bound_observation_revision),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

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

CREATE TABLE asset_pipeline_receipt_stages (
    receipt_id TEXT NOT NULL REFERENCES catalog_operation_receipts(receipt_id) ON DELETE CASCADE,
    asset_id TEXT NOT NULL,
    stage TEXT NOT NULL,
    desired_version INTEGER NOT NULL CHECK (desired_version > 0),
    PRIMARY KEY (receipt_id, asset_id, stage),
    FOREIGN KEY (asset_id, stage) REFERENCES asset_pipeline_state(asset_id, stage) ON DELETE CASCADE
) STRICT;

CREATE TABLE asset_pipeline_state (
    asset_id TEXT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    source_content_id TEXT NOT NULL REFERENCES content_objects(content_id),
    stage TEXT NOT NULL CHECK (stage IN ('analyze','derivatives','transcode','enrich')),
    pipeline_version TEXT NOT NULL,
    desired_version INTEGER NOT NULL CHECK (desired_version > 0),
    applied_version INTEGER NOT NULL DEFAULT 0
        CHECK (applied_version >= 0 AND applied_version <= desired_version),
    priority INTEGER NOT NULL CHECK (priority BETWEEN 1 AND 3),
    terminal_error TEXT,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (asset_id, stage)
) STRICT;

CREATE TABLE asset_quality_scores (
    asset_id TEXT PRIMARY KEY REFERENCES assets(asset_id) ON DELETE CASCADE,
    score REAL NOT NULL CHECK (score BETWEEN 1 AND 10),
    model_version TEXT NOT NULL DEFAULT 'v1',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE asset_reindex_requests (
    receipt_id TEXT PRIMARY KEY REFERENCES catalog_operation_receipts(receipt_id) ON DELETE CASCADE,
    repository_id TEXT REFERENCES repositories(repo_id) ON DELETE CASCADE,
    tasks TEXT NOT NULL CHECK (json_valid(tasks)),
    page_limit INTEGER NOT NULL CHECK (page_limit BETWEEN 1 AND 500),
    cursor TEXT,
    missing_only INTEGER NOT NULL CHECK (missing_only IN (0,1)),
    reset_semantic INTEGER NOT NULL CHECK (reset_semantic IN (0,1)),
    priority INTEGER NOT NULL CHECK (priority BETWEEN 1 AND 3),
    requested_revision INTEGER NOT NULL DEFAULT 1 CHECK (requested_revision > 0),
    applied_revision INTEGER NOT NULL DEFAULT 0 CHECK (applied_revision BETWEEN 0 AND requested_revision),
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE asset_stack_members (
    media_item_id TEXT PRIMARY KEY REFERENCES media_items(media_item_id) ON DELETE CASCADE,
    stack_id TEXT NOT NULL REFERENCES asset_stacks(stack_id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
) STRICT;

CREATE TABLE asset_stacks (
    stack_id TEXT PRIMARY KEY CHECK (stack_id = lower(stack_id) AND length(stack_id) = 36),
    owner_id INTEGER REFERENCES users(user_id),
    repository_id TEXT REFERENCES repositories(repo_id) ON DELETE CASCADE,
    stack_kind TEXT NOT NULL DEFAULT 'manual' CHECK (stack_kind IN ('manual', 'burst')),
    cover_media_item_id TEXT REFERENCES media_items(media_item_id) ON DELETE SET NULL,
    group_key TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE asset_tags (
    asset_id TEXT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    tag_id INTEGER NOT NULL REFERENCES tags(tag_id) ON DELETE CASCADE,
    confidence REAL NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    source TEXT NOT NULL DEFAULT 'system'
        CHECK (source IN ('system', 'user', 'ai', 'bioclip_classify', 'zeroshot')),
    PRIMARY KEY (asset_id, tag_id)
) STRICT;

CREATE TABLE auth_security_verifications (
    verification_id TEXT PRIMARY KEY CHECK (verification_id = lower(verification_id) AND length(verification_id) = 36),
    token_hash TEXT NOT NULL UNIQUE CHECK (length(token_hash) = 64),
    user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    auth_version INTEGER NOT NULL CHECK (auth_version >= 0),
    purpose TEXT NOT NULL CHECK (purpose IN ('totp_setup', 'totp_disable', 'recovery_regenerate', 'passkey_mutation')),
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL CHECK (expires_at > created_at),
    consumed_at INTEGER,
    CHECK (consumed_at IS NULL OR consumed_at >= created_at)
) STRICT;

CREATE TABLE catalog_backup_requests (
    receipt_id TEXT PRIMARY KEY REFERENCES catalog_operation_receipts(receipt_id) ON DELETE CASCADE,
    force INTEGER NOT NULL CHECK (force IN (0,1)),
    priority INTEGER NOT NULL CHECK (priority BETWEEN 1 AND 3)
) STRICT;

CREATE TABLE catalog_operation_receipts (
    receipt_id TEXT PRIMARY KEY
        CHECK (receipt_id = lower(receipt_id) AND length(receipt_id) = 36),
    kind TEXT NOT NULL CHECK (kind IN ('ingest','reprocess','retry','reindex','rebuild','backup')),
    subject_id TEXT NOT NULL,
    desired_version INTEGER NOT NULL CHECK (desired_version > 0),
    applied_version INTEGER NOT NULL DEFAULT 0
        CHECK (applied_version >= 0 AND applied_version <= desired_version),
    state TEXT NOT NULL DEFAULT 'pending'
        CHECK (state IN ('pending','completed','failed')),
    terminal_error TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE classifier_definitions (
    id INTEGER PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    tag_name TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT 'smart_album',
    positive_prompts TEXT NOT NULL CHECK (json_valid(positive_prompts) AND json_type(positive_prompts) = 'array'),
    negative_prompts TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(negative_prompts) AND json_type(negative_prompts) = 'array'),
    threshold REAL NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    positive_prototype BLOB,
    negative_prototype BLOB,
    prototype_model TEXT,
    prototype_dimensions INTEGER,
    prototype_built_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE cloud_credentials (
    credential_id TEXT PRIMARY KEY CHECK (credential_id = lower(credential_id) AND length(credential_id) = 36),
    provider TEXT NOT NULL,
    display_name TEXT NOT NULL,
    identity_hash TEXT NOT NULL,
    masked_identity TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'connected',
    artifact_dir TEXT,
    owner_id INTEGER NOT NULL REFERENCES users(user_id),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    public_config TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(public_config)),
    secret_ciphertext BLOB,
    UNIQUE (provider, identity_hash)
) STRICT;

CREATE TABLE cloud_import_runs (
    run_id TEXT PRIMARY KEY CHECK (run_id = lower(run_id) AND length(run_id) = 36),
    repository_id TEXT NOT NULL REFERENCES repositories(repo_id) ON DELETE CASCADE,
    credential_id TEXT NOT NULL REFERENCES cloud_credentials(credential_id) ON DELETE RESTRICT,
    owner_id INTEGER NOT NULL REFERENCES users(user_id),
    provider TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'cancelling', 'completed', 'failed', 'interrupted', 'cancelled')),
    resume_of_run_id TEXT REFERENCES cloud_import_runs(run_id) ON DELETE SET NULL,
    total_seen INTEGER NOT NULL DEFAULT 0,
    downloaded_count INTEGER NOT NULL DEFAULT 0,
    imported_count INTEGER NOT NULL DEFAULT 0,
    skipped_count INTEGER NOT NULL DEFAULT 0,
    failed_count INTEGER NOT NULL DEFAULT 0,
    error TEXT,
    started_at INTEGER,
    finished_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE cloud_sync_cursors (
    repository_id TEXT NOT NULL REFERENCES repositories(repo_id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    cursor_value TEXT NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL,
    credential_id TEXT NOT NULL REFERENCES cloud_credentials(credential_id) ON DELETE CASCADE,
    PRIMARY KEY (repository_id, credential_id, provider)
) STRICT;

CREATE TABLE cloud_sync_files (
    repository_id TEXT NOT NULL REFERENCES repositories(repo_id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    remote_key TEXT NOT NULL,
    etag TEXT NOT NULL DEFAULT '',
    local_hash TEXT NOT NULL DEFAULT '',
    asset_id TEXT REFERENCES assets(asset_id) ON DELETE SET NULL,
    synced_at INTEGER NOT NULL,
    credential_id TEXT NOT NULL REFERENCES cloud_credentials(credential_id) ON DELETE CASCADE,
    PRIMARY KEY (repository_id, credential_id, provider, remote_key)
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

CREATE TABLE duplicate_group_assets (
    group_id TEXT NOT NULL REFERENCES duplicate_groups(group_id) ON DELETE CASCADE,
    asset_id TEXT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'candidate' CHECK (role IN ('candidate', 'keeper', 'duplicate')),
    file_size INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (group_id, asset_id)
) STRICT;

CREATE TABLE duplicate_group_edges (
    group_id TEXT NOT NULL REFERENCES duplicate_groups(group_id) ON DELETE CASCADE,
    asset_id_a TEXT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    asset_id_b TEXT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    method TEXT NOT NULL CHECK (method IN ('exact', 'phash')),
    distance REAL NOT NULL DEFAULT 0,
    confidence REAL NOT NULL DEFAULT 1 CHECK (confidence BETWEEN 0 AND 1),
    PRIMARY KEY (group_id, asset_id_a, asset_id_b, method),
    CHECK (asset_id_a < asset_id_b)
) STRICT;

CREATE TABLE duplicate_groups (
    group_id TEXT PRIMARY KEY CHECK (group_id = lower(group_id) AND length(group_id) = 36),
    repository_id TEXT NOT NULL REFERENCES repositories(repo_id) ON DELETE CASCADE,
    owner_id INTEGER REFERENCES users(user_id) ON DELETE CASCADE,
    method TEXT NOT NULL CHECK (method IN ('exact', 'phash', 'mixed')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'merged', 'dismissed')),
    asset_count INTEGER NOT NULL DEFAULT 0,
    total_size INTEGER NOT NULL DEFAULT 0,
    recommended_keeper_asset_id TEXT REFERENCES assets(asset_id) ON DELETE SET NULL,
    keeper_asset_id TEXT REFERENCES assets(asset_id) ON DELETE SET NULL,
    detection_version TEXT NOT NULL DEFAULT 'duplicates-v1',
    detected_at INTEGER NOT NULL,
    resolved_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE embedding_spaces (
    id INTEGER PRIMARY KEY,
    embedding_type TEXT NOT NULL,
    model_id TEXT NOT NULL,
    dimensions INTEGER NOT NULL CHECK (dimensions > 0),
    distance_metric TEXT NOT NULL CHECK (distance_metric = 'l2'),
    search_enabled INTEGER NOT NULL DEFAULT 0 CHECK (search_enabled IN (0, 1)),
    is_default_search INTEGER NOT NULL DEFAULT 0 CHECK (is_default_search IN (0, 1)),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE embeddings (
    id INTEGER PRIMARY KEY,
    asset_id TEXT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    embedding_type TEXT NOT NULL,
    embedding_model TEXT NOT NULL,
    embedding_dimensions INTEGER NOT NULL CHECK (embedding_dimensions > 0),
    vector BLOB,
    is_primary INTEGER NOT NULL DEFAULT 0 CHECK (is_primary IN (0, 1)),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    space_id INTEGER NOT NULL REFERENCES embedding_spaces(id) ON DELETE RESTRICT,
    UNIQUE (asset_id, embedding_type, embedding_model)
) STRICT;

CREATE TABLE event_constraints (
    constraint_id TEXT PRIMARY KEY
        CHECK (constraint_id = lower(constraint_id) AND length(constraint_id) = 36),
    owner_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('include', 'exclude', 'must_link', 'cannot_link')),
    event_id TEXT,
    left_media_item_id TEXT NOT NULL,
    right_media_item_id TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (event_id, owner_id)
        REFERENCES events(event_id, owner_id) ON DELETE CASCADE,
    FOREIGN KEY (left_media_item_id, owner_id)
        REFERENCES media_items(media_item_id, owner_id) ON DELETE CASCADE,
    FOREIGN KEY (right_media_item_id, owner_id)
        REFERENCES media_items(media_item_id, owner_id) ON DELETE CASCADE,
    CHECK (
        (kind IN ('include', 'exclude') AND event_id IS NOT NULL AND right_media_item_id IS NULL)
        OR
        (kind IN ('must_link', 'cannot_link') AND event_id IS NULL
            AND right_media_item_id IS NOT NULL
            AND left_media_item_id < right_media_item_id)
    )
) STRICT;

CREATE TABLE event_dirty_ranges (
    dirty_range_id TEXT PRIMARY KEY
        CHECK (dirty_range_id = lower(dirty_range_id) AND length(dirty_range_id) = 36),
    owner_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    range_start INTEGER NOT NULL,
    range_end INTEGER NOT NULL,
    reason TEXT NOT NULL,
    claimed_at INTEGER,
    claim_token TEXT
        CHECK (claim_token IS NULL OR
               (claim_token = lower(claim_token) AND length(claim_token) = 36)),
    created_at INTEGER NOT NULL,
    CHECK (range_start <= range_end),
    CHECK ((claimed_at IS NULL) = (claim_token IS NULL))
) STRICT;

CREATE TABLE event_media_items (
    event_id TEXT NOT NULL,
    owner_id INTEGER NOT NULL,
    media_item_id TEXT NOT NULL,
    position INTEGER NOT NULL CHECK (position >= 0),
    source TEXT NOT NULL DEFAULT 'automatic'
        CHECK (source IN ('automatic', 'user')),
    confidence REAL CHECK (confidence IS NULL OR confidence BETWEEN 0 AND 1),
    evidence TEXT NOT NULL DEFAULT '{}'
        CHECK (json_valid(evidence) AND json_type(evidence) = 'object'),
    derivation_run_id TEXT
        CHECK (derivation_run_id IS NULL OR
               (derivation_run_id = lower(derivation_run_id) AND length(derivation_run_id) = 36)),
    created_at INTEGER NOT NULL,
    PRIMARY KEY (event_id, media_item_id),
    FOREIGN KEY (event_id, owner_id)
        REFERENCES events(event_id, owner_id) ON DELETE CASCADE,
    FOREIGN KEY (media_item_id, owner_id)
        REFERENCES media_items(media_item_id, owner_id) ON DELETE CASCADE
) STRICT;

CREATE TABLE event_owner_state (
    owner_id INTEGER PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    active_algorithm_version TEXT NOT NULL,
    initialized_at INTEGER NOT NULL,
    last_full_rebuild_at INTEGER,
    automatic_rebuild_paused INTEGER NOT NULL DEFAULT 0
        CHECK (automatic_rebuild_paused IN (0, 1)),
    revision INTEGER NOT NULL DEFAULT 0 CHECK (revision >= 0),
    updated_at INTEGER NOT NULL
, source_revision INTEGER NOT NULL DEFAULT 0, published_revision INTEGER NOT NULL DEFAULT 0, rebuild_lease_token TEXT, rebuild_lease_expires_at INTEGER) STRICT;

CREATE TABLE event_projection_pipeline_state (
    owner_id INTEGER PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    source_revision INTEGER NOT NULL CHECK (source_revision > 0),
    projection_version INTEGER NOT NULL CHECK (projection_version > 0),
    applied_revision INTEGER NOT NULL DEFAULT 0
        CHECK (applied_revision >= 0 AND applied_revision <= source_revision),
    priority INTEGER NOT NULL CHECK (priority BETWEEN 1 AND 3),
    cursor TEXT,
    terminal_error TEXT,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE event_rebuild_runs (
    run_id TEXT PRIMARY KEY
        CHECK (run_id = lower(run_id) AND length(run_id) = 36),
    owner_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    state TEXT NOT NULL CHECK (state IN ('queued','running','succeeded','failed','stale')),
    requested_revision INTEGER NOT NULL,
    published_revision INTEGER,
    requested_at INTEGER NOT NULL,
    started_at INTEGER,
    finished_at INTEGER,
    event_count INTEGER NOT NULL DEFAULT 0,
    member_count INTEGER NOT NULL DEFAULT 0,
    error_code TEXT
) STRICT;

CREATE TABLE event_redirects (
    old_event_id TEXT PRIMARY KEY
        CHECK (old_event_id = lower(old_event_id) AND length(old_event_id) = 36),
    owner_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    new_event_id TEXT NOT NULL,
    reason TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    CHECK (old_event_id != new_event_id),
    FOREIGN KEY (old_event_id, owner_id)
        REFERENCES events(event_id, owner_id) ON DELETE CASCADE,
    FOREIGN KEY (new_event_id, owner_id)
        REFERENCES events(event_id, owner_id) ON DELETE CASCADE
) STRICT;

CREATE TABLE face_cluster_members (
    id INTEGER PRIMARY KEY,
    cluster_id INTEGER NOT NULL REFERENCES face_clusters(cluster_id) ON DELETE CASCADE,
    face_id INTEGER NOT NULL REFERENCES face_items(id) ON DELETE CASCADE,
    similarity_score REAL NOT NULL CHECK (similarity_score BETWEEN 0 AND 1),
    confidence REAL NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    is_manual INTEGER NOT NULL DEFAULT 0 CHECK (is_manual IN (0, 1)),
    created_at INTEGER NOT NULL,
    UNIQUE (cluster_id, face_id),
    UNIQUE (face_id)
) STRICT;

CREATE TABLE face_clusters (
    cluster_id INTEGER PRIMARY KEY,
    owner_id INTEGER REFERENCES users(user_id) ON DELETE CASCADE,
    cluster_name TEXT,
    representative_face_id INTEGER REFERENCES face_items(id) ON DELETE SET NULL,
    confidence_score REAL NOT NULL DEFAULT 0,
    member_count INTEGER NOT NULL DEFAULT 0 CHECK (member_count >= 0),
    is_confirmed INTEGER NOT NULL DEFAULT 0 CHECK (is_confirmed IN (0, 1)),
    is_hidden INTEGER NOT NULL DEFAULT 0 CHECK (is_hidden IN (0, 1)),
    hidden_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE face_items (
    id INTEGER PRIMARY KEY,
    asset_id TEXT NOT NULL REFERENCES face_results(asset_id) ON DELETE CASCADE,
    face_id TEXT,
    bounding_box TEXT NOT NULL CHECK (json_valid(bounding_box)),
    confidence REAL NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    age_group TEXT,
    gender TEXT,
    ethnicity TEXT,
    expression TEXT,
    face_size INTEGER,
    face_image_path TEXT,
    embedding BLOB,
    embedding_model TEXT,
    is_primary INTEGER NOT NULL DEFAULT 0 CHECK (is_primary IN (0, 1)),
    quality_score REAL CHECK (quality_score IS NULL OR quality_score BETWEEN 0 AND 1),
    blur_score REAL CHECK (blur_score IS NULL OR blur_score BETWEEN 0 AND 1),
    pose_angles TEXT CHECK (pose_angles IS NULL OR json_valid(pose_angles)),
    created_at INTEGER NOT NULL
, repository_id TEXT
    REFERENCES repositories(repo_id) ON DELETE CASCADE) STRICT;

CREATE TABLE face_results (
    asset_id TEXT PRIMARY KEY REFERENCES assets(asset_id) ON DELETE CASCADE,
    model_id TEXT NOT NULL,
    total_faces INTEGER NOT NULL DEFAULT 0 CHECK (total_faces >= 0),
    processing_time_ms INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE host_actions (
    action_id TEXT PRIMARY KEY
        CHECK (action_id = lower(action_id) AND length(action_id) = 36),
    request_id TEXT NOT NULL UNIQUE CHECK (length(request_id) BETWEEN 1 AND 200),
    request_hash TEXT NOT NULL CHECK (length(request_hash) = 64),
    kind TEXT NOT NULL CHECK (kind IN (
        'authorize_storage_location',
        'open_repository',
        'locate_storage_location',
        'locate_repository'
    )),
    actor TEXT NOT NULL CHECK (length(actor) BETWEEN 1 AND 200),
    actor_user_id INTEGER REFERENCES users(user_id) ON DELETE SET NULL,
    host_instance_id TEXT NOT NULL DEFAULT '',
    session_id TEXT NOT NULL DEFAULT '',
    request_summary TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(request_summary)),
    expected_version INTEGER NOT NULL DEFAULT 0 CHECK (expected_version >= 0),
    nonce TEXT NOT NULL CHECK (length(nonce) >= 32),
    status TEXT NOT NULL CHECK (status IN (
        'pending', 'running', 'needs_decision', 'succeeded',
        'failed', 'cancelled', 'expired'
    )),
    selected_path TEXT,
    result TEXT CHECK (result IS NULL OR json_valid(result)),
    error_code TEXT,
    error_message TEXT,
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    completed_at INTEGER
) STRICT;

CREATE TABLE lifecycle_audit_events (
    event_id TEXT PRIMARY KEY
        CHECK (event_id = lower(event_id) AND length(event_id) = 36),
    occurred_at INTEGER NOT NULL,
    actor TEXT NOT NULL CHECK (length(actor) BETWEEN 1 AND 200),
    actor_user_id INTEGER REFERENCES users(user_id) ON DELETE SET NULL,
    host_instance_id TEXT NOT NULL DEFAULT '',
    request_id TEXT NOT NULL DEFAULT '',
    operation_id TEXT,
    action TEXT NOT NULL CHECK (length(action) BETWEEN 1 AND 100),
    target_type TEXT NOT NULL CHECK (target_type IN ('repository', 'storage_location', 'runtime_config')),
    target_id TEXT,
    source TEXT NOT NULL CHECK (source IN ('web', 'desktop_host', 'server', 'recovery', 'test')),
    confirmation_type TEXT NOT NULL DEFAULT 'none',
    old_path TEXT,
    new_path TEXT,
    result TEXT NOT NULL CHECK (result IN ('succeeded', 'failed', 'rejected', 'recovered')),
    failure_stage TEXT,
    details TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(details))
) STRICT;

CREATE TABLE lifecycle_operations (
    operation_id TEXT PRIMARY KEY
        CHECK (operation_id = lower(operation_id) AND length(operation_id) = 36),
    request_id TEXT NOT NULL UNIQUE CHECK (length(request_id) BETWEEN 1 AND 200),
    kind TEXT NOT NULL CHECK (kind IN (
        'create_repository',
        'create_storage_location',
        'open_repository',
        'register_repository_copy',
        'switch_default_storage_location',
        'relocate_storage_location',
        'rename_repository'
    )),
    payload_hash TEXT NOT NULL CHECK (length(payload_hash) = 64),
    payload TEXT NOT NULL CHECK (json_valid(payload)),
    actor TEXT NOT NULL CHECK (length(actor) BETWEEN 1 AND 200),
    actor_user_id INTEGER REFERENCES users(user_id) ON DELETE SET NULL,
    host_instance_id TEXT NOT NULL DEFAULT '',
    target_type TEXT NOT NULL CHECK (target_type IN ('repository', 'storage_location', 'runtime_config')),
    target_id TEXT,
    phase TEXT NOT NULL CHECK (phase IN (
        'prepared',
        'filesystem_applied',
        'catalog_committed',
        'rollback_required',
        'completed',
        'failed'
    )),
    status TEXT NOT NULL CHECK (status IN ('running', 'completed', 'failed', 'rolled_back')),
    result TEXT CHECK (result IS NULL OR json_valid(result)),
    rollback_data TEXT CHECK (rollback_data IS NULL OR json_valid(rollback_data)),
    error TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    completed_at INTEGER
) STRICT;

CREATE TABLE location_cluster_assets (
    cluster_id TEXT NOT NULL REFERENCES location_clusters(cluster_id) ON DELETE CASCADE,
    asset_id TEXT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (cluster_id, asset_id)
) STRICT;

CREATE TABLE location_clusters (
    cluster_id TEXT PRIMARY KEY CHECK (cluster_id = lower(cluster_id) AND length(cluster_id) = 36),
    owner_id INTEGER REFERENCES users(user_id) ON DELETE CASCADE,
    repository_id TEXT NOT NULL REFERENCES repositories(repo_id) ON DELETE CASCADE,
    geohash TEXT NOT NULL,
    precision INTEGER NOT NULL DEFAULT 7 CHECK (precision > 0),
    centroid_latitude REAL NOT NULL CHECK (centroid_latitude BETWEEN -90 AND 90),
    centroid_longitude REAL NOT NULL CHECK (centroid_longitude BETWEEN -180 AND 180),
    photo_count INTEGER NOT NULL DEFAULT 0 CHECK (photo_count >= 0),
    label TEXT,
    country TEXT,
    region TEXT,
    city TEXT,
    provider TEXT,
    geocode_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (geocode_status IN ('pending', 'disabled', 'cached', 'resolved', 'failed')),
    geocoded_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
, geocode_attempt_count INTEGER NOT NULL DEFAULT 0
        CHECK (geocode_attempt_count >= 0), geocode_next_attempt_at INTEGER) STRICT;

CREATE TABLE location_projection_receipt_scopes (
    receipt_id TEXT NOT NULL REFERENCES catalog_operation_receipts(receipt_id) ON DELETE CASCADE,
    repository_id TEXT NOT NULL,
    owner_id INTEGER NOT NULL,
    desired_revision INTEGER NOT NULL CHECK (desired_revision > 0),
    PRIMARY KEY (receipt_id, repository_id, owner_id),
    FOREIGN KEY (repository_id, owner_id)
        REFERENCES location_projection_state(repository_id, owner_id) ON DELETE CASCADE
) STRICT;

CREATE TABLE location_projection_state (
    repository_id TEXT NOT NULL
        REFERENCES repositories(repo_id) ON DELETE CASCADE,
    owner_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    source_revision INTEGER NOT NULL DEFAULT 1 CHECK (source_revision > 0),
    published_revision INTEGER NOT NULL DEFAULT 0
        CHECK (published_revision >= 0 AND published_revision <= source_revision),
    updated_at INTEGER NOT NULL, terminal_error TEXT,
    PRIMARY KEY (repository_id, owner_id)
) STRICT;

CREATE TABLE location_resolution_pipeline_state (
    scope TEXT PRIMARY KEY CHECK (scope = 'all'),
    source_revision INTEGER NOT NULL CHECK (source_revision > 0),
    projection_version INTEGER NOT NULL CHECK (projection_version > 0),
    applied_revision INTEGER NOT NULL DEFAULT 0
        CHECK (applied_revision >= 0 AND applied_revision <= projection_version),
    terminal_error TEXT,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE media_item_assets (
    asset_id TEXT PRIMARY KEY REFERENCES assets(asset_id) ON DELETE CASCADE,
    media_item_id TEXT NOT NULL REFERENCES media_items(media_item_id) ON DELETE CASCADE,
    relation TEXT NOT NULL DEFAULT 'alternative'
        CHECK (relation IN (
            'original', 'alternative', 'component', 'preview',
            'raw_original', 'jpeg_original', 'edited_version',
            'live_photo_still', 'live_photo_video'
        )),
    position INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
) STRICT;

CREATE TABLE media_items (
    media_item_id TEXT PRIMARY KEY CHECK (media_item_id = lower(media_item_id) AND length(media_item_id) = 36),
    owner_id INTEGER REFERENCES users(user_id),
    repository_id TEXT REFERENCES repositories(repo_id) ON DELETE CASCADE,
    media_kind TEXT NOT NULL DEFAULT 'photo' CHECK (media_kind IN ('photo', 'video', 'audio', 'live_photo')),
    primary_asset_id TEXT REFERENCES assets(asset_id) ON DELETE SET NULL,
    group_key TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE music_album_artists (
    album_id TEXT NOT NULL REFERENCES music_albums(album_id) ON DELETE CASCADE,
    artist_id TEXT NOT NULL REFERENCES music_artists(artist_id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position >= 0),
    role TEXT NOT NULL DEFAULT 'album_artist'
        CHECK (role IN ('album_artist', 'album_credit')),
    PRIMARY KEY (album_id, position),
    UNIQUE (album_id, artist_id, position)
) STRICT;

CREATE TABLE music_albums (
    album_id TEXT PRIMARY KEY CHECK (album_id = lower(album_id) AND length(album_id) = 36),
    owner_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    title TEXT NOT NULL CHECK (length(trim(title)) > 0),
    release_date TEXT,
    release_precision TEXT NOT NULL DEFAULT 'unknown'
        CHECK (release_precision IN ('unknown', 'year', 'month', 'day')),
    edition TEXT NOT NULL DEFAULT '',
    release_identifier TEXT NOT NULL DEFAULT '',
    source_group TEXT NOT NULL DEFAULT '',
    artist_source TEXT NOT NULL DEFAULT 'extracted'
        CHECK (artist_source IN ('extracted', 'manual')),
    cover_asset_id TEXT REFERENCES assets(asset_id) ON DELETE SET NULL,
    revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
, favorite INTEGER NOT NULL DEFAULT 0 CHECK (favorite IN (0, 1))) STRICT;

CREATE TABLE music_artists (
    artist_id TEXT PRIMARY KEY CHECK (artist_id = lower(artist_id) AND length(artist_id) = 36),
    owner_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    display_name TEXT NOT NULL CHECK (length(trim(display_name)) > 0),
    normalized_name TEXT NOT NULL,
    external_id TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
, favorite INTEGER NOT NULL DEFAULT 0 CHECK (favorite IN (0, 1))) STRICT;

CREATE TABLE music_playback_entries (
    entry_id TEXT NOT NULL UNIQUE CHECK (entry_id = lower(entry_id) AND length(entry_id) = 36),
    session_id TEXT NOT NULL REFERENCES music_playback_sessions(session_id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL CHECK (sequence >= 0),
    track_id TEXT REFERENCES music_tracks(track_id) ON DELETE SET NULL,
    source_entry_id TEXT,
    saved_title TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (session_id, sequence)
) STRICT;

CREATE TABLE music_playback_sessions (
    session_id TEXT PRIMARY KEY CHECK (session_id = lower(session_id) AND length(session_id) = 36),
    owner_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    source_kind TEXT NOT NULL CHECK (source_kind IN ('query', 'album', 'playlist', 'liked')),
    source_id TEXT NOT NULL DEFAULT '',
    source_revision INTEGER NOT NULL DEFAULT 0 CHECK (source_revision >= 0),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'expired', 'deleted')),
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE music_playlist_entries (
    entry_id TEXT PRIMARY KEY CHECK (entry_id = lower(entry_id) AND length(entry_id) = 36),
    playlist_id TEXT NOT NULL REFERENCES music_playlists(playlist_id) ON DELETE CASCADE,
    track_id TEXT REFERENCES music_tracks(track_id) ON DELETE SET NULL,
    saved_title TEXT NOT NULL DEFAULT '',
    position INTEGER NOT NULL CHECK (position >= 0),
    idempotency_key TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE music_playlists (
    playlist_id TEXT PRIMARY KEY CHECK (playlist_id = lower(playlist_id) AND length(playlist_id) = 36),
    owner_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    title TEXT NOT NULL CHECK (length(trim(title)) > 0),
    description TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE music_track_artists (
    track_id TEXT NOT NULL REFERENCES music_tracks(track_id) ON DELETE CASCADE,
    artist_id TEXT NOT NULL REFERENCES music_artists(artist_id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position >= 0),
    role TEXT NOT NULL DEFAULT 'track_artist'
        CHECK (role IN ('track_artist', 'featured_artist', 'composer', 'conductor')),
    PRIMARY KEY (track_id, position),
    UNIQUE (track_id, artist_id, position)
) STRICT;

CREATE TABLE music_track_lyrics (
    track_id TEXT PRIMARY KEY REFERENCES music_tracks(track_id) ON DELETE CASCADE,
    content TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0)
) STRICT;

CREATE TABLE music_track_overrides (
    track_id TEXT NOT NULL REFERENCES music_tracks(track_id) ON DELETE CASCADE,
    field TEXT NOT NULL CHECK (field IN (
        'title', 'album_title', 'artist_name', 'album_artist_name', 'genre',
        'release_date', 'release_precision', 'edition', 'disc_number', 'disc_total',
        'track_number', 'track_total', 'compilation', 'designation', 'album_id'
    )),
    value TEXT,
    is_present INTEGER NOT NULL CHECK (is_present IN (0, 1)),
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (track_id, field)
) STRICT;

CREATE TABLE music_tracks (
    track_id TEXT PRIMARY KEY REFERENCES assets(asset_id) ON DELETE CASCADE
        CHECK (track_id = lower(track_id) AND length(track_id) = 36),
    owner_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    designation TEXT NOT NULL DEFAULT 'music' CHECK (designation IN ('music', 'other')),
    album_id TEXT REFERENCES music_albums(album_id) ON DELETE SET NULL,
    title TEXT NOT NULL DEFAULT '',
    album_title TEXT NOT NULL DEFAULT '',
    artist_name TEXT NOT NULL DEFAULT '',
    album_artist_name TEXT NOT NULL DEFAULT '',
    genre TEXT NOT NULL DEFAULT '',
    release_date TEXT,
    release_precision TEXT NOT NULL DEFAULT 'unknown'
        CHECK (release_precision IN ('unknown', 'year', 'month', 'day')),
    edition TEXT NOT NULL DEFAULT '',
    release_identifier TEXT NOT NULL DEFAULT '',
    disc_number INTEGER CHECK (disc_number IS NULL OR disc_number > 0),
    disc_total INTEGER CHECK (disc_total IS NULL OR disc_total > 0),
    track_number INTEGER CHECK (track_number IS NULL OR track_number > 0),
    track_total INTEGER CHECK (track_total IS NULL OR track_total > 0),
    is_compilation INTEGER NOT NULL DEFAULT 0 CHECK (is_compilation IN (0, 1)),
    extracted_artists TEXT NOT NULL DEFAULT '[]'
        CHECK (json_valid(extracted_artists) AND json_type(extracted_artists) = 'array'),
    extracted_album_artists TEXT NOT NULL DEFAULT '[]'
        CHECK (json_valid(extracted_album_artists) AND json_type(extracted_album_artists) = 'array'),
    extracted_artist_ids TEXT NOT NULL DEFAULT '[]'
        CHECK (json_valid(extracted_artist_ids) AND json_type(extracted_artist_ids) = 'array'),
    extracted_album_artist_ids TEXT NOT NULL DEFAULT '[]'
        CHECK (json_valid(extracted_album_artist_ids) AND json_type(extracted_album_artist_ids) = 'array'),
    extracted_source_revision INTEGER NOT NULL DEFAULT 0 CHECK (extracted_source_revision >= 0),
    revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE ocr_index_metadata (
    asset_id TEXT PRIMARY KEY
        CHECK (asset_id = lower(asset_id) AND length(asset_id) = 36),
    revision INTEGER NOT NULL CHECK (revision > 0),
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE ocr_index_outbox (
    asset_id TEXT PRIMARY KEY
        CHECK (asset_id = lower(asset_id) AND length(asset_id) = 36),
    revision INTEGER NOT NULL CHECK (revision > 0),
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE ocr_projection_pipeline_state (
    scope TEXT PRIMARY KEY CHECK (scope = 'all'),
    source_revision INTEGER NOT NULL CHECK (source_revision > 0),
    projection_version INTEGER NOT NULL CHECK (projection_version > 0),
    applied_revision INTEGER NOT NULL DEFAULT 0
        CHECK (applied_revision >= 0 AND applied_revision <= source_revision),
    cursor TEXT,
    terminal_error TEXT,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE ocr_results (
    asset_id TEXT PRIMARY KEY REFERENCES assets(asset_id) ON DELETE CASCADE,
    model_id TEXT NOT NULL,
    total_count INTEGER NOT NULL DEFAULT 0 CHECK (total_count >= 0),
    processing_time_ms INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE ocr_text_items (
    id INTEGER PRIMARY KEY,
    asset_id TEXT NOT NULL REFERENCES ocr_results(asset_id) ON DELETE CASCADE,
    text_content TEXT NOT NULL,
    confidence REAL NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    bounding_box TEXT NOT NULL CHECK (json_valid(bounding_box)),
    text_length INTEGER NOT NULL CHECK (text_length >= 0),
    area_pixels REAL,
    created_at INTEGER NOT NULL
) STRICT;

CREATE TABLE pending_totp_enrollments (
    enrollment_id TEXT PRIMARY KEY CHECK (enrollment_id = lower(enrollment_id) AND length(enrollment_id) = 36),
    user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    secret_ciphertext BLOB NOT NULL,
    auth_version INTEGER NOT NULL CHECK (auth_version >= 0),
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL CHECK (expires_at > created_at),
    consumed_at INTEGER,
    CHECK (consumed_at IS NULL OR consumed_at >= created_at)
) STRICT;

CREATE TABLE refresh_tokens (
    token_id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(user_id),
    token TEXT NOT NULL UNIQUE,
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    is_revoked INTEGER NOT NULL DEFAULT 0 CHECK (is_revoked IN (0, 1)),
    auth_version INTEGER NOT NULL DEFAULT 0 CHECK (auth_version >= 0),
    assurance TEXT NOT NULL DEFAULT 'password' CHECK (assurance IN ('password', 'mfa', 'passkey'))
) STRICT;

CREATE TABLE storage_locations (
    storage_location_id TEXT PRIMARY KEY CHECK (storage_location_id = lower(storage_location_id) AND length(storage_location_id) = 36),
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    kind TEXT NOT NULL CHECK (kind IN ('default', 'external')),
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'offline', 'error', 'maintenance')),
    mount_fingerprint TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE repositories (
    repo_id TEXT PRIMARY KEY CHECK (repo_id = lower(repo_id) AND length(repo_id) = 36),
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    config TEXT CHECK (config IS NULL OR json_valid(config)),
    reachability TEXT NOT NULL DEFAULT 'active'
        CHECK (reachability IN ('active', 'offline', 'identity_error', 'recovery_required', 'maintenance')),
    activity TEXT NOT NULL DEFAULT 'idle'
        CHECK (activity IN ('idle', 'scanning', 'importing', 'processing', 'paused')),
    pause_reason TEXT NOT NULL DEFAULT ''
        CHECK (pause_reason IN ('', 'low_space', 'maintenance', 'manual')),
    last_sync INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    default_owner_id INTEGER REFERENCES users(user_id),
    role TEXT NOT NULL DEFAULT 'regular' CHECK (role IN ('primary', 'regular')),
    storage_location_id TEXT NOT NULL REFERENCES storage_locations(storage_location_id) ON DELETE RESTRICT
) STRICT;

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

CREATE TABLE repository_cloud_bindings (
    repository_id TEXT NOT NULL REFERENCES repositories(repo_id) ON DELETE CASCADE,
    credential_id TEXT NOT NULL REFERENCES cloud_credentials(credential_id) ON DELETE RESTRICT,
    owner_id INTEGER NOT NULL REFERENCES users(user_id),
    provider TEXT NOT NULL,
    remote_scope TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(remote_scope)),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    last_import_run_id TEXT REFERENCES cloud_import_runs(run_id) ON DELETE SET NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (repository_id, credential_id)
) STRICT;

CREATE TABLE repository_defaults (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    strategy TEXT NOT NULL DEFAULT 'date' CHECK (strategy IN ('date', 'flat', 'cas')),
    duplicate_handling TEXT NOT NULL DEFAULT 'rename'
        CHECK (duplicate_handling IN ('rename', 'uuid')),
    updated_at INTEGER NOT NULL
) STRICT;

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
        REFERENCES "repository_scan_runs"(run_id) ON DELETE SET NULL,
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

CREATE TABLE repository_observation_state (
    repository_id TEXT PRIMARY KEY
        REFERENCES repositories(repo_id) ON DELETE CASCADE,
    desired_epoch INTEGER NOT NULL DEFAULT 0 CHECK (desired_epoch >= 0),
    applied_epoch INTEGER NOT NULL DEFAULT 0
        CHECK (applied_epoch >= 0 AND applied_epoch <= desired_epoch),
    next_revision INTEGER NOT NULL DEFAULT 1 CHECK (next_revision > 0),
    active_run_id TEXT
        REFERENCES "repository_scan_runs"(run_id) ON DELETE SET NULL,
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
, terminal_error TEXT, full_verification_requested_epoch INTEGER NOT NULL DEFAULT 0) STRICT;

CREATE TABLE repository_observations (
    observation_id TEXT PRIMARY KEY
        CHECK (observation_id = lower(observation_id) AND length(observation_id) = 36),
    repository_id TEXT NOT NULL
        REFERENCES repositories(repo_id) ON DELETE CASCADE,
    revision INTEGER NOT NULL CHECK (revision > 0),
    run_id TEXT
        REFERENCES "repository_scan_runs"(run_id) ON DELETE SET NULL,
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

CREATE TABLE repository_scan_frontier (
    run_id TEXT NOT NULL
        REFERENCES "repository_scan_runs"(run_id) ON DELETE CASCADE,
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

CREATE TABLE repository_staging_commits (
    commit_id TEXT PRIMARY KEY
        CHECK (commit_id = lower(commit_id) AND length(commit_id) = 36),
    repository_id TEXT NOT NULL
        REFERENCES repositories(repo_id) ON DELETE CASCADE,
    owner_id INTEGER NOT NULL REFERENCES users(user_id),
    source_kind TEXT NOT NULL CHECK (source_kind IN ('upload', 'cloud')),
    staging_path TEXT NOT NULL,
    target_path TEXT,
    original_filename TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    full_hash TEXT NOT NULL
        CHECK (full_hash = lower(full_hash) AND length(full_hash) = 64),
    file_size INTEGER NOT NULL CHECK (file_size >= 0),
    quick_fingerprint TEXT,
    quick_fingerprint_version TEXT,
    status TEXT NOT NULL DEFAULT 'prepared'
        CHECK (status IN ('prepared', 'committing', 'committed', 'quarantined', 'completed')),
    node_id TEXT REFERENCES repository_nodes(node_id) ON DELETE SET NULL,
    asset_id TEXT REFERENCES assets(asset_id) ON DELETE SET NULL,
    failure_code TEXT,
    failure_detail TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    completed_at INTEGER
) STRICT;

CREATE TABLE reverse_geocode_cache (
    source_key TEXT NOT NULL CHECK (length(trim(source_key)) > 0),
    geohash TEXT NOT NULL CHECK (length(trim(geohash)) > 0),
    provider TEXT NOT NULL,
    language TEXT NOT NULL DEFAULT '',
    latitude REAL NOT NULL CHECK (latitude BETWEEN -90 AND 90),
    longitude REAL NOT NULL CHECK (longitude BETWEEN -180 AND 180),
    label TEXT,
    country TEXT,
    region TEXT,
    city TEXT,
    raw_response TEXT CHECK (raw_response IS NULL OR json_valid(raw_response)),
    queried_at INTEGER NOT NULL,
    expires_at INTEGER,
    PRIMARY KEY (source_key, geohash)
) STRICT;

CREATE TABLE search_embeddings (
    id INTEGER PRIMARY KEY,
    asset_id TEXT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    space_id INTEGER NOT NULL REFERENCES embedding_spaces(id) ON DELETE RESTRICT,
    frame_ts_ms INTEGER,
    vector BLOB NOT NULL,
    model_id TEXT NOT NULL,
    created_at INTEGER NOT NULL
) STRICT;

CREATE TABLE semantic_vector_index_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    mode TEXT NOT NULL CHECK (mode IN ('flat', 'ann')),
    row_count INTEGER NOT NULL DEFAULT 0 CHECK (row_count >= 0),
    trained_row_count INTEGER NOT NULL DEFAULT 0 CHECK (trained_row_count >= 0),
    rebuild_pending INTEGER NOT NULL DEFAULT 0 CHECK (rebuild_pending IN (0, 1)),
    config TEXT NOT NULL CHECK (json_valid(config)),
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    llm_agent_enabled INTEGER NOT NULL DEFAULT 0 CHECK (llm_agent_enabled IN (0, 1)),
    llm_provider TEXT NOT NULL DEFAULT '' CHECK (llm_provider IN ('', 'ark', 'openai', 'deepseek', 'ollama')),
    llm_model_name TEXT NOT NULL DEFAULT '',
    llm_base_url TEXT NOT NULL DEFAULT '',
    llm_api_key_ciphertext BLOB,
    llm_api_key_configured INTEGER NOT NULL DEFAULT 0 CHECK (llm_api_key_configured IN (0, 1)),
    ml_auto TEXT NOT NULL DEFAULT 'disable' CHECK (ml_auto IN ('enable', 'disable')),
    ml_semantic_enabled INTEGER NOT NULL DEFAULT 0 CHECK (ml_semantic_enabled IN (0, 1)),
    ml_ocr_enabled INTEGER NOT NULL DEFAULT 0 CHECK (ml_ocr_enabled IN (0, 1)),
    ml_caption_enabled INTEGER NOT NULL DEFAULT 0 CHECK (ml_caption_enabled IN (0, 1)),
    ml_face_enabled INTEGER NOT NULL DEFAULT 0 CHECK (ml_face_enabled IN (0, 1)),
    ml_bioclip_enabled INTEGER NOT NULL DEFAULT 0 CHECK (ml_bioclip_enabled IN (0, 1)),
    ml_video_semantic_enabled INTEGER NOT NULL DEFAULT 0 CHECK (ml_video_semantic_enabled IN (0, 1)),
    ml_video_max_frames INTEGER NOT NULL DEFAULT 8 CHECK (ml_video_max_frames > 0),
    ml_video_long_threshold_seconds INTEGER NOT NULL DEFAULT 300 CHECK (ml_video_long_threshold_seconds > 0),
    ml_video_scene_threshold REAL NOT NULL DEFAULT 0.4 CHECK (ml_video_scene_threshold >= 0 AND ml_video_scene_threshold <= 1),
    backup_enabled INTEGER NOT NULL DEFAULT 1 CHECK (backup_enabled IN (0, 1)),
    backup_interval_hours INTEGER NOT NULL DEFAULT 24 CHECK (backup_interval_hours > 0),
    backup_keep_last INTEGER NOT NULL DEFAULT 14 CHECK (backup_keep_last > 0),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    updated_by INTEGER REFERENCES users(user_id)
, geocoding_provider TEXT NOT NULL DEFAULT 'disabled'
        CHECK (geocoding_provider IN ('disabled', 'nominatim')), geocoding_nominatim_endpoint TEXT NOT NULL
        DEFAULT 'https://nominatim.openstreetmap.org/reverse'
        CHECK (
            length(CAST(trim(geocoding_nominatim_endpoint) AS BLOB)) BETWEEN 1 AND 2048
            AND (
                (
                    substr(lower(trim(geocoding_nominatim_endpoint)), 1, 7) = 'http://'
                    AND length(substr(trim(geocoding_nominatim_endpoint), 8)) > 0
                    AND substr(trim(geocoding_nominatim_endpoint), 8, 1) NOT IN ('/', '?', '#', ':', ' ')
                )
                OR (
                    substr(lower(trim(geocoding_nominatim_endpoint)), 1, 8) = 'https://'
                    AND length(substr(trim(geocoding_nominatim_endpoint), 9)) > 0
                    AND substr(trim(geocoding_nominatim_endpoint), 9, 1) NOT IN ('/', '?', '#', ':', ' ')
                )
            )
            AND instr(trim(geocoding_nominatim_endpoint), '@') = 0
            AND instr(trim(geocoding_nominatim_endpoint), '#') = 0
        ), geocoding_language TEXT NOT NULL DEFAULT 'en'
        CHECK (
            length(CAST(trim(geocoding_language) AS BLOB)) BETWEEN 1 AND 64
            AND instr(geocoding_language, char(10)) = 0
            AND instr(geocoding_language, char(13)) = 0
        ), geocoding_user_agent TEXT NOT NULL DEFAULT 'Lumilio-Photos/1.0'
        CHECK (
            length(CAST(trim(geocoding_user_agent) AS BLOB)) BETWEEN 1 AND 512
            AND instr(geocoding_user_agent, char(0)) = 0
            AND instr(geocoding_user_agent, char(1)) = 0
            AND instr(geocoding_user_agent, char(2)) = 0
            AND instr(geocoding_user_agent, char(3)) = 0
            AND instr(geocoding_user_agent, char(4)) = 0
            AND instr(geocoding_user_agent, char(5)) = 0
            AND instr(geocoding_user_agent, char(6)) = 0
            AND instr(geocoding_user_agent, char(7)) = 0
            AND instr(geocoding_user_agent, char(8)) = 0
            AND instr(geocoding_user_agent, char(9)) = 0
            AND instr(geocoding_user_agent, char(10)) = 0
            AND instr(geocoding_user_agent, char(11)) = 0
            AND instr(geocoding_user_agent, char(12)) = 0
            AND instr(geocoding_user_agent, char(13)) = 0
            AND instr(geocoding_user_agent, char(14)) = 0
            AND instr(geocoding_user_agent, char(15)) = 0
            AND instr(geocoding_user_agent, char(16)) = 0
            AND instr(geocoding_user_agent, char(17)) = 0
            AND instr(geocoding_user_agent, char(18)) = 0
            AND instr(geocoding_user_agent, char(19)) = 0
            AND instr(geocoding_user_agent, char(20)) = 0
            AND instr(geocoding_user_agent, char(21)) = 0
            AND instr(geocoding_user_agent, char(22)) = 0
            AND instr(geocoding_user_agent, char(23)) = 0
            AND instr(geocoding_user_agent, char(24)) = 0
            AND instr(geocoding_user_agent, char(25)) = 0
            AND instr(geocoding_user_agent, char(26)) = 0
            AND instr(geocoding_user_agent, char(27)) = 0
            AND instr(geocoding_user_agent, char(28)) = 0
            AND instr(geocoding_user_agent, char(29)) = 0
            AND instr(geocoding_user_agent, char(30)) = 0
            AND instr(geocoding_user_agent, char(31)) = 0
            AND instr(geocoding_user_agent, char(127)) = 0
        ), geocoding_revision INTEGER NOT NULL DEFAULT 1
        CHECK (geocoding_revision > 0)) STRICT;

CREATE TABLE share_links (
    share_id TEXT PRIMARY KEY CHECK (share_id = lower(share_id) AND length(share_id) = 36),
    owner_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    token_hash BLOB NOT NULL UNIQUE,
    title TEXT NOT NULL,
    description TEXT,
    source_kind TEXT NOT NULL CHECK (source_kind IN ('asset_snapshot', 'album', 'person', 'utility_query', 'pin')),
    source_ref TEXT,
    asset_ids TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(asset_ids) AND json_type(asset_ids) = 'array'),
    asset_count INTEGER NOT NULL DEFAULT 0,
    allow_download INTEGER NOT NULL DEFAULT 0 CHECK (allow_download IN (0, 1)),
    include_originals INTEGER NOT NULL DEFAULT 0 CHECK (include_originals IN (0, 1)),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    revoked_at INTEGER,
    last_viewed_at INTEGER,
    view_count INTEGER NOT NULL DEFAULT 0
) STRICT;

CREATE TABLE species_predictions (
    prediction_id INTEGER PRIMARY KEY,
    asset_id TEXT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    score REAL NOT NULL CHECK (score BETWEEN 0 AND 1),
    created_at INTEGER NOT NULL,
    UNIQUE (asset_id, label)
) STRICT;

CREATE TABLE system_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    library_id TEXT NOT NULL UNIQUE CHECK (length(library_id) = 32),
    bootstrap_phase TEXT NOT NULL DEFAULT 'fresh'
        CHECK (bootstrap_phase IN ('fresh', 'catalog_ready', 'admin_created', 'ready')),
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE tags (
    tag_id INTEGER PRIMARY KEY,
    tag_name TEXT NOT NULL UNIQUE,
    category TEXT,
    is_ai_generated INTEGER NOT NULL DEFAULT 1 CHECK (is_ai_generated IN (0, 1))
) STRICT;

CREATE TABLE thumbnails (
    thumbnail_id INTEGER PRIMARY KEY,
    asset_id TEXT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    size TEXT NOT NULL CHECK (size IN ('small', 'medium', 'large', 'waveform')),
    storage_path TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    repository_id TEXT REFERENCES repositories(repo_id) ON DELETE CASCADE,
    UNIQUE (asset_id, size)
) STRICT;

CREATE TABLE user_mfa_recovery_codes (
    recovery_code_id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    used_at INTEGER,
    UNIQUE (user_id, code_hash)
) STRICT;

CREATE TABLE user_mfa_totp_credentials (
    user_id INTEGER PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    secret_ciphertext BLOB NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    enabled_at INTEGER NOT NULL,
    last_used_at INTEGER,
    credential_version INTEGER NOT NULL DEFAULT 1 CHECK (credential_version > 0),
    last_used_counter INTEGER NOT NULL DEFAULT -1 CHECK (last_used_counter >= -1)
) STRICT;

CREATE TABLE user_webauthn_credentials (
    user_webauthn_credential_id INTEGER PRIMARY KEY,
    credential_id BLOB NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    public_key BLOB NOT NULL,
    sign_count INTEGER NOT NULL DEFAULT 0,
    transports TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(transports) AND json_type(transports) = 'array'),
    attestation_type TEXT NOT NULL DEFAULT 'none',
    aaguid BLOB,
    user_present INTEGER NOT NULL DEFAULT 0 CHECK (user_present IN (0, 1)),
    user_verified INTEGER NOT NULL DEFAULT 0 CHECK (user_verified IN (0, 1)),
    backup_eligible INTEGER NOT NULL DEFAULT 0 CHECK (backup_eligible IN (0, 1)),
    backup_state INTEGER NOT NULL DEFAULT 0 CHECK (backup_state IN (0, 1)),
    created_at INTEGER NOT NULL,
    last_used_at INTEGER
) STRICT;

CREATE TABLE users (
    user_id INTEGER PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    is_active INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0, 1)),
    last_login INTEGER,
    display_name TEXT NOT NULL DEFAULT '',
    avatar_asset_id TEXT REFERENCES assets(asset_id) ON DELETE SET NULL,
    role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user')),
    webauthn_user_handle BLOB NOT NULL,
    auth_version INTEGER NOT NULL DEFAULT 0,
    password_change_required INTEGER NOT NULL DEFAULT 0 CHECK (password_change_required IN (0, 1))
) STRICT;

-- ===== VIRTUAL TABLES (5) =====

CREATE VIRTUAL TABLE asset_search_fts USING fts5(
    original_filename,
    content = 'assets',
    content_rowid = 'rowid',
    tokenize = 'trigram'
);

CREATE VIRTUAL TABLE location_search_fts USING fts5(
    label,
    country,
    region,
    city,
    geohash,
    content = 'location_clusters',
    content_rowid = 'rowid',
    tokenize = 'trigram'
);

CREATE VIRTUAL TABLE music_search_fts USING fts5(
    track_id UNINDEXED,
    title,
    artist,
    album,
    filename,
    tokenize = 'unicode61'
);

CREATE VIRTUAL TABLE search_embeddings_vec USING vec1(
    embedding,
    space_id,
    owner_id,
    is_deleted,
    asset_type
);

CREATE VIRTUAL TABLE species_search_fts USING fts5(
    label,
    content = 'species_predictions',
    content_rowid = 'rowid',
    tokenize = 'trigram'
);

-- ===== VIEWS (3) =====

CREATE VIEW active_asset_occurrences AS
WITH RECURSIVE reachable_nodes AS (
    SELECT repository_id, node_id
    FROM repository_nodes
    WHERE parent_node_id IS NULL AND lifecycle = 'active'
    UNION ALL
    SELECT child.repository_id, child.node_id
    FROM repository_nodes child
    JOIN reachable_nodes parent
      ON parent.repository_id = child.repository_id
     AND parent.node_id = child.parent_node_id
    WHERE child.lifecycle = 'active'
)
SELECT
    location.asset_id,
    node.repository_id,
    node.node_id,
    location.location_id,
    content.full_hash,
    content.file_size,
    observation.quick_fingerprint,
    observation.quick_fingerprint_version,
    node.observation_revision
FROM asset_locations location
JOIN repository_nodes node
  ON node.node_id = location.node_id
 AND node.lifecycle = 'active'
JOIN reachable_nodes reachable
  ON reachable.repository_id = node.repository_id
 AND reachable.node_id = node.node_id
JOIN assets asset ON asset.asset_id = location.asset_id
JOIN content_objects content ON content.content_id = asset.content_id
LEFT JOIN repository_observations observation
  ON observation.repository_id = node.repository_id
 AND observation.mapped_node_id = node.node_id
 AND observation.revision = node.observation_revision
WHERE location.unbound_observation_revision IS NULL;

CREATE VIEW active_asset_occurrence_paths AS
WITH RECURSIVE node_paths AS (
    SELECT repository_id, node_id, CAST('' AS TEXT) AS relative_path
    FROM repository_nodes
    WHERE parent_node_id IS NULL AND lifecycle = 'active'
    UNION ALL
    SELECT
        child.repository_id,
        child.node_id,
        CASE
          WHEN parent.relative_path = '' THEN child.name
          ELSE parent.relative_path || '/' || child.name
        END
    FROM repository_nodes child
    JOIN node_paths parent
      ON parent.repository_id = child.repository_id
     AND parent.node_id = child.parent_node_id
    WHERE child.lifecycle = 'active'
)
SELECT occurrence.*, node_paths.relative_path
FROM active_asset_occurrences occurrence
JOIN node_paths
  ON node_paths.repository_id = occurrence.repository_id
 AND node_paths.node_id = occurrence.node_id;

CREATE VIEW media_item_browse_facts AS
SELECT
    mi.media_item_id,
    mi.owner_id,
    mi.repository_id,
    mi.media_kind,
    mi.primary_asset_id,

    COUNT(mia.asset_id) AS component_count,

    MAX(CASE WHEN mia.relation = 'raw_original' THEN 1 ELSE 0 END) AS has_raw,
    MAX(CASE WHEN mia.relation = 'jpeg_original' THEN 1 ELSE 0 END) AS has_jpeg,
    MAX(CASE WHEN mia.relation = 'edited_version' THEN 1 ELSE 0 END) AS has_edited,
    MAX(CASE WHEN mia.relation = 'live_photo_video' THEN 1 ELSE 0 END) AS has_live_motion,

    asm.stack_id,
    asm.position AS stack_position,
    s.stack_kind

FROM media_items mi
JOIN media_item_assets mia
  ON mia.media_item_id = mi.media_item_id
LEFT JOIN asset_stack_members asm
  ON asm.media_item_id = mi.media_item_id
LEFT JOIN asset_stacks s
  ON s.stack_id = asm.stack_id
GROUP BY
    mi.media_item_id,
    mi.owner_id,
    mi.repository_id,
    mi.media_kind,
    mi.primary_asset_id,
    asm.stack_id,
    asm.position,
    s.stack_kind;

-- ===== INDEXES (155) =====

CREATE INDEX embeddings_asset_type_idx ON embeddings (asset_id, embedding_type);

CREATE INDEX embeddings_primary_idx ON embeddings (embedding_type, is_primary) WHERE is_primary = 1;

CREATE INDEX embeddings_space_primary_asset_idx ON embeddings (space_id, is_primary, asset_id);

CREATE INDEX embeddings_type_model_idx ON embeddings (embedding_type, embedding_model);

CREATE INDEX face_cluster_members_cluster_idx ON face_cluster_members (cluster_id);

CREATE INDEX face_cluster_members_face_idx ON face_cluster_members (face_id);

CREATE INDEX face_cluster_members_similarity_idx ON face_cluster_members (similarity_score);

CREATE INDEX face_clusters_confirmed_idx ON face_clusters (is_confirmed) WHERE is_confirmed = 1;

CREATE INDEX face_clusters_hidden_idx ON face_clusters (is_hidden, updated_at DESC);

CREATE INDEX face_clusters_owner_idx ON face_clusters (owner_id);

CREATE INDEX face_clusters_representative_idx ON face_clusters (representative_face_id);

CREATE INDEX face_items_age_group_idx ON face_items (age_group);

CREATE INDEX face_items_asset_id_idx ON face_items (asset_id);

CREATE INDEX face_items_cluster_candidate_idx ON face_items (confidence, face_size) WHERE embedding IS NOT NULL;

CREATE INDEX face_items_confidence_idx ON face_items (confidence);

CREATE INDEX face_items_embedding_model_idx ON face_items (embedding_model) WHERE embedding IS NOT NULL;

CREATE INDEX face_items_ethnicity_idx ON face_items (ethnicity);

CREATE INDEX face_items_expression_idx ON face_items (expression);

CREATE INDEX face_items_face_id_idx ON face_items (face_id) WHERE face_id IS NOT NULL;

CREATE INDEX face_items_gender_idx ON face_items (gender);

CREATE INDEX face_items_is_primary_idx ON face_items (is_primary) WHERE is_primary = 1;

CREATE INDEX face_results_asset_id_idx ON face_results (asset_id);

CREATE INDEX face_results_created_at_idx ON face_results (created_at);

CREATE INDEX face_results_model_id_idx ON face_results (model_id);

CREATE INDEX host_actions_actor_idx ON host_actions (actor_user_id, created_at DESC);

CREATE INDEX host_actions_pending_idx ON host_actions (status, expires_at, created_at);

CREATE INDEX idx_agent_pending_effects_thread ON agent_pending_effects (user_id, thread_id, created_at DESC);

CREATE INDEX idx_agent_pins_user ON agent_pins (user_id, created_at DESC);

CREATE INDEX idx_agent_refs_expiry ON agent_refs (expires_at);

CREATE INDEX idx_agent_runs_thread_created ON agent_runs (user_id, thread_id, created_at DESC);

CREATE INDEX idx_album_assets_album_order ON album_assets (album_id, position, added_time, asset_id);

CREATE INDEX idx_album_assets_asset ON album_assets (asset_id);

CREATE INDEX idx_albums_type ON albums (album_type);

CREATE INDEX idx_albums_user_created_at ON albums (user_id, created_at DESC, album_id DESC);

CREATE INDEX idx_albums_user_id ON albums (user_id);

CREATE INDEX idx_asset_locations_active_asset
    ON asset_locations (asset_id, node_id)
    WHERE unbound_observation_revision IS NULL;

CREATE INDEX idx_asset_pipeline_pending
    ON asset_pipeline_state (stage, updated_at, asset_id)
    WHERE desired_version > applied_version;

CREATE INDEX idx_asset_stack_members_stack ON asset_stack_members (stack_id);

CREATE INDEX idx_asset_stack_members_stack_position ON asset_stack_members (stack_id, position, media_item_id);

CREATE INDEX idx_asset_stacks_kind ON asset_stacks (stack_kind, stack_id);

CREATE INDEX idx_asset_tags_tag_source_asset ON asset_tags (tag_id, source, asset_id);

CREATE INDEX idx_assets_owner_deleted ON assets (owner_id, is_deleted, asset_id);

CREATE INDEX idx_assets_taken_time ON assets (taken_time DESC, asset_id);

CREATE INDEX idx_auth_security_verifications_lookup
    ON auth_security_verifications (user_id, auth_version, purpose, expires_at, consumed_at);

CREATE INDEX idx_catalog_operation_receipts_subject
    ON catalog_operation_receipts (kind, subject_id, created_at DESC);

CREATE INDEX idx_classifier_definitions_enabled ON classifier_definitions (enabled);

CREATE INDEX idx_cloud_credentials_owner_created ON cloud_credentials (owner_id, created_at DESC);

CREATE INDEX idx_cloud_credentials_provider_status ON cloud_credentials (provider, status);

CREATE INDEX idx_cloud_import_runs_credential_created ON cloud_import_runs (credential_id, created_at DESC);

CREATE INDEX idx_cloud_import_runs_owner_created ON cloud_import_runs (owner_id, created_at DESC);

CREATE INDEX idx_cloud_import_runs_repository_created ON cloud_import_runs (repository_id, created_at DESC);

CREATE INDEX idx_cloud_import_runs_status ON cloud_import_runs (status);

CREATE INDEX idx_duplicate_group_assets_asset ON duplicate_group_assets (asset_id);

CREATE INDEX idx_duplicate_group_edges_assets ON duplicate_group_edges (asset_id_a, asset_id_b);

CREATE INDEX idx_duplicate_groups_owner_repo ON duplicate_groups (owner_id, repository_id);

CREATE INDEX idx_duplicate_groups_repo_status ON duplicate_groups (repository_id, status, detected_at DESC);

CREATE INDEX idx_duplicate_groups_status ON duplicate_groups (status);

CREATE INDEX idx_event_dirty_ranges_owner_claim
    ON event_dirty_ranges (owner_id, claimed_at, range_start, range_end);

CREATE INDEX idx_event_media_items_order
    ON event_media_items (event_id, position, media_item_id);

CREATE INDEX idx_event_rebuild_runs_active
    ON event_rebuild_runs(owner_id, state)
    WHERE state IN ('queued','running');

CREATE INDEX idx_event_rebuild_runs_owner_time
    ON event_rebuild_runs(owner_id, requested_at DESC, run_id DESC);

CREATE INDEX idx_event_redirects_target
    ON event_redirects (owner_id, new_event_id);

CREATE INDEX idx_events_owner_list
    ON events (owner_id, start_at DESC, event_id DESC);

CREATE INDEX idx_location_cluster_assets_asset ON location_cluster_assets (asset_id);

CREATE INDEX idx_location_clusters_geocode_due
    ON location_clusters (geocode_status, geocode_next_attempt_at, updated_at);

CREATE INDEX idx_location_clusters_repository_owner ON location_clusters (repository_id, owner_id);

CREATE INDEX idx_location_clusters_status ON location_clusters (geocode_status);

CREATE INDEX idx_location_projection_state_pending
    ON location_projection_state (updated_at, repository_id, owner_id)
    WHERE source_revision > published_revision;

CREATE INDEX idx_media_item_assets_item ON media_item_assets (media_item_id);

CREATE INDEX idx_media_item_assets_item_relation ON media_item_assets (media_item_id, relation, asset_id);

CREATE INDEX idx_media_items_owner_repository ON media_items (owner_id, repository_id);

CREATE INDEX idx_media_items_primary_asset ON media_items (primary_asset_id);

CREATE INDEX idx_media_items_repository_owner ON media_items (repository_id, owner_id, media_item_id);

CREATE INDEX idx_music_album_artists_artist ON music_album_artists (artist_id, album_id, position);

CREATE INDEX idx_music_albums_owner_order
    ON music_albums (owner_id, lower(title), album_id);

CREATE INDEX idx_music_artists_owner_name
    ON music_artists (owner_id, normalized_name, artist_id);

CREATE INDEX idx_music_playback_entries_page
    ON music_playback_entries (session_id, sequence, entry_id);

CREATE INDEX idx_music_playlist_entries_order
    ON music_playlist_entries (playlist_id, position, entry_id);

CREATE INDEX idx_music_playlists_owner_order
    ON music_playlists (owner_id, updated_at DESC, playlist_id);

CREATE INDEX idx_music_track_artists_artist ON music_track_artists (artist_id, track_id, position);

CREATE INDEX idx_music_tracks_album_order
    ON music_tracks (album_id, disc_number, track_number, track_id);

CREATE INDEX idx_music_tracks_owner_designation
    ON music_tracks (owner_id, designation, track_id);

CREATE INDEX idx_music_tracks_owner_order
    ON music_tracks (owner_id, lower(title), track_id);

CREATE INDEX idx_pending_totp_enrollments_expires_at
    ON pending_totp_enrollments (expires_at);

CREATE INDEX idx_pending_totp_enrollments_user_id
    ON pending_totp_enrollments (user_id, auth_version, consumed_at);

CREATE INDEX idx_refresh_tokens_tokens_token ON refresh_tokens (token);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens (user_id);

CREATE INDEX idx_refresh_tokens_user_version ON refresh_tokens (user_id, auth_version, is_revoked);

CREATE INDEX idx_repositories_activity ON repositories (activity);

CREATE INDEX idx_repositories_default_owner ON repositories (default_owner_id);

CREATE INDEX idx_repositories_path ON repositories (path);

CREATE INDEX idx_repositories_reachability ON repositories (reachability);

CREATE INDEX idx_repositories_role ON repositories (role);

CREATE INDEX idx_repository_cloud_bindings_credential ON repository_cloud_bindings (credential_id);

CREATE INDEX idx_repository_cloud_bindings_owner ON repository_cloud_bindings (owner_id, created_at DESC);

CREATE INDEX idx_repository_nodes_children
    ON repository_nodes (repository_id, parent_node_id, lifecycle, node_id);

CREATE INDEX idx_repository_nodes_native_identity
    ON repository_nodes (repository_id, volume_identity, native_identity_kind, native_identity_value)
    WHERE lifecycle = 'active' AND native_identity_value IS NOT NULL;

CREATE INDEX idx_repository_nodes_run_coverage
    ON repository_nodes (repository_id, parent_node_id, last_seen_run_id)
    WHERE lifecycle = 'active';

CREATE INDEX idx_repository_observation_state_work
    ON repository_observation_state (desired_epoch, applied_epoch, controller_lease_expires_at)
    WHERE desired_epoch > applied_epoch OR full_verification_required = 1;

CREATE INDEX idx_repository_observations_node_revision
    ON repository_observations (repository_id, mapped_node_id, revision DESC)
    WHERE mapped_node_id IS NOT NULL;

CREATE INDEX idx_repository_observations_pending
    ON repository_observations (repository_id, processing_state, revision)
    WHERE processing_state IN ('pending', 'retryable_error');

CREATE INDEX idx_repository_scan_frontier_claim
    ON repository_scan_frontier (run_id, state, lease_expires_at, directory_node_id);

CREATE INDEX idx_repository_scan_runs_full_verification
 ON repository_scan_runs(repository_id, finished_at DESC)
 WHERE status = 'completed' AND full_verification_performed = 1;

CREATE INDEX idx_repository_scan_runs_history
    ON repository_scan_runs (repository_id, created_at DESC, run_id);

CREATE INDEX idx_repository_staging_commits_recovery
    ON repository_staging_commits (status, updated_at, commit_id)
    WHERE status IN ('prepared', 'committing', 'committed');

CREATE INDEX idx_reverse_geocode_cache_source_geohash
    ON reverse_geocode_cache (source_key, geohash);

CREATE INDEX idx_share_links_owner ON share_links (owner_id, created_at DESC);

CREATE INDEX idx_share_links_status_expires ON share_links (status, expires_at);

CREATE INDEX idx_species_predictions_asset_id ON species_predictions (asset_id);

CREATE INDEX idx_species_predictions_label ON species_predictions (label);

CREATE INDEX idx_species_predictions_label_asset_score
    ON species_predictions (label, asset_id, score DESC) WHERE score >= 0.5;

CREATE INDEX idx_species_predictions_label_score ON species_predictions (label, score DESC);

CREATE INDEX idx_species_predictions_score ON species_predictions (score DESC);

CREATE INDEX idx_thumbnails_asset_id ON thumbnails (asset_id);

CREATE INDEX idx_user_mfa_recovery_codes_unused ON user_mfa_recovery_codes (user_id) WHERE used_at IS NULL;

CREATE INDEX idx_user_mfa_recovery_codes_user_id ON user_mfa_recovery_codes (user_id);

CREATE INDEX idx_user_webauthn_credentials_user_id ON user_webauthn_credentials (user_id);

CREATE INDEX idx_users_role ON users (role);

CREATE INDEX lifecycle_audit_events_target_idx ON lifecycle_audit_events (target_type, target_id, occurred_at DESC);

CREATE INDEX lifecycle_audit_events_time_idx ON lifecycle_audit_events (occurred_at DESC, event_id DESC);

CREATE INDEX lifecycle_operations_status_idx ON lifecycle_operations (status, updated_at);

CREATE INDEX lifecycle_operations_target_idx ON lifecycle_operations (target_type, target_id, status);

CREATE INDEX ocr_index_outbox_updated_at_idx
    ON ocr_index_outbox (updated_at, asset_id);

CREATE INDEX ocr_results_asset_id_idx ON ocr_results (asset_id);

CREATE INDEX ocr_results_created_at_idx ON ocr_results (created_at);

CREATE INDEX ocr_results_model_id_idx ON ocr_results (model_id);

CREATE INDEX ocr_text_items_asset_id_idx ON ocr_text_items (asset_id);

CREATE INDEX ocr_text_items_confidence_idx ON ocr_text_items (confidence);

CREATE INDEX ocr_text_items_text_length_idx ON ocr_text_items (text_length);

CREATE INDEX repositories_storage_location_id_idx ON repositories (storage_location_id);

CREATE INDEX search_embeddings_asset_idx ON search_embeddings (asset_id);

CREATE UNIQUE INDEX asset_locations_one_active_node
    ON asset_locations (node_id)
    WHERE unbound_observation_revision IS NULL;

CREATE UNIQUE INDEX embedding_spaces_default_per_type_idx
    ON embedding_spaces (embedding_type) WHERE is_default_search = 1;

CREATE UNIQUE INDEX embedding_spaces_identity_idx
    ON embedding_spaces (embedding_type, model_id, dimensions, distance_metric);

CREATE UNIQUE INDEX embeddings_one_primary_per_asset_type_idx
    ON embeddings (asset_id, embedding_type) WHERE is_primary = 1;

CREATE UNIQUE INDEX face_cluster_members_face_unique_idx ON face_cluster_members (face_id);

CREATE UNIQUE INDEX idx_agent_runs_one_active_thread ON agent_runs (user_id, thread_id)
    WHERE activation_state = 'active'
      AND status IN ('running', 'cancel_requested', 'awaiting_confirmation');

CREATE UNIQUE INDEX idx_asset_stacks_burst_group_key ON asset_stacks (group_key)
    WHERE stack_kind = 'burst' AND group_key IS NOT NULL;

CREATE UNIQUE INDEX idx_event_constraints_event_item
    ON event_constraints (owner_id, kind, event_id, left_media_item_id)
    WHERE kind IN ('include', 'exclude');

CREATE UNIQUE INDEX idx_event_constraints_one_include
    ON event_constraints (left_media_item_id)
    WHERE kind = 'include';

CREATE UNIQUE INDEX idx_event_constraints_pair
    ON event_constraints (owner_id, kind, left_media_item_id, right_media_item_id)
    WHERE kind IN ('must_link', 'cannot_link');

CREATE UNIQUE INDEX idx_event_media_items_current
    ON event_media_items (media_item_id);

CREATE UNIQUE INDEX idx_media_items_id_owner
    ON media_items (media_item_id, owner_id);

CREATE UNIQUE INDEX idx_music_playlist_entries_idempotency
    ON music_playlist_entries (playlist_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE UNIQUE INDEX idx_pending_totp_enrollments_active_user
    ON pending_totp_enrollments (user_id)
    WHERE consumed_at IS NULL;

CREATE UNIQUE INDEX idx_users_webauthn_user_handle ON users (webauthn_user_handle);

CREATE UNIQUE INDEX location_clusters_scope_key
    ON location_clusters (coalesce(owner_id, -1), repository_id, geohash);

CREATE UNIQUE INDEX repositories_one_primary_idx ON repositories (role) WHERE role = 'primary';

CREATE UNIQUE INDEX repository_nodes_one_active_child
    ON repository_nodes (repository_id, parent_node_id, name_key)
    WHERE lifecycle = 'active' AND parent_node_id IS NOT NULL;

CREATE UNIQUE INDEX repository_nodes_one_active_root
    ON repository_nodes (repository_id)
    WHERE parent_node_id IS NULL AND lifecycle = 'active';

CREATE UNIQUE INDEX repository_observations_source_delivery
    ON repository_observations (repository_id, source, source_event_key)
    WHERE source_event_key IS NOT NULL;

CREATE UNIQUE INDEX storage_locations_one_default_idx ON storage_locations (kind) WHERE kind = 'default';

CREATE UNIQUE INDEX repository_scan_runs_one_active
    ON repository_scan_runs (repository_id)
    WHERE status IN ('queued', 'crawling', 'catching_up', 'finalizing');

CREATE UNIQUE INDEX search_embeddings_asset_frame_uniq
    ON search_embeddings (asset_id, frame_ts_ms) WHERE frame_ts_ms IS NOT NULL;

CREATE UNIQUE INDEX search_embeddings_asset_primary_uniq
    ON search_embeddings (asset_id) WHERE frame_ts_ms IS NULL;

-- ===== TRIGGERS (41) =====

CREATE TRIGGER asset_search_fts_delete AFTER DELETE ON assets BEGIN
    INSERT INTO asset_search_fts (asset_search_fts, rowid, original_filename)
    VALUES ('delete', old.rowid, old.original_filename);
END;

CREATE TRIGGER asset_search_fts_insert AFTER INSERT ON assets BEGIN
    INSERT INTO asset_search_fts (rowid, original_filename)
    VALUES (new.rowid, new.original_filename);
END;

CREATE TRIGGER asset_search_fts_update AFTER UPDATE OF original_filename ON assets BEGIN
    INSERT INTO asset_search_fts (asset_search_fts, rowid, original_filename)
    VALUES ('delete', old.rowid, old.original_filename);
    INSERT INTO asset_search_fts (rowid, original_filename)
    VALUES (new.rowid, new.original_filename);
END;

CREATE TRIGGER event_cover_membership_insert
AFTER INSERT ON events
WHEN NEW.status = 'active'
BEGIN
    SELECT CASE WHEN NEW.generated_cover_media_item_id IS NOT NULL
        THEN RAISE(ABORT, 'cover cannot precede event membership') END;
    SELECT CASE WHEN NEW.cover_override_media_item_id IS NOT NULL
        THEN RAISE(ABORT, 'cover cannot precede event membership') END;
END;

CREATE TRIGGER event_cover_membership_update
BEFORE UPDATE OF generated_cover_media_item_id, cover_override_media_item_id, owner_id ON events
WHEN NEW.status = 'active'
BEGIN
    SELECT CASE WHEN NEW.generated_cover_media_item_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM event_media_items
        WHERE event_id = NEW.event_id AND owner_id = NEW.owner_id
          AND media_item_id = NEW.generated_cover_media_item_id
    ) THEN RAISE(ABORT, 'generated cover must be an event member') END;
    SELECT CASE WHEN NEW.cover_override_media_item_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM event_media_items
        WHERE event_id = NEW.event_id AND owner_id = NEW.owner_id
          AND media_item_id = NEW.cover_override_media_item_id
    ) THEN RAISE(ABORT, 'cover override must be an event member') END;
END;

CREATE TRIGGER event_membership_active_insert
BEFORE INSERT ON event_media_items
BEGIN
    SELECT CASE WHEN NOT EXISTS (
        SELECT 1 FROM events
        WHERE event_id = NEW.event_id
          AND owner_id = NEW.owner_id
          AND status = 'active'
    ) THEN RAISE(ABORT, 'event membership requires active owner event') END;
END;

CREATE TRIGGER event_membership_active_update
BEFORE UPDATE ON event_media_items
BEGIN
    SELECT CASE WHEN NOT EXISTS (
        SELECT 1 FROM events
        WHERE event_id = NEW.event_id
          AND owner_id = NEW.owner_id
          AND status = 'active'
    ) THEN RAISE(ABORT, 'event membership requires active owner event') END;
END;

CREATE TRIGGER event_redirect_integrity_insert
BEFORE INSERT ON event_redirects
BEGIN
    SELECT CASE WHEN NOT EXISTS (
        SELECT 1 FROM events
        WHERE event_id = NEW.old_event_id AND owner_id = NEW.owner_id
          AND status = 'redirected'
    ) THEN RAISE(ABORT, 'redirect source must be redirected') END;
    SELECT CASE WHEN NOT EXISTS (
        SELECT 1 FROM events
        WHERE event_id = NEW.new_event_id AND owner_id = NEW.owner_id
          AND status = 'active'
    ) THEN RAISE(ABORT, 'redirect target must be active') END;
    SELECT CASE WHEN EXISTS (
        SELECT 1 FROM event_redirects WHERE old_event_id = NEW.new_event_id
    ) THEN RAISE(ABORT, 'redirect chains are forbidden') END;
END;

CREATE TRIGGER event_redirect_integrity_update
BEFORE UPDATE OF old_event_id, owner_id, new_event_id ON event_redirects
BEGIN
    SELECT CASE WHEN NOT EXISTS (
        SELECT 1 FROM events
        WHERE event_id = NEW.old_event_id AND owner_id = NEW.owner_id
          AND status = 'redirected'
    ) THEN RAISE(ABORT, 'redirect source must be redirected') END;
    SELECT CASE WHEN NOT EXISTS (
        SELECT 1 FROM events
        WHERE event_id = NEW.new_event_id AND owner_id = NEW.owner_id
          AND status = 'active'
    ) THEN RAISE(ABORT, 'redirect target must be active') END;
    SELECT CASE WHEN EXISTS (
        SELECT 1 FROM event_redirects
        WHERE old_event_id = NEW.new_event_id
          AND old_event_id <> OLD.old_event_id
    ) THEN RAISE(ABORT, 'redirect chains are forbidden') END;
END;

CREATE TRIGGER event_redirect_requires_empty
BEFORE UPDATE OF status ON events
WHEN NEW.status IN ('redirected', 'retired')
BEGIN
    SELECT CASE WHEN EXISTS (
        SELECT 1 FROM event_media_items WHERE event_id = NEW.event_id
    ) THEN RAISE(ABORT, 'inactive event must have no members') END;
    SELECT CASE WHEN NEW.generated_cover_media_item_id IS NOT NULL
                  OR NEW.cover_override_media_item_id IS NOT NULL
        THEN RAISE(ABORT, 'inactive event must have no covers') END;
END;

CREATE TRIGGER face_cluster_members_count_delete AFTER DELETE ON face_cluster_members BEGIN
    UPDATE face_clusters
    SET member_count = max(member_count - 1, 0)
    WHERE cluster_id = old.cluster_id;
END;

CREATE TRIGGER face_cluster_members_count_insert AFTER INSERT ON face_cluster_members BEGIN
    UPDATE face_clusters
    SET member_count = member_count + 1
    WHERE cluster_id = new.cluster_id;
END;

CREATE TRIGGER face_items_count_delete AFTER DELETE ON face_items BEGIN
    UPDATE face_results
    SET total_faces = (SELECT count(*) FROM face_items WHERE asset_id = old.asset_id)
    WHERE asset_id = old.asset_id;
END;

CREATE TRIGGER face_items_count_insert AFTER INSERT ON face_items BEGIN
    UPDATE face_results
    SET total_faces = (SELECT count(*) FROM face_items WHERE asset_id = new.asset_id)
    WHERE asset_id = new.asset_id;
END;

CREATE TRIGGER location_projection_asset_delete
BEFORE DELETE ON assets
BEGIN
    INSERT INTO location_projection_state (
        repository_id, owner_id, source_revision, published_revision, updated_at
    )
    SELECT DISTINCT
        node.repository_id,
        old.owner_id,
        1,
        0,
        CAST(unixepoch('subsec') * 1000000 AS INTEGER)
    FROM asset_locations location
    JOIN repository_nodes node ON node.node_id = location.node_id
    WHERE location.asset_id = old.asset_id
    ON CONFLICT (repository_id, owner_id) DO UPDATE SET
        source_revision = location_projection_state.source_revision + 1,
        updated_at = excluded.updated_at;
END;

CREATE TRIGGER location_projection_asset_facts_update
AFTER UPDATE OF owner_id, is_deleted, type, gps_latitude, gps_longitude, gps_geohash_7 ON assets
WHEN old.owner_id IS NOT new.owner_id
  OR old.is_deleted IS NOT new.is_deleted
  OR old.type IS NOT new.type
  OR old.gps_latitude IS NOT new.gps_latitude
  OR old.gps_longitude IS NOT new.gps_longitude
  OR old.gps_geohash_7 IS NOT new.gps_geohash_7
BEGIN
    INSERT INTO location_projection_state (
        repository_id, owner_id, source_revision, published_revision, updated_at
    )
    SELECT DISTINCT
        scope.repository_id,
        scope.owner_id,
        1,
        0,
        CAST(unixepoch('subsec') * 1000000 AS INTEGER)
    FROM (
        SELECT node.repository_id, old.owner_id AS owner_id
        FROM asset_locations location
        JOIN repository_nodes node ON node.node_id = location.node_id
        WHERE location.asset_id = old.asset_id
        UNION
        SELECT node.repository_id, new.owner_id AS owner_id
        FROM asset_locations location
        JOIN repository_nodes node ON node.node_id = location.node_id
        WHERE location.asset_id = new.asset_id
    ) scope
    WHERE scope.owner_id IS NOT NULL
    ON CONFLICT (repository_id, owner_id) DO UPDATE SET
        source_revision = location_projection_state.source_revision + 1,
        updated_at = excluded.updated_at;
END;

CREATE TRIGGER location_projection_clear_terminal_on_source_advance
AFTER UPDATE OF source_revision ON location_projection_state
WHEN NEW.source_revision > OLD.source_revision AND OLD.terminal_error IS NOT NULL
BEGIN
    UPDATE location_projection_state
    SET terminal_error = NULL,
        updated_at = unixepoch('subsec') * 1000000
    WHERE repository_id = NEW.repository_id
      AND owner_id = NEW.owner_id;
END;

CREATE TRIGGER location_projection_location_delete
BEFORE DELETE ON asset_locations
BEGIN
    INSERT INTO location_projection_state (
        repository_id, owner_id, source_revision, published_revision, updated_at
    )
    SELECT
        node.repository_id,
        asset.owner_id,
        1,
        0,
        CAST(unixepoch('subsec') * 1000000 AS INTEGER)
    FROM repository_nodes node
    JOIN assets asset ON asset.asset_id = old.asset_id
    WHERE node.node_id = old.node_id
    ON CONFLICT (repository_id, owner_id) DO UPDATE SET
        source_revision = location_projection_state.source_revision + 1,
        updated_at = excluded.updated_at;
END;

CREATE TRIGGER location_projection_location_insert
AFTER INSERT ON asset_locations
BEGIN
    INSERT INTO location_projection_state (
        repository_id, owner_id, source_revision, published_revision, updated_at
    )
    SELECT
        node.repository_id,
        asset.owner_id,
        1,
        0,
        CAST(unixepoch('subsec') * 1000000 AS INTEGER)
    FROM repository_nodes node
    JOIN assets asset ON asset.asset_id = new.asset_id
    WHERE node.node_id = new.node_id
    ON CONFLICT (repository_id, owner_id) DO UPDATE SET
        source_revision = location_projection_state.source_revision + 1,
        updated_at = excluded.updated_at;
END;

CREATE TRIGGER location_projection_location_update
AFTER UPDATE OF node_id, asset_id, unbound_observation_revision ON asset_locations
WHEN old.node_id IS NOT new.node_id
  OR old.asset_id IS NOT new.asset_id
  OR old.unbound_observation_revision IS NOT new.unbound_observation_revision
BEGIN
    INSERT INTO location_projection_state (
        repository_id, owner_id, source_revision, published_revision, updated_at
    )
    SELECT DISTINCT
        scope.repository_id,
        scope.owner_id,
        1,
        0,
        CAST(unixepoch('subsec') * 1000000 AS INTEGER)
    FROM (
        SELECT node.repository_id, asset.owner_id
        FROM repository_nodes node
        JOIN assets asset ON asset.asset_id = old.asset_id
        WHERE node.node_id = old.node_id
        UNION
        SELECT node.repository_id, asset.owner_id
        FROM repository_nodes node
        JOIN assets asset ON asset.asset_id = new.asset_id
        WHERE node.node_id = new.node_id
    ) scope
    WHERE 1 = 1
    ON CONFLICT (repository_id, owner_id) DO UPDATE SET
        source_revision = location_projection_state.source_revision + 1,
        updated_at = excluded.updated_at;
END;

CREATE TRIGGER location_projection_node_delete
BEFORE DELETE ON repository_nodes
BEGIN
    UPDATE location_projection_state
    SET source_revision = source_revision + 1,
        updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
    WHERE repository_id = old.repository_id;
END;

CREATE TRIGGER location_projection_node_insert
AFTER INSERT ON repository_nodes
BEGIN
    UPDATE location_projection_state
    SET source_revision = source_revision + 1,
        updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
    WHERE repository_id = new.repository_id;
END;

CREATE TRIGGER location_projection_node_update
AFTER UPDATE OF repository_id, parent_node_id, lifecycle ON repository_nodes
WHEN old.repository_id IS NOT new.repository_id
  OR old.parent_node_id IS NOT new.parent_node_id
  OR old.lifecycle IS NOT new.lifecycle
BEGIN
    UPDATE location_projection_state
    SET source_revision = source_revision + 1,
        updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
    WHERE repository_id IN (old.repository_id, new.repository_id);
END;

CREATE TRIGGER location_search_fts_delete AFTER DELETE ON location_clusters BEGIN
    INSERT INTO location_search_fts (
        location_search_fts, rowid, label, country, region, city, geohash
    ) VALUES (
        'delete', old.rowid, old.label, old.country, old.region, old.city, old.geohash
    );
END;

CREATE TRIGGER location_search_fts_insert AFTER INSERT ON location_clusters BEGIN
    INSERT INTO location_search_fts (rowid, label, country, region, city, geohash)
    VALUES (new.rowid, new.label, new.country, new.region, new.city, new.geohash);
END;

CREATE TRIGGER location_search_fts_update
AFTER UPDATE OF label, country, region, city, geohash ON location_clusters BEGIN
    INSERT INTO location_search_fts (
        location_search_fts, rowid, label, country, region, city, geohash
    ) VALUES (
        'delete', old.rowid, old.label, old.country, old.region, old.city, old.geohash
    );
    INSERT INTO location_search_fts (rowid, label, country, region, city, geohash)
    VALUES (new.rowid, new.label, new.country, new.region, new.city, new.geohash);
END;

CREATE TRIGGER music_asset_insert AFTER INSERT ON assets
WHEN new.type = 'AUDIO' AND new.owner_id IS NOT NULL
BEGIN
    INSERT INTO music_tracks (track_id, owner_id, title, created_at, updated_at)
    VALUES (new.asset_id, new.owner_id, new.original_filename,
            CAST(unixepoch('subsec') * 1000000 AS INTEGER),
            CAST(unixepoch('subsec') * 1000000 AS INTEGER))
    ON CONFLICT (track_id) DO NOTHING;
END;

CREATE TRIGGER music_asset_no_longer_audio AFTER UPDATE OF type ON assets
WHEN new.type <> 'AUDIO'
BEGIN
    DELETE FROM music_tracks WHERE track_id = new.asset_id;
END;

CREATE TRIGGER music_asset_owner_update AFTER UPDATE OF owner_id, type ON assets
WHEN new.type = 'AUDIO' AND new.owner_id IS NOT NULL
BEGIN
    INSERT INTO music_tracks (track_id, owner_id, title, created_at, updated_at)
    VALUES (new.asset_id, new.owner_id, new.original_filename,
            CAST(unixepoch('subsec') * 1000000 AS INTEGER),
            CAST(unixepoch('subsec') * 1000000 AS INTEGER))
    ON CONFLICT (track_id) DO UPDATE SET owner_id = excluded.owner_id,
        updated_at = excluded.updated_at;
END;

CREATE TRIGGER music_track_search_delete AFTER DELETE ON music_tracks BEGIN
    DELETE FROM music_search_fts WHERE track_id = old.track_id;
END;

CREATE TRIGGER music_track_search_insert AFTER INSERT ON music_tracks BEGIN
    INSERT INTO music_search_fts (track_id, title, artist, album, filename)
    SELECT new.track_id, new.title, new.artist_name, new.album_title, a.original_filename
    FROM assets a WHERE a.asset_id = new.track_id;
END;

CREATE TRIGGER music_track_search_update AFTER UPDATE OF title, artist_name, album_title ON music_tracks BEGIN
    DELETE FROM music_search_fts WHERE track_id = old.track_id;
    INSERT INTO music_search_fts (track_id, title, artist, album, filename)
    SELECT new.track_id, new.title, new.artist_name, new.album_title, a.original_filename
    FROM assets a WHERE a.asset_id = new.track_id;
END;

CREATE TRIGGER ocr_text_items_count_delete AFTER DELETE ON ocr_text_items BEGIN
    UPDATE ocr_results
    SET total_count = (SELECT count(*) FROM ocr_text_items WHERE asset_id = old.asset_id)
    WHERE asset_id = old.asset_id;
END;

CREATE TRIGGER ocr_text_items_count_insert AFTER INSERT ON ocr_text_items BEGIN
    UPDATE ocr_results
    SET total_count = (SELECT count(*) FROM ocr_text_items WHERE asset_id = new.asset_id)
    WHERE asset_id = new.asset_id;
END;

CREATE TRIGGER search_embeddings_vec_asset_metadata_update
AFTER UPDATE OF owner_id, is_deleted, type ON assets BEGIN
    DELETE FROM search_embeddings_vec
    WHERE rowid IN (
        SELECT id FROM search_embeddings WHERE asset_id = new.asset_id
    );
    INSERT INTO search_embeddings_vec (
        rowid, embedding, space_id, owner_id, is_deleted, asset_type
    )
    SELECT
        embedding.id,
        embedding.vector,
        embedding.space_id,
        new.owner_id,
        new.is_deleted,
        new.type
    FROM search_embeddings embedding
    WHERE embedding.asset_id = new.asset_id;
END;

CREATE TRIGGER search_embeddings_vec_delete AFTER DELETE ON search_embeddings BEGIN
    DELETE FROM search_embeddings_vec WHERE rowid = old.id;
    UPDATE semantic_vector_index_state
    SET row_count = max(row_count - 1, 0),
        rebuild_pending = CASE
            WHEN mode = 'ann' AND row_count - 1 < 5000 THEN 1
            WHEN mode = 'ann'
             AND trained_row_count > 0
             AND max(row_count - 1, 0) * 2 < trained_row_count THEN 1
            ELSE rebuild_pending
        END
    WHERE id = 1;
END;

CREATE TRIGGER search_embeddings_vec_insert AFTER INSERT ON search_embeddings BEGIN
    INSERT INTO search_embeddings_vec (
        rowid, embedding, space_id, owner_id, is_deleted, asset_type
    )
    SELECT
        new.id, new.vector, new.space_id, asset.owner_id, asset.is_deleted, asset.type
    FROM assets asset
    WHERE asset.asset_id = new.asset_id;
    UPDATE semantic_vector_index_state
    SET row_count = row_count + 1,
        rebuild_pending = CASE
            WHEN mode = 'flat' AND row_count + 1 >= 5000 THEN 1
            WHEN mode = 'ann'
             AND trained_row_count > 0
             AND row_count + 1 >= trained_row_count * 2 THEN 1
            ELSE rebuild_pending
        END
    WHERE id = 1;
END;

CREATE TRIGGER search_embeddings_vec_update
AFTER UPDATE OF vector, space_id, asset_id ON search_embeddings BEGIN
    DELETE FROM search_embeddings_vec WHERE rowid = old.id;
    INSERT INTO search_embeddings_vec (
        rowid, embedding, space_id, owner_id, is_deleted, asset_type
    )
    SELECT
        new.id, new.vector, new.space_id, asset.owner_id, asset.is_deleted, asset.type
    FROM assets asset
    WHERE asset.asset_id = new.asset_id;
END;

CREATE TRIGGER species_search_fts_delete AFTER DELETE ON species_predictions BEGIN
    INSERT INTO species_search_fts (species_search_fts, rowid, label)
    VALUES ('delete', old.rowid, old.label);
END;

CREATE TRIGGER species_search_fts_insert AFTER INSERT ON species_predictions BEGIN
    INSERT INTO species_search_fts (rowid, label) VALUES (new.rowid, new.label);
END;

CREATE TRIGGER species_search_fts_update AFTER UPDATE OF label ON species_predictions BEGIN
    INSERT INTO species_search_fts (species_search_fts, rowid, label)
    VALUES ('delete', old.rowid, old.label);
    INSERT INTO species_search_fts (rowid, label) VALUES (new.rowid, new.label);
END;

-- ===== SEEDS =====

-- Seed rows. Values match the historical fresh-catalog defaults exactly.

INSERT INTO settings (id, created_at, updated_at) VALUES (1, 0, 0);

INSERT INTO repository_defaults (id, updated_at) VALUES (1, 0);

INSERT INTO system_state (id, library_id, updated_at)
VALUES (1, lower(hex(randomblob(16))), 0);

INSERT INTO semantic_vector_index_state (
    id, mode, row_count, trained_row_count, rebuild_pending, config, updated_at
) VALUES (
    1, 'flat', 0, 0, 0, '{"index":"flat","distance":"l2"}', 0
);

INSERT INTO classifier_definitions (
    slug, display_name, tag_name, category, positive_prompts, negative_prompts,
    threshold, created_at, updated_at
) VALUES
(
    'documents', 'Documents', 'document', 'smart_album',
    '["a scanned document","a photo of a page of text","a document or paperwork","a page from a book or contract","an official form or letter"]',
    '["a receipt or invoice","a natural scene photograph","a photo of people","a drawing or illustration"]',
    0.03, 0, 0
),
(
    'receipts', 'Receipts', 'receipt', 'smart_album',
    '["a receipt","a store receipt","a restaurant receipt","a photo of an invoice","a bill or purchase receipt"]',
    '["a page from a book or contract","an official form or letter","a natural scene photograph","a photo of people"]',
    0.03, 0, 0
),
(
    'illustration', 'Illustration', 'illustration', 'smart_album',
    '["a digital illustration","a drawing or artwork","a cartoon or anime image","a painting","computer generated art","a comic book page","a manga page with text and speech bubbles","a comic panel with dialogue","an illustrated story page","a screenshot of a digital comic"]',
    '["a real photograph","a photo taken with a camera","a natural scene photograph","a photo of people"]',
    0.03, 0, 0
);

-- Establish the flat Vec1 index shape used by the search runtime. This also
-- creates the Vec1 internal configuration rows.
INSERT INTO search_embeddings_vec (cmd, arg)
VALUES ('rebuild', '{"index":"flat","distance":"l2"}');

PRAGMA user_version = 1;
