DROP INDEX IF EXISTS idx_albums_type;

ALTER TABLE albums
    DROP COLUMN IF EXISTS album_type;

DROP TYPE IF EXISTS album_type;
