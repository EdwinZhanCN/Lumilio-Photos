-- Catalog-owned requested/applied truth for asynchronous work. River is a
-- disposable delivery controller and has no foreign key into these tables.
CREATE TABLE asset_pipeline_state (
    asset_id TEXT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    source_content_id TEXT NOT NULL REFERENCES content_objects(content_id),
    stage TEXT NOT NULL CHECK (stage IN ('analyze','derivatives','transcode','enrich')),
    pipeline_version TEXT NOT NULL,
    desired_version INTEGER NOT NULL CHECK (desired_version > 0),
    applied_version INTEGER NOT NULL DEFAULT 0
        CHECK (applied_version >= 0 AND applied_version <= desired_version),
    terminal_error TEXT,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (asset_id, stage)
) STRICT;

CREATE INDEX idx_asset_pipeline_pending
    ON asset_pipeline_state (stage, updated_at, asset_id)
    WHERE desired_version > applied_version;

CREATE TABLE asset_pipeline_receipt_stages (
    receipt_id TEXT NOT NULL REFERENCES catalog_operation_receipts(receipt_id) ON DELETE CASCADE,
    asset_id TEXT NOT NULL,
    stage TEXT NOT NULL,
    desired_version INTEGER NOT NULL CHECK (desired_version > 0),
    PRIMARY KEY (receipt_id, asset_id, stage),
    FOREIGN KEY (asset_id, stage) REFERENCES asset_pipeline_state(asset_id, stage) ON DELETE CASCADE
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

CREATE INDEX idx_catalog_operation_receipts_subject
    ON catalog_operation_receipts (kind, subject_id, created_at DESC);

CREATE TABLE catalog_backup_requests (
    receipt_id TEXT PRIMARY KEY REFERENCES catalog_operation_receipts(receipt_id) ON DELETE CASCADE,
    force INTEGER NOT NULL CHECK (force IN (0,1))
) STRICT;

CREATE TABLE asset_reindex_requests (
    receipt_id TEXT PRIMARY KEY REFERENCES catalog_operation_receipts(receipt_id) ON DELETE CASCADE,
    repository_id TEXT REFERENCES repositories(repo_id) ON DELETE CASCADE,
    tasks TEXT NOT NULL CHECK (json_valid(tasks)),
    page_limit INTEGER NOT NULL CHECK (page_limit BETWEEN 1 AND 500),
    cursor TEXT,
    missing_only INTEGER NOT NULL CHECK (missing_only IN (0,1)),
    reset_semantic INTEGER NOT NULL CHECK (reset_semantic IN (0,1)),
    requested_revision INTEGER NOT NULL DEFAULT 1 CHECK (requested_revision > 0),
    applied_revision INTEGER NOT NULL DEFAULT 0 CHECK (applied_revision BETWEEN 0 AND requested_revision),
    updated_at INTEGER NOT NULL
) STRICT;

-- Payload is a versioned domain envelope. Delivery columns are intentionally
-- limited to at-least-once bookkeeping; no River state or identifier is copied
-- back into the catalog.
CREATE TABLE domain_outbox (
    outbox_id TEXT PRIMARY KEY
        CHECK (outbox_id = lower(outbox_id) AND length(outbox_id) = 36),
    envelope_version INTEGER NOT NULL CHECK (envelope_version > 0),
    command_kind TEXT NOT NULL,
    subject_key TEXT NOT NULL,
    desired_version INTEGER NOT NULL CHECK (desired_version > 0),
    envelope TEXT NOT NULL CHECK (json_valid(envelope)),
    available_at INTEGER NOT NULL,
    delivered_at INTEGER,
    delivery_attempts INTEGER NOT NULL DEFAULT 0 CHECK (delivery_attempts >= 0),
    last_error TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE (command_kind, subject_key, desired_version)
) STRICT;

CREATE INDEX idx_domain_outbox_pending
    ON domain_outbox (available_at, created_at, outbox_id)
    WHERE delivered_at IS NULL;

-- Projection domains retain enforceable owners instead of sharing a
-- polymorphic generic desired/applied table.
CREATE TABLE event_projection_pipeline_state (
    owner_id INTEGER PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    source_revision INTEGER NOT NULL CHECK (source_revision > 0),
    projection_version INTEGER NOT NULL CHECK (projection_version > 0),
    applied_revision INTEGER NOT NULL DEFAULT 0
        CHECK (applied_revision >= 0 AND applied_revision <= source_revision),
    cursor TEXT,
    terminal_error TEXT,
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

CREATE TABLE location_resolution_pipeline_state (
    scope TEXT PRIMARY KEY CHECK (scope = 'all'),
    source_revision INTEGER NOT NULL CHECK (source_revision > 0),
    projection_version INTEGER NOT NULL CHECK (projection_version > 0),
    applied_revision INTEGER NOT NULL DEFAULT 0
        CHECK (applied_revision >= 0 AND applied_revision <= projection_version),
    terminal_error TEXT,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE location_projection_receipt_scopes (
    receipt_id TEXT NOT NULL REFERENCES catalog_operation_receipts(receipt_id) ON DELETE CASCADE,
    repository_id TEXT NOT NULL,
    owner_id INTEGER NOT NULL,
    desired_revision INTEGER NOT NULL CHECK (desired_revision > 0),
    PRIMARY KEY (receipt_id, repository_id, owner_id),
    FOREIGN KEY (repository_id, owner_id)
        REFERENCES location_projection_state(repository_id, owner_id) ON DELETE CASCADE
) STRICT;

-- Final macro failures are product facts. These two pre-existing ledgers own
-- their requested/applied identities, so the terminal marker belongs beside
-- them rather than in River or a generic task table. A newer request clears
-- the marker as part of its existing desired-state mutation.
ALTER TABLE repository_observation_state ADD COLUMN terminal_error TEXT;
ALTER TABLE location_projection_state ADD COLUMN terminal_error TEXT;

-- Source facts advance independently of macro delivery.  When they do, a
-- previously terminal location projection becomes a fresh desired revision.
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

PRAGMA user_version = 8;
