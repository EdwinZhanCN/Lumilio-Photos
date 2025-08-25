-- Extensions required by Lumilio Photos schema (golang-migrate: up)
-- - pgcrypto: provides gen_random_uuid() used as DEFAULT for UUID PKs.
-- - vector: provides the VECTOR data type and related index operators (e.g., HNSW).
-- Note: These statements are idempotent due to IF NOT EXISTS.

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS vector;
