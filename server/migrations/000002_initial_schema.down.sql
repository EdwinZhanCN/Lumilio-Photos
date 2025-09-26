-- Down migration for 000002_initial_schema.up.sql
-- Drops all objects created by the initial schema in a safe, dependency-aware order.
-- Notes:
-- - Indexes are dropped explicitly before their owning tables.
-- - No CASCADE is used; dependencies are removed in reverse order of creation.
-- - IF EXISTS guards are used to make the migration idempotent.

-- 1) Junction tables and dependents first
DROP TABLE IF EXISTS album_assets;
DROP TABLE IF EXISTS asset_tags;

-- 2) Objects depending on assets
DROP INDEX IF EXISTS idx_species_predictions_asset_id;
DROP TABLE IF EXISTS species_predictions;

DROP INDEX IF EXISTS idx_thumbnails_asset_id;
DROP TABLE IF EXISTS thumbnails;

-- 3) Albums (depends on users and assets)
DROP INDEX IF EXISTS idx_albums_user_id;
DROP TABLE IF EXISTS albums;

-- 4) Tags (standalone after asset_tags is dropped)
DROP TABLE IF EXISTS tags;

-- 5) Assets and related indexes (depends on users)
DROP INDEX IF EXISTS assets_hnsw_idx;
DROP INDEX IF EXISTS idx_assets_type_taken_time_coalesce;
DROP INDEX IF EXISTS idx_assets_owner_id;
DROP INDEX IF EXISTS idx_assets_type;
DROP INDEX IF EXISTS idx_assets_hash;
DROP INDEX IF EXISTS idx_assets_taken_time;
DROP TABLE IF EXISTS assets;

-- 6) Refresh tokens and indexes (depends on users)
DROP INDEX IF EXISTS idx_refresh_tokens_user_id;
DROP INDEX IF EXISTS idx_refresh_tokens_tokens_token;
DROP TABLE IF EXISTS refresh_tokens;

-- 7) Users (base table)
DROP TABLE IF EXISTS users;
