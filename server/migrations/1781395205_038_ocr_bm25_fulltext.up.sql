-- Lift OCR search document granularity from per-text-region to per-asset.
-- OCR engines split sentences across multiple ocr_text_items rows; searching
-- per-row misses cross-fragment queries. Concatenating all text into
-- ocr_results.full_text gives BM25 a complete document per asset.

-- Ensure the BM25 access method and CJK text search configuration exist here
-- too. Some development databases may already have marked the earlier extension
-- migration as applied before chinese_zh was added to it.
CREATE EXTENSION IF NOT EXISTS pg_textsearch;
CREATE EXTENSION IF NOT EXISTS zhparser;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_ts_config WHERE cfgname = 'chinese_zh'
    ) THEN
        CREATE TEXT SEARCH CONFIGURATION chinese_zh (PARSER = zhparser);
    END IF;

    ALTER TEXT SEARCH CONFIGURATION chinese_zh
        DROP MAPPING IF EXISTS FOR n,v,a,i,e,l,j;
    ALTER TEXT SEARCH CONFIGURATION chinese_zh
        ADD MAPPING FOR n,v,a,i,e,l,j WITH simple;
END
$$;

-- 1. Add the asset-level concatenated text column.
ALTER TABLE ocr_results
ADD COLUMN IF NOT EXISTS full_text TEXT NOT NULL DEFAULT '';

-- 2. Backfill from existing ocr_text_items (space-delimited, ordered by id).
UPDATE ocr_results r
SET full_text = COALESCE((
    SELECT string_agg(t.text_content, ' ' ORDER BY t.id)
    FROM ocr_text_items t
    WHERE t.asset_id = r.asset_id
), '');

-- 3. BM25 index on the asset-level document using zhparser for CJK segmentation.
CREATE INDEX IF NOT EXISTS ocr_results_bm25_idx
ON ocr_results
USING bm25(full_text) WITH (text_config = 'public.chinese_zh');

-- 4. Drop superseded per-row tsvector infrastructure.
DROP INDEX IF EXISTS ocr_text_items_search_vector_idx;
DROP INDEX IF EXISTS ocr_text_items_fulltext_idx;
ALTER TABLE ocr_text_items DROP COLUMN IF EXISTS search_vector;
