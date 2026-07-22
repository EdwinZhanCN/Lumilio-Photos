CREATE TABLE public.repository_roots (
    root_id uuid NOT NULL,
    name text NOT NULL,
    path text NOT NULL,
    kind text NOT NULL,
    status text NOT NULL DEFAULT 'active',
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT repository_roots_pkey PRIMARY KEY (root_id),
    CONSTRAINT repository_roots_path_key UNIQUE (path),
    CONSTRAINT repository_roots_kind_check CHECK ((kind = ANY (ARRAY['default'::text, 'external'::text]))),
    CONSTRAINT repository_roots_status_check CHECK ((status = ANY (ARRAY['active'::text, 'offline'::text, 'error'::text])))
);

CREATE UNIQUE INDEX repository_roots_one_default_idx
    ON public.repository_roots (kind)
    WHERE kind = 'default';

ALTER TABLE public.repositories
    ADD COLUMN root_id uuid,
    ADD CONSTRAINT repositories_root_id_fkey
        FOREIGN KEY (root_id) REFERENCES public.repository_roots(root_id) ON DELETE SET NULL;

CREATE INDEX repositories_root_id_idx ON public.repositories(root_id);
