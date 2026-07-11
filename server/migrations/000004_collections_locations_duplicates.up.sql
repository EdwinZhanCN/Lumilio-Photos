-- Squashed baseline migration. Destructive reset: historical migrations were intentionally removed.
--
-- Name: albums; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.albums (
    album_id integer NOT NULL,
    user_id integer NOT NULL,
    album_name character varying(100) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    description text,
    cover_asset_id uuid,
    album_type public.album_type DEFAULT 'default'::public.album_type NOT NULL,
    CONSTRAINT albums_pkey PRIMARY KEY (album_id),
    CONSTRAINT albums_cover_asset_id_fkey FOREIGN KEY (cover_asset_id) REFERENCES public.assets(asset_id),
    CONSTRAINT albums_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id)
);


--
-- Name: albums_album_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.albums_album_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: albums_album_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.albums_album_id_seq OWNED BY public.albums.album_id;


--
-- Name: album_assets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.album_assets (
    album_id integer NOT NULL,
    asset_id uuid NOT NULL,
    "position" integer DEFAULT 0,
    added_time timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT album_assets_pkey PRIMARY KEY (album_id, asset_id),
    CONSTRAINT album_assets_album_id_fkey FOREIGN KEY (album_id) REFERENCES public.albums(album_id) ON DELETE CASCADE,
    CONSTRAINT album_assets_asset_id_fkey FOREIGN KEY (asset_id) REFERENCES public.assets(asset_id)
);


--
-- Name: media_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.media_items (
    media_item_id uuid DEFAULT gen_random_uuid() NOT NULL,
    owner_id integer,
    repository_id uuid,
    media_kind text DEFAULT 'photo'::text NOT NULL,
    primary_asset_id uuid,
    group_key text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT media_items_pkey PRIMARY KEY (media_item_id),
    CONSTRAINT media_items_media_kind_check CHECK (media_kind = ANY (ARRAY['photo'::text, 'video'::text, 'audio'::text, 'live_photo'::text])),
    CONSTRAINT media_items_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES public.users(user_id),
    CONSTRAINT media_items_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES public.repositories(repo_id),
    CONSTRAINT media_items_primary_asset_id_fkey FOREIGN KEY (primary_asset_id) REFERENCES public.assets(asset_id) ON DELETE SET NULL
);


--
-- Name: media_item_assets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.media_item_assets (
    asset_id uuid NOT NULL,
    media_item_id uuid NOT NULL,
    relation public.stack_relation DEFAULT 'alternative'::public.stack_relation NOT NULL,
    "position" integer DEFAULT 0,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT media_item_assets_pkey PRIMARY KEY (asset_id),
    CONSTRAINT media_item_assets_asset_id_fkey FOREIGN KEY (asset_id) REFERENCES public.assets(asset_id) ON DELETE CASCADE,
    CONSTRAINT media_item_assets_media_item_id_fkey FOREIGN KEY (media_item_id) REFERENCES public.media_items(media_item_id) ON DELETE CASCADE
);


--
-- Name: asset_stacks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.asset_stacks (
    stack_id uuid DEFAULT gen_random_uuid() NOT NULL,
    owner_id integer,
    repository_id uuid,
    stack_kind text DEFAULT 'manual'::text NOT NULL,
    cover_media_item_id uuid,
    group_key text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT asset_stacks_pkey PRIMARY KEY (stack_id),
    CONSTRAINT asset_stacks_stack_kind_check CHECK (stack_kind = ANY (ARRAY['manual'::text, 'burst'::text])),
    CONSTRAINT asset_stacks_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES public.users(user_id),
    CONSTRAINT asset_stacks_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES public.repositories(repo_id),
    CONSTRAINT asset_stacks_cover_media_item_id_fkey FOREIGN KEY (cover_media_item_id) REFERENCES public.media_items(media_item_id) ON DELETE SET NULL
);


