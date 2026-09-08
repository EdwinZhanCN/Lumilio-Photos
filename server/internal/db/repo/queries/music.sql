-- Music is always owner-scoped at the query boundary. The caller never gets
-- an unscoped music row and therefore cannot accidentally turn an admin read
-- into a cross-owner library read.

-- name: ListMusicTracks :many
WITH sort_params AS (
    SELECT CAST(sqlc.arg('sort_order') AS TEXT) AS sort_order
)
SELECT
    mt.track_id, mt.owner_id, mt.designation, mt.album_id, mt.title,
    mt.album_title, mt.artist_name, mt.album_artist_name, mt.genre,
    mt.release_date, mt.release_precision, mt.edition, mt.release_identifier,
    mt.disc_number, mt.disc_total, mt.track_number, mt.track_total,
    mt.is_compilation, mt.extracted_source_revision, mt.revision,
    mt.created_at, mt.updated_at,
    a.original_filename, a.mime_type, a.duration, a.taken_time,
    a.is_deleted, a.liked
FROM music_tracks mt
JOIN assets a ON a.asset_id = mt.track_id
CROSS JOIN sort_params
WHERE mt.owner_id = sqlc.arg('owner_id')
  AND a.type = 'AUDIO'
  AND a.is_deleted = 0
  AND (
    sqlc.arg('query') = ''
    OR mt.title LIKE '%' || sqlc.arg('query') || '%'
    OR mt.artist_name LIKE '%' || sqlc.arg('query') || '%'
    OR mt.album_artist_name LIKE '%' || sqlc.arg('query') || '%'
    OR mt.album_title LIKE '%' || sqlc.arg('query') || '%'
    OR a.original_filename LIKE '%' || sqlc.arg('query') || '%'
  )
  AND (sqlc.arg('liked_only') = 0 OR a.liked = 1)
  AND (CAST(sqlc.arg('artist_filter') AS TEXT) = '' OR EXISTS (
    SELECT 1 FROM music_track_artists ta
    WHERE ta.track_id = mt.track_id AND ta.artist_id = sqlc.arg('artist_filter')
  ))
  AND sort_params.sort_order IN ('', 'title', 'artist', 'album', 'track')
ORDER BY
    CASE WHEN sort_params.sort_order = 'title' THEN lower(mt.title) END ASC,
    CASE WHEN sort_params.sort_order = 'artist' THEN lower(mt.artist_name) END ASC,
    CASE WHEN sort_params.sort_order = 'album' THEN lower(mt.album_title) END ASC,
    CASE WHEN sort_params.sort_order = 'track' THEN mt.disc_number END ASC,
    CASE WHEN sort_params.sort_order = 'track' THEN mt.track_number END ASC,
    COALESCE(a.taken_time, a.upload_time) DESC,
    mt.track_id ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountMusicTracks :one
SELECT COUNT(*)
FROM music_tracks mt
JOIN assets a ON a.asset_id = mt.track_id
WHERE mt.owner_id = sqlc.arg('owner_id')
  AND a.type = 'AUDIO'
  AND a.is_deleted = 0
  AND (
    sqlc.arg('query') = ''
    OR mt.title LIKE '%' || sqlc.arg('query') || '%'
    OR mt.artist_name LIKE '%' || sqlc.arg('query') || '%'
    OR mt.album_artist_name LIKE '%' || sqlc.arg('query') || '%'
    OR mt.album_title LIKE '%' || sqlc.arg('query') || '%'
    OR a.original_filename LIKE '%' || sqlc.arg('query') || '%'
  )
  AND (sqlc.arg('liked_only') = 0 OR a.liked = 1)
  AND (CAST(sqlc.arg('artist_filter') AS TEXT) = '' OR EXISTS (
    SELECT 1 FROM music_track_artists ta
    WHERE ta.track_id = mt.track_id AND ta.artist_id = sqlc.arg('artist_filter')
  ));

-- name: GetMusicTrack :one
SELECT
    mt.track_id, mt.owner_id, mt.designation, mt.album_id, mt.title,
    mt.album_title, mt.artist_name, mt.album_artist_name, mt.genre,
    mt.release_date, mt.release_precision, mt.edition, mt.release_identifier,
    mt.disc_number, mt.disc_total, mt.track_number, mt.track_total,
    mt.is_compilation, mt.extracted_source_revision, mt.revision,
    mt.created_at, mt.updated_at,
    a.original_filename, a.mime_type, a.duration, a.taken_time,
    a.is_deleted, a.liked
