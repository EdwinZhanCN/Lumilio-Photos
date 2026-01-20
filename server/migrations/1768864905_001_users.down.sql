-- Reverse migration - drops all objects created in up migration
-- WARNING: This will permanently delete data

DROP INDEX IF EXISTS idx_refresh_tokens_tokens_token;
DROP INDEX IF EXISTS idx_refresh_tokens_user_id;
DROP TABLE IF EXISTS refresh_tokens CASCADE;
DROP TABLE IF EXISTS users CASCADE;
