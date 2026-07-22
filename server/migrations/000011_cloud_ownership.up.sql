-- Cloud credentials are user-owned secrets. Repository bindings retain that
-- owner as the stable destination owner for imported assets, while each run
-- snapshots it for deterministic retries, audit, and history.
ALTER TABLE cloud_credentials
    RENAME COLUMN created_by_user_id TO owner_id;

ALTER TABLE cloud_credentials
    RENAME CONSTRAINT cloud_credentials_created_by_user_id_fkey TO cloud_credentials_owner_id_fkey;

DO $$
DECLARE
    host_owner_id integer;
BEGIN
    SELECT user_id
    INTO host_owner_id
    FROM users
    ORDER BY created_at ASC, user_id ASC
    LIMIT 1;

    IF EXISTS (SELECT 1 FROM cloud_credentials WHERE owner_id IS NULL) THEN
        IF host_owner_id IS NULL THEN
            RAISE EXCEPTION 'cannot assign legacy cloud credentials without an initial administrator';
        END IF;
        UPDATE cloud_credentials
        SET owner_id = host_owner_id
        WHERE owner_id IS NULL;
    END IF;
END $$;

ALTER TABLE cloud_credentials
    ALTER COLUMN owner_id SET NOT NULL;

CREATE INDEX idx_cloud_credentials_owner_created
    ON cloud_credentials (owner_id, created_at DESC);

ALTER TABLE repository_cloud_bindings
    ADD COLUMN owner_id integer;

UPDATE repository_cloud_bindings binding
SET owner_id = credential.owner_id
FROM cloud_credentials credential
WHERE credential.credential_id = binding.credential_id;

ALTER TABLE repository_cloud_bindings
    ALTER COLUMN owner_id SET NOT NULL,
    ADD CONSTRAINT repository_cloud_bindings_owner_id_fkey
        FOREIGN KEY (owner_id) REFERENCES users(user_id);

CREATE INDEX idx_repository_cloud_bindings_owner
    ON repository_cloud_bindings (owner_id, created_at DESC);

ALTER TABLE cloud_import_runs
    ADD COLUMN owner_id integer;

UPDATE cloud_import_runs run
SET owner_id = credential.owner_id
FROM cloud_credentials credential
WHERE credential.credential_id = run.credential_id;

ALTER TABLE cloud_import_runs
    ALTER COLUMN owner_id SET NOT NULL,
    ADD CONSTRAINT cloud_import_runs_owner_id_fkey
        FOREIGN KEY (owner_id) REFERENCES users(user_id);

CREATE INDEX idx_cloud_import_runs_owner_created
    ON cloud_import_runs (owner_id, created_at DESC);