FROM music_tracks mt
JOIN assets a ON a.asset_id = mt.track_id
WHERE mt.track_id = sqlc.arg('track_id')
  AND mt.owner_id = sqlc.arg('owner_id')
  AND a.type = 'AUDIO';

-- name: GetMusicTrackSource :one
SELECT * FROM music_tracks
WHERE track_id = sqlc.arg('track_id') AND owner_id = sqlc.arg('owner_id');

-- name: UpsertMusicTrack :one
INSERT INTO music_tracks (
    track_id, owner_id, designation, album_id, title, album_title,
    artist_name, album_artist_name, genre, release_date, release_precision,
    edition, release_identifier, disc_number, disc_total, track_number,
    track_total, is_compilation, extracted_artists, extracted_album_artists,
    extracted_artist_ids, extracted_album_artist_ids, extracted_source_revision,
    revision, created_at, updated_at
)
VALUES (
    sqlc.arg('track_id'), sqlc.arg('owner_id'), sqlc.arg('designation'),
    sqlc.narg('album_id'), sqlc.arg('title'), sqlc.arg('album_title'),
    sqlc.arg('artist_name'), sqlc.arg('album_artist_name'), sqlc.arg('genre'),
    sqlc.narg('release_date'), sqlc.arg('release_precision'), sqlc.arg('edition'),
    sqlc.arg('release_identifier'), sqlc.narg('disc_number'), sqlc.narg('disc_total'),
    sqlc.narg('track_number'), sqlc.narg('track_total'), sqlc.arg('is_compilation'),
    sqlc.arg('extracted_artists'), sqlc.arg('extracted_album_artists'),
    sqlc.arg('extracted_artist_ids'), sqlc.arg('extracted_album_artist_ids'),
    sqlc.arg('extracted_source_revision'), 1,
    CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    CAST(unixepoch('subsec') * 1000000 AS INTEGER)
)
ON CONFLICT (track_id) DO UPDATE SET
    owner_id = excluded.owner_id,
    designation = excluded.designation,
    title = excluded.title,
    album_title = excluded.album_title,
    artist_name = excluded.artist_name,
    album_artist_name = excluded.album_artist_name,
    genre = excluded.genre,
    release_date = excluded.release_date,
    release_precision = excluded.release_precision,
    edition = excluded.edition,
    release_identifier = excluded.release_identifier,
    album_id = excluded.album_id,
    disc_number = excluded.disc_number,
    disc_total = excluded.disc_total,
    track_number = excluded.track_number,
    track_total = excluded.track_total,
    is_compilation = excluded.is_compilation,
    extracted_artists = excluded.extracted_artists,
    extracted_album_artists = excluded.extracted_album_artists,
    extracted_artist_ids = excluded.extracted_artist_ids,
    extracted_album_artist_ids = excluded.extracted_album_artist_ids,
    extracted_source_revision = excluded.extracted_source_revision,
    revision = music_tracks.revision + 1,
    updated_at = excluded.updated_at
RETURNING *;

-- name: UpdateMusicTrack :one
UPDATE music_tracks
SET designation = sqlc.arg('designation'),
    album_id = sqlc.narg('album_id'),
    title = sqlc.arg('title'),
    album_title = sqlc.arg('album_title'),
    artist_name = sqlc.arg('artist_name'),
    album_artist_name = sqlc.arg('album_artist_name'),
    genre = sqlc.arg('genre'),
    release_date = sqlc.narg('release_date'),
    release_precision = sqlc.arg('release_precision'),
    edition = sqlc.arg('edition'),
    disc_number = sqlc.narg('disc_number'),
    disc_total = sqlc.narg('disc_total'),
    track_number = sqlc.narg('track_number'),
    track_total = sqlc.narg('track_total'),
    is_compilation = sqlc.arg('is_compilation'),
    revision = revision + 1,
    updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE track_id = sqlc.arg('track_id')
  AND owner_id = sqlc.arg('owner_id')
  AND revision = sqlc.arg('expected_revision')
RETURNING *;

-- name: ListMusicTrackOverrides :many
SELECT * FROM music_track_overrides
WHERE track_id = sqlc.arg('track_id')
ORDER BY field;

