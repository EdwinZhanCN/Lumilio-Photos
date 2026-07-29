-- Stable, owner-scoped media Events and their deterministic rebuild state.

CREATE UNIQUE INDEX idx_media_items_id_owner
    ON media_items (media_item_id, owner_id);

CREATE TABLE events (
    event_id TEXT PRIMARY KEY
        CHECK (event_id = lower(event_id) AND length(event_id) = 36),
    owner_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'redirected')),
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

CREATE INDEX idx_events_owner_list
    ON events (owner_id, start_at DESC, event_id DESC);

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

CREATE UNIQUE INDEX idx_event_media_items_current
    ON event_media_items (media_item_id);
CREATE INDEX idx_event_media_items_order
    ON event_media_items (event_id, position, media_item_id);

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

CREATE UNIQUE INDEX idx_event_constraints_event_item
    ON event_constraints (owner_id, kind, event_id, left_media_item_id)
    WHERE kind IN ('include', 'exclude');
CREATE UNIQUE INDEX idx_event_constraints_pair
    ON event_constraints (owner_id, kind, left_media_item_id, right_media_item_id)
    WHERE kind IN ('must_link', 'cannot_link');
CREATE UNIQUE INDEX idx_event_constraints_one_include
    ON event_constraints (left_media_item_id)
    WHERE kind = 'include';

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

CREATE INDEX idx_event_redirects_target
    ON event_redirects (owner_id, new_event_id);

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

CREATE INDEX idx_event_dirty_ranges_owner_claim
    ON event_dirty_ranges (owner_id, claimed_at, range_start, range_end);

CREATE TABLE event_owner_state (
    owner_id INTEGER PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    active_algorithm_version TEXT NOT NULL,
    initialized_at INTEGER NOT NULL,
    last_full_rebuild_at INTEGER,
    automatic_rebuild_paused INTEGER NOT NULL DEFAULT 0
        CHECK (automatic_rebuild_paused IN (0, 1)),
    revision INTEGER NOT NULL DEFAULT 0 CHECK (revision >= 0),
    updated_at INTEGER NOT NULL
) STRICT;

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
WHEN NEW.status = 'redirected'
BEGIN
    SELECT CASE WHEN EXISTS (
        SELECT 1 FROM event_media_items WHERE event_id = NEW.event_id
    ) THEN RAISE(ABORT, 'redirected event must have no members') END;
    SELECT CASE WHEN NEW.generated_cover_media_item_id IS NOT NULL
                  OR NEW.cover_override_media_item_id IS NOT NULL
        THEN RAISE(ABORT, 'redirected event must have no covers') END;
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
