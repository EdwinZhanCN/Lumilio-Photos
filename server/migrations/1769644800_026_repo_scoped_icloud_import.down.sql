-- Dropping credential_id from the primary key can collide: multiple
-- credentials may have synced the same (repository_id, provider, remote_key).
-- Mirror the up migration's "start fresh" stance and clear the per-credential
-- sync state before restoring the narrower primary keys.
DELETE FROM cloud_sync_files;
DELETE FROM cloud_sync_cursors;

ALTER TABLE cloud_sync_files
    DROP CONSTRAINT cloud_sync_files_pkey;
ALTER TABLE cloud_sync_files
    DROP COLUMN IF EXISTS credential_id;
ALTER TABLE cloud_sync_files
    ADD PRIMARY KEY (repository_id, provider, remote_key);

ALTER TABLE cloud_sync_cursors
    DROP CONSTRAINT cloud_sync_cursors_pkey;
ALTER TABLE cloud_sync_cursors
    DROP COLUMN IF EXISTS credential_id;
ALTER TABLE cloud_sync_cursors
    ADD PRIMARY KEY (repository_id, provider);

DROP TABLE IF EXISTS repository_cloud_bindings;
DROP TABLE IF EXISTS cloud_import_runs;
DROP TABLE IF EXISTS cloud_credentials;
