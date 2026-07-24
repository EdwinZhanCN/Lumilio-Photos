ALTER TABLE public.settings
    DROP COLUMN IF EXISTS ml_video_semantic_enabled,
    DROP COLUMN IF EXISTS ml_video_max_frames,
    DROP COLUMN IF EXISTS ml_video_long_threshold_seconds,
    DROP COLUMN IF EXISTS ml_video_scene_threshold;
