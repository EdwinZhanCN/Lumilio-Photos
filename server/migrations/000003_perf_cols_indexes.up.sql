-- Performance migration:
-- 1. Add generated (stored) columns for frequently used sort/filter keys so queries
--    no longer need to compute JSONB expressions per row.
-- 2. Add supporting partial indexes to accelerate ORDER BY / WHERE patterns used
--    in asset listing APIs.
--
-- Assumptions:
-- - Existing data has either valid ISO8601 timestamps in specific_metadata->>'date_taken'
--   or missing/empty => we fall back to upload_time.
-- - specific_metadata->>'rating' is either NULL / '' / numeric string.
--
-- NOTE: If any row contains an invalid timestamp string in date_taken, this migration
--       will fail. Clean / normalize first if uncertain.

-- 1) Add generated columns (STORED so they are materialized and indexable)

ALTER TABLE assets
  ADD COLUMN taken_time timestamptz
    GENERATED ALWAYS AS (
      COALESCE(
        NULLIF(specific_metadata->>'date_taken','')::timestamptz,
        upload_time
      )
    ) STORED,
  ADD COLUMN rating_int integer
    GENERATED ALWAYS AS (
      NULLIF(specific_metadata->>'rating','')::int
    ) STORED;

-- (Optional future columns – uncomment if you later want them)
-- ALTER TABLE assets
--   ADD COLUMN liked_bool boolean
--     GENERATED ALWAYS AS (
--       CASE WHEN specific_metadata ? 'liked'
--            THEN (specific_metadata->>'liked')::boolean
--            ELSE NULL
--       END
--     ) STORED,
--   ADD COLUMN is_raw_bool boolean
--     GENERATED ALWAYS AS (
--       CASE WHEN specific_metadata ? 'is_raw'
--            THEN (specific_metadata->>'is_raw')::boolean
--            ELSE NULL
--       END
--     ) STORED;

-- 2) Indexes to support the common listing queries
--    (type / owner_id filtered, then sorted by taken_time or rating)
--    We add DESC on sort key to match default descending ordering in queries.
--    Partial indexes exclude deleted rows.

CREATE INDEX idx_assets_type_taken_time
  ON assets (type, taken_time DESC)
  WHERE is_deleted = false;

CREATE INDEX idx_assets_owner_taken_time
  ON assets (owner_id, taken_time DESC)
  WHERE is_deleted = false;

CREATE INDEX idx_assets_type_rating
  ON assets (type, rating_int DESC)
  WHERE is_deleted = false;

-- End of migration
