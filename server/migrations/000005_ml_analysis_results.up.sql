-- Squashed baseline migration. Destructive reset: historical migrations were intentionally removed.
--
-- Name: classifier_definitions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.classifier_definitions (
    id integer NOT NULL,
    slug character varying(64) NOT NULL,
    display_name character varying(100) NOT NULL,
    tag_name character varying(50) NOT NULL,
    category character varying(50) DEFAULT 'smart_album'::character varying NOT NULL,
    positive_prompts text[] NOT NULL,
    negative_prompts text[] DEFAULT '{}'::text[] NOT NULL,
    threshold real DEFAULT 0.0 NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    positive_prototype vector,
    negative_prototype vector,
    prototype_model character varying(100),
    prototype_dimensions integer,
    prototype_built_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT classifier_definitions_pkey PRIMARY KEY (id),
    CONSTRAINT classifier_definitions_slug_key UNIQUE (slug)
);


--
-- Name: classifier_definitions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.classifier_definitions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: classifier_definitions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.classifier_definitions_id_seq OWNED BY public.classifier_definitions.id;


--
-- Name: embedding_spaces; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.embedding_spaces (
    id bigint NOT NULL,
    embedding_type character varying(50) NOT NULL,
    model_id character varying(100) NOT NULL,
    dimensions integer NOT NULL,
    distance_metric character varying(20) NOT NULL,
    search_enabled boolean DEFAULT false NOT NULL,
    is_default_search boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT embedding_spaces_pkey PRIMARY KEY (id),
    CONSTRAINT embedding_spaces_dimensions_check CHECK ((dimensions > 0)),
    CONSTRAINT embedding_spaces_distance_metric_check CHECK (((distance_metric)::text = 'l2'::text))
);


--
-- Name: embedding_spaces_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.embedding_spaces_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: embedding_spaces_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.embedding_spaces_id_seq OWNED BY public.embedding_spaces.id;


--
-- Name: embeddings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.embeddings (
    id integer NOT NULL,
    asset_id uuid NOT NULL,
    embedding_type character varying(50) NOT NULL,
    embedding_model character varying(100) NOT NULL,
    embedding_dimensions integer NOT NULL,
    vector vector,
    is_primary boolean DEFAULT false,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    space_id bigint NOT NULL,
    CONSTRAINT embeddings_pkey PRIMARY KEY (id),
    CONSTRAINT embeddings_asset_id_embedding_type_embedding_model_key UNIQUE (asset_id, embedding_type, embedding_model),
    CONSTRAINT embeddings_asset_id_fkey FOREIGN KEY (asset_id) REFERENCES public.assets(asset_id) ON DELETE CASCADE,
    CONSTRAINT embeddings_space_id_fkey FOREIGN KEY (space_id) REFERENCES public.embedding_spaces(id) ON DELETE RESTRICT
);


--
-- Name: embeddings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.embeddings_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: embeddings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.embeddings_id_seq OWNED BY public.embeddings.id;


--
-- Name: face_cluster_members; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.face_cluster_members (
    id integer NOT NULL,
    cluster_id integer NOT NULL,
    face_id integer NOT NULL,
    similarity_score real NOT NULL,
    confidence real NOT NULL,
    is_manual boolean DEFAULT false,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT face_cluster_members_pkey PRIMARY KEY (id),
    CONSTRAINT face_cluster_members_cluster_id_face_id_key UNIQUE (cluster_id, face_id),
    CONSTRAINT chk_assignment_confidence_range CHECK (((confidence >= (0.0)::double precision) AND (confidence <= (1.0)::double precision))),
    CONSTRAINT chk_similarity_range CHECK (((similarity_score >= (0.0)::double precision) AND (similarity_score <= (1.0)::double precision)))
);

--
-- Name: face_cluster_members_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.face_cluster_members_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: face_cluster_members_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.face_cluster_members_id_seq OWNED BY public.face_cluster_members.id;


--
-- Name: face_clusters; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.face_clusters (
    cluster_id integer NOT NULL,
    cluster_name character varying(255),
    representative_face_id integer,
    confidence_score real DEFAULT 0.0,
    member_count integer DEFAULT 0,
    is_confirmed boolean DEFAULT false,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT face_clusters_pkey PRIMARY KEY (cluster_id)
);

--
-- Name: face_clusters_cluster_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.face_clusters_cluster_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: face_clusters_cluster_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.face_clusters_cluster_id_seq OWNED BY public.face_clusters.cluster_id;


