-- Reverse migration - drops all objects created in up migration
-- WARNING: This will permanently delete data

DROP INDEX IF EXISTS idx_albums_user_id;
DROP TABLE IF EXISTS album_assets CASCADE;
DROP TABLE IF EXISTS albums CASCADE;
DROP TABLE IF EXISTS asset_tags CASCADE;
DROP TABLE IF EXISTS tags CASCADE;
