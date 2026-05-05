CREATE EXTENSION IF NOT EXISTS postgis;

ALTER TABLE assets
ADD COLUMN IF NOT EXISTS gps_latitude DOUBLE PRECISION,
ADD COLUMN IF NOT EXISTS gps_longitude DOUBLE PRECISION,
ADD COLUMN IF NOT EXISTS gps_geohash_5 TEXT,
ADD COLUMN IF NOT EXISTS gps_geohash_7 TEXT;

ALTER TABLE assets
ADD CONSTRAINT chk_assets_gps_latitude_range
CHECK (gps_latitude IS NULL OR gps_latitude BETWEEN -90 AND 90);

ALTER TABLE assets
ADD CONSTRAINT chk_assets_gps_longitude_range
CHECK (gps_longitude IS NULL OR gps_longitude BETWEEN -180 AND 180);

UPDATE assets
SET
    gps_latitude = (specific_metadata->>'gps_latitude')::DOUBLE PRECISION,
    gps_longitude = (specific_metadata->>'gps_longitude')::DOUBLE PRECISION,
    gps_geohash_5 = ST_GeoHash(
        ST_SetSRID(
            ST_MakePoint(
                (specific_metadata->>'gps_longitude')::DOUBLE PRECISION,
                (specific_metadata->>'gps_latitude')::DOUBLE PRECISION
            ),
            4326
        ),
        5
    ),
    gps_geohash_7 = ST_GeoHash(
        ST_SetSRID(
            ST_MakePoint(
                (specific_metadata->>'gps_longitude')::DOUBLE PRECISION,
                (specific_metadata->>'gps_latitude')::DOUBLE PRECISION
            ),
            4326
        ),
        7
    )
WHERE jsonb_typeof(specific_metadata->'gps_latitude') = 'number'
  AND jsonb_typeof(specific_metadata->'gps_longitude') = 'number'
  AND (specific_metadata->>'gps_latitude')::DOUBLE PRECISION BETWEEN -90 AND 90
  AND (specific_metadata->>'gps_longitude')::DOUBLE PRECISION BETWEEN -180 AND 180;

CREATE INDEX IF NOT EXISTS idx_assets_gps_lat_lng
ON assets(gps_latitude, gps_longitude)
WHERE gps_latitude IS NOT NULL
  AND gps_longitude IS NOT NULL
  AND is_deleted = false;

CREATE INDEX IF NOT EXISTS idx_assets_gps_geohash_7
ON assets(gps_geohash_7)
WHERE gps_geohash_7 IS NOT NULL
  AND is_deleted = false;

CREATE INDEX IF NOT EXISTS idx_assets_gps_geohash_5
ON assets(gps_geohash_5)
WHERE gps_geohash_5 IS NOT NULL
  AND is_deleted = false;

CREATE TABLE IF NOT EXISTS location_clusters (
    cluster_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id INTEGER NOT NULL DEFAULT 0,
    repository_id UUID NOT NULL REFERENCES repositories(repo_id) ON DELETE CASCADE,
    geohash TEXT NOT NULL,
    precision INTEGER NOT NULL DEFAULT 7 CHECK (precision > 0),
    centroid_latitude DOUBLE PRECISION NOT NULL CHECK (centroid_latitude BETWEEN -90 AND 90),
    centroid_longitude DOUBLE PRECISION NOT NULL CHECK (centroid_longitude BETWEEN -180 AND 180),
    photo_count INTEGER NOT NULL DEFAULT 0 CHECK (photo_count >= 0),
    label TEXT,
    country TEXT,
    region TEXT,
    city TEXT,
    provider TEXT,
    geocode_status TEXT NOT NULL DEFAULT 'pending' CHECK (geocode_status IN ('pending', 'disabled', 'cached', 'resolved', 'failed')),
    geocoded_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (owner_id, repository_id, geohash)
);

CREATE INDEX IF NOT EXISTS idx_location_clusters_repository_owner
ON location_clusters(repository_id, owner_id);

CREATE INDEX IF NOT EXISTS idx_location_clusters_status
ON location_clusters(geocode_status);

CREATE OR REPLACE FUNCTION set_location_clusters_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_location_clusters_updated_at
BEFORE UPDATE ON location_clusters
FOR EACH ROW
EXECUTE FUNCTION set_location_clusters_updated_at();

CREATE TABLE IF NOT EXISTS location_cluster_assets (
    cluster_id UUID NOT NULL REFERENCES location_clusters(cluster_id) ON DELETE CASCADE,
    asset_id UUID NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (cluster_id, asset_id)
);

CREATE INDEX IF NOT EXISTS idx_location_cluster_assets_asset
ON location_cluster_assets(asset_id);

CREATE TABLE IF NOT EXISTS reverse_geocode_cache (
    cache_key TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    language TEXT NOT NULL DEFAULT '',
    latitude DOUBLE PRECISION NOT NULL CHECK (latitude BETWEEN -90 AND 90),
    longitude DOUBLE PRECISION NOT NULL CHECK (longitude BETWEEN -180 AND 180),
    label TEXT,
    country TEXT,
    region TEXT,
    city TEXT,
    raw_response JSONB,
    queried_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_reverse_geocode_cache_provider_language
ON reverse_geocode_cache(provider, language);
