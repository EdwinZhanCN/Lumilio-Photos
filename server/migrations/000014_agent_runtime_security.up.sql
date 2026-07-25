-- User-scoped Agent runtime state. Legacy checkpoints remain unreadable because
-- every new checkpoint key is namespaced through agent_threads.

ALTER TABLE public.agent_pins
    ADD COLUMN last_successful_refresh_at timestamptz;

CREATE TABLE public.agent_threads (
    user_id integer NOT NULL REFERENCES public.users(user_id) ON DELETE CASCADE,
    thread_id text NOT NULL,
    checkpoint_key text NOT NULL UNIQUE,
    mode text NOT NULL,
    context_bindings jsonb NOT NULL DEFAULT '[]'::jsonb,
    mention_bindings jsonb NOT NULL DEFAULT '[]'::jsonb,
    policy_version integer NOT NULL,
    status text NOT NULL DEFAULT 'active',
    active_run_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT agent_threads_pkey PRIMARY KEY (user_id, thread_id),
    CONSTRAINT agent_threads_mode_check CHECK (mode = ANY (ARRAY['free', 'review', 'organize', 'analyze', 'curate'])),
    CONSTRAINT agent_threads_status_check CHECK (status = ANY (ARRAY[
        'active', 'awaiting_confirmation', 'completed', 'cancelled', 'failed'
    ]))
);

CREATE TABLE public.agent_runs (
    run_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id integer NOT NULL,
    thread_id text NOT NULL,
    status text NOT NULL DEFAULT 'running',
    cancel_requested_at timestamptz,
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT agent_runs_thread_fkey
        FOREIGN KEY (user_id, thread_id)
        REFERENCES public.agent_threads(user_id, thread_id)
        ON DELETE CASCADE,
    CONSTRAINT agent_runs_status_check CHECK (status = ANY (ARRAY[
        'running', 'cancel_requested', 'awaiting_confirmation',
        'cancelled', 'completed', 'failed'
    ]))
);

ALTER TABLE public.agent_threads
    ADD CONSTRAINT agent_threads_active_run_fkey
    FOREIGN KEY (active_run_id) REFERENCES public.agent_runs(run_id) ON DELETE SET NULL;

CREATE UNIQUE INDEX idx_agent_runs_one_active_thread
    ON public.agent_runs(user_id, thread_id)
    WHERE status = ANY (ARRAY['running', 'cancel_requested', 'awaiting_confirmation']);

CREATE INDEX idx_agent_runs_thread_created
    ON public.agent_runs(user_id, thread_id, created_at DESC);

CREATE TABLE public.agent_refs (
    user_id integer NOT NULL,
    thread_id text NOT NULL,
    ref_id text NOT NULL,
    sequence integer NOT NULL,
    plan jsonb NOT NULL,
    asset_ids uuid[] NOT NULL DEFAULT '{}'::uuid[],
    summary text NOT NULL DEFAULT '',
    truncated boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_accessed_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    CONSTRAINT agent_refs_pkey PRIMARY KEY (user_id, thread_id, ref_id),
    CONSTRAINT agent_refs_thread_fkey
        FOREIGN KEY (user_id, thread_id)
        REFERENCES public.agent_threads(user_id, thread_id)
        ON DELETE CASCADE
);

CREATE INDEX idx_agent_refs_expiry ON public.agent_refs(expires_at);

CREATE TABLE public.agent_pending_effects (
    effect_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id integer NOT NULL,
    thread_id text NOT NULL,
    initiating_run_id uuid NOT NULL REFERENCES public.agent_runs(run_id) ON DELETE CASCADE,
    executing_run_id uuid REFERENCES public.agent_runs(run_id) ON DELETE SET NULL,
    tool_name text NOT NULL,
    effect_class text NOT NULL,
    policy_version integer NOT NULL,
    membership_snapshot uuid[] NOT NULL DEFAULT '{}'::uuid[],
    payload jsonb NOT NULL,
    target jsonb NOT NULL DEFAULT '{}'::jsonb,
    idempotency_key text NOT NULL,
    status text NOT NULL DEFAULT 'pending',
    receipt jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    committed_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT agent_pending_effects_thread_fkey
        FOREIGN KEY (user_id, thread_id)
        REFERENCES public.agent_threads(user_id, thread_id)
        ON DELETE CASCADE,
    CONSTRAINT agent_pending_effects_idempotency_key_key UNIQUE (idempotency_key),
    CONSTRAINT agent_pending_effects_status_check CHECK (status = ANY (ARRAY[
        'pending', 'committed', 'rejected', 'cancelled', 'failed'
    ]))
);

CREATE INDEX idx_agent_pending_effects_thread
    ON public.agent_pending_effects(user_id, thread_id, created_at DESC);
