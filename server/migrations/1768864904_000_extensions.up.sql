-- Extensions required by Lumilio Photos schema.
-- This file should run before any objects that depend on these extensions.
--
-- PostgreSQL 16 provides gen_random_uuid() in core; pgcrypto is not needed.
-- vector: provides the VECTOR data type and related index operators.
-- pg_trgm: provides trigram indexes for fast ILIKE filename search.

CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
