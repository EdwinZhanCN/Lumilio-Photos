ALTER TABLE album_assets
DROP CONSTRAINT album_assets_album_id_fkey;

ALTER TABLE album_assets
ADD CONSTRAINT album_assets_album_id_fkey
FOREIGN KEY (album_id) REFERENCES albums(album_id);
