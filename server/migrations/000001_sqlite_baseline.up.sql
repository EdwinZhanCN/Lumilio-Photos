-- Lumilio Photos SQLite baseline.
-- Destructive reset is intentional: catalogs from earlier storage engines are
-- not converted.
-- UUIDs are canonical lowercase TEXT, timestamps are UTC Unix microseconds,
-- JSON and array-shaped values are validated TEXT, and vectors are BLOBs.

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

CREATE TABLE registration_sessions (
    session_id TEXT PRIMARY KEY CHECK (session_id = lower(session_id) AND length(session_id) = 36),
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
    webauthn_user_handle BLOB NOT NULL,
    totp_secret_ciphertext BLOB,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
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
) STRICT;

INSERT INTO settings (id, created_at, updated_at) VALUES (1, 0, 0);

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
    last_used_at INTEGER
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

CREATE TABLE refresh_tokens (
    token_id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(user_id),
    token TEXT NOT NULL UNIQUE,
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    is_revoked INTEGER NOT NULL DEFAULT 0 CHECK (is_revoked IN (0, 1))
) STRICT;

CREATE TABLE system_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    library_id TEXT NOT NULL UNIQUE CHECK (length(library_id) = 32),
    bootstrap_phase TEXT NOT NULL DEFAULT 'fresh'
        CHECK (bootstrap_phase IN ('fresh', 'catalog_ready', 'admin_created', 'ready')),
    updated_at INTEGER NOT NULL
) STRICT;

INSERT INTO system_state (id, library_id, updated_at)
VALUES (1, lower(hex(randomblob(16))), 0);

CREATE TABLE repository_roots (
    root_id TEXT PRIMARY KEY CHECK (root_id = lower(root_id) AND length(root_id) = 36),
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    kind TEXT NOT NULL CHECK (kind IN ('default', 'external')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'offline', 'error')),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE repositories (
    repo_id TEXT PRIMARY KEY CHECK (repo_id = lower(repo_id) AND length(repo_id) = 36),
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    config TEXT CHECK (config IS NULL OR json_valid(config)),
    status TEXT NOT NULL DEFAULT 'active',
    last_sync INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    default_owner_id INTEGER REFERENCES users(user_id),
    role TEXT NOT NULL DEFAULT 'regular' CHECK (role IN ('primary', 'regular')),
    root_id TEXT REFERENCES repository_roots(root_id) ON DELETE SET NULL
) STRICT;

CREATE TABLE repository_defaults (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    strategy TEXT NOT NULL DEFAULT 'date' CHECK (strategy IN ('date', 'flat', 'cas')),
    duplicate_handling TEXT NOT NULL DEFAULT 'rename'
        CHECK (duplicate_handling IN ('rename', 'uuid')),
    updated_at INTEGER NOT NULL
) STRICT;

INSERT INTO repository_defaults (id, updated_at) VALUES (1, 0);

CREATE TABLE assets (
    asset_id TEXT PRIMARY KEY CHECK (asset_id = lower(asset_id) AND length(asset_id) = 36),
    owner_id INTEGER REFERENCES users(user_id),
    type TEXT NOT NULL CHECK (type IN ('PHOTO', 'VIDEO', 'AUDIO')),
    original_filename TEXT NOT NULL,
    storage_path TEXT,
    mime_type TEXT NOT NULL,
    file_size INTEGER NOT NULL CHECK (file_size >= 0),
    content_hash TEXT NOT NULL,
    quick_fingerprint TEXT,
    quick_fingerprint_version TEXT,
    width INTEGER,
    height INTEGER,
    duration REAL,
    upload_time INTEGER NOT NULL,
    taken_time INTEGER,
    capture_offset_minutes INTEGER CHECK (capture_offset_minutes IS NULL OR capture_offset_minutes BETWEEN -840 AND 840),
    is_deleted INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0, 1)),
    deleted_at INTEGER,
    specific_metadata TEXT CHECK (specific_metadata IS NULL OR json_valid(specific_metadata)),
    rating INTEGER,
    liked INTEGER NOT NULL DEFAULT 0 CHECK (liked IN (0, 1)),
    repository_id TEXT REFERENCES repositories(repo_id),
    status TEXT NOT NULL DEFAULT '{"state":"processing","message":"Pending processing"}' CHECK (json_valid(status)),
    updated_at INTEGER NOT NULL,
    gps_latitude REAL CHECK (gps_latitude IS NULL OR gps_latitude BETWEEN -90 AND 90),
    gps_longitude REAL CHECK (gps_longitude IS NULL OR gps_longitude BETWEEN -180 AND 180),
    gps_geohash_5 TEXT,
    gps_geohash_7 TEXT,
    exif_raw TEXT CHECK (exif_raw IS NULL OR json_valid(exif_raw)),
    UNIQUE (repository_id, storage_path)
) STRICT;

