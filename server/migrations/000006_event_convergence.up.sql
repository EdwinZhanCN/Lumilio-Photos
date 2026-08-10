-- Event convergence state.  The old dirty-range table is retained as a
-- recovery ledger, but revisions are the correctness authority.

PRAGMA foreign_keys = OFF;

DROP TRIGGER IF EXISTS event_membership_active_insert;
DROP TRIGGER IF EXISTS event_membership_active_update;
DROP TRIGGER IF EXISTS event_redirect_requires_empty;
DROP TRIGGER IF EXISTS event_cover_membership_insert;
DROP TRIGGER IF EXISTS event_cover_membership_update;
DROP TRIGGER IF EXISTS event_redirect_integrity_insert;
DROP TRIGGER IF EXISTS event_redirect_integrity_update;

CREATE TABLE events_converged (
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

INSERT INTO events_converged(
    event_id,owner_id,status,start_at,end_at,timezone,generated_title,
    title_override,generated_cover_media_item_id,cover_override_media_item_id,
    is_hidden,algorithm_version,created_at,updated_at
)
SELECT event_id,owner_id,status,start_at,end_at,timezone,generated_title,
       title_override,generated_cover_media_item_id,cover_override_media_item_id,
       is_hidden,algorithm_version,created_at,updated_at
FROM events;

DROP INDEX IF EXISTS idx_events_owner_list;
DROP TABLE events;
ALTER TABLE events_converged RENAME TO events;
CREATE INDEX idx_events_owner_list
    ON events (owner_id, start_at DESC, event_id DESC);

PRAGMA foreign_keys = ON;

ALTER TABLE event_owner_state ADD COLUMN source_revision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE event_owner_state ADD COLUMN published_revision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE event_owner_state ADD COLUMN rebuild_lease_token TEXT;
ALTER TABLE event_owner_state ADD COLUMN rebuild_lease_expires_at INTEGER;
UPDATE event_owner_state
SET source_revision=revision + CASE WHEN EXISTS (
        SELECT 1 FROM event_dirty_ranges
        WHERE event_dirty_ranges.owner_id=event_owner_state.owner_id
    ) THEN 1 ELSE 0 END,
    published_revision=revision;

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

CREATE INDEX idx_event_rebuild_runs_owner_time
    ON event_rebuild_runs(owner_id, requested_at DESC, run_id DESC);

CREATE INDEX idx_event_rebuild_runs_active
    ON event_rebuild_runs(owner_id, state)
    WHERE state IN ('queued','running');

-- Fresh installs and upgrades must use the same safety triggers.  The table
-- rebuild above removes triggers attached to the old events table.
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
