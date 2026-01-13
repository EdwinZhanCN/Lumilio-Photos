-- Reverse migration - drops all objects created in up migration
-- WARNING: This will permanently delete data

DROP EXTENSION IF EXISTS vector CASCADE;
DROP EXTENSION IF EXISTS pgcrypto CASCADE;
