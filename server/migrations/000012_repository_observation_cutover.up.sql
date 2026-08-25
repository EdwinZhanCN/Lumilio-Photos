-- ROE v2 is a forward-only destructive media-catalog cutover. Repository
-- registrations, users, settings, lifecycle journals, and original files are
-- intentionally outside this transaction. The application creates and
-- validates an Online Backup before this migration is allowed to begin.

-- Containers that are themselves derived from media must not survive as
-- empty, misleading projections. User-authored album/tag definitions survive;
-- their asset memberships are removed by the Asset cascade below.
-- Triggers compiled against the old Asset table are recreated after the
-- replacement takes its canonical name; otherwise SQLite quite correctly
-- rejects the transient no-`assets` statement boundary.
DROP TRIGGER search_embeddings_vec_insert;
DROP TRIGGER search_embeddings_vec_update;
DROP TRIGGER search_embeddings_vec_asset_metadata_update;
DROP TRIGGER asset_search_fts_insert;
DROP TRIGGER asset_search_fts_delete;
DROP TRIGGER asset_search_fts_update;

DELETE FROM event_redirects;
DELETE FROM events;
DELETE FROM duplicate_groups;
DELETE FROM location_clusters;
DELETE FROM face_clusters;
DELETE FROM asset_stacks;
DELETE FROM media_items;
DELETE FROM cloud_sync_files;
DELETE FROM cloud_import_runs;

-- Queue consumers are not started until this maintenance transaction and its
-- verified backup boundary complete. Every pre-cutover job is discarded:
-- media jobs reference Assets that are about to be purged, and obsolete scan,
-- discovery, and path-bearing ingest payloads must never reach the v2 runtime.
DELETE FROM river_job;

-- This is the destructive catalog boundary. FK actions clear thumbnails,
-- metadata, faces, OCR, ML/search projections, shares, memberships, and all
-- other rows owned by an old path-keyed Asset. SET NULL relationships leave
-- their user-created container intact without publishing stale media state.
DELETE FROM assets;
DELETE FROM repository_scan_runs;

DROP TABLE repository_file_index;
DROP TABLE repository_scan_runs;
DROP TABLE assets;

-- Give the normalized ROE tables their permanent production names. SQLite
-- rewrites inbound FK targets (including asset_locations and observation
-- state) as part of these renames.
ALTER TABLE assets_v2 RENAME TO assets;
ALTER TABLE repository_scan_runs_v2 RENAME TO repository_scan_runs;

-- Derived private files live inside one repository. Once an Asset may have
-- multiple Locations, the derived row must remember the repository that owns
-- its private path instead of inferring one from Asset identity.
ALTER TABLE thumbnails ADD COLUMN repository_id TEXT
    REFERENCES repositories(repo_id) ON DELETE CASCADE;
ALTER TABLE face_items ADD COLUMN repository_id TEXT
    REFERENCES repositories(repo_id) ON DELETE CASCADE;

-- Private staging is a small explicit filesystem/catalog commit journal. It is
-- the only durable place allowed to carry a private staging or inbox target;
-- River receives only commit_id, and completed records are bounded history.
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

CREATE INDEX idx_repository_staging_commits_recovery
    ON repository_staging_commits (status, updated_at, commit_id)
    WHERE status IN ('prepared', 'committing', 'committed');

DROP INDEX idx_assets_v2_taken_time;
DROP INDEX idx_assets_v2_owner_deleted;
CREATE INDEX idx_assets_taken_time ON assets (taken_time DESC, asset_id);
CREATE INDEX idx_assets_owner_deleted ON assets (owner_id, is_deleted, asset_id);

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

CREATE TRIGGER asset_search_fts_insert AFTER INSERT ON assets BEGIN
    INSERT INTO asset_search_fts (rowid, original_filename)
    VALUES (new.rowid, new.original_filename);
END;

CREATE TRIGGER asset_search_fts_delete AFTER DELETE ON assets BEGIN
    INSERT INTO asset_search_fts (asset_search_fts, rowid, original_filename)
    VALUES ('delete', old.rowid, old.original_filename);
END;

