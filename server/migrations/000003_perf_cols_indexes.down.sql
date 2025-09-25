-- Down migration for 000003_perf_cols_indexes.up.sql
-- Reverts performance enhancements (generated columns + supporting indexes).

-- 1) Drop indexes first (they depend on the generated columns)
DROP INDEX IF EXISTS idx_assets_type_taken_time;
DROP INDEX IF EXISTS idx_assets_owner_taken_time;
DROP INDEX IF EXISTS idx_assets_type_rating;

-- 2) Drop generated columns
ALTER TABLE assets
  DROP COLUMN IF EXISTS taken_time,
  DROP COLUMN IF EXISTS rating_int;

-- (If you later added the optional columns liked_bool / is_raw_bool, also drop them here)
-- ALTER TABLE assets
--   DROP COLUMN IF EXISTS liked_bool,
--   DROP COLUMN IF EXISTS is_raw_bool;
