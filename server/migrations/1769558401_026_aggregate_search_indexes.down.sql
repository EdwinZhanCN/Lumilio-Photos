DROP INDEX IF EXISTS location_clusters_search_vector_idx;
ALTER TABLE location_clusters DROP COLUMN IF EXISTS search_vector;
DROP INDEX IF EXISTS ocr_text_items_search_vector_idx;
ALTER TABLE ocr_text_items DROP COLUMN IF EXISTS search_vector;
