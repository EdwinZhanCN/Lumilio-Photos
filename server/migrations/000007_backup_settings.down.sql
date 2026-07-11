ALTER TABLE public.settings
    DROP COLUMN IF EXISTS backup_enabled,
    DROP COLUMN IF EXISTS backup_interval_hours,
    DROP COLUMN IF EXISTS backup_keep_last;
