-- Restore per-row tsvector search infrastructure.

ALTER TABLE ocr_text_items
ADD COLUMN IF NOT EXISTS search_vector tsvector
GENERATED ALWAYS AS (to_tsvector('simple', COALESCE(text_content, ''))) STORED;

CREATE INDEX IF NOT EXISTS ocr_text_items_search_vector_idx
ON ocr_text_items
USING GIN (search_vector);

CREATE INDEX IF NOT EXISTS ocr_text_items_fulltext_idx
ON ocr_text_items
USING GIN (to_tsvector('simple', text_content));

-- Drop the BM25 index and full_text column.
DROP INDEX IF EXISTS ocr_results_bm25_idx;
ALTER TABLE ocr_results DROP COLUMN IF EXISTS full_text;