--
-- Name: asset_stack_members; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.asset_stack_members (
    media_item_id uuid NOT NULL,
    stack_id uuid NOT NULL,
    "position" integer DEFAULT 0,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT asset_stack_members_pkey PRIMARY KEY (media_item_id),
    CONSTRAINT asset_stack_members_media_item_id_fkey FOREIGN KEY (media_item_id) REFERENCES public.media_items(media_item_id) ON DELETE CASCADE,
    CONSTRAINT asset_stack_members_stack_id_fkey FOREIGN KEY (stack_id) REFERENCES public.asset_stacks(stack_id) ON DELETE CASCADE
);


-- Every physical asset starts as a one-component logical media item. Structural
-- detectors later merge RAW/JPEG and Live Photo components into one item;
-- presentation stacks only ever group media_item_id values.
CREATE FUNCTION public.create_media_item_for_asset() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    new_media_item_id uuid;
BEGIN
    INSERT INTO public.media_items (
        owner_id,
        repository_id,
        media_kind,
        primary_asset_id
    ) VALUES (
        NEW.owner_id,
        NEW.repository_id,
        CASE lower(NEW.type)
            WHEN 'video' THEN 'video'
            WHEN 'audio' THEN 'audio'
            ELSE 'photo'
        END,
        NEW.asset_id
    )
    RETURNING media_item_id INTO new_media_item_id;

    INSERT INTO public.media_item_assets (asset_id, media_item_id, relation, "position")
    VALUES (NEW.asset_id, new_media_item_id, 'alternative', 0);

    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_assets_create_media_item
AFTER INSERT ON public.assets
FOR EACH ROW EXECUTE FUNCTION public.create_media_item_for_asset();


--
-- Name: duplicate_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.duplicate_groups (
    group_id uuid DEFAULT gen_random_uuid() NOT NULL,
    repository_id uuid NOT NULL,
    owner_id integer,
    method text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    asset_count integer DEFAULT 0 NOT NULL,
    total_size bigint DEFAULT 0 NOT NULL,
    recommended_keeper_asset_id uuid,
    keeper_asset_id uuid,
    detection_version text DEFAULT 'duplicates-v1'::text NOT NULL,
    detected_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    resolved_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT duplicate_groups_pkey PRIMARY KEY (group_id),
    CONSTRAINT duplicate_groups_keeper_asset_id_fkey FOREIGN KEY (keeper_asset_id) REFERENCES public.assets(asset_id) ON DELETE SET NULL,
    CONSTRAINT duplicate_groups_recommended_keeper_asset_id_fkey FOREIGN KEY (recommended_keeper_asset_id) REFERENCES public.assets(asset_id) ON DELETE SET NULL,
    CONSTRAINT duplicate_groups_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES public.repositories(repo_id) ON DELETE CASCADE,
    CONSTRAINT duplicate_groups_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES public.users(user_id) ON DELETE CASCADE,
    CONSTRAINT duplicate_groups_method_check CHECK ((method = ANY (ARRAY['exact'::text, 'phash'::text, 'mixed'::text]))),
    CONSTRAINT duplicate_groups_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'merged'::text, 'dismissed'::text])))
);


--
-- Name: duplicate_group_assets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.duplicate_group_assets (
    group_id uuid NOT NULL,
    asset_id uuid NOT NULL,
    role text DEFAULT 'candidate'::text NOT NULL,
    file_size bigint DEFAULT 0 NOT NULL,
    CONSTRAINT duplicate_group_assets_pkey PRIMARY KEY (group_id, asset_id),
    CONSTRAINT duplicate_group_assets_asset_id_fkey FOREIGN KEY (asset_id) REFERENCES public.assets(asset_id) ON DELETE CASCADE,
    CONSTRAINT duplicate_group_assets_group_id_fkey FOREIGN KEY (group_id) REFERENCES public.duplicate_groups(group_id) ON DELETE CASCADE,
    CONSTRAINT duplicate_group_assets_role_check CHECK ((role = ANY (ARRAY['candidate'::text, 'keeper'::text, 'duplicate'::text])))
);


