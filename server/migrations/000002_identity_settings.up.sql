-- Squashed baseline migration. Destructive reset: historical migrations were intentionally removed.
--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    user_id integer NOT NULL,
    username character varying(50) NOT NULL,
    password character varying(255) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    is_active boolean DEFAULT true,
    last_login timestamp with time zone,
    display_name character varying(100) DEFAULT ''::character varying NOT NULL,
    avatar_asset_id uuid,
    role character varying(20) DEFAULT 'user'::character varying NOT NULL,
    webauthn_user_handle bytea NOT NULL,
    CONSTRAINT users_pkey PRIMARY KEY (user_id),
    CONSTRAINT users_username_key UNIQUE (username),
    CONSTRAINT users_role_check CHECK (((role)::text = ANY ((ARRAY['admin'::character varying, 'user'::character varying])::text[])))
);


--
-- Name: users_user_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.users_user_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: users_user_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.users_user_id_seq OWNED BY public.users.user_id;


--
-- Name: registration_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.registration_sessions (
    session_id uuid DEFAULT gen_random_uuid() NOT NULL,
    username character varying(50) NOT NULL,
    password_hash character varying(255) NOT NULL,
    role character varying(20) NOT NULL,
    webauthn_user_handle bytea NOT NULL,
    totp_secret_ciphertext bytea,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    CONSTRAINT registration_sessions_pkey PRIMARY KEY (session_id),
    CONSTRAINT registration_sessions_username_key UNIQUE (username)
);


--
-- Name: settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.settings (
    id integer NOT NULL,
    llm_agent_enabled boolean DEFAULT false NOT NULL,
    llm_provider text DEFAULT 'ark'::text NOT NULL,
    llm_model_name text DEFAULT ''::text NOT NULL,
    llm_base_url text DEFAULT ''::text NOT NULL,
    llm_api_key_ciphertext bytea,
    llm_api_key_configured boolean DEFAULT false NOT NULL,
    ml_auto text DEFAULT 'disable'::text NOT NULL,
    ml_semantic_enabled boolean DEFAULT false NOT NULL,
    ml_ocr_enabled boolean DEFAULT false NOT NULL,
    ml_caption_enabled boolean DEFAULT false NOT NULL,
    ml_face_enabled boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by integer,
    ml_bioclip_enabled boolean DEFAULT false NOT NULL,
    CONSTRAINT settings_pkey PRIMARY KEY (id),
    CONSTRAINT settings_updated_by_fkey FOREIGN KEY (updated_by) REFERENCES public.users(user_id),
    CONSTRAINT settings_id_check CHECK ((id = 1)),
    CONSTRAINT settings_ml_auto_check CHECK ((ml_auto = ANY (ARRAY['enable'::text, 'disable'::text])))
);


--
-- Name: user_mfa_recovery_codes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_mfa_recovery_codes (
    recovery_code_id integer NOT NULL,
    user_id integer NOT NULL,
    code_hash character varying(64) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    used_at timestamp with time zone,
    CONSTRAINT user_mfa_recovery_codes_pkey PRIMARY KEY (recovery_code_id),
    CONSTRAINT user_mfa_recovery_codes_user_id_code_hash_key UNIQUE (user_id, code_hash),
    CONSTRAINT user_mfa_recovery_codes_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE
);


--
-- Name: user_mfa_recovery_codes_recovery_code_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_mfa_recovery_codes_recovery_code_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_mfa_recovery_codes_recovery_code_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_mfa_recovery_codes_recovery_code_id_seq OWNED BY public.user_mfa_recovery_codes.recovery_code_id;


--
-- Name: user_mfa_totp_credentials; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_mfa_totp_credentials (
    user_id integer NOT NULL,
    secret_ciphertext bytea NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    enabled_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    last_used_at timestamp with time zone,
    CONSTRAINT user_mfa_totp_credentials_pkey PRIMARY KEY (user_id),
    CONSTRAINT user_mfa_totp_credentials_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE
);