--
-- Name: face_results; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.face_results (
    asset_id uuid NOT NULL,
    model_id character varying(100) NOT NULL,
    total_faces integer DEFAULT 0 NOT NULL,
    processing_time_ms integer,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT face_results_pkey PRIMARY KEY (asset_id),
    CONSTRAINT face_results_asset_id_fkey FOREIGN KEY (asset_id) REFERENCES public.assets(asset_id) ON DELETE CASCADE,
    CONSTRAINT chk_total_faces_nonnegative CHECK ((total_faces >= 0))
);

--
-- Name: face_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.face_items (
    id integer NOT NULL,
    asset_id uuid NOT NULL,
    face_id character varying(100),
    bounding_box jsonb NOT NULL,
    confidence real NOT NULL,
    age_group character varying(20),
    gender character varying(20),
    ethnicity character varying(30),
    expression character varying(30),
    face_size integer,
    face_image_path character varying(512),
    embedding vector(512),
    embedding_model character varying(100),
    is_primary boolean DEFAULT false,
    quality_score real,
    blur_score real,
    pose_angles jsonb,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT face_items_pkey PRIMARY KEY (id),
    CONSTRAINT face_items_asset_id_fkey FOREIGN KEY (asset_id) REFERENCES public.face_results(asset_id) ON DELETE CASCADE,
    CONSTRAINT chk_blur_range CHECK (((blur_score >= (0.0)::double precision) AND (blur_score <= (1.0)::double precision))),
    CONSTRAINT chk_confidence_range CHECK (((confidence >= (0.0)::double precision) AND (confidence <= (1.0)::double precision))),
    CONSTRAINT chk_quality_range CHECK (((quality_score >= (0.0)::double precision) AND (quality_score <= (1.0)::double precision)))
);

--
-- Name: face_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.face_items_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: face_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.face_items_id_seq OWNED BY public.face_items.id;


--
-- Name: ocr_results; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ocr_results (
    asset_id uuid NOT NULL,
    model_id character varying(100) NOT NULL,
    total_count integer DEFAULT 0 NOT NULL,
    processing_time_ms integer,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    full_text text DEFAULT ''::text NOT NULL,
    CONSTRAINT ocr_results_pkey PRIMARY KEY (asset_id),
    CONSTRAINT ocr_results_asset_id_fkey FOREIGN KEY (asset_id) REFERENCES public.assets(asset_id) ON DELETE CASCADE,
    CONSTRAINT chk_total_count_nonnegative CHECK ((total_count >= 0))
);

--
-- Name: ocr_text_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ocr_text_items (
    id integer NOT NULL,
    asset_id uuid NOT NULL,
    text_content text NOT NULL,
    confidence real NOT NULL,
    bounding_box jsonb NOT NULL,
    text_length integer NOT NULL,
    area_pixels real,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ocr_text_items_pkey PRIMARY KEY (id),
    CONSTRAINT ocr_text_items_asset_id_fkey FOREIGN KEY (asset_id) REFERENCES public.ocr_results(asset_id) ON DELETE CASCADE,
    CONSTRAINT chk_confidence_range CHECK (((confidence >= (0.0)::double precision) AND (confidence <= (1.0)::double precision)))
);

--
-- Name: ocr_text_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ocr_text_items_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ocr_text_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ocr_text_items_id_seq OWNED BY public.ocr_text_items.id;


--
-- Name: species_predictions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.species_predictions (
    prediction_id integer NOT NULL,
    asset_id uuid NOT NULL,
    label character varying(255) NOT NULL,
    score real NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    CONSTRAINT species_predictions_pkey PRIMARY KEY (prediction_id),
    CONSTRAINT species_predictions_asset_id_label_key UNIQUE (asset_id, label),
    CONSTRAINT species_predictions_asset_id_fkey FOREIGN KEY (asset_id) REFERENCES public.assets(asset_id) ON DELETE CASCADE,
    CONSTRAINT species_predictions_score_check CHECK (((score >= (0)::double precision) AND (score <= (1)::double precision)))
);


--
-- Name: species_predictions_prediction_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.species_predictions_prediction_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: species_predictions_prediction_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.species_predictions_prediction_id_seq OWNED BY public.species_predictions.prediction_id;


--
-- Name: classifier_definitions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.classifier_definitions ALTER COLUMN id SET DEFAULT nextval('public.classifier_definitions_id_seq'::regclass);


--
-- Name: embedding_spaces id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.embedding_spaces ALTER COLUMN id SET DEFAULT nextval('public.embedding_spaces_id_seq'::regclass);