-- name: UpsertMusicTrackOverride :exec
INSERT INTO music_track_overrides (track_id, field, value, is_present, updated_at)
VALUES (sqlc.arg('track_id'), sqlc.arg('field'), sqlc.narg('value'), sqlc.arg('is_present'),
        CAST(unixepoch('subsec') * 1000000 AS INTEGER))
ON CONFLICT (track_id, field) DO UPDATE SET
    value = excluded.value,
    is_present = excluded.is_present,
    updated_at = excluded.updated_at;

-- name: DeleteMusicTrackOverrides :exec
DELETE FROM music_track_overrides WHERE track_id = sqlc.arg('track_id');

-- name: ListMusicTrackArtists :many
SELECT ar.artist_id, ar.owner_id, ar.display_name, ar.normalized_name,
       ar.external_id, ta.position, ta.role
FROM music_track_artists ta
JOIN music_artists ar ON ar.artist_id = ta.artist_id
JOIN music_tracks mt ON mt.track_id = ta.track_id
WHERE ta.track_id = sqlc.arg('track_id') AND mt.owner_id = sqlc.arg('owner_id')
ORDER BY ta.position, ar.artist_id;

-- name: DeleteMusicTrackArtists :exec
DELETE FROM music_track_artists
WHERE music_track_artists.track_id = sqlc.arg('track_id')
  AND EXISTS (SELECT 1 FROM music_tracks mt WHERE mt.track_id = sqlc.arg('track_id') AND mt.owner_id = sqlc.arg('owner_id'));

-- name: AddMusicTrackArtist :exec
INSERT INTO music_track_artists (track_id, artist_id, position, role)
SELECT sqlc.arg('track_id'), sqlc.arg('artist_id'), sqlc.arg('position'), sqlc.arg('role')
WHERE EXISTS (
    SELECT 1 FROM music_tracks mt WHERE mt.track_id = sqlc.arg('track_id') AND mt.owner_id = sqlc.arg('owner_id')
)
AND EXISTS (
    SELECT 1 FROM music_artists WHERE artist_id = sqlc.arg('artist_id') AND owner_id = sqlc.arg('owner_id')
);

-- name: ListMusicAlbums :many
SELECT ma.album_id, ma.owner_id, ma.title, ma.release_date, ma.release_precision,
       ma.edition, ma.release_identifier, ma.source_group, ma.artist_source,
       ma.cover_asset_id, ma.favorite,
       ma.revision, ma.created_at, ma.updated_at,
       COUNT(DISTINCT a.asset_id) AS track_count
FROM music_albums ma
LEFT JOIN music_tracks mt ON mt.album_id = ma.album_id
LEFT JOIN assets a ON a.asset_id = mt.track_id AND a.is_deleted = 0
WHERE ma.owner_id = sqlc.arg('owner_id')
  AND (sqlc.arg('favorites_only') = 0 OR ma.favorite = 1)
  AND (sqlc.arg('query') = '' OR ma.title LIKE '%' || sqlc.arg('query') || '%')
GROUP BY ma.album_id
ORDER BY lower(ma.title), ma.album_id
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountMusicAlbums :one
SELECT COUNT(*) FROM music_albums
WHERE owner_id = sqlc.arg('owner_id')
  AND (sqlc.arg('favorites_only') = 0 OR favorite = 1)
  AND (sqlc.arg('query') = '' OR title LIKE '%' || sqlc.arg('query') || '%');

-- name: GetMusicAlbum :one
SELECT ma.album_id, ma.owner_id, ma.title, ma.release_date, ma.release_precision,
       ma.edition, ma.release_identifier, ma.source_group, ma.artist_source,
       ma.cover_asset_id, ma.favorite,
       ma.revision, ma.created_at, ma.updated_at,
       COUNT(DISTINCT a.asset_id) AS track_count
FROM music_albums ma
LEFT JOIN music_tracks mt ON mt.album_id = ma.album_id
LEFT JOIN assets a ON a.asset_id = mt.track_id AND a.is_deleted = 0
WHERE ma.album_id = sqlc.arg('album_id') AND ma.owner_id = sqlc.arg('owner_id')
GROUP BY ma.album_id;

-- name: GetMusicAlbumByIdentity :one
SELECT * FROM music_albums
WHERE owner_id = sqlc.arg('owner_id')
  AND title = sqlc.arg('title')
  AND release_identifier = sqlc.arg('release_identifier')
LIMIT 1;

