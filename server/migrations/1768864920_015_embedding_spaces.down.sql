DROP INDEX IF EXISTS embeddings_space_primary_asset_idx;

ALTER TABLE embeddings
DROP CONSTRAINT IF EXISTS embeddings_space_id_fkey;

ALTER TABLE embeddings
ALTER COLUMN vector TYPE VECTOR(1024)
USING vector::VECTOR(1024);

ALTER TABLE embeddings
DROP COLUMN IF EXISTS space_id;

DROP INDEX IF EXISTS embedding_spaces_default_per_type_idx;
DROP INDEX IF EXISTS embedding_spaces_identity_idx;

DROP TABLE IF EXISTS embedding_spaces;

CREATE INDEX embeddings_vector_idx ON embeddings USING hnsw (vector vector_l2_ops)
WITH (m = 16, ef_construction = 200);
