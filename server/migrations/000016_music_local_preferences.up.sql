-- Personal local listening state belongs to the owner-scoped Music entities.
ALTER TABLE music_albums ADD COLUMN favorite INTEGER NOT NULL DEFAULT 0 CHECK (favorite IN (0, 1));
ALTER TABLE music_artists ADD COLUMN favorite INTEGER NOT NULL DEFAULT 0 CHECK (favorite IN (0, 1));

CREATE TABLE music_track_lyrics (
    track_id TEXT PRIMARY KEY REFERENCES music_tracks(track_id) ON DELETE CASCADE,
    content TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0)
);
