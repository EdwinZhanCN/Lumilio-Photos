-- Reverse migration - drops all objects created in up migration
-- WARNING: This will permanently delete data

DROP INDEX IF EXISTS embeddings_vector_idx;
DROP INDEX IF EXISTS embeddings_primary_idx;
DROP INDEX IF EXISTS embeddings_type_model_idx;
DROP INDEX IF EXISTS embeddings_asset_type_idx;
DROP INDEX IF EXISTS idx_thumbnails_asset_id;
DROP INDEX IF EXISTS idx_assets_rating_liked;
DROP INDEX IF EXISTS idx_assets_liked;
DROP INDEX IF EXISTS idx_assets_rating;
DROP INDEX IF EXISTS idx_assets_repository_id;
DROP INDEX IF EXISTS idx_assets_taken_time;
DROP INDEX IF EXISTS idx_assets_hash;
DROP INDEX IF EXISTS idx_assets_type;
DROP INDEX IF EXISTS idx_assets_owner_id;
DROP TABLE IF EXISTS embeddings CASCADE;
DROP TABLE IF EXISTS thumbnails CASCADE;
DROP TABLE IF EXISTS assets CASCADE;
DROP TABLE IF EXISTS repositories CASCADE;
DROP EXTENSION IF EXISTS vector CASCADE;
