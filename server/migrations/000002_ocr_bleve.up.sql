DROP TRIGGER IF EXISTS ocr_search_fts_insert;
DROP TRIGGER IF EXISTS ocr_search_fts_delete;
DROP TRIGGER IF EXISTS ocr_search_fts_update;
DROP TABLE IF EXISTS ocr_search_fts;

ALTER TABLE ocr_results DROP COLUMN full_text;

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

CREATE INDEX ocr_index_outbox_updated_at_idx
    ON ocr_index_outbox (updated_at, asset_id);

INSERT INTO ocr_index_metadata (asset_id, revision, updated_at)
SELECT asset_id, 1, updated_at
FROM ocr_results;

INSERT INTO ocr_index_outbox (asset_id, revision, updated_at)
SELECT asset_id, 1, updated_at
FROM ocr_results;