-- name: CreateMusicAlbum :one
INSERT INTO music_albums (
    album_id, owner_id, title, release_date, release_precision, edition,
    release_identifier, source_group, artist_source, cover_asset_id,
    revision, created_at, updated_at
)
VALUES (
    sqlc.arg('album_id'), sqlc.arg('owner_id'), sqlc.arg('title'), sqlc.narg('release_date'),
    sqlc.arg('release_precision'), sqlc.arg('edition'), sqlc.arg('release_identifier'),
    sqlc.arg('source_group'), sqlc.arg('artist_source'), sqlc.narg('cover_asset_id'), 1,
    CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    CAST(unixepoch('subsec') * 1000000 AS INTEGER)
)
RETURNING *;

-- name: UpdateMusicAlbum :one
UPDATE music_albums
SET favorite = sqlc.arg('favorite'), title = sqlc.arg('title'), release_date = sqlc.narg('release_date'),
    release_precision = sqlc.arg('release_precision'), edition = sqlc.arg('edition'),
    artist_source = sqlc.arg('artist_source'),
    cover_asset_id = sqlc.narg('cover_asset_id'), revision = revision + 1,
    updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE album_id = sqlc.arg('album_id') AND owner_id = sqlc.arg('owner_id')
  AND revision = sqlc.arg('expected_revision')
RETURNING *;

-- name: SetMusicAlbumArtistSource :exec
UPDATE music_albums
SET artist_source = sqlc.arg('artist_source'),
    revision = revision + 1,
    updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE album_id = sqlc.arg('album_id') AND owner_id = sqlc.arg('owner_id');

-- name: ListMusicAlbumTracks :many
SELECT
    mt.track_id, mt.owner_id, mt.designation, mt.album_id, mt.title,
    mt.album_title, mt.artist_name, mt.album_artist_name, mt.genre,
    mt.release_date, mt.release_precision, mt.edition, mt.release_identifier,
    mt.disc_number, mt.disc_total, mt.track_number, mt.track_total,
    mt.is_compilation, mt.extracted_source_revision, mt.revision,
    mt.created_at, mt.updated_at,
    a.original_filename, a.mime_type, a.duration, a.taken_time,
    a.is_deleted, a.liked
FROM music_tracks mt
JOIN assets a ON a.asset_id = mt.track_id
WHERE mt.album_id = sqlc.arg('album_id') AND mt.owner_id = sqlc.arg('owner_id')
  AND a.type = 'AUDIO' AND a.is_deleted = 0
ORDER BY mt.disc_number IS NULL, mt.disc_number, mt.track_number IS NULL,
         mt.track_number, mt.track_id;

-- name: AssignMusicTrackAlbum :exec
UPDATE music_tracks
SET album_id = sqlc.narg('album_id'), revision = revision + 1,
    updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE track_id = sqlc.arg('track_id') AND owner_id = sqlc.arg('owner_id');

-- name: ListMusicAlbumArtists :many
SELECT ar.artist_id, ar.owner_id, ar.display_name, ar.normalized_name,
       ar.external_id, aa.position, aa.role
FROM music_album_artists aa
JOIN music_artists ar ON ar.artist_id = aa.artist_id
JOIN music_albums ma ON ma.album_id = aa.album_id
WHERE aa.album_id = sqlc.arg('album_id') AND ma.owner_id = sqlc.arg('owner_id')
ORDER BY aa.position, ar.artist_id;

-- name: DeleteMusicAlbumArtists :exec
DELETE FROM music_album_artists
WHERE music_album_artists.album_id = sqlc.arg('album_id')
  AND EXISTS (SELECT 1 FROM music_albums ma WHERE ma.album_id = sqlc.arg('album_id') AND ma.owner_id = sqlc.arg('owner_id'));

-- name: AddMusicAlbumArtist :exec
INSERT INTO music_album_artists (album_id, artist_id, position, role)
SELECT sqlc.arg('album_id'), sqlc.arg('artist_id'), sqlc.arg('position'), sqlc.arg('role')
WHERE EXISTS (
    SELECT 1 FROM music_albums ma WHERE ma.album_id = sqlc.arg('album_id') AND ma.owner_id = sqlc.arg('owner_id')
)
AND EXISTS (
    SELECT 1 FROM music_artists WHERE artist_id = sqlc.arg('artist_id') AND owner_id = sqlc.arg('owner_id')
);

-- name: FindMusicArtistsByNormalized :many
SELECT * FROM music_artists
WHERE owner_id = sqlc.arg('owner_id') AND normalized_name = sqlc.arg('normalized_name')
ORDER BY artist_id;

