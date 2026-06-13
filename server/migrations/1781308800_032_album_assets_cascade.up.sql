-- Deleting an album must take its association rows with it: album_assets
-- rows are meaningless without the album, and the missing CASCADE made
-- DELETE /albums/{id} fail with a foreign key violation (23503) for any
-- non-empty album. Assets themselves are soft-deleted, so the asset_id FK
-- is unaffected.
ALTER TABLE album_assets
DROP CONSTRAINT album_assets_album_id_fkey;

ALTER TABLE album_assets
ADD CONSTRAINT album_assets_album_id_fkey
FOREIGN KEY (album_id) REFERENCES albums(album_id) ON DELETE CASCADE;
