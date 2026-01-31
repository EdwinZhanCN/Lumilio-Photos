-- Rollback indexes for unified asset query API

DROP INDEX IF EXISTS idx_album_assets_asset;
DROP INDEX IF EXISTS idx_assets_list_opt;
DROP INDEX IF EXISTS idx_assets_metadata;
