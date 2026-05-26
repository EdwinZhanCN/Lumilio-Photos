CREATE TYPE album_type AS ENUM ('default', 'bio');

ALTER TABLE albums
    ADD COLUMN album_type album_type NOT NULL DEFAULT 'default';

CREATE INDEX idx_albums_type ON albums(album_type);
