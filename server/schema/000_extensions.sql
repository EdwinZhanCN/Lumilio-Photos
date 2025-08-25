-- Extensions required by Lumilio Photos schema.
-- This file should run before any objects that depend on these extensions.
--
-- pgcrypto: provides gen_random_uuid() used as DEFAULT for UUID primary keys.
-- vector: provides the VECTOR data type and related index operators.

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS vector;
