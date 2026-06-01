-- Aggregate PostgreSQL search support.
--
-- Embedding search uses the per-space pgvector HNSW indexes created by
-- embedding_service.ensureSearchIndexForSpace.
-- OCR/place text search use stored tsvector columns so retrievers can hit GIN indexes.

ALTER TABLE ocr_text_items
ADD COLUMN IF NOT EXISTS search_vector tsvector
GENERATED ALWAYS AS (to_tsvector('simple', COALESCE(text_content, ''))) STORED;

CREATE INDEX IF NOT EXISTS ocr_text_items_search_vector_idx
ON ocr_text_items
USING GIN (search_vector);

ALTER TABLE location_clusters
ADD COLUMN IF NOT EXISTS search_vector tsvector
GENERATED ALWAYS AS (
    to_tsvector(
        'simple',
        COALESCE(label, '') || ' ' ||
        COALESCE(country, '') || ' ' ||
        COALESCE(region, '') || ' ' ||
        COALESCE(city, '') || ' ' ||
        COALESCE(geohash, '')
    )
) STORED;

CREATE INDEX IF NOT EXISTS location_clusters_search_vector_idx
ON location_clusters
USING GIN (search_vector);