CREATE TABLE repository_scan_runs (
    scan_id TEXT PRIMARY KEY CHECK (scan_id = lower(scan_id) AND length(scan_id) = 36),
    repository_id TEXT NOT NULL REFERENCES repositories(repo_id) ON DELETE CASCADE,
    mode TEXT NOT NULL CHECK (mode IN ('periodic', 'manual')),
    requested_by TEXT,
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled')),
    started_at INTEGER NOT NULL,
    finished_at INTEGER,
    discovered_count INTEGER NOT NULL DEFAULT 0,
    updated_count INTEGER NOT NULL DEFAULT 0,
    deleted_count INTEGER NOT NULL DEFAULT 0,
    skipped_count INTEGER NOT NULL DEFAULT 0,
    error TEXT
) STRICT;

CREATE TABLE tags (
    tag_id INTEGER PRIMARY KEY,
    tag_name TEXT NOT NULL UNIQUE,
    category TEXT,
    is_ai_generated INTEGER NOT NULL DEFAULT 1 CHECK (is_ai_generated IN (0, 1))
) STRICT;

CREATE TABLE asset_tags (
    asset_id TEXT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    tag_id INTEGER NOT NULL REFERENCES tags(tag_id) ON DELETE CASCADE,
    confidence REAL NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    source TEXT NOT NULL DEFAULT 'system'
        CHECK (source IN ('system', 'user', 'ai', 'bioclip_classify', 'zeroshot')),
    PRIMARY KEY (asset_id, tag_id)
) STRICT;