--
-- Name: embeddings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.embeddings ALTER COLUMN id SET DEFAULT nextval('public.embeddings_id_seq'::regclass);


--
-- Name: face_cluster_members id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.face_cluster_members ALTER COLUMN id SET DEFAULT nextval('public.face_cluster_members_id_seq'::regclass);


--
-- Name: face_clusters cluster_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.face_clusters ALTER COLUMN cluster_id SET DEFAULT nextval('public.face_clusters_cluster_id_seq'::regclass);


--
-- Name: face_items id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.face_items ALTER COLUMN id SET DEFAULT nextval('public.face_items_id_seq'::regclass);


--
-- Name: ocr_text_items id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ocr_text_items ALTER COLUMN id SET DEFAULT nextval('public.ocr_text_items_id_seq'::regclass);


--
-- Name: species_predictions prediction_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.species_predictions ALTER COLUMN prediction_id SET DEFAULT nextval('public.species_predictions_prediction_id_seq'::regclass);

--
-- Name: classifier_definitions classifier_definitions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

--
-- Name: embedding_spaces_default_per_type_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX embedding_spaces_default_per_type_idx ON public.embedding_spaces USING btree (embedding_type) WHERE (is_default_search = true);


--
-- Name: embedding_spaces_identity_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX embedding_spaces_identity_idx ON public.embedding_spaces USING btree (embedding_type, model_id, dimensions, distance_metric);


--
-- Name: embeddings_asset_type_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX embeddings_asset_type_idx ON public.embeddings USING btree (asset_id, embedding_type);


--
-- Name: embeddings_one_primary_per_asset_type_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX embeddings_one_primary_per_asset_type_idx ON public.embeddings USING btree (asset_id, embedding_type) WHERE (is_primary = true);


--
-- Name: embeddings_primary_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX embeddings_primary_idx ON public.embeddings USING btree (embedding_type, is_primary) WHERE (is_primary = true);


--
-- Name: embeddings_space_primary_asset_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX embeddings_space_primary_asset_idx ON public.embeddings USING btree (space_id, is_primary, asset_id);


--
-- Name: embeddings_type_model_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX embeddings_type_model_idx ON public.embeddings USING btree (embedding_type, embedding_model);


--
-- Name: face_cluster_members_cluster_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX face_cluster_members_cluster_idx ON public.face_cluster_members USING btree (cluster_id);


--
-- Name: face_cluster_members_face_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX face_cluster_members_face_idx ON public.face_cluster_members USING btree (face_id);


--
-- Name: face_cluster_members_face_unique_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX face_cluster_members_face_unique_idx ON public.face_cluster_members USING btree (face_id);


--
-- Name: face_cluster_members_similarity_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX face_cluster_members_similarity_idx ON public.face_cluster_members USING btree (similarity_score);


--
-- Name: face_clusters_confirmed_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX face_clusters_confirmed_idx ON public.face_clusters USING btree (is_confirmed) WHERE (is_confirmed = true);


--
-- Name: face_clusters_representative_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX face_clusters_representative_idx ON public.face_clusters USING btree (representative_face_id);


--
-- Name: face_items_age_group_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX face_items_age_group_idx ON public.face_items USING btree (age_group);


--
-- Name: face_items_asset_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX face_items_asset_id_idx ON public.face_items USING btree (asset_id);


--
-- Name: face_items_cluster_candidate_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX face_items_cluster_candidate_idx ON public.face_items USING btree (confidence, face_size) WHERE (embedding IS NOT NULL);


--
-- Name: face_items_confidence_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX face_items_confidence_idx ON public.face_items USING btree (confidence);


--
-- Name: face_items_embedding_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX face_items_embedding_idx ON public.face_items USING hnsw (embedding vector_cosine_ops) WITH (m='16', ef_construction='200');


--
-- Name: face_items_embedding_model_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX face_items_embedding_model_idx ON public.face_items USING btree (embedding_model) WHERE (embedding IS NOT NULL);


--
-- Name: face_items_ethnicity_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX face_items_ethnicity_idx ON public.face_items USING btree (ethnicity);


--
-- Name: face_items_expression_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX face_items_expression_idx ON public.face_items USING btree (expression);


--
-- Name: face_items_face_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX face_items_face_id_idx ON public.face_items USING btree (face_id) WHERE (face_id IS NOT NULL);


--
-- Name: face_items_gender_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX face_items_gender_idx ON public.face_items USING btree (gender);


--
-- Name: face_items_is_primary_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX face_items_is_primary_idx ON public.face_items USING btree (is_primary) WHERE (is_primary = true);


