-- Indexes for unified asset query API optimization

-- GIN index for JSONB metadata filtering (camera_model, lens_model, is_raw)
-- This improves filter performance on specific_metadata fields
CREATE INDEX IF NOT EXISTS idx_assets_metadata ON assets USING GIN (specific_metadata);

-- Composite index for optimized list queries with common filters
-- Covers: owner_id, type, and sorting by taken_time/upload_time
CREATE INDEX IF NOT EXISTS idx_assets_list_opt ON assets (owner_id, type, COALESCE(taken_time, upload_time) DESC) WHERE is_deleted = false;

-- Index on album_assets for JOIN performance in unified queries
CREATE INDEX IF NOT EXISTS idx_album_assets_asset ON album_assets (asset_id);
