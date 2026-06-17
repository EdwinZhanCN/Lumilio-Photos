-- Squashed baseline migration. Destructive reset: historical migrations were intentionally removed.
--
-- Name: repositories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.repositories (
    repo_id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    path text NOT NULL,
    config jsonb,
    status text DEFAULT 'active'::text,
    last_sync timestamp with time zone,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    default_owner_id integer,
    role text DEFAULT 'regular'::text NOT NULL,
    CONSTRAINT repositories_pkey PRIMARY KEY (repo_id),
    CONSTRAINT repositories_path_key UNIQUE (path),
    CONSTRAINT repositories_default_owner_id_fkey FOREIGN KEY (default_owner_id) REFERENCES public.users(user_id),
    CONSTRAINT repositories_role_check CHECK ((role = ANY (ARRAY['primary'::text, 'regular'::text])))
);


--
-- Name: repository_defaults; Type: TABLE; Schema: public; Owner: -
-- Storage-owned, runtime-mutable defaults applied when creating new
-- repositories. Single row (id = 1). The default root is the immutable storage
-- root (config), so it is not stored here.
--

CREATE TABLE public.repository_defaults (
    id integer NOT NULL,
    strategy text DEFAULT 'date'::text NOT NULL,
    duplicate_handling text DEFAULT 'rename'::text NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT repository_defaults_pkey PRIMARY KEY (id),
    CONSTRAINT repository_defaults_id_check CHECK ((id = 1)),
    CONSTRAINT repository_defaults_strategy_check CHECK ((strategy = ANY (ARRAY['date'::text, 'flat'::text, 'cas'::text]))),
    CONSTRAINT repository_defaults_duplicate_handling_check CHECK ((duplicate_handling = ANY (ARRAY['rename'::text, 'uuid'::text, 'overwrite'::text])))
);

INSERT INTO public.repository_defaults (id) VALUES (1) ON CONFLICT DO NOTHING;


--
-- Name: assets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.assets (
    asset_id uuid DEFAULT gen_random_uuid() NOT NULL,
    owner_id integer,
    type character varying(20) NOT NULL,
    original_filename character varying(255) NOT NULL,
    storage_path character varying(512),
    mime_type character varying(50) NOT NULL,
    file_size bigint NOT NULL,
    hash character varying(64),
    width integer,
    height integer,
    duration double precision,
    upload_time timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    taken_time timestamp with time zone,
    capture_offset_minutes smallint,
    is_deleted boolean DEFAULT false,
    deleted_at timestamp with time zone,
    specific_metadata jsonb,
    rating integer,
    liked boolean DEFAULT false,
    repository_id uuid,
    status jsonb DEFAULT '{"state": "processing", "message": "Pending processing"}'::jsonb NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    gps_latitude double precision,
    gps_longitude double precision,
    gps_geohash_5 text,
    gps_geohash_7 text,
    exif_raw jsonb,
    CONSTRAINT assets_pkey PRIMARY KEY (asset_id),
    CONSTRAINT assets_repository_id_storage_path_key UNIQUE (repository_id, storage_path),
    CONSTRAINT assets_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES public.users(user_id),
    CONSTRAINT assets_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES public.repositories(repo_id),
    CONSTRAINT assets_capture_offset_minutes_check CHECK (((capture_offset_minutes IS NULL) OR ((capture_offset_minutes >= '-840'::integer) AND (capture_offset_minutes <= 840)))),
    CONSTRAINT assets_type_check CHECK (((type)::text = ANY ((ARRAY['PHOTO'::character varying, 'VIDEO'::character varying, 'AUDIO'::character varying])::text[]))),
    CONSTRAINT chk_assets_gps_latitude_range CHECK (((gps_latitude IS NULL) OR ((gps_latitude >= ('-90'::integer)::double precision) AND (gps_latitude <= (90)::double precision)))),
    CONSTRAINT chk_assets_gps_longitude_range CHECK (((gps_longitude IS NULL) OR ((gps_longitude >= ('-180'::integer)::double precision) AND (gps_longitude <= (180)::double precision))))
);


--
-- Name: repository_scan_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.repository_scan_runs (
    scan_id uuid NOT NULL,
    repository_id uuid NOT NULL,
    mode text NOT NULL,
    requested_by text,
    status text NOT NULL,
    started_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    finished_at timestamp with time zone,
    discovered_count bigint DEFAULT 0 NOT NULL,
    updated_count bigint DEFAULT 0 NOT NULL,
    deleted_count bigint DEFAULT 0 NOT NULL,
    skipped_count bigint DEFAULT 0 NOT NULL,
    error text,
    CONSTRAINT repository_scan_runs_pkey PRIMARY KEY (scan_id),
    CONSTRAINT repository_scan_runs_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES public.repositories(repo_id) ON DELETE CASCADE,
    CONSTRAINT repository_scan_runs_mode_check CHECK ((mode = ANY (ARRAY['periodic'::text, 'manual'::text]))),
    CONSTRAINT repository_scan_runs_status_check CHECK ((status = ANY (ARRAY['queued'::text, 'running'::text, 'completed'::text, 'failed'::text, 'cancelled'::text])))
);