CREATE TABLE thumbnails (
    thumbnail_id INTEGER PRIMARY KEY,
    asset_id TEXT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    size TEXT NOT NULL CHECK (size IN ('small', 'medium', 'large')),
    storage_path TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    UNIQUE (asset_id, size)
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

CREATE TABLE album_assets (
    album_id INTEGER NOT NULL REFERENCES albums(album_id) ON DELETE CASCADE,
    asset_id TEXT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    added_time INTEGER NOT NULL,
    PRIMARY KEY (album_id, asset_id)
) STRICT;

CREATE TABLE media_items (
    media_item_id TEXT PRIMARY KEY CHECK (media_item_id = lower(media_item_id) AND length(media_item_id) = 36),
    owner_id INTEGER REFERENCES users(user_id),
    repository_id TEXT REFERENCES repositories(repo_id),
    media_kind TEXT NOT NULL DEFAULT 'photo' CHECK (media_kind IN ('photo', 'video', 'audio', 'live_photo')),
    primary_asset_id TEXT REFERENCES assets(asset_id) ON DELETE SET NULL,
    group_key TEXT,
    created_at INTEGER NOT NULL,
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

CREATE TABLE asset_stacks (
    stack_id TEXT PRIMARY KEY CHECK (stack_id = lower(stack_id) AND length(stack_id) = 36),
    owner_id INTEGER REFERENCES users(user_id),
    repository_id TEXT REFERENCES repositories(repo_id),
    stack_kind TEXT NOT NULL DEFAULT 'manual' CHECK (stack_kind IN ('manual', 'burst')),
    cover_media_item_id TEXT REFERENCES media_items(media_item_id) ON DELETE SET NULL,
    group_key TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE asset_stack_members (
    media_item_id TEXT PRIMARY KEY REFERENCES media_items(media_item_id) ON DELETE CASCADE,
    stack_id TEXT NOT NULL REFERENCES asset_stacks(stack_id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
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
) STRICT;

CREATE UNIQUE INDEX location_clusters_scope_key
    ON location_clusters (coalesce(owner_id, -1), repository_id, geohash);

CREATE TABLE location_cluster_assets (
    cluster_id TEXT NOT NULL REFERENCES location_clusters(cluster_id) ON DELETE CASCADE,
    asset_id TEXT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (cluster_id, asset_id)
) STRICT;

CREATE TABLE reverse_geocode_cache (
    cache_key TEXT PRIMARY KEY,
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
    expires_at INTEGER
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

CREATE TABLE face_results (
    asset_id TEXT PRIMARY KEY REFERENCES assets(asset_id) ON DELETE CASCADE,
    model_id TEXT NOT NULL,
    total_faces INTEGER NOT NULL DEFAULT 0 CHECK (total_faces >= 0),
    processing_time_ms INTEGER,
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

CREATE TABLE ocr_results (
    asset_id TEXT PRIMARY KEY REFERENCES assets(asset_id) ON DELETE CASCADE,
    model_id TEXT NOT NULL,
    total_count INTEGER NOT NULL DEFAULT 0 CHECK (total_count >= 0),
    processing_time_ms INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    full_text TEXT NOT NULL DEFAULT ''
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

CREATE TABLE species_predictions (
    prediction_id INTEGER PRIMARY KEY,
    asset_id TEXT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    score REAL NOT NULL CHECK (score BETWEEN 0 AND 1),
    created_at INTEGER NOT NULL,
    UNIQUE (asset_id, label)
) STRICT;

CREATE TABLE asset_quality_scores (
    asset_id TEXT PRIMARY KEY REFERENCES assets(asset_id) ON DELETE CASCADE,
    score REAL NOT NULL CHECK (score BETWEEN 1 AND 10),
    model_version TEXT NOT NULL DEFAULT 'v1',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
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

CREATE TABLE agent_checkpoints (
    id TEXT PRIMARY KEY,
    data BLOB NOT NULL,
    updated_at INTEGER NOT NULL
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
    status TEXT NOT NULL DEFAULT 'queued',
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

CREATE TABLE repository_cloud_bindings (
    repository_id TEXT NOT NULL REFERENCES repositories(repo_id) ON DELETE CASCADE,
    credential_id TEXT NOT NULL REFERENCES cloud_credentials(credential_id) ON DELETE RESTRICT,
    owner_id INTEGER NOT NULL REFERENCES users(user_id),
    provider TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    last_import_run_id TEXT REFERENCES cloud_import_runs(run_id) ON DELETE SET NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (repository_id, provider)
) STRICT;

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
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (user_id, thread_id) REFERENCES agent_threads(user_id, thread_id) ON DELETE CASCADE
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

-- Identity and storage access paths.
CREATE INDEX idx_refresh_tokens_tokens_token ON refresh_tokens (token);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens (user_id);
CREATE INDEX idx_registration_sessions_expires_at ON registration_sessions (expires_at);
CREATE INDEX idx_user_mfa_recovery_codes_unused ON user_mfa_recovery_codes (user_id) WHERE used_at IS NULL;
CREATE INDEX idx_user_mfa_recovery_codes_user_id ON user_mfa_recovery_codes (user_id);
CREATE INDEX idx_user_webauthn_credentials_user_id ON user_webauthn_credentials (user_id);
CREATE INDEX idx_users_role ON users (role);
CREATE UNIQUE INDEX idx_users_webauthn_user_handle ON users (webauthn_user_handle);

CREATE UNIQUE INDEX repository_roots_one_default_idx ON repository_roots (kind) WHERE kind = 'default';
CREATE INDEX repositories_root_id_idx ON repositories (root_id);
CREATE INDEX idx_repositories_default_owner ON repositories (default_owner_id);
CREATE INDEX idx_repositories_path ON repositories (path);
CREATE INDEX idx_repositories_role ON repositories (role);
CREATE INDEX idx_repositories_status ON repositories (status);
CREATE UNIQUE INDEX repositories_one_primary_idx ON repositories (role) WHERE role = 'primary';

CREATE INDEX idx_asset_tags_tag_source_asset ON asset_tags (tag_id, source, asset_id);
CREATE INDEX idx_assets_camera_model_active
    ON assets (json_extract(specific_metadata, '$.camera_model'))
    WHERE is_deleted = 0 AND json_type(specific_metadata, '$.camera_model') IS NOT NULL;
CREATE INDEX idx_assets_gps_geohash_5 ON assets (gps_geohash_5)
    WHERE gps_geohash_5 IS NOT NULL AND is_deleted = 0;
CREATE INDEX idx_assets_gps_geohash_7 ON assets (gps_geohash_7)
    WHERE gps_geohash_7 IS NOT NULL AND is_deleted = 0;
CREATE INDEX idx_assets_gps_lat_lng ON assets (gps_latitude, gps_longitude)
    WHERE gps_latitude IS NOT NULL AND gps_longitude IS NOT NULL AND is_deleted = 0;
CREATE INDEX idx_assets_content_hash ON assets (content_hash);
CREATE INDEX idx_assets_quick_fingerprint ON assets (quick_fingerprint, file_size)
    WHERE quick_fingerprint IS NOT NULL;

CREATE INDEX idx_assets_lens_model_active
    ON assets (json_extract(specific_metadata, '$.lens_model'))
    WHERE is_deleted = 0 AND json_type(specific_metadata, '$.lens_model') IS NOT NULL;
CREATE INDEX idx_assets_liked ON assets (liked) WHERE liked = 1;
CREATE INDEX idx_assets_list_opt ON assets (owner_id, type, coalesce(taken_time, upload_time) DESC)
    WHERE is_deleted = 0;
CREATE INDEX idx_assets_mime_time_active
    ON assets (mime_type, coalesce(taken_time, upload_time) DESC, asset_id DESC)
    WHERE is_deleted = 0;
CREATE INDEX idx_assets_owner_id ON assets (owner_id);
CREATE INDEX idx_assets_owner_time_active
    ON assets (owner_id, coalesce(taken_time, upload_time) DESC, asset_id DESC)
    WHERE is_deleted = 0;
CREATE INDEX idx_assets_rating ON assets (rating) WHERE rating IS NOT NULL;
CREATE INDEX idx_assets_rating_liked ON assets (rating, liked) WHERE rating IS NOT NULL OR liked = 1;
CREATE INDEX idx_assets_repo_time_active
    ON assets (repository_id, coalesce(taken_time, upload_time) DESC, asset_id DESC)
    WHERE is_deleted = 0;
CREATE INDEX idx_assets_repo_type_time_active
    ON assets (repository_id, type, coalesce(taken_time, upload_time) DESC, asset_id DESC)
    WHERE is_deleted = 0;
CREATE INDEX idx_assets_repository_id ON assets (repository_id);
CREATE INDEX idx_assets_status_state_time_active
    ON assets (json_extract(status, '$.state'), coalesce(taken_time, upload_time) DESC, asset_id DESC)
    WHERE is_deleted = 0;
CREATE INDEX idx_assets_taken_time ON assets (taken_time);
CREATE INDEX idx_assets_type ON assets (type);
CREATE INDEX idx_assets_type_taken_time_coalesce
    ON assets (type, coalesce(taken_time, upload_time) DESC)
    WHERE is_deleted = 0;
CREATE INDEX idx_repository_scan_runs_repo_started ON repository_scan_runs (repository_id, started_at DESC);
CREATE INDEX idx_repository_scan_runs_running ON repository_scan_runs (repository_id) WHERE status = 'running';
CREATE UNIQUE INDEX repository_scan_runs_one_running ON repository_scan_runs (repository_id) WHERE status = 'running';
CREATE INDEX idx_thumbnails_asset_id ON thumbnails (asset_id);

-- Collections, logical media, duplicate, and location access paths.
CREATE INDEX idx_album_assets_album_order ON album_assets (album_id, position, added_time, asset_id);
CREATE INDEX idx_album_assets_asset ON album_assets (asset_id);
CREATE INDEX idx_albums_type ON albums (album_type);
CREATE INDEX idx_albums_user_created_at ON albums (user_id, created_at DESC, album_id DESC);
CREATE INDEX idx_albums_user_id ON albums (user_id);
CREATE INDEX idx_asset_stack_members_stack ON asset_stack_members (stack_id);
CREATE UNIQUE INDEX idx_asset_stacks_burst_group_key ON asset_stacks (group_key)
    WHERE stack_kind = 'burst' AND group_key IS NOT NULL;
CREATE INDEX idx_media_item_assets_item ON media_item_assets (media_item_id);
CREATE INDEX idx_media_item_assets_item_relation ON media_item_assets (media_item_id, relation, asset_id);
CREATE INDEX idx_media_items_primary_asset ON media_items (primary_asset_id);
CREATE INDEX idx_media_items_repository_owner ON media_items (repository_id, owner_id, media_item_id);
CREATE INDEX idx_media_items_owner_repository ON media_items (owner_id, repository_id);
CREATE INDEX idx_asset_stack_members_stack_position ON asset_stack_members (stack_id, position, media_item_id);
CREATE INDEX idx_asset_stacks_kind ON asset_stacks (stack_kind, stack_id);
CREATE INDEX idx_duplicate_group_assets_asset ON duplicate_group_assets (asset_id);
CREATE INDEX idx_duplicate_group_edges_assets ON duplicate_group_edges (asset_id_a, asset_id_b);
CREATE INDEX idx_duplicate_groups_repo_status ON duplicate_groups (repository_id, status, detected_at DESC);
CREATE INDEX idx_duplicate_groups_status ON duplicate_groups (status);
CREATE INDEX idx_duplicate_groups_owner_repo ON duplicate_groups (owner_id, repository_id);
CREATE INDEX idx_location_cluster_assets_asset ON location_cluster_assets (asset_id);
CREATE INDEX idx_location_clusters_repository_owner ON location_clusters (repository_id, owner_id);
CREATE INDEX idx_location_clusters_status ON location_clusters (geocode_status);
CREATE INDEX idx_reverse_geocode_cache_provider_language ON reverse_geocode_cache (provider, language);

-- ML and analysis access paths. Vector KNN indexes are the vec0 virtual tables below.
CREATE UNIQUE INDEX embedding_spaces_default_per_type_idx
    ON embedding_spaces (embedding_type) WHERE is_default_search = 1;
CREATE UNIQUE INDEX embedding_spaces_identity_idx
    ON embedding_spaces (embedding_type, model_id, dimensions, distance_metric);
CREATE INDEX embeddings_asset_type_idx ON embeddings (asset_id, embedding_type);
CREATE UNIQUE INDEX embeddings_one_primary_per_asset_type_idx
    ON embeddings (asset_id, embedding_type) WHERE is_primary = 1;
CREATE INDEX embeddings_primary_idx ON embeddings (embedding_type, is_primary) WHERE is_primary = 1;
CREATE INDEX embeddings_space_primary_asset_idx ON embeddings (space_id, is_primary, asset_id);
CREATE INDEX embeddings_type_model_idx ON embeddings (embedding_type, embedding_model);
CREATE INDEX face_cluster_members_cluster_idx ON face_cluster_members (cluster_id);
CREATE INDEX face_cluster_members_face_idx ON face_cluster_members (face_id);
CREATE UNIQUE INDEX face_cluster_members_face_unique_idx ON face_cluster_members (face_id);
CREATE INDEX face_cluster_members_similarity_idx ON face_cluster_members (similarity_score);
CREATE INDEX face_clusters_confirmed_idx ON face_clusters (is_confirmed) WHERE is_confirmed = 1;
CREATE INDEX face_clusters_owner_idx ON face_clusters (owner_id);
CREATE INDEX face_clusters_hidden_idx ON face_clusters (is_hidden, updated_at DESC);
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
CREATE INDEX idx_classifier_definitions_enabled ON classifier_definitions (enabled);
CREATE INDEX idx_species_predictions_asset_id ON species_predictions (asset_id);
CREATE INDEX idx_species_predictions_label ON species_predictions (label);
CREATE INDEX idx_species_predictions_label_asset_score
    ON species_predictions (label, asset_id, score DESC) WHERE score >= 0.5;
CREATE INDEX idx_species_predictions_label_score ON species_predictions (label, score DESC);
CREATE INDEX idx_species_predictions_score ON species_predictions (score DESC);
CREATE INDEX ocr_results_asset_id_idx ON ocr_results (asset_id);
CREATE INDEX ocr_results_created_at_idx ON ocr_results (created_at);
CREATE INDEX ocr_results_model_id_idx ON ocr_results (model_id);
CREATE INDEX ocr_text_items_asset_id_idx ON ocr_text_items (asset_id);
CREATE INDEX ocr_text_items_confidence_idx ON ocr_text_items (confidence);
CREATE INDEX ocr_text_items_text_length_idx ON ocr_text_items (text_length);
CREATE UNIQUE INDEX search_embeddings_asset_primary_uniq
    ON search_embeddings (asset_id) WHERE frame_ts_ms IS NULL;
CREATE UNIQUE INDEX search_embeddings_asset_frame_uniq
    ON search_embeddings (asset_id, frame_ts_ms) WHERE frame_ts_ms IS NOT NULL;
CREATE INDEX search_embeddings_asset_idx ON search_embeddings (asset_id);

-- Cloud, sharing, and Agent runtime access paths.
CREATE INDEX idx_agent_pins_user ON agent_pins (user_id, created_at DESC);
CREATE INDEX idx_cloud_credentials_provider_status ON cloud_credentials (provider, status);
CREATE INDEX idx_cloud_credentials_owner_created ON cloud_credentials (owner_id, created_at DESC);
CREATE INDEX idx_cloud_import_runs_credential_created ON cloud_import_runs (credential_id, created_at DESC);
CREATE INDEX idx_cloud_import_runs_repository_created ON cloud_import_runs (repository_id, created_at DESC);
CREATE INDEX idx_cloud_import_runs_status ON cloud_import_runs (status);
CREATE INDEX idx_cloud_import_runs_owner_created ON cloud_import_runs (owner_id, created_at DESC);
CREATE INDEX idx_repository_cloud_bindings_credential ON repository_cloud_bindings (credential_id);
CREATE INDEX idx_repository_cloud_bindings_owner ON repository_cloud_bindings (owner_id, created_at DESC);
CREATE INDEX idx_share_links_owner ON share_links (owner_id, created_at DESC);
CREATE INDEX idx_share_links_status_expires ON share_links (status, expires_at);
CREATE UNIQUE INDEX idx_agent_runs_one_active_thread ON agent_runs (user_id, thread_id)
    WHERE status IN ('running', 'cancel_requested', 'awaiting_confirmation');
CREATE INDEX idx_agent_runs_thread_created ON agent_runs (user_id, thread_id, created_at DESC);
CREATE INDEX idx_agent_refs_expiry ON agent_refs (expires_at);
CREATE INDEX idx_agent_pending_effects_thread ON agent_pending_effects (user_id, thread_id, created_at DESC);

-- Exact vector search. Authoritative BLOBs remain in ordinary STRICT tables;
-- these vec0 tables are rebuildable query structures keyed by the same rowid.
CREATE VIRTUAL TABLE search_embeddings_vec USING vec0(embedding float[768]);
CREATE VIRTUAL TABLE face_embeddings_vec USING vec0(embedding float[512]);

CREATE TRIGGER search_embeddings_vec_insert AFTER INSERT ON search_embeddings BEGIN
    INSERT INTO search_embeddings_vec (rowid, embedding) VALUES (new.id, new.vector);
END;
CREATE TRIGGER search_embeddings_vec_update AFTER UPDATE OF vector ON search_embeddings BEGIN
    DELETE FROM search_embeddings_vec WHERE rowid = old.id;
    INSERT INTO search_embeddings_vec (rowid, embedding) VALUES (new.id, new.vector);
END;
CREATE TRIGGER search_embeddings_vec_delete AFTER DELETE ON search_embeddings BEGIN
    DELETE FROM search_embeddings_vec WHERE rowid = old.id;
END;

CREATE TRIGGER face_embeddings_vec_insert AFTER INSERT ON face_items
WHEN new.embedding IS NOT NULL BEGIN
    INSERT INTO face_embeddings_vec (rowid, embedding) VALUES (new.id, new.embedding);
END;
CREATE TRIGGER face_embeddings_vec_update AFTER UPDATE OF embedding ON face_items BEGIN
    DELETE FROM face_embeddings_vec WHERE rowid = old.id;
    INSERT INTO face_embeddings_vec (rowid, embedding)
    SELECT new.id, new.embedding WHERE new.embedding IS NOT NULL;
END;
CREATE TRIGGER face_embeddings_vec_delete AFTER DELETE ON face_items
WHEN old.embedding IS NOT NULL BEGIN
    DELETE FROM face_embeddings_vec WHERE rowid = old.id;
END;

-- FTS5 trigram indexes provide indexed substring and full-text lookup.
CREATE VIRTUAL TABLE asset_search_fts USING fts5(
    original_filename,
    content = 'assets',
    content_rowid = 'rowid',
    tokenize = 'trigram'
);
CREATE VIRTUAL TABLE ocr_search_fts USING fts5(
    full_text,
    content = 'ocr_results',
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
CREATE VIRTUAL TABLE species_search_fts USING fts5(
    label,
    content = 'species_predictions',
    content_rowid = 'rowid',
    tokenize = 'trigram'
);

CREATE TRIGGER asset_search_fts_insert AFTER INSERT ON assets BEGIN
    INSERT INTO asset_search_fts (rowid, original_filename) VALUES (new.rowid, new.original_filename);
END;
CREATE TRIGGER asset_search_fts_delete AFTER DELETE ON assets BEGIN
    INSERT INTO asset_search_fts (asset_search_fts, rowid, original_filename)
    VALUES ('delete', old.rowid, old.original_filename);
END;
CREATE TRIGGER asset_search_fts_update AFTER UPDATE OF original_filename ON assets BEGIN
    INSERT INTO asset_search_fts (asset_search_fts, rowid, original_filename)
    VALUES ('delete', old.rowid, old.original_filename);
    INSERT INTO asset_search_fts (rowid, original_filename) VALUES (new.rowid, new.original_filename);
END;

CREATE TRIGGER ocr_search_fts_insert AFTER INSERT ON ocr_results BEGIN
    INSERT INTO ocr_search_fts (rowid, full_text) VALUES (new.rowid, new.full_text);
END;
CREATE TRIGGER ocr_search_fts_delete AFTER DELETE ON ocr_results BEGIN
    INSERT INTO ocr_search_fts (ocr_search_fts, rowid, full_text)
    VALUES ('delete', old.rowid, old.full_text);
END;
CREATE TRIGGER ocr_search_fts_update AFTER UPDATE OF full_text ON ocr_results BEGIN
    INSERT INTO ocr_search_fts (ocr_search_fts, rowid, full_text)
    VALUES ('delete', old.rowid, old.full_text);
    INSERT INTO ocr_search_fts (rowid, full_text) VALUES (new.rowid, new.full_text);
END;

CREATE TRIGGER location_search_fts_insert AFTER INSERT ON location_clusters BEGIN
    INSERT INTO location_search_fts (rowid, label, country, region, city, geohash)
    VALUES (new.rowid, new.label, new.country, new.region, new.city, new.geohash);
END;
CREATE TRIGGER location_search_fts_delete AFTER DELETE ON location_clusters BEGIN
    INSERT INTO location_search_fts (
        location_search_fts, rowid, label, country, region, city, geohash
    ) VALUES (
        'delete', old.rowid, old.label, old.country, old.region, old.city, old.geohash
    );
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

CREATE TRIGGER species_search_fts_insert AFTER INSERT ON species_predictions BEGIN
    INSERT INTO species_search_fts (rowid, label) VALUES (new.rowid, new.label);
END;
CREATE TRIGGER species_search_fts_delete AFTER DELETE ON species_predictions BEGIN
    INSERT INTO species_search_fts (species_search_fts, rowid, label)
    VALUES ('delete', old.rowid, old.label);
END;
CREATE TRIGGER species_search_fts_update AFTER UPDATE OF label ON species_predictions BEGIN
    INSERT INTO species_search_fts (species_search_fts, rowid, label)
    VALUES ('delete', old.rowid, old.label);
    INSERT INTO species_search_fts (rowid, label) VALUES (new.rowid, new.label);
END;

-- Small deterministic aggregate-maintenance triggers.
CREATE TRIGGER face_cluster_members_count_insert AFTER INSERT ON face_cluster_members BEGIN
    UPDATE face_clusters
    SET member_count = member_count + 1
    WHERE cluster_id = new.cluster_id;
END;
CREATE TRIGGER face_cluster_members_count_delete AFTER DELETE ON face_cluster_members BEGIN
    UPDATE face_clusters
    SET member_count = max(member_count - 1, 0)
    WHERE cluster_id = old.cluster_id;
END;
CREATE TRIGGER face_items_count_insert AFTER INSERT ON face_items BEGIN
    UPDATE face_results
    SET total_faces = (SELECT count(*) FROM face_items WHERE asset_id = new.asset_id)
    WHERE asset_id = new.asset_id;
END;
CREATE TRIGGER face_items_count_delete AFTER DELETE ON face_items BEGIN
    UPDATE face_results
    SET total_faces = (SELECT count(*) FROM face_items WHERE asset_id = old.asset_id)
    WHERE asset_id = old.asset_id;
END;
CREATE TRIGGER ocr_text_items_count_insert AFTER INSERT ON ocr_text_items BEGIN
    UPDATE ocr_results
    SET total_count = (SELECT count(*) FROM ocr_text_items WHERE asset_id = new.asset_id)
    WHERE asset_id = new.asset_id;
END;
CREATE TRIGGER ocr_text_items_count_delete AFTER DELETE ON ocr_text_items BEGIN
    UPDATE ocr_results
    SET total_count = (SELECT count(*) FROM ocr_text_items WHERE asset_id = old.asset_id)
    WHERE asset_id = old.asset_id;
END;

-- Canonical browse facts view: composition and stack filter derive exclusively from here.
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

PRAGMA user_version = 3;
