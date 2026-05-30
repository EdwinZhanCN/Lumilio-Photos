DROP INDEX IF EXISTS idx_repositories_default_owner;
ALTER TABLE repositories DROP COLUMN IF EXISTS default_owner_id;
