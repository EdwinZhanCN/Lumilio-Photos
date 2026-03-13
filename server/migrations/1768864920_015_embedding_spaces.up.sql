CREATE TABLE embedding_spaces (
    id BIGSERIAL PRIMARY KEY,
    embedding_type VARCHAR(50) NOT NULL,
    model_id VARCHAR(100) NOT NULL,
    dimensions INTEGER NOT NULL CHECK (dimensions > 0),
    distance_metric VARCHAR(20) NOT NULL CHECK (distance_metric IN ('l2')),
    search_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    is_default_search BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX embedding_spaces_identity_idx
ON embedding_spaces (embedding_type, model_id, dimensions, distance_metric);

CREATE UNIQUE INDEX embedding_spaces_default_per_type_idx
ON embedding_spaces (embedding_type)
WHERE is_default_search = TRUE;

INSERT INTO embedding_spaces (
    embedding_type,
    model_id,
    dimensions,
    distance_metric,
    search_enabled,
    is_default_search
)
SELECT
    src.embedding_type,
    src.embedding_model,
    src.embedding_dimensions,
    'l2',
    FALSE,
    FALSE
FROM (
    SELECT DISTINCT embedding_type, embedding_model, embedding_dimensions
    FROM embeddings
) AS src;

ALTER TABLE embeddings
ADD COLUMN space_id BIGINT;

UPDATE embeddings AS e
SET space_id = es.id
FROM embedding_spaces AS es
WHERE es.embedding_type = e.embedding_type
  AND es.model_id = e.embedding_model
  AND es.dimensions = e.embedding_dimensions
  AND es.distance_metric = 'l2';

WITH clip_space_counts AS (
    SELECT COUNT(*) AS count
    FROM embedding_spaces
    WHERE embedding_type = 'clip'
)
UPDATE embedding_spaces AS es
SET search_enabled = TRUE,
    is_default_search = TRUE,
    updated_at = NOW()
FROM clip_space_counts AS c
WHERE es.embedding_type = 'clip'
  AND c.count = 1;

ALTER TABLE embeddings
ALTER COLUMN vector TYPE VECTOR
USING vector::VECTOR;

ALTER TABLE embeddings
ALTER COLUMN space_id SET NOT NULL;

ALTER TABLE embeddings
ADD CONSTRAINT embeddings_space_id_fkey
FOREIGN KEY (space_id) REFERENCES embedding_spaces(id) ON DELETE RESTRICT;

DROP INDEX IF EXISTS embeddings_vector_idx;

CREATE INDEX embeddings_space_primary_asset_idx
ON embeddings (space_id, is_primary, asset_id);
