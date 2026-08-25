-- Location clusters are an eventually consistent projection over Asset GPS
-- facts and active repository occurrences. This ledger is the correctness
-- boundary for coalesced River work: source mutations advance the revision in
-- the same transaction, while a worker publishes a revision only after every
-- bounded reconciliation turn has converged.
CREATE TABLE location_projection_state (
    repository_id TEXT NOT NULL
        REFERENCES repositories(repo_id) ON DELETE CASCADE,
    owner_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    source_revision INTEGER NOT NULL DEFAULT 1 CHECK (source_revision > 0),
    published_revision INTEGER NOT NULL DEFAULT 0
        CHECK (published_revision >= 0 AND published_revision <= source_revision),
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (repository_id, owner_id)
) STRICT;

CREATE INDEX idx_location_projection_state_pending
    ON location_projection_state (updated_at, repository_id, owner_id)
    WHERE source_revision > published_revision;

INSERT INTO location_projection_state (
    repository_id, owner_id, source_revision, published_revision, updated_at
)
SELECT
    scope.repository_id,
    scope.owner_id,
    1,
    0,
    CAST(unixepoch('subsec') * 1000000 AS INTEGER)
FROM (
    SELECT node.repository_id, asset.owner_id
    FROM asset_locations location
    JOIN repository_nodes node ON node.node_id = location.node_id
    JOIN assets asset ON asset.asset_id = location.asset_id
    UNION
    SELECT repository_id, owner_id
    FROM location_clusters
    WHERE owner_id IS NOT NULL
) scope;

-- Asset facts affect every repository in which that Asset has a Location.
-- IS NOT comparisons keep idempotent metadata retries from manufacturing a
-- new projection revision.
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

-- Location binding/unbinding and node reassignment change occurrence
-- membership even when the Asset's GPS facts are unchanged.
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

-- Reachability is a property of the repository graph. A structural node
-- mutation conservatively invalidates every owner scope already known for the
-- repository; Location insertion creates the first state row when necessary.
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

CREATE TRIGGER location_projection_node_delete
BEFORE DELETE ON repository_nodes
BEGIN
    UPDATE location_projection_state
    SET source_revision = source_revision + 1,
        updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
    WHERE repository_id = old.repository_id;
END;
