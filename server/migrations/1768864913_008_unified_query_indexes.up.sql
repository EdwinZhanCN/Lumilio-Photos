-- Indexes for unified asset query API optimization

-- GIN index for JSONB metadata filtering (camera_model, lens_model, is_raw)
-- This improves filter performance on specific_metadata fields
CREATE INDEX IF NOT EXISTS idx_assets_metadata ON assets USING GIN (specific_metadata);

-- Composite index for optimized list queries with common filters
-- Covers: owner_id, type, and sorting by taken_time/upload_time
CREATE INDEX IF NOT EXISTS idx_assets_list_opt ON assets (owner_id, type, COALESCE(taken_time, upload_time) DESC) WHERE is_deleted = false;

CREATE INDEX IF NOT EXISTS idx_assets_repo_time_active
    ON assets (repository_id, COALESCE(taken_time, upload_time) DESC, asset_id DESC)
    WHERE is_deleted = false;

CREATE INDEX IF NOT EXISTS idx_assets_repo_type_time_active
    ON assets (repository_id, type, COALESCE(taken_time, upload_time) DESC, asset_id DESC)
    WHERE is_deleted = false;

CREATE INDEX IF NOT EXISTS idx_assets_owner_time_active
    ON assets (owner_id, COALESCE(taken_time, upload_time) DESC, asset_id DESC)
    WHERE is_deleted = false;

CREATE INDEX IF NOT EXISTS idx_assets_mime_time_active
    ON assets (mime_type, COALESCE(taken_time, upload_time) DESC, asset_id DESC)
    WHERE is_deleted = false;

CREATE INDEX IF NOT EXISTS idx_assets_filename_trgm_active
    ON assets USING GIN (original_filename gin_trgm_ops)
    WHERE is_deleted = false;

CREATE INDEX IF NOT EXISTS idx_assets_camera_model_active
    ON assets ((specific_metadata->>'camera_model'))
    WHERE is_deleted = false
      AND specific_metadata ? 'camera_model';

CREATE INDEX IF NOT EXISTS idx_assets_lens_model_active
    ON assets ((specific_metadata->>'lens_model'))
    WHERE is_deleted = false
      AND specific_metadata ? 'lens_model';

CREATE INDEX IF NOT EXISTS idx_assets_is_raw_text_active
    ON assets ((specific_metadata->>'is_raw'))
    WHERE is_deleted = false
      AND specific_metadata ? 'is_raw';

CREATE INDEX IF NOT EXISTS idx_assets_status_state_time_active
    ON assets ((status->>'state'), COALESCE(taken_time, upload_time) DESC, asset_id DESC)
    WHERE is_deleted = false;

-- Index on album_assets for JOIN performance in unified queries
CREATE INDEX IF NOT EXISTS idx_album_assets_asset ON album_assets (asset_id);
