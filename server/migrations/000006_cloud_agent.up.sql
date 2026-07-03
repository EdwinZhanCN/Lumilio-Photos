-- Squashed baseline migration. Destructive reset: historical migrations were intentionally removed.
--
-- Name: agent_checkpoints; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agent_checkpoints (
    id text NOT NULL,
    data bytea NOT NULL,
    updated_at timestamp with time zone DEFAULT now(),
    CONSTRAINT agent_checkpoints_pkey PRIMARY KEY (id)
);


--
-- Name: agent_pins; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agent_pins (
    pin_id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id integer NOT NULL,
    title text DEFAULT ''::text NOT NULL,
    widget text DEFAULT 'cover_card'::text NOT NULL,
    mode text DEFAULT 'frozen'::text NOT NULL,
    plan jsonb DEFAULT '{}'::jsonb NOT NULL,
    summary text DEFAULT ''::text NOT NULL,
    asset_ids uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    truncated boolean DEFAULT false NOT NULL,
    layout_x integer DEFAULT 0 NOT NULL,
    layout_y integer DEFAULT 0 NOT NULL,
    layout_w integer DEFAULT 4 NOT NULL,
    layout_h integer DEFAULT 4 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT agent_pins_pkey PRIMARY KEY (pin_id),
    CONSTRAINT agent_pins_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE,
    CONSTRAINT agent_pins_mode_check CHECK ((mode = ANY (ARRAY['frozen'::text, 'live'::text])))
);


--
-- Name: cloud_credentials; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cloud_credentials (
    credential_id uuid DEFAULT gen_random_uuid() NOT NULL,
    provider text NOT NULL,
    display_name text NOT NULL,
    identity_hash text NOT NULL,
    masked_identity text NOT NULL,
    status text DEFAULT 'connected'::text NOT NULL,
    artifact_dir text,
    created_by_user_id integer,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    public_config jsonb DEFAULT '{}'::jsonb NOT NULL,
    secret_ciphertext bytea,
    CONSTRAINT cloud_credentials_pkey PRIMARY KEY (credential_id),
    CONSTRAINT cloud_credentials_provider_identity_hash_key UNIQUE (provider, identity_hash),
    CONSTRAINT cloud_credentials_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(user_id)
);


--
-- Name: cloud_import_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cloud_import_runs (
    run_id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    credential_id uuid NOT NULL,
    provider text NOT NULL,
    status text DEFAULT 'queued'::text NOT NULL,
    total_seen bigint DEFAULT 0 NOT NULL,
    downloaded_count bigint DEFAULT 0 NOT NULL,
    imported_count bigint DEFAULT 0 NOT NULL,
    skipped_count bigint DEFAULT 0 NOT NULL,
    failed_count bigint DEFAULT 0 NOT NULL,
    error text,
    started_at timestamp with time zone,
    finished_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT cloud_import_runs_pkey PRIMARY KEY (run_id),
    CONSTRAINT cloud_import_runs_credential_id_fkey FOREIGN KEY (credential_id) REFERENCES public.cloud_credentials(credential_id) ON DELETE RESTRICT,
    CONSTRAINT cloud_import_runs_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES public.repositories(repo_id) ON DELETE CASCADE
);


--
-- Name: cloud_sync_cursors; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cloud_sync_cursors (
    repository_id uuid NOT NULL,
    provider text NOT NULL,
    cursor_value text DEFAULT ''::text NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    credential_id uuid NOT NULL,
    CONSTRAINT cloud_sync_cursors_pkey PRIMARY KEY (repository_id, credential_id, provider),
    CONSTRAINT cloud_sync_cursors_credential_id_fkey FOREIGN KEY (credential_id) REFERENCES public.cloud_credentials(credential_id) ON DELETE CASCADE,
    CONSTRAINT cloud_sync_cursors_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES public.repositories(repo_id) ON DELETE CASCADE
);


