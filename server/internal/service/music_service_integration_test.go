package service

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/testutil"
)

func musicTestCatalog(t *testing.T) (*db.DB, *musicService, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0700))
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(dir, "catalog.sqlite3")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = catalog.Close(context.Background()) })
	require.NoError(t, catalog.Migrate(ctx))
	rootID, repositoryID := uuid.New(), uuid.New()
	_, err = catalog.SQL.ExecContext(ctx, `
 INSERT INTO users (user_id, username, password, created_at, updated_at, display_name, role, webauthn_user_handle)
 VALUES (1, 'music-owner', 'unused', 1, 1, 'Owner', 'admin', x'01'), (2, 'other-owner', 'unused', 1, 1, 'Other', 'user', x'02');
 INSERT INTO repository_roots (root_id, name, path, kind, created_at, updated_at) VALUES (?, 'Root', '/music', 'default', 1, 1);
 INSERT INTO repositories (repo_id, name, path, created_at, updated_at, default_owner_id, role, root_id) VALUES (?, 'Music', '/music/library', 1, 1, 1, 'primary', ?);`, rootID, repositoryID, rootID)
	require.NoError(t, err)
	return catalog, NewMusicService(catalog.Queries, catalog.Queries, catalog.Writer).(*musicService), repositoryID
}

func seedMusicTrack(t *testing.T, catalog *db.DB, service *musicService, repositoryID uuid.UUID, metadata string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	_, err := testutil.InsertAssetOccurrence(ctx, catalog.SQL, testutil.AssetOccurrenceParams{AssetID: id, RepositoryID: repositoryID, OwnerID: 1, AssetType: "AUDIO", Filename: id.String() + ".mp3", MIMEType: "audio/mpeg", FileSize: 100})
	require.NoError(t, err)
	require.NoError(t, service.write(ctx, func(tx *sql.Tx, q *repo.Queries) error {
		return SyncMusicTrackFromAssetTx(ctx, tx, q, id, dbtypes.SpecificMetadata(metadata))
	}))
	return id
}

func TestMusicLocalFavoritesAndLyricsRespectOwnerAndRevision(t *testing.T) {
	ctx := context.Background()
	catalog, service, repositoryID := musicTestCatalog(t)
	id := seedMusicTrack(t, catalog, service, repositoryID, `{"title":"Track","release_id":"fixture-release","album":"Release","artist":"Artist","album_artist":"Artist"}`)
	track, err := service.GetTrack(ctx, 1, id)
	require.NoError(t, err)
	require.NotNil(t, track.AlbumID)
	album, err := service.GetAlbum(ctx, 1, *track.AlbumID)
	require.NoError(t, err)
	require.NotEmpty(t, album.Artists)
	favorite := true
	album, err = service.UpdateAlbum(ctx, 1, album.AlbumID, MusicAlbumPatch{Favorite: &favorite}, album.Revision)
	require.NoError(t, err)
	require.True(t, album.Favorite)
	favorites, err := service.ListAlbums(ctx, 1, "", true, 20, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, favorites.Total)
	require.Equal(t, album.Artists, favorites.Items[0].Artists)
	other, err := service.ListAlbums(ctx, 2, "", true, 20, 0)
	require.NoError(t, err)
	require.Zero(t, other.Total)
	_, err = service.UpdateAlbum(ctx, 2, album.AlbumID, MusicAlbumPatch{Favorite: &favorite}, album.Revision)
	require.ErrorIs(t, err, sql.ErrNoRows)
	artist, err := service.GetArtist(ctx, 1, album.Artists[0].ArtistID)
	require.NoError(t, err)
	artist, err = service.UpdateArtist(ctx, 1, artist.ArtistID, MusicArtistPatch{Favorite: &favorite}, artist.Revision)
	require.NoError(t, err)
	require.True(t, artist.Favorite)
	artists, err := service.ListArtists(ctx, 1, "", true, 20, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, artists.Total)
	lyrics, err := service.GetLyrics(ctx, 1, id)
	require.NoError(t, err)
	require.Equal(t, MusicLyrics{}, lyrics)
	lyrics, err = service.UpdateLyrics(ctx, 1, id, "Line one\n第二行", 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, lyrics.Revision)
	_, err = service.UpdateLyrics(ctx, 1, id, "stale", 0)
	require.ErrorIs(t, err, ErrMusicConflict)
	_, err = service.GetLyrics(ctx, 2, id)
	require.ErrorIs(t, err, sql.ErrNoRows)
	_, err = service.UpdateLyrics(ctx, 2, id, "other owner", 1)
	require.ErrorIs(t, err, sql.ErrNoRows)
	lyrics, err = service.GetLyrics(ctx, 1, id)
	require.NoError(t, err)
	require.Equal(t, "Line one\n第二行", lyrics.Content)
}