--
-- Name: user_webauthn_credentials; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_webauthn_credentials (
    user_webauthn_credential_id integer NOT NULL,
    credential_id bytea NOT NULL,
    user_id integer NOT NULL,
    public_key bytea NOT NULL,
    sign_count bigint DEFAULT 0 NOT NULL,
    transports jsonb DEFAULT '[]'::jsonb NOT NULL,
    attestation_type character varying(50) DEFAULT 'none'::character varying NOT NULL,
    aaguid bytea,
    user_present boolean DEFAULT false NOT NULL,
    user_verified boolean DEFAULT false NOT NULL,
    backup_eligible boolean DEFAULT false NOT NULL,
    backup_state boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    last_used_at timestamp with time zone,
    CONSTRAINT user_webauthn_credentials_pkey PRIMARY KEY (user_webauthn_credential_id),
    CONSTRAINT user_webauthn_credentials_credential_id_key UNIQUE (credential_id),
    CONSTRAINT user_webauthn_credentials_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE
);


--
-- Name: user_webauthn_credentials_user_webauthn_credential_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_webauthn_credentials_user_webauthn_credential_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_webauthn_credentials_user_webauthn_credential_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_webauthn_credentials_user_webauthn_credential_id_seq OWNED BY public.user_webauthn_credentials.user_webauthn_credential_id;


--
-- Name: refresh_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.refresh_tokens (
    token_id integer NOT NULL,
    user_id integer NOT NULL,
    token character varying(255) NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    is_revoked boolean DEFAULT false,
    CONSTRAINT refresh_tokens_pkey PRIMARY KEY (token_id),
    CONSTRAINT refresh_tokens_token_key UNIQUE (token),
    CONSTRAINT refresh_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id)
);


--
-- Name: refresh_tokens_token_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.refresh_tokens_token_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: refresh_tokens_token_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.refresh_tokens_token_id_seq OWNED BY public.refresh_tokens.token_id;


--
-- Name: refresh_tokens token_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.refresh_tokens ALTER COLUMN token_id SET DEFAULT nextval('public.refresh_tokens_token_id_seq'::regclass);


--
-- Name: user_mfa_recovery_codes recovery_code_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_mfa_recovery_codes ALTER COLUMN recovery_code_id SET DEFAULT nextval('public.user_mfa_recovery_codes_recovery_code_id_seq'::regclass);


--
-- Name: user_webauthn_credentials user_webauthn_credential_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_webauthn_credentials ALTER COLUMN user_webauthn_credential_id SET DEFAULT nextval('public.user_webauthn_credentials_user_webauthn_credential_id_seq'::regclass);


--
-- Name: users user_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users ALTER COLUMN user_id SET DEFAULT nextval('public.users_user_id_seq'::regclass);

--
-- Name: refresh_tokens refresh_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

--
-- Name: idx_refresh_tokens_tokens_token; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_refresh_tokens_tokens_token ON public.refresh_tokens USING btree (token);


--
-- Name: idx_refresh_tokens_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_refresh_tokens_user_id ON public.refresh_tokens USING btree (user_id);


--
-- Name: idx_registration_sessions_expires_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_registration_sessions_expires_at ON public.registration_sessions USING btree (expires_at);


--
-- Name: idx_user_mfa_recovery_codes_unused; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_mfa_recovery_codes_unused ON public.user_mfa_recovery_codes USING btree (user_id) WHERE (used_at IS NULL);


--
-- Name: idx_user_mfa_recovery_codes_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_mfa_recovery_codes_user_id ON public.user_mfa_recovery_codes USING btree (user_id);


--
-- Name: idx_user_webauthn_credentials_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_webauthn_credentials_user_id ON public.user_webauthn_credentials USING btree (user_id);


--
-- Name: idx_users_role; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_role ON public.users USING btree (role);


--
-- Name: idx_users_webauthn_user_handle; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_users_webauthn_user_handle ON public.users USING btree (webauthn_user_handle);


--
-- Name: refresh_tokens refresh_tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--


--
-- Name: system_state; Type: TABLE; Schema: public; Owner: -
-- Single-row source of truth for the first-run bootstrap phase. Computed from
-- the setup gates (rotated DB credential, admin user, primary repository) and
-- cached here so request paths read one column instead of re-probing.
--

CREATE TABLE public.system_state (
    id integer NOT NULL,
    bootstrap_phase text DEFAULT 'fresh'::text NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT system_state_pkey PRIMARY KEY (id),
    CONSTRAINT system_state_id_check CHECK ((id = 1)),
    CONSTRAINT system_state_bootstrap_phase_check CHECK ((bootstrap_phase = ANY (ARRAY['fresh'::text, 'db_rotated'::text, 'admin_created'::text, 'ready'::text])))
);

INSERT INTO public.system_state (id) VALUES (1) ON CONFLICT DO NOTHING;