--
-- Name: face_results_asset_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX face_results_asset_id_idx ON public.face_results USING btree (asset_id);


--
-- Name: face_results_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX face_results_created_at_idx ON public.face_results USING btree (created_at);


--
-- Name: face_results_model_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX face_results_model_id_idx ON public.face_results USING btree (model_id);


--
-- Name: idx_classifier_definitions_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_classifier_definitions_enabled ON public.classifier_definitions USING btree (enabled);


--
-- Name: idx_species_predictions_asset_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_species_predictions_asset_id ON public.species_predictions USING btree (asset_id);


--
-- Name: idx_species_predictions_label; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_species_predictions_label ON public.species_predictions USING btree (label);


--
-- Name: idx_species_predictions_label_asset_score; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_species_predictions_label_asset_score ON public.species_predictions USING btree (label, asset_id, score DESC) WHERE (score >= (0.5)::double precision);


--
-- Name: idx_species_predictions_label_score; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_species_predictions_label_score ON public.species_predictions USING btree (label, score DESC);


--
-- Name: idx_species_predictions_label_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_species_predictions_label_trgm ON public.species_predictions USING gin (label public.gin_trgm_ops);


--
-- Name: idx_species_predictions_score; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_species_predictions_score ON public.species_predictions USING btree (score DESC);


--
-- Name: ocr_results_asset_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ocr_results_asset_id_idx ON public.ocr_results USING btree (asset_id);


--
-- Name: ocr_results_trgm_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ocr_results_trgm_idx ON public.ocr_results USING gin (full_text gin_trgm_ops);


--
-- Name: ocr_results_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ocr_results_created_at_idx ON public.ocr_results USING btree (created_at);


--
-- Name: ocr_results_model_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ocr_results_model_id_idx ON public.ocr_results USING btree (model_id);


--
-- Name: ocr_text_items_asset_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ocr_text_items_asset_id_idx ON public.ocr_text_items USING btree (asset_id);


--
-- Name: ocr_text_items_confidence_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ocr_text_items_confidence_idx ON public.ocr_text_items USING btree (confidence);


--
-- Name: ocr_text_items_text_length_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ocr_text_items_text_length_idx ON public.ocr_text_items USING btree (text_length);


--
-- Name: face_cluster_members face_cluster_members_count_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER face_cluster_members_count_trigger AFTER INSERT OR DELETE ON public.face_cluster_members FOR EACH ROW EXECUTE FUNCTION public.update_cluster_member_count();


--
-- Name: face_items face_items_update_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER face_items_update_trigger AFTER INSERT OR DELETE OR UPDATE ON public.face_items FOR EACH ROW EXECUTE FUNCTION public.update_face_results_updated_at();


--
-- Name: ocr_text_items ocr_text_items_update_trigger; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER ocr_text_items_update_trigger AFTER INSERT OR DELETE OR UPDATE ON public.ocr_text_items FOR EACH ROW EXECUTE FUNCTION public.update_ocr_updated_at();


--
-- Name: face_cluster_members face_cluster_members_cluster_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.face_cluster_members
    ADD CONSTRAINT face_cluster_members_cluster_id_fkey FOREIGN KEY (cluster_id) REFERENCES public.face_clusters(cluster_id) ON DELETE CASCADE;


--
-- Name: face_cluster_members face_cluster_members_face_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.face_cluster_members
    ADD CONSTRAINT face_cluster_members_face_id_fkey FOREIGN KEY (face_id) REFERENCES public.face_items(id) ON DELETE CASCADE;


--
-- Name: face_clusters face_clusters_representative_face_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.face_clusters
    ADD CONSTRAINT face_clusters_representative_face_id_fkey FOREIGN KEY (representative_face_id) REFERENCES public.face_items(id) ON DELETE SET NULL;


--
-- Name: asset_quality_scores; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.asset_quality_scores (
    asset_id uuid NOT NULL,
    score real NOT NULL,
    model_version character varying(100) DEFAULT 'v1'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT asset_quality_scores_pkey PRIMARY KEY (asset_id),
    CONSTRAINT asset_quality_scores_asset_id_fkey FOREIGN KEY (asset_id) REFERENCES public.assets(asset_id) ON DELETE CASCADE,
    CONSTRAINT chk_aesthetic_score_range CHECK (((score >= 1.0) AND (score <= 10.0)))
);


--
-- Name: asset_quality_scores asset_quality_scores_asset_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

-- FK defined inline above; no additional ALTER needed.