func TestMusicPlaybackMatchesArtistIdentityAndSortAcrossPages(t *testing.T) {
	ctx := context.Background()
	catalog, service, repositoryID := musicTestCatalog(t)
	seedMusicTrack(t, catalog, service, repositoryID, `{"title":"Zulu","artist":"Exact","album_artist":"Exact","album":"Release"}`)
	seedMusicTrack(t, catalog, service, repositoryID, `{"title":"Alpha","artist":"Exact","album_artist":"Exact","album":"Release"}`)
	seedMusicTrack(t, catalog, service, repositoryID, `{"title":"Other","artist":"Exact remix","album":"Other"}`)
	artists, err := service.ListArtists(ctx, 1, "Exact", false, 20, 0)
	require.NoError(t, err)
	var artistID uuid.UUID
	for _, artist := range artists.Items {
		if artist.DisplayName == "Exact" {
			artistID = artist.ArtistID
		}
	}
	require.NotEqual(t, uuid.Nil, artistID)
	tracks, err := service.ListTracks(ctx, 1, "", "title", artistID.String(), false, 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 2, tracks.Total)
	require.Equal(t, "Zulu", tracks.Items[0].Title)
	session, err := service.CreatePlaybackSession(ctx, 1, PlaybackSource{Kind: "query", Sort: "title", ArtistID: artistID.String()})
	require.NoError(t, err)
	page, err := service.ListPlaybackEntries(ctx, 1, session.SessionID, 20, 0)
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	require.Equal(t, "Alpha", page.Items[0].TrackTitle)
	require.Equal(t, "Zulu", page.Items[1].TrackTitle)
}

func TestMusicAutomaticCoverRemainsDerivedWhenEditingAlbum(t *testing.T) {
	ctx := context.Background()
	catalog, service, repositoryID := musicTestCatalog(t)
	id := seedMusicTrack(t, catalog, service, repositoryID, `{"title":"Cover track","release_id":"cover-release","album":"Release","artist":"Artist"}`)
	track, err := service.GetTrack(ctx, 1, id)
	require.NoError(t, err)
	require.NotNil(t, track.AlbumID)
	_, err = catalog.SQL.ExecContext(ctx, `INSERT INTO thumbnails (asset_id, size, storage_path, mime_type, created_at) VALUES (?, 'medium', 'cover.jpg', 'image/jpeg', 1)`, id)
	require.NoError(t, err)
	albums, err := service.ListAlbums(ctx, 1, "", false, 20, 0)
	require.NoError(t, err)
	require.Len(t, albums.Items, 1)
	require.Equal(t, &id, albums.Items[0].CoverAssetID)
	album, err := service.GetAlbum(ctx, 1, *track.AlbumID)
	require.NoError(t, err)
	require.Equal(t, &id, album.CoverAssetID)
	title := "Renamed release"
	_, err = service.UpdateAlbum(ctx, 1, album.AlbumID, MusicAlbumPatch{Title: &title}, album.Revision)
	require.NoError(t, err)
	var manualCover sql.NullString
	require.NoError(t, catalog.SQL.QueryRowContext(ctx, `SELECT cover_asset_id FROM music_albums WHERE album_id = ?`, album.AlbumID).Scan(&manualCover))
	require.False(t, manualCover.Valid, "editing a title must not pin the automatically selected cover")
	_, err = catalog.SQL.ExecContext(ctx, `UPDATE assets SET is_deleted = 1 WHERE asset_id = ?`, id)
	require.NoError(t, err)
	album, err = service.GetAlbum(ctx, 1, album.AlbumID)
	require.NoError(t, err)
	require.Nil(t, album.CoverAssetID, "trashed tracks cannot supply an automatic cover")
}
