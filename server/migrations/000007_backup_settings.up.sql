-- Runtime-mutable database-backup settings (exec-plans/active/db-backup-upgrade.md
-- Phase 1). Column defaults double as the seed values: rows created before this
-- migration pick them up without an UpsertSettings change.
ALTER TABLE public.settings
    ADD COLUMN backup_enabled boolean DEFAULT true NOT NULL,
    ADD COLUMN backup_interval_hours integer DEFAULT 24 NOT NULL,
    ADD COLUMN backup_keep_last integer DEFAULT 14 NOT NULL;