--
-- Name: duplicate_group_edges; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.duplicate_group_edges (
    group_id uuid NOT NULL,
    asset_id_a uuid NOT NULL,
    asset_id_b uuid NOT NULL,
    method text NOT NULL,
    distance double precision DEFAULT 0 NOT NULL,
    confidence double precision DEFAULT 1.0 NOT NULL,
    CONSTRAINT duplicate_group_edges_pkey PRIMARY KEY (group_id, asset_id_a, asset_id_b, method),
    CONSTRAINT duplicate_group_edges_asset_id_a_fkey FOREIGN KEY (asset_id_a) REFERENCES public.assets(asset_id) ON DELETE CASCADE,
    CONSTRAINT duplicate_group_edges_asset_id_b_fkey FOREIGN KEY (asset_id_b) REFERENCES public.assets(asset_id) ON DELETE CASCADE,
    CONSTRAINT duplicate_group_edges_group_id_fkey FOREIGN KEY (group_id) REFERENCES public.duplicate_groups(group_id) ON DELETE CASCADE,
    CONSTRAINT duplicate_group_edges_check CHECK ((asset_id_a < asset_id_b)),
    CONSTRAINT duplicate_group_edges_method_check CHECK ((method = ANY (ARRAY['exact'::text, 'phash'::text])))
);


--
-- Name: location_clusters; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.location_clusters (
    cluster_id uuid DEFAULT gen_random_uuid() NOT NULL,
    owner_id integer,
    repository_id uuid NOT NULL,
    geohash text NOT NULL,
    "precision" integer DEFAULT 7 NOT NULL,
    centroid_latitude double precision NOT NULL,
    centroid_longitude double precision NOT NULL,
    photo_count integer DEFAULT 0 NOT NULL,
    label text,
    country text,
    region text,
    city text,
    provider text,
    geocode_status text DEFAULT 'pending'::text NOT NULL,
    geocoded_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    search_vector tsvector GENERATED ALWAYS AS (to_tsvector('simple'::regconfig, ((((((((COALESCE(label, ''::text) || ' '::text) || COALESCE(country, ''::text)) || ' '::text) || COALESCE(region, ''::text)) || ' '::text) || COALESCE(city, ''::text)) || ' '::text) || COALESCE(geohash, ''::text)))) STORED,
    CONSTRAINT location_clusters_pkey PRIMARY KEY (cluster_id),
    CONSTRAINT location_clusters_owner_id_repository_id_geohash_key UNIQUE NULLS NOT DISTINCT (owner_id, repository_id, geohash),
    CONSTRAINT location_clusters_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES public.users(user_id) ON DELETE CASCADE,
    CONSTRAINT location_clusters_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES public.repositories(repo_id) ON DELETE CASCADE,
    CONSTRAINT location_clusters_centroid_latitude_check CHECK (((centroid_latitude >= ('-90'::integer)::double precision) AND (centroid_latitude <= (90)::double precision))),
    CONSTRAINT location_clusters_centroid_longitude_check CHECK (((centroid_longitude >= ('-180'::integer)::double precision) AND (centroid_longitude <= (180)::double precision))),
    CONSTRAINT location_clusters_geocode_status_check CHECK ((geocode_status = ANY (ARRAY['pending'::text, 'disabled'::text, 'cached'::text, 'resolved'::text, 'failed'::text]))),
    CONSTRAINT location_clusters_photo_count_check CHECK ((photo_count >= 0)),
    CONSTRAINT location_clusters_precision_check CHECK (("precision" > 0))
);


--
-- Name: location_cluster_assets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.location_cluster_assets (
    cluster_id uuid NOT NULL,
    asset_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT location_cluster_assets_pkey PRIMARY KEY (cluster_id, asset_id),
    CONSTRAINT location_cluster_assets_asset_id_fkey FOREIGN KEY (asset_id) REFERENCES public.assets(asset_id) ON DELETE CASCADE,
    CONSTRAINT location_cluster_assets_cluster_id_fkey FOREIGN KEY (cluster_id) REFERENCES public.location_clusters(cluster_id) ON DELETE CASCADE
);


