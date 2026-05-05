DROP TABLE IF EXISTS reverse_geocode_cache;
DROP TABLE IF EXISTS location_cluster_assets;
DROP TRIGGER IF EXISTS trg_location_clusters_updated_at ON location_clusters;
DROP TABLE IF EXISTS location_clusters;
DROP FUNCTION IF EXISTS set_location_clusters_updated_at();

DROP INDEX IF EXISTS idx_assets_gps_geohash_5;
DROP INDEX IF EXISTS idx_assets_gps_geohash_7;
DROP INDEX IF EXISTS idx_assets_gps_lat_lng;

ALTER TABLE assets
DROP CONSTRAINT IF EXISTS chk_assets_gps_longitude_range,
DROP CONSTRAINT IF EXISTS chk_assets_gps_latitude_range;

ALTER TABLE assets
DROP COLUMN IF EXISTS gps_geohash_7,
DROP COLUMN IF EXISTS gps_geohash_5,
DROP COLUMN IF EXISTS gps_longitude,
DROP COLUMN IF EXISTS gps_latitude;