--
-- Name: tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tags (
    tag_id integer NOT NULL,
    tag_name character varying(50) NOT NULL,
    category character varying(50),
    is_ai_generated boolean DEFAULT true,
    CONSTRAINT tags_pkey PRIMARY KEY (tag_id),
    CONSTRAINT tags_tag_name_key UNIQUE (tag_name)
);


--
-- Name: asset_tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.asset_tags (
    asset_id uuid NOT NULL,
    tag_id integer NOT NULL,
    confidence numeric(4,3) NOT NULL,
    source character varying(20) DEFAULT 'system'::character varying NOT NULL,
    CONSTRAINT asset_tags_pkey PRIMARY KEY (asset_id, tag_id),
    CONSTRAINT asset_tags_asset_id_fkey FOREIGN KEY (asset_id) REFERENCES public.assets(asset_id),
    CONSTRAINT asset_tags_tag_id_fkey FOREIGN KEY (tag_id) REFERENCES public.tags(tag_id),
    CONSTRAINT asset_tags_confidence_check CHECK (((confidence >= (0)::numeric) AND (confidence <= (1)::numeric))),
    CONSTRAINT asset_tags_source_check CHECK (((source)::text = ANY ((ARRAY['system'::character varying, 'user'::character varying, 'ai'::character varying, 'bioclip_classify'::character varying, 'zeroshot'::character varying])::text[])))
);


--
-- Name: tags_tag_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.tags_tag_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: tags_tag_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.tags_tag_id_seq OWNED BY public.tags.tag_id;


--
-- Name: thumbnails; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.thumbnails (
    thumbnail_id integer NOT NULL,
    asset_id uuid NOT NULL,
    size character varying(20) NOT NULL,
    storage_path character varying(512) NOT NULL,
    mime_type character varying(50) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT thumbnails_pkey PRIMARY KEY (thumbnail_id),
    CONSTRAINT thumbnails_asset_id_size_key UNIQUE (asset_id, size),
    CONSTRAINT thumbnails_asset_id_fkey FOREIGN KEY (asset_id) REFERENCES public.assets(asset_id),
    CONSTRAINT thumbnails_size_check CHECK (((size)::text = ANY ((ARRAY['small'::character varying, 'medium'::character varying, 'large'::character varying])::text[])))
);


--
-- Name: thumbnails_thumbnail_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.thumbnails_thumbnail_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: thumbnails_thumbnail_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.thumbnails_thumbnail_id_seq OWNED BY public.thumbnails.thumbnail_id;


--
-- Name: tags tag_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tags ALTER COLUMN tag_id SET DEFAULT nextval('public.tags_tag_id_seq'::regclass);


--
-- Name: thumbnails thumbnail_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thumbnails ALTER COLUMN thumbnail_id SET DEFAULT nextval('public.thumbnails_thumbnail_id_seq'::regclass);

--
-- Name: asset_tags asset_tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

--
-- Name: idx_asset_tags_tag_source_asset; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_asset_tags_tag_source_asset ON public.asset_tags USING btree (tag_id, source, asset_id);


--
-- Name: idx_assets_camera_model_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_camera_model_active ON public.assets USING btree (((specific_metadata ->> 'camera_model'::text))) WHERE ((is_deleted = false) AND (specific_metadata ? 'camera_model'::text));


--
-- Name: idx_assets_filename_trgm_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_filename_trgm_active ON public.assets USING gin (original_filename public.gin_trgm_ops) WHERE (is_deleted = false);


--
-- Name: idx_assets_gps_geohash_5; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_gps_geohash_5 ON public.assets USING btree (gps_geohash_5) WHERE ((gps_geohash_5 IS NOT NULL) AND (is_deleted = false));


--
-- Name: idx_assets_gps_geohash_7; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_gps_geohash_7 ON public.assets USING btree (gps_geohash_7) WHERE ((gps_geohash_7 IS NOT NULL) AND (is_deleted = false));


--
-- Name: idx_assets_gps_lat_lng; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_gps_lat_lng ON public.assets USING btree (gps_latitude, gps_longitude) WHERE ((gps_latitude IS NOT NULL) AND (gps_longitude IS NOT NULL) AND (is_deleted = false));


