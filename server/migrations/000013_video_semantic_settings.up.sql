-- Video semantic search settings (exec-plans/active/video-semantic-search.md).
-- Column defaults double as seed values for existing rows.
ALTER TABLE public.settings
    ADD COLUMN ml_video_semantic_enabled boolean DEFAULT false NOT NULL,
    ADD COLUMN ml_video_max_frames integer DEFAULT 8 NOT NULL,
    ADD COLUMN ml_video_long_threshold_seconds integer DEFAULT 300 NOT NULL,
    ADD COLUMN ml_video_scene_threshold double precision DEFAULT 0.4 NOT NULL;
