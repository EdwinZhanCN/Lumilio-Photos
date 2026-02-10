-- Reverse migration - drops all objects created in up migration
-- WARNING: This will permanently delete data

DROP FUNCTION IF EXISTS update_caption_updated_at();
DROP INDEX IF EXISTS captions_summary_fulltext_idx;
DROP INDEX IF EXISTS captions_fulltext_idx;
DROP INDEX IF EXISTS captions_confidence_idx;
DROP INDEX IF EXISTS captions_created_at_idx;
DROP INDEX IF EXISTS captions_model_id_idx;
DROP INDEX IF EXISTS captions_asset_id_idx;
DROP TABLE IF EXISTS captions CASCADE;