--
-- Name: cloud_sync_files; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cloud_sync_files (
    repository_id uuid NOT NULL,
    provider text NOT NULL,
    remote_key text NOT NULL,
    etag text DEFAULT ''::text NOT NULL,
    local_hash text DEFAULT ''::text NOT NULL,
    asset_id uuid,
    synced_at timestamp with time zone DEFAULT now() NOT NULL,
    credential_id uuid NOT NULL,
    CONSTRAINT cloud_sync_files_pkey PRIMARY KEY (repository_id, credential_id, provider, remote_key),
    CONSTRAINT cloud_sync_files_credential_id_fkey FOREIGN KEY (credential_id) REFERENCES public.cloud_credentials(credential_id) ON DELETE CASCADE,
    CONSTRAINT cloud_sync_files_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES public.repositories(repo_id) ON DELETE CASCADE
);


--
-- Name: repository_cloud_bindings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.repository_cloud_bindings (
    repository_id uuid NOT NULL,
    credential_id uuid NOT NULL,
    provider text NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    last_import_run_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT repository_cloud_bindings_pkey PRIMARY KEY (repository_id, provider),
    CONSTRAINT repository_cloud_bindings_credential_id_fkey FOREIGN KEY (credential_id) REFERENCES public.cloud_credentials(credential_id) ON DELETE RESTRICT,
    CONSTRAINT repository_cloud_bindings_last_import_run_id_fkey FOREIGN KEY (last_import_run_id) REFERENCES public.cloud_import_runs(run_id) ON DELETE SET NULL,
    CONSTRAINT repository_cloud_bindings_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES public.repositories(repo_id) ON DELETE CASCADE
);
--
-- Name: idx_agent_pins_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_pins_user ON public.agent_pins USING btree (user_id, created_at DESC);


--
-- Name: idx_cloud_credentials_provider_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_cloud_credentials_provider_status ON public.cloud_credentials USING btree (provider, status);


--
-- Name: idx_cloud_import_runs_credential_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_cloud_import_runs_credential_created ON public.cloud_import_runs USING btree (credential_id, created_at DESC);


--
-- Name: idx_cloud_import_runs_repository_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_cloud_import_runs_repository_created ON public.cloud_import_runs USING btree (repository_id, created_at DESC);


--
-- Name: idx_cloud_import_runs_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_cloud_import_runs_status ON public.cloud_import_runs USING btree (status);


--
-- Name: idx_repository_cloud_bindings_credential; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_repository_cloud_bindings_credential ON public.repository_cloud_bindings USING btree (credential_id);


--
-- Name: share_links; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.share_links (
    share_id uuid DEFAULT gen_random_uuid() NOT NULL,
    owner_id integer NOT NULL,
    token_hash bytea NOT NULL,
    title text NOT NULL,
    description text,
    source_kind text NOT NULL,
    source_ref text,
    asset_ids uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    asset_count integer DEFAULT 0 NOT NULL,
    allow_download boolean DEFAULT false NOT NULL,
    include_originals boolean DEFAULT false NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    revoked_at timestamp with time zone,
    last_viewed_at timestamp with time zone,
    view_count bigint DEFAULT 0 NOT NULL,
    CONSTRAINT share_links_pkey PRIMARY KEY (share_id),
    CONSTRAINT share_links_token_hash_key UNIQUE (token_hash),
    CONSTRAINT share_links_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES public.users(user_id) ON DELETE CASCADE,
    CONSTRAINT share_links_status_check CHECK ((status = ANY (ARRAY['active'::text, 'revoked'::text]))),
    CONSTRAINT share_links_source_kind_check CHECK ((source_kind = ANY (ARRAY['asset_snapshot'::text, 'album'::text, 'person'::text, 'utility_query'::text, 'pin'::text])))
);


--
-- Name: idx_share_links_owner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_share_links_owner ON public.share_links USING btree (owner_id, created_at DESC);


--
-- Name: idx_share_links_status_expires; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_share_links_status_expires ON public.share_links USING btree (status, expires_at);


--
-- Name: agent_pins agent_pins_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--