--
-- Name: reverse_geocode_cache; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reverse_geocode_cache (
    cache_key text NOT NULL,
    provider text NOT NULL,
    language text DEFAULT ''::text NOT NULL,
    latitude double precision NOT NULL,
    longitude double precision NOT NULL,
    label text,
    country text,
    region text,
    city text,
    raw_response jsonb,
    queried_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    expires_at timestamp with time zone,
    CONSTRAINT reverse_geocode_cache_pkey PRIMARY KEY (cache_key),
    CONSTRAINT reverse_geocode_cache_latitude_check CHECK (((latitude >= ('-90'::integer)::double precision) AND (latitude <= (90)::double precision))),
    CONSTRAINT reverse_geocode_cache_longitude_check CHECK (((longitude >= ('-180'::integer)::double precision) AND (longitude <= (180)::double precision)))
);


--
-- Name: albums album_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.albums ALTER COLUMN album_id SET DEFAULT nextval('public.albums_album_id_seq'::regclass);

--
-- Name: idx_album_assets_album_order; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_album_assets_album_order ON public.album_assets USING btree (album_id, "position", added_time, asset_id);


--
-- Name: idx_album_assets_asset; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_album_assets_asset ON public.album_assets USING btree (asset_id);


--
-- Name: idx_albums_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_albums_type ON public.albums USING btree (album_type);


--
-- Name: idx_albums_user_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_albums_user_created_at ON public.albums USING btree (user_id, created_at DESC, album_id DESC);


--
-- Name: idx_albums_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_albums_user_id ON public.albums USING btree (user_id);


--
-- Name: idx_asset_stack_members_stack; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_asset_stack_members_stack ON public.asset_stack_members USING btree (stack_id);

CREATE UNIQUE INDEX idx_asset_stacks_burst_group_key ON public.asset_stacks USING btree (group_key)
WHERE stack_kind = 'burst' AND group_key IS NOT NULL;

CREATE INDEX idx_media_item_assets_item ON public.media_item_assets USING btree (media_item_id);

CREATE INDEX idx_media_items_owner_repository ON public.media_items USING btree (owner_id, repository_id);


--
-- Name: idx_duplicate_group_assets_asset; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_duplicate_group_assets_asset ON public.duplicate_group_assets USING btree (asset_id);


--
-- Name: idx_duplicate_group_edges_assets; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_duplicate_group_edges_assets ON public.duplicate_group_edges USING btree (asset_id_a, asset_id_b);


--
-- Name: idx_duplicate_groups_repo_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_duplicate_groups_repo_status ON public.duplicate_groups USING btree (repository_id, status, detected_at DESC);


--
-- Name: idx_duplicate_groups_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_duplicate_groups_status ON public.duplicate_groups USING btree (status);


--
-- Name: idx_duplicate_groups_owner_repo; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_duplicate_groups_owner_repo ON public.duplicate_groups USING btree (owner_id, repository_id);


--
-- Name: idx_location_cluster_assets_asset; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_location_cluster_assets_asset ON public.location_cluster_assets USING btree (asset_id);


--
-- Name: idx_location_clusters_repository_owner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_location_clusters_repository_owner ON public.location_clusters USING btree (repository_id, owner_id);


--
-- Name: idx_location_clusters_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_location_clusters_status ON public.location_clusters USING btree (geocode_status);


--
-- Name: idx_reverse_geocode_cache_provider_language; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reverse_geocode_cache_provider_language ON public.reverse_geocode_cache USING btree (provider, language);


--
-- Name: location_clusters_search_vector_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX location_clusters_search_vector_idx ON public.location_clusters USING gin (search_vector);


--
-- Name: location_clusters trg_location_clusters_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_location_clusters_updated_at BEFORE UPDATE ON public.location_clusters FOR EACH ROW EXECUTE FUNCTION public.set_location_clusters_updated_at();


--
-- Name: album_assets album_assets_album_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--
