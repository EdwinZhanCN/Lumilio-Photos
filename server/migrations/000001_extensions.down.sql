-- Extensions teardown for Lumilio Photos schema (golang-migrate: down)
-- This down migration reverses 000001_extensions.up.sql by removing the extensions.
-- It assumes all dependent objects (e.g., tables using VECTOR, gen_random_uuid defaults)
-- have been dropped by earlier down migrations. Uses RESTRICT semantics (default)
-- so it will fail if dependencies still exist, preventing accidental cascade drops.

DROP EXTENSION IF EXISTS vector;
DROP EXTENSION IF EXISTS pgcrypto;
