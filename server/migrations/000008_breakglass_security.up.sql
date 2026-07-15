ALTER TABLE public.users
    ADD COLUMN auth_version bigint NOT NULL DEFAULT 0,
    ADD COLUMN password_change_required boolean NOT NULL DEFAULT false;