-- name: ListMusicArtists :many
SELECT ar.artist_id, ar.owner_id, ar.display_name, ar.normalized_name,
       ar.external_id, ar.favorite, ar.revision, ar.created_at, ar.updated_at,
       COUNT(DISTINCT ta_asset.asset_id) AS track_count,
       COUNT(DISTINCT aa.album_id) AS album_count
FROM music_artists ar
LEFT JOIN music_track_artists ta ON ta.artist_id = ar.artist_id
LEFT JOIN assets ta_asset ON ta_asset.asset_id = ta.track_id AND ta_asset.is_deleted = 0
LEFT JOIN music_album_artists aa ON aa.artist_id = ar.artist_id
WHERE ar.owner_id = sqlc.arg('owner_id')
  AND (sqlc.arg('favorites_only') = 0 OR ar.favorite = 1)
  AND (sqlc.arg('query') = '' OR ar.display_name LIKE '%' || sqlc.arg('query') || '%')
GROUP BY ar.artist_id
ORDER BY lower(ar.display_name), ar.artist_id
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountMusicArtists :one
SELECT COUNT(*) FROM music_artists
WHERE owner_id = sqlc.arg('owner_id')
  AND (sqlc.arg('favorites_only') = 0 OR favorite = 1)
  AND (sqlc.arg('query') = '' OR display_name LIKE '%' || sqlc.arg('query') || '%');

-- name: GetMusicArtist :one
SELECT ar.artist_id, ar.owner_id, ar.display_name, ar.normalized_name,
       ar.external_id, ar.favorite, ar.revision, ar.created_at, ar.updated_at,
       COUNT(DISTINCT ta_asset.asset_id) AS track_count,
       COUNT(DISTINCT aa.album_id) AS album_count
FROM music_artists ar
LEFT JOIN music_track_artists ta ON ta.artist_id = ar.artist_id
LEFT JOIN assets ta_asset ON ta_asset.asset_id = ta.track_id AND ta_asset.is_deleted = 0
LEFT JOIN music_album_artists aa ON aa.artist_id = ar.artist_id
WHERE ar.artist_id = sqlc.arg('artist_id') AND ar.owner_id = sqlc.arg('owner_id')
GROUP BY ar.artist_id;

-- name: CreateMusicArtist :one
INSERT INTO music_artists (
    artist_id, owner_id, display_name, normalized_name, external_id,
    revision, created_at, updated_at
)
VALUES (
    sqlc.arg('artist_id'), sqlc.arg('owner_id'), sqlc.arg('display_name'),
    sqlc.arg('normalized_name'), sqlc.arg('external_id'), 1,
    CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    CAST(unixepoch('subsec') * 1000000 AS INTEGER)
)
RETURNING *;

-- name: UpdateMusicArtist :one
UPDATE music_artists
SET favorite = sqlc.arg('favorite'), display_name = sqlc.arg('display_name'),
    normalized_name = sqlc.arg('normalized_name'),
    revision = revision + 1,
    updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE artist_id = sqlc.arg('artist_id') AND owner_id = sqlc.arg('owner_id')
  AND revision = sqlc.arg('expected_revision')
RETURNING *;

-- name: ListMusicPlaylistEntries :many
SELECT pe.entry_id, pe.playlist_id, pe.track_id, pe.saved_title, pe.position,
       pe.idempotency_key, pe.created_at, pe.updated_at,
       mt.title AS track_title, mt.artist_name AS track_artist,
       mt.album_title AS track_album, a.duration, a.mime_type, a.is_deleted
FROM music_playlist_entries pe
JOIN music_playlists p ON p.playlist_id = pe.playlist_id
LEFT JOIN music_tracks mt ON mt.track_id = pe.track_id AND mt.owner_id = p.owner_id
LEFT JOIN assets a ON a.asset_id = mt.track_id
WHERE pe.playlist_id = sqlc.arg('playlist_id') AND p.owner_id = sqlc.arg('owner_id')
ORDER BY pe.position, pe.entry_id;

-- name: ListMusicPlaylists :many
SELECT p.playlist_id, p.owner_id, p.title, p.description, p.revision,
       p.created_at, p.updated_at, COUNT(pe.entry_id) AS entry_count