CREATE TRIGGER asset_search_fts_update AFTER UPDATE OF original_filename ON assets BEGIN
    INSERT INTO asset_search_fts (asset_search_fts, rowid, original_filename)
    VALUES ('delete', old.rowid, old.original_filename);
    INSERT INTO asset_search_fts (rowid, original_filename)
    VALUES (new.rowid, new.original_filename);
END;

-- Read-only occurrence projection for repository-scoped browsing and
-- diagnostics. Relative paths remain an on-demand node-graph traversal and
-- deliberately are not stored by this view or by Asset identity.
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

-- Folder constraints are evaluated from the node graph at query time. This
-- view is deliberately a projection: no full path is persisted as Asset or
-- Location identity, and a directory rename still mutates one graph edge.
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

DROP INDEX repository_scan_runs_v2_one_active;
DROP INDEX idx_repository_scan_runs_v2_history;
CREATE UNIQUE INDEX repository_scan_runs_one_active
    ON repository_scan_runs (repository_id)
    WHERE status IN ('queued', 'crawling', 'catching_up', 'finalizing');
CREATE INDEX idx_repository_scan_runs_history
    ON repository_scan_runs (repository_id, created_at DESC, run_id);

-- Allocate one durable migration receipt per currently reachable repository.
-- A short-lived mapping table keeps UUID generation stable across the state,
-- run, and outbox inserts and is removed before commit.
CREATE TABLE roe_cutover_runs (
    repository_id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL
) STRICT;

INSERT INTO roe_cutover_runs (repository_id, run_id, created_at)
SELECT
    repo_id,
    lower(
        substr(hex(randomblob(16)), 1, 8) || '-' ||
        substr(hex(randomblob(16)), 1, 4) || '-' ||
        '4' || substr(hex(randomblob(16)), 1, 3) || '-' ||
        '8' || substr(hex(randomblob(16)), 1, 3) || '-' ||
        substr(hex(randomblob(16)), 1, 12)
    ),
    CAST(unixepoch('subsec') * 1000000 AS INTEGER)
FROM repositories
WHERE reachability = 'active';

INSERT INTO repository_observation_state (
    repository_id,
    desired_epoch,
    applied_epoch,
    adapter_kind,
    cursor_health,
    full_verification_required,
    updated_at
)
SELECT repository_id, 1, 0, 'periodic', 'unavailable', 1, created_at
FROM roe_cutover_runs
WHERE true
ON CONFLICT (repository_id) DO UPDATE SET
    desired_epoch = MAX(repository_observation_state.desired_epoch + 1, 1),
    full_verification_required = 1,
    cursor_health = 'unavailable',
    updated_at = excluded.updated_at;

INSERT INTO repository_scan_runs (
    run_id,
    repository_id,
    requested_epoch,
    mode,
    requested_by,
    status,
    created_at,
    updated_at
)
SELECT
    cutover.run_id,
    cutover.repository_id,
    state.desired_epoch,
    'migration',
    'schema-000012',
    'queued',
    cutover.created_at,
    cutover.created_at
FROM roe_cutover_runs cutover
JOIN repository_observation_state state
  ON state.repository_id = cutover.repository_id;

INSERT INTO repository_outbox (
    outbox_id,
    repository_id,
    effect_key,
    effect_kind,
    entity_id,
    expected_revision,
    payload,
    status,
    created_at,
    updated_at
)
SELECT
    lower(
        substr(hex(randomblob(16)), 1, 8) || '-' ||
        substr(hex(randomblob(16)), 1, 4) || '-' ||
        '4' || substr(hex(randomblob(16)), 1, 3) || '-' ||
        '8' || substr(hex(randomblob(16)), 1, 3) || '-' ||
        substr(hex(randomblob(16)), 1, 12)
    ),
    cutover.repository_id,
    'controller:migration:' || cutover.run_id,
    'controller',
    cutover.run_id,
    state.desired_epoch,
    '{}',
    'pending',
    cutover.created_at,
    cutover.created_at
FROM roe_cutover_runs cutover
JOIN repository_observation_state state
  ON state.repository_id = cutover.repository_id;

DROP TABLE roe_cutover_runs;
