DROP INDEX IF EXISTS public.repositories_root_id_idx;

ALTER TABLE public.repositories
    DROP CONSTRAINT IF EXISTS repositories_root_id_fkey,
    DROP COLUMN IF EXISTS root_id;

DROP INDEX IF EXISTS public.repository_roots_one_default_idx;
DROP TABLE IF EXISTS public.repository_roots;