FROM music_playlists p
LEFT JOIN music_playlist_entries pe ON pe.playlist_id = p.playlist_id
WHERE p.owner_id = sqlc.arg('owner_id')
GROUP BY p.playlist_id
ORDER BY p.updated_at DESC, p.playlist_id
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountMusicPlaylists :one
SELECT COUNT(*) FROM music_playlists WHERE owner_id = sqlc.arg('owner_id');

-- name: GetMusicPlaylist :one
SELECT p.playlist_id, p.owner_id, p.title, p.description, p.revision,
       p.created_at, p.updated_at, COUNT(pe.entry_id) AS entry_count
FROM music_playlists p
LEFT JOIN music_playlist_entries pe ON pe.playlist_id = p.playlist_id
WHERE p.playlist_id = sqlc.arg('playlist_id') AND p.owner_id = sqlc.arg('owner_id')
GROUP BY p.playlist_id;

-- name: CreateMusicPlaylist :one
INSERT INTO music_playlists (
    playlist_id, owner_id, title, description, revision, created_at, updated_at
)
VALUES (
    sqlc.arg('playlist_id'), sqlc.arg('owner_id'), sqlc.arg('title'), sqlc.arg('description'),
    1, CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    CAST(unixepoch('subsec') * 1000000 AS INTEGER)
)
RETURNING *;

-- name: UpdateMusicPlaylist :one
UPDATE music_playlists
SET title = sqlc.arg('title'), description = sqlc.arg('description'),
    revision = revision + 1,
    updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE playlist_id = sqlc.arg('playlist_id') AND owner_id = sqlc.arg('owner_id')
  AND revision = sqlc.arg('expected_revision')
RETURNING *;

-- name: DeleteMusicPlaylist :exec
DELETE FROM music_playlists
WHERE playlist_id = sqlc.arg('playlist_id') AND owner_id = sqlc.arg('owner_id');

-- name: GetMusicPlaylistEntryByIdempotency :one
SELECT pe.* FROM music_playlist_entries pe
JOIN music_playlists p ON p.playlist_id = pe.playlist_id
WHERE pe.playlist_id = sqlc.arg('playlist_id')
  AND pe.idempotency_key = sqlc.arg('idempotency_key')
  AND p.owner_id = sqlc.arg('owner_id');

-- name: CreateMusicPlaylistEntry :exec
INSERT INTO music_playlist_entries (
    entry_id, playlist_id, track_id, saved_title, position, idempotency_key,
    created_at, updated_at
)
VALUES (
    sqlc.arg('entry_id'),
    (SELECT p.playlist_id FROM music_playlists p
     WHERE p.playlist_id = sqlc.arg('playlist_id') AND p.owner_id = sqlc.arg('owner_id')),
    (SELECT mt.track_id FROM music_tracks mt JOIN assets a ON a.asset_id = mt.track_id
     WHERE mt.track_id = sqlc.arg('track_id') AND mt.owner_id = sqlc.arg('owner_id') AND a.type = 'AUDIO'),
    COALESCE(sqlc.narg('saved_title'),
        (SELECT mt.title FROM music_tracks mt WHERE mt.track_id = sqlc.arg('track_id'))),
    sqlc.arg('position'), sqlc.narg('idempotency_key'),
    CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    CAST(unixepoch('subsec') * 1000000 AS INTEGER)
);

-- name: DeleteMusicPlaylistEntry :exec
DELETE FROM music_playlist_entries
WHERE entry_id = sqlc.arg('entry_id')
  AND music_playlist_entries.playlist_id = sqlc.arg('playlist_id')
  AND EXISTS (SELECT 1 FROM music_playlists p WHERE p.playlist_id = sqlc.arg('playlist_id') AND p.owner_id = sqlc.arg('owner_id'));

-- name: UpdateMusicPlaylistEntryPosition :exec
UPDATE music_playlist_entries
SET position = sqlc.arg('position'), updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE music_playlist_entries.entry_id = sqlc.arg('entry_id') AND music_playlist_entries.playlist_id = sqlc.arg('playlist_id')
  AND EXISTS (SELECT 1 FROM music_playlists p WHERE p.playlist_id = sqlc.arg('playlist_id') AND p.owner_id = sqlc.arg('owner_id'));

-- name: BumpMusicPlaylistRevision :exec
UPDATE music_playlists
SET revision = revision + 1, updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE playlist_id = sqlc.arg('playlist_id') AND owner_id = sqlc.arg('owner_id')
  AND revision = sqlc.arg('expected_revision');

