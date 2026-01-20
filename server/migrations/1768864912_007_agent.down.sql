-- Reverse migration - drops all objects created in up migration
-- WARNING: This will permanently delete data

DROP TABLE IF EXISTS agent_checkpoints CASCADE;
