-- Reverse migration - drops all objects created in up migration
-- WARNING: This will permanently delete data

DROP FUNCTION IF EXISTS update_ai_description_updated_at();
DROP INDEX IF EXISTS ai_descriptions_summary_fulltext_idx;
DROP INDEX IF EXISTS ai_descriptions_fulltext_idx;
DROP INDEX IF EXISTS ai_descriptions_confidence_idx;
DROP INDEX IF EXISTS ai_descriptions_created_at_idx;
DROP INDEX IF EXISTS ai_descriptions_model_id_idx;
DROP INDEX IF EXISTS ai_descriptions_asset_id_idx;
DROP TABLE IF EXISTS ai_descriptions CASCADE;
