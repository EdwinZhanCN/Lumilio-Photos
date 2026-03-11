DROP INDEX IF EXISTS idx_users_role;

ALTER TABLE users
    DROP COLUMN IF EXISTS role,
    DROP COLUMN IF EXISTS avatar_url,
    DROP COLUMN IF EXISTS display_name;