-- name: ListUncatalogedAudioAssets :many
SELECT a.* FROM assets a
LEFT JOIN music_tracks mt ON mt.track_id = a.asset_id
WHERE a.type = 'AUDIO' AND a.is_deleted = 0
  AND a.owner_id = sqlc.arg('owner_id') AND mt.track_id IS NULL
ORDER BY a.asset_id
LIMIT sqlc.arg('limit');

-- name: ListMusicOwners :many
SELECT user_id FROM users ORDER BY user_id;

-- name: CreateMusicPlaybackSession :one
INSERT INTO music_playback_sessions (
    session_id, owner_id, source_kind, source_id, source_revision, status,
    expires_at, created_at, updated_at
)
VALUES (
    sqlc.arg('session_id'), sqlc.arg('owner_id'), sqlc.arg('source_kind'),
    sqlc.arg('source_id'), sqlc.arg('source_revision'), 'active', sqlc.arg('expires_at'),
    CAST(unixepoch('subsec') * 1000000 AS INTEGER),
    CAST(unixepoch('subsec') * 1000000 AS INTEGER)
)
RETURNING *;

-- name: CreateMusicPlaybackEntry :exec
INSERT INTO music_playback_entries (
    entry_id, session_id, sequence, track_id, source_entry_id, saved_title
)
VALUES (sqlc.arg('entry_id'), sqlc.arg('session_id'), sqlc.arg('sequence'),
        sqlc.narg('track_id'), sqlc.narg('source_entry_id'), sqlc.arg('saved_title'));

-- name: GetMusicPlaybackSession :one
SELECT * FROM music_playback_sessions
WHERE session_id = sqlc.arg('session_id') AND owner_id = sqlc.arg('owner_id');

-- name: ListMusicPlaybackEntries :many
SELECT pe.entry_id, pe.session_id, pe.sequence, pe.track_id,
       pe.source_entry_id, pe.saved_title,
       mt.title AS track_title, mt.artist_name AS track_artist,
       mt.album_title AS track_album, a.mime_type, a.duration, a.is_deleted
FROM music_playback_entries pe
JOIN music_playback_sessions ps ON ps.session_id = pe.session_id
LEFT JOIN music_tracks mt ON mt.track_id = pe.track_id AND mt.owner_id = ps.owner_id
LEFT JOIN assets a ON a.asset_id = mt.track_id AND a.owner_id = ps.owner_id
WHERE pe.session_id = sqlc.arg('session_id') AND ps.owner_id = sqlc.arg('owner_id')
ORDER BY pe.sequence, pe.entry_id
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountMusicPlaybackEntries :one
SELECT COUNT(*) FROM music_playback_entries pe
JOIN music_playback_sessions ps ON ps.session_id = pe.session_id
WHERE pe.session_id = sqlc.arg('session_id') AND ps.owner_id = sqlc.arg('owner_id');

-- name: ExpireMusicPlaybackSession :exec
UPDATE music_playback_sessions
SET status = 'expired', updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE session_id = sqlc.arg('session_id') AND owner_id = sqlc.arg('owner_id');

-- name: GetMusicAlbumAutomaticCover :one
SELECT a.asset_id
FROM music_tracks mt
JOIN assets a ON a.asset_id = mt.track_id AND a.is_deleted = 0
JOIN thumbnails thumb ON thumb.asset_id = a.asset_id AND thumb.size = 'medium'
WHERE mt.album_id = sqlc.arg('album_id') AND mt.owner_id = sqlc.arg('owner_id')
  AND a.owner_id = sqlc.arg('owner_id')
ORDER BY COALESCE(mt.disc_number, 1), COALESCE(mt.track_number, 0), mt.track_id
LIMIT 1;

-- name: GetMusicTrackLyrics :one
SELECT COALESCE(l.content, '') AS content, COALESCE(l.revision, 0) AS revision
FROM music_tracks mt
LEFT JOIN music_track_lyrics l ON l.track_id = mt.track_id
JOIN assets a ON a.asset_id = mt.track_id AND a.is_deleted = 0
WHERE mt.track_id = sqlc.arg('track_id') AND mt.owner_id = sqlc.arg('owner_id');

-- name: UpsertMusicTrackLyrics :exec
INSERT INTO music_track_lyrics (track_id, content, revision)
VALUES (sqlc.arg('track_id'), sqlc.arg('content'), 1)
ON CONFLICT(track_id) DO UPDATE SET content = excluded.content, revision = music_track_lyrics.revision + 1;
