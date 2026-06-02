-- 027_cloud_provider_credentials: provider-neutral cloud credentials.
-- This is intentionally destructive for the experimental iCloud credential model.

DELETE FROM cloud_sync_files;
DELETE FROM cloud_sync_cursors;
DELETE FROM repository_cloud_bindings;
DELETE FROM cloud_import_runs;
DELETE FROM cloud_credentials;

DROP INDEX IF EXISTS idx_cloud_credentials_provider_status;
ALTER TABLE cloud_credentials
    DROP CONSTRAINT IF EXISTS cloud_credentials_provider_account_identifier_hash_domain_key;

ALTER TABLE cloud_credentials
    RENAME COLUMN account_identifier_hash TO identity_hash;
ALTER TABLE cloud_credentials
    RENAME COLUMN masked_account TO masked_identity;
ALTER TABLE cloud_credentials
    RENAME COLUMN cookie_dir TO artifact_dir;

ALTER TABLE cloud_credentials
    DROP COLUMN IF EXISTS domain,
    ADD COLUMN public_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN secret_ciphertext BYTEA;

ALTER TABLE cloud_credentials
    ALTER COLUMN artifact_dir DROP NOT NULL;

ALTER TABLE cloud_credentials
    ADD CONSTRAINT cloud_credentials_provider_identity_hash_key UNIQUE (provider, identity_hash);

CREATE INDEX idx_cloud_credentials_provider_status
    ON cloud_credentials(provider, status);