--
-- Name: idx_assets_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_hash ON public.assets USING btree (hash);


--
-- Name: idx_assets_is_raw_text_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_is_raw_text_active ON public.assets USING btree (((specific_metadata ->> 'is_raw'::text))) WHERE ((is_deleted = false) AND (specific_metadata ? 'is_raw'::text));


--
-- Name: idx_assets_lens_model_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_lens_model_active ON public.assets USING btree (((specific_metadata ->> 'lens_model'::text))) WHERE ((is_deleted = false) AND (specific_metadata ? 'lens_model'::text));


--
-- Name: idx_assets_liked; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_liked ON public.assets USING btree (liked) WHERE (liked = true);


--
-- Name: idx_assets_list_opt; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_list_opt ON public.assets USING btree (owner_id, type, COALESCE(taken_time, upload_time) DESC) WHERE (is_deleted = false);


--
-- Name: idx_assets_metadata; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_metadata ON public.assets USING gin (specific_metadata);


--
-- Name: idx_assets_mime_time_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_mime_time_active ON public.assets USING btree (mime_type, COALESCE(taken_time, upload_time) DESC, asset_id DESC) WHERE (is_deleted = false);


--
-- Name: idx_assets_owner_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_owner_id ON public.assets USING btree (owner_id);


--
-- Name: idx_assets_owner_time_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_owner_time_active ON public.assets USING btree (owner_id, COALESCE(taken_time, upload_time) DESC, asset_id DESC) WHERE (is_deleted = false);


--
-- Name: idx_assets_rating; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_rating ON public.assets USING btree (rating) WHERE (rating IS NOT NULL);


--
-- Name: idx_assets_rating_liked; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_rating_liked ON public.assets USING btree (rating, liked) WHERE ((rating IS NOT NULL) OR (liked = true));


--
-- Name: idx_assets_repo_time_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_repo_time_active ON public.assets USING btree (repository_id, COALESCE(taken_time, upload_time) DESC, asset_id DESC) WHERE (is_deleted = false);


--
-- Name: idx_assets_repo_type_time_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_repo_type_time_active ON public.assets USING btree (repository_id, type, COALESCE(taken_time, upload_time) DESC, asset_id DESC) WHERE (is_deleted = false);


--
-- Name: idx_assets_repository_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_repository_id ON public.assets USING btree (repository_id);


--
-- Name: idx_assets_status_state_time_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_status_state_time_active ON public.assets USING btree (((status ->> 'state'::text)), COALESCE(taken_time, upload_time) DESC, asset_id DESC) WHERE (is_deleted = false);


--
-- Name: idx_assets_taken_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_taken_time ON public.assets USING btree (taken_time);


--
-- Name: idx_assets_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_type ON public.assets USING btree (type);


--
-- Name: idx_assets_type_taken_time_coalesce; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assets_type_taken_time_coalesce ON public.assets USING btree (type, COALESCE(taken_time, upload_time) DESC) WHERE (is_deleted = false);


--
-- Name: idx_repositories_default_owner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_repositories_default_owner ON public.repositories USING btree (default_owner_id);


--
-- Name: idx_repositories_path; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_repositories_path ON public.repositories USING btree (path);


--
-- Name: idx_repositories_role; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_repositories_role ON public.repositories USING btree (role);


--
-- Name: idx_repositories_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_repositories_status ON public.repositories USING btree (status);


--
-- Name: idx_repository_scan_runs_repo_started; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_repository_scan_runs_repo_started ON public.repository_scan_runs USING btree (repository_id, started_at DESC);


--
-- Name: idx_repository_scan_runs_running; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_repository_scan_runs_running ON public.repository_scan_runs USING btree (repository_id) WHERE (status = 'running'::text);


--
-- Name: idx_thumbnails_asset_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_thumbnails_asset_id ON public.thumbnails USING btree (asset_id);


--
-- Name: repositories_one_primary_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX repositories_one_primary_idx ON public.repositories USING btree (role) WHERE (role = 'primary'::text);


--
-- Name: repository_scan_runs_one_running; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX repository_scan_runs_one_running ON public.repository_scan_runs USING btree (repository_id) WHERE (status = 'running'::text);


--
-- Name: assets trg_assets_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_assets_updated_at BEFORE UPDATE ON public.assets FOR EACH ROW EXECUTE FUNCTION public.set_assets_updated_at();


--
-- Name: asset_tags asset_tags_asset_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

-- Name: assets assets_owner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

--
-- Name: users users_avatar_asset_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_avatar_asset_id_fkey FOREIGN KEY (avatar_asset_id) REFERENCES public.assets(asset_id) ON DELETE SET NULL;
