DROP TABLE IF EXISTS public.agent_pending_effects CASCADE;
DROP TABLE IF EXISTS public.agent_refs CASCADE;
ALTER TABLE IF EXISTS public.agent_threads
    DROP CONSTRAINT IF EXISTS agent_threads_active_run_fkey;
DROP TABLE IF EXISTS public.agent_runs CASCADE;
DROP TABLE IF EXISTS public.agent_threads CASCADE;
ALTER TABLE IF EXISTS public.agent_pins
    DROP COLUMN IF EXISTS last_successful_refresh_at;
