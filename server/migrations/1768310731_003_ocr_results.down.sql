-- Reverse migration - drops all objects created in up migration
-- WARNING: This will permanently delete data

DROP FUNCTION IF EXISTS update_ocr_updated_at();
DROP INDEX IF EXISTS ocr_text_items_bbox_idx;
DROP INDEX IF EXISTS ocr_text_items_fulltext_idx;
DROP INDEX IF EXISTS ocr_text_items_text_length_idx;
DROP INDEX IF EXISTS ocr_text_items_confidence_idx;
DROP INDEX IF EXISTS ocr_text_items_asset_id_idx;
DROP INDEX IF EXISTS ocr_results_created_at_idx;
DROP INDEX IF EXISTS ocr_results_model_id_idx;
DROP INDEX IF EXISTS ocr_results_asset_id_idx;
DROP TABLE IF EXISTS ocr_text_items CASCADE;
DROP TABLE IF EXISTS ocr_results CASCADE;
DROP EXTENSION IF EXISTS postgis CASCADE;
