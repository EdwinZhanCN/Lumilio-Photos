-- First-class, owner-scoped music projections. Assets remain the durable file
-- identity; these tables own only music meaning, credits, playlists, and
-- transient playback snapshots.

-- Waveforms are derivative images, but unlike photo thumbnails they are PNGs
-- and have a distinct public size so clients never mistake them for a cover.
DROP INDEX idx_thumbnails_asset_id;
ALTER TABLE thumbnails RENAME TO thumbnails_before_music;
CREATE TABLE thumbnails (
    thumbnail_id INTEGER PRIMARY KEY,
    asset_id TEXT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    size TEXT NOT NULL CHECK (size IN ('small', 'medium', 'large', 'waveform')),
    storage_path TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    repository_id TEXT REFERENCES repositories(repo_id) ON DELETE CASCADE,
    UNIQUE (asset_id, size)
) STRICT;
INSERT INTO thumbnails (thumbnail_id, asset_id, size, storage_path, mime_type, created_at, repository_id)
SELECT thumbnail_id, asset_id, size, storage_path, mime_type, created_at, repository_id
FROM thumbnails_before_music;
DROP TABLE thumbnails_before_music;
CREATE INDEX idx_thumbnails_asset_id ON thumbnails (asset_id);

CREATE TABLE music_albums (
    album_id TEXT PRIMARY KEY CHECK (album_id = lower(album_id) AND length(album_id) = 36),
    owner_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    title TEXT NOT NULL CHECK (length(trim(title)) > 0),
    release_date TEXT,
    release_precision TEXT NOT NULL DEFAULT 'unknown'
        CHECK (release_precision IN ('unknown', 'year', 'month', 'day')),
    edition TEXT NOT NULL DEFAULT '',
    release_identifier TEXT NOT NULL DEFAULT '',
    source_group TEXT NOT NULL DEFAULT '',
    artist_source TEXT NOT NULL DEFAULT 'extracted'
        CHECK (artist_source IN ('extracted', 'manual')),
    cover_asset_id TEXT REFERENCES assets(asset_id) ON DELETE SET NULL,
    revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE INDEX idx_music_albums_owner_order
    ON music_albums (owner_id, lower(title), album_id);

CREATE TABLE music_artists (
    artist_id TEXT PRIMARY KEY CHECK (artist_id = lower(artist_id) AND length(artist_id) = 36),
    owner_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    display_name TEXT NOT NULL CHECK (length(trim(display_name)) > 0),
    normalized_name TEXT NOT NULL,
    external_id TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE INDEX idx_music_artists_owner_name
    ON music_artists (owner_id, normalized_name, artist_id);

CREATE TABLE music_album_artists (
    album_id TEXT NOT NULL REFERENCES music_albums(album_id) ON DELETE CASCADE,
    artist_id TEXT NOT NULL REFERENCES music_artists(artist_id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position >= 0),
    role TEXT NOT NULL DEFAULT 'album_artist'
        CHECK (role IN ('album_artist', 'album_credit')),
    PRIMARY KEY (album_id, position),
    UNIQUE (album_id, artist_id, position)
) STRICT;

CREATE INDEX idx_music_album_artists_artist ON music_album_artists (artist_id, album_id, position);

CREATE TABLE music_tracks (
    track_id TEXT PRIMARY KEY REFERENCES assets(asset_id) ON DELETE CASCADE
        CHECK (track_id = lower(track_id) AND length(track_id) = 36),
    owner_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    designation TEXT NOT NULL DEFAULT 'music' CHECK (designation IN ('music', 'other')),
    album_id TEXT REFERENCES music_albums(album_id) ON DELETE SET NULL,
    title TEXT NOT NULL DEFAULT '',
    album_title TEXT NOT NULL DEFAULT '',
    artist_name TEXT NOT NULL DEFAULT '',
    album_artist_name TEXT NOT NULL DEFAULT '',
    genre TEXT NOT NULL DEFAULT '',
    release_date TEXT,
    release_precision TEXT NOT NULL DEFAULT 'unknown'
        CHECK (release_precision IN ('unknown', 'year', 'month', 'day')),
    edition TEXT NOT NULL DEFAULT '',
    release_identifier TEXT NOT NULL DEFAULT '',
    disc_number INTEGER CHECK (disc_number IS NULL OR disc_number > 0),
    disc_total INTEGER CHECK (disc_total IS NULL OR disc_total > 0),
    track_number INTEGER CHECK (track_number IS NULL OR track_number > 0),
    track_total INTEGER CHECK (track_total IS NULL OR track_total > 0),
    is_compilation INTEGER NOT NULL DEFAULT 0 CHECK (is_compilation IN (0, 1)),
    extracted_artists TEXT NOT NULL DEFAULT '[]'
        CHECK (json_valid(extracted_artists) AND json_type(extracted_artists) = 'array'),
    extracted_album_artists TEXT NOT NULL DEFAULT '[]'
        CHECK (json_valid(extracted_album_artists) AND json_type(extracted_album_artists) = 'array'),
    extracted_artist_ids TEXT NOT NULL DEFAULT '[]'
        CHECK (json_valid(extracted_artist_ids) AND json_type(extracted_artist_ids) = 'array'),
    extracted_album_artist_ids TEXT NOT NULL DEFAULT '[]'
        CHECK (json_valid(extracted_album_artist_ids) AND json_type(extracted_album_artist_ids) = 'array'),
    extracted_source_revision INTEGER NOT NULL DEFAULT 0 CHECK (extracted_source_revision >= 0),
    revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE INDEX idx_music_tracks_owner_order
    ON music_tracks (owner_id, lower(title), track_id);
CREATE INDEX idx_music_tracks_album_order
    ON music_tracks (album_id, disc_number, track_number, track_id);
CREATE INDEX idx_music_tracks_owner_designation
    ON music_tracks (owner_id, designation, track_id);

CREATE TABLE music_track_artists (
    track_id TEXT NOT NULL REFERENCES music_tracks(track_id) ON DELETE CASCADE,
    artist_id TEXT NOT NULL REFERENCES music_artists(artist_id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position >= 0),
    role TEXT NOT NULL DEFAULT 'track_artist'
        CHECK (role IN ('track_artist', 'featured_artist', 'composer', 'conductor')),
    PRIMARY KEY (track_id, position),
    UNIQUE (track_id, artist_id, position)
) STRICT;

CREATE INDEX idx_music_track_artists_artist ON music_track_artists (artist_id, track_id, position);

CREATE TABLE music_track_overrides (
    track_id TEXT NOT NULL REFERENCES music_tracks(track_id) ON DELETE CASCADE,
    field TEXT NOT NULL CHECK (field IN (
        'title', 'album_title', 'artist_name', 'album_artist_name', 'genre',
        'release_date', 'release_precision', 'edition', 'disc_number', 'disc_total',
        'track_number', 'track_total', 'compilation', 'designation', 'album_id'
    )),
    value TEXT,
    is_present INTEGER NOT NULL CHECK (is_present IN (0, 1)),
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (track_id, field)
) STRICT;

CREATE TABLE music_playlists (
    playlist_id TEXT PRIMARY KEY CHECK (playlist_id = lower(playlist_id) AND length(playlist_id) = 36),
    owner_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    title TEXT NOT NULL CHECK (length(trim(title)) > 0),
    description TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE INDEX idx_music_playlists_owner_order
    ON music_playlists (owner_id, updated_at DESC, playlist_id);

CREATE TABLE music_playlist_entries (
    entry_id TEXT PRIMARY KEY CHECK (entry_id = lower(entry_id) AND length(entry_id) = 36),
    playlist_id TEXT NOT NULL REFERENCES music_playlists(playlist_id) ON DELETE CASCADE,
    track_id TEXT REFERENCES music_tracks(track_id) ON DELETE SET NULL,
    saved_title TEXT NOT NULL DEFAULT '',
    position INTEGER NOT NULL CHECK (position >= 0),
    idempotency_key TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE INDEX idx_music_playlist_entries_order
    ON music_playlist_entries (playlist_id, position, entry_id);
CREATE UNIQUE INDEX idx_music_playlist_entries_idempotency
    ON music_playlist_entries (playlist_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE TABLE music_playback_sessions (
    session_id TEXT PRIMARY KEY CHECK (session_id = lower(session_id) AND length(session_id) = 36),
    owner_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    source_kind TEXT NOT NULL CHECK (source_kind IN ('query', 'album', 'playlist', 'liked')),
    source_id TEXT NOT NULL DEFAULT '',
    source_revision INTEGER NOT NULL DEFAULT 0 CHECK (source_revision >= 0),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'expired', 'deleted')),
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE music_playback_entries (
    entry_id TEXT NOT NULL UNIQUE CHECK (entry_id = lower(entry_id) AND length(entry_id) = 36),
    session_id TEXT NOT NULL REFERENCES music_playback_sessions(session_id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL CHECK (sequence >= 0),
    track_id TEXT REFERENCES music_tracks(track_id) ON DELETE SET NULL,
    source_entry_id TEXT,
    saved_title TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (session_id, sequence)
) STRICT;

CREATE INDEX idx_music_playback_entries_page
    ON music_playback_entries (session_id, sequence, entry_id);

-- This is a projection index, not the owner boundary. Every API query still
-- joins the owner-scoped source table and uses LIKE as a safe fallback for
-- scripts and CJK queries that do not tokenize as expected.
CREATE VIRTUAL TABLE music_search_fts USING fts5(
    track_id UNINDEXED,
    title,
    artist,
    album,
    filename,
    tokenize = 'unicode61'
);

CREATE TRIGGER music_track_search_insert AFTER INSERT ON music_tracks BEGIN
    INSERT INTO music_search_fts (track_id, title, artist, album, filename)
    SELECT new.track_id, new.title, new.artist_name, new.album_title, a.original_filename
    FROM assets a WHERE a.asset_id = new.track_id;
END;
CREATE TRIGGER music_track_search_update AFTER UPDATE OF title, artist_name, album_title ON music_tracks BEGIN
    DELETE FROM music_search_fts WHERE track_id = old.track_id;
    INSERT INTO music_search_fts (track_id, title, artist, album, filename)
    SELECT new.track_id, new.title, new.artist_name, new.album_title, a.original_filename
    FROM assets a WHERE a.asset_id = new.track_id;
END;
CREATE TRIGGER music_track_search_delete AFTER DELETE ON music_tracks BEGIN
    DELETE FROM music_search_fts WHERE track_id = old.track_id;
END;

-- Each audio Asset has a catalog entry immediately, before asynchronous
-- metadata extraction runs. Existing rows are reconciled by the bounded
-- startup backfill in the service.
CREATE TRIGGER music_asset_insert AFTER INSERT ON assets
WHEN new.type = 'AUDIO' AND new.owner_id IS NOT NULL
BEGIN
    INSERT INTO music_tracks (track_id, owner_id, title, created_at, updated_at)
    VALUES (new.asset_id, new.owner_id, new.original_filename,
            CAST(unixepoch('subsec') * 1000000 AS INTEGER),
            CAST(unixepoch('subsec') * 1000000 AS INTEGER))
    ON CONFLICT (track_id) DO NOTHING;
END;

CREATE TRIGGER music_asset_owner_update AFTER UPDATE OF owner_id, type ON assets
WHEN new.type = 'AUDIO' AND new.owner_id IS NOT NULL
BEGIN
    INSERT INTO music_tracks (track_id, owner_id, title, created_at, updated_at)
    VALUES (new.asset_id, new.owner_id, new.original_filename,
            CAST(unixepoch('subsec') * 1000000 AS INTEGER),
            CAST(unixepoch('subsec') * 1000000 AS INTEGER))
    ON CONFLICT (track_id) DO UPDATE SET owner_id = excluded.owner_id,
        updated_at = excluded.updated_at;
END;

CREATE TRIGGER music_asset_no_longer_audio AFTER UPDATE OF type ON assets
WHEN new.type <> 'AUDIO'
BEGIN
    DELETE FROM music_tracks WHERE track_id = new.asset_id;
END;

PRAGMA user_version = 8;
