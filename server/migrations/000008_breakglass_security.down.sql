ALTER TABLE public.users
    DROP COLUMN IF EXISTS password_change_required,
    DROP COLUMN IF EXISTS auth_version;

