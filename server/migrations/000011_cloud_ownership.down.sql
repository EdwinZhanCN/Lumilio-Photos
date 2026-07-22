DROP INDEX IF EXISTS idx_cloud_import_runs_owner_created;
ALTER TABLE cloud_import_runs
    DROP CONSTRAINT IF EXISTS cloud_import_runs_owner_id_fkey,
    DROP COLUMN IF EXISTS owner_id;

DROP INDEX IF EXISTS idx_repository_cloud_bindings_owner;
ALTER TABLE repository_cloud_bindings
    DROP CONSTRAINT IF EXISTS repository_cloud_bindings_owner_id_fkey,
    DROP COLUMN IF EXISTS owner_id;

DROP INDEX IF EXISTS idx_cloud_credentials_owner_created;
ALTER TABLE cloud_credentials
    ALTER COLUMN owner_id DROP NOT NULL;

ALTER TABLE cloud_credentials
    RENAME CONSTRAINT cloud_credentials_owner_id_fkey TO cloud_credentials_created_by_user_id_fkey;

ALTER TABLE cloud_credentials
    RENAME COLUMN owner_id TO created_by_user_id;
