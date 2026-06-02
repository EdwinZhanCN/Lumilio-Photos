DELETE FROM cloud_sync_files;
DELETE FROM cloud_sync_cursors;
DELETE FROM repository_cloud_bindings;
DELETE FROM cloud_import_runs;
DELETE FROM cloud_credentials;

DROP INDEX IF EXISTS idx_cloud_credentials_provider_status;
ALTER TABLE cloud_credentials
    DROP CONSTRAINT IF EXISTS cloud_credentials_provider_identity_hash_key;

ALTER TABLE cloud_credentials
    RENAME COLUMN identity_hash TO account_identifier_hash;
ALTER TABLE cloud_credentials
    RENAME COLUMN masked_identity TO masked_account;
ALTER TABLE cloud_credentials
    RENAME COLUMN artifact_dir TO cookie_dir;

ALTER TABLE cloud_credentials
    ADD COLUMN domain TEXT NOT NULL DEFAULT 'com',
    DROP COLUMN IF EXISTS public_config,
    DROP COLUMN IF EXISTS secret_ciphertext;

ALTER TABLE cloud_credentials
    ALTER COLUMN cookie_dir SET NOT NULL;

ALTER TABLE cloud_credentials
    ADD CONSTRAINT cloud_credentials_provider_account_identifier_hash_domain_key UNIQUE (provider, account_identifier_hash, domain);

CREATE INDEX idx_cloud_credentials_provider_status
    ON cloud_credentials(provider, status);
