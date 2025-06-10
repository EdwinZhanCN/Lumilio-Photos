-- Migration script for Asset Management System
-- This script migrates from photo-only to universal asset management

BEGIN;

-- Create assets table
CREATE TABLE IF NOT EXISTS assets (
    asset_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id INTEGER,
    type VARCHAR(20) NOT NULL CHECK (type IN ('PHOTO', 'VIDEO', 'AUDIO', 'DOCUMENT')),
    original_filename VARCHAR(255) NOT NULL,
    storage_path VARCHAR(512) NOT NULL,
    mime_type VARCHAR(50) NOT NULL,
    file_size BIGINT NOT NULL,
    hash VARCHAR(64),
    width INTEGER,
    height INTEGER,
    duration DOUBLE PRECISION, -- For video/audio assets in seconds
    upload_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    is_deleted BOOLEAN DEFAULT FALSE,
    deleted_at TIMESTAMP WITH TIME ZONE,
    specific_metadata JSONB
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_assets_owner_id ON assets(owner_id);
CREATE INDEX IF NOT EXISTS idx_assets_type ON assets(type);
CREATE INDEX IF NOT EXISTS idx_assets_hash ON assets(hash);
CREATE INDEX IF NOT EXISTS idx_assets_upload_time ON assets(upload_time);

-- Update thumbnails table to reference assets instead of photos
-- First, add the new column
ALTER TABLE thumbnails ADD COLUMN IF NOT EXISTS asset_id UUID;

-- Migrate existing photo thumbnails to asset references
-- (This assumes existing photos will be migrated to assets separately)
UPDATE thumbnails SET asset_id = photo_id WHERE asset_id IS NULL AND photo_id IS NOT NULL;

-- Add foreign key constraint for asset_id
ALTER TABLE thumbnails ADD CONSTRAINT fk_thumbnails_asset_id 
    FOREIGN KEY (asset_id) REFERENCES assets(asset_id) ON DELETE CASCADE;

-- Add mime_type column to thumbnails if not exists
ALTER TABLE thumbnails ADD COLUMN IF NOT EXISTS mime_type VARCHAR(50) DEFAULT 'image/jpeg';

-- Create asset_tags junction table (similar to photo_tags)
CREATE TABLE IF NOT EXISTS asset_tags (
    asset_id UUID NOT NULL,
    tag_id INTEGER NOT NULL,
    PRIMARY KEY (asset_id, tag_id),
    FOREIGN KEY (asset_id) REFERENCES assets(asset_id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(tag_id) ON DELETE CASCADE
);

-- Create album_assets junction table (similar to album_photos)
CREATE TABLE IF NOT EXISTS album_assets (
    album_id INTEGER NOT NULL,
    asset_id UUID NOT NULL,
    PRIMARY KEY (album_id, asset_id),
    FOREIGN KEY (album_id) REFERENCES albums(album_id) ON DELETE CASCADE,
    FOREIGN KEY (asset_id) REFERENCES assets(asset_id) ON DELETE CASCADE
);

-- Migrate existing photos to assets table
-- This converts existing photos to the new asset format
INSERT INTO assets (
    asset_id, 
    owner_id, 
    type, 
    original_filename, 
    storage_path, 
    mime_type, 
    file_size, 
    hash, 
    width, 
    height,
    upload_time,
    is_deleted,
    deleted_at,
    specific_metadata
)
SELECT 
    photo_id as asset_id,
    owner_id,
    'PHOTO' as type,
    original_filename,
    storage_path,
    mime_type,
    file_size,
    hash,
    width,
    height,
    upload_time,
    is_deleted,
    deleted_at,
    jsonb_build_object(
        'camera_make', camera_make,
        'camera_model', camera_model,
        'lens_model', lens_model,
        'focal_length', focal_length,
        'aperture', aperture,
        'shutter_speed', shutter_speed,
        'iso', iso,
        'date_taken', date_taken,
        'gps_latitude', gps_latitude,
        'gps_longitude', gps_longitude,
        'gps_altitude', gps_altitude
    ) as specific_metadata
FROM photos 
WHERE NOT EXISTS (
    SELECT 1 FROM assets WHERE assets.asset_id = photos.photo_id
);

-- Migrate photo_tags to asset_tags
INSERT INTO asset_tags (asset_id, tag_id)
SELECT photo_id as asset_id, tag_id
FROM photo_tags pt
WHERE NOT EXISTS (
    SELECT 1 FROM asset_tags at 
    WHERE at.asset_id = pt.photo_id AND at.tag_id = pt.tag_id
);

-- Migrate album_photos to album_assets
INSERT INTO album_assets (album_id, asset_id)
SELECT album_id, photo_id as asset_id
FROM album_photos ap
WHERE NOT EXISTS (
    SELECT 1 FROM album_assets aa 
    WHERE aa.album_id = ap.album_id AND aa.asset_id = ap.photo_id
);

COMMIT;

-- Optional: After successful migration and testing, you can drop the old tables
-- DROP TABLE IF EXISTS album_photos;
-- DROP TABLE IF EXISTS photo_tags;
-- Dropping the photos table should be done carefully after ensuring all data is migrated
-- DROP TABLE IF EXISTS photos;
