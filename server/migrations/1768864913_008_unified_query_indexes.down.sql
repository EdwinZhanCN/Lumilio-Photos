-- Rollback indexes for unified asset query API

DROP INDEX IF EXISTS idx_album_assets_asset;
DROP INDEX IF EXISTS idx_assets_status_state_time_active;
DROP INDEX IF EXISTS idx_assets_is_raw_text_active;
DROP INDEX IF EXISTS idx_assets_lens_model_active;
DROP INDEX IF EXISTS idx_assets_camera_model_active;
DROP INDEX IF EXISTS idx_assets_filename_trgm_active;
DROP INDEX IF EXISTS idx_assets_mime_time_active;
DROP INDEX IF EXISTS idx_assets_owner_time_active;
DROP INDEX IF EXISTS idx_assets_repo_type_time_active;
DROP INDEX IF EXISTS idx_assets_repo_time_active;
DROP INDEX IF EXISTS idx_assets_list_opt;
DROP INDEX IF EXISTS idx_assets_metadata;
