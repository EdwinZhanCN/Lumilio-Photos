package service

import (
	"testing"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"

	"github.com/stretchr/testify/require"
)

func TestEffectiveAudioMetadataKeepsOrderedCreditsAndUsesFilenameFallback(t *testing.T) {
	metadata := dbtypes.SpecificMetadata(`{
		"artists":["A & B","Guest"],
		"album_artists":["Release Artist"],
		"artist_ids":["artist-a","artist-guest"],
		"album_artist_ids":["artist-release"],
		"album":"Release",
		"release_date":"2024-02",
		"release_precision":"month",
		"disc_number":2,
		"track_number":3,
		"compilation":true
	}`)

	values, artists, albumArtists, artistIDs, albumArtistIDs, releaseID, err := effectiveAudioMetadata(metadata, "fallback.flac")
	require.NoError(t, err)
	require.Equal(t, []string{"A & B", "Guest"}, artists)
	require.Equal(t, []string{"Release Artist"}, albumArtists)
	require.Equal(t, []string{"artist-a", "artist-guest"}, artistIDs)
	require.Equal(t, []string{"artist-release"}, albumArtistIDs)
	require.Empty(t, releaseID)
	require.Equal(t, "fallback.flac", values.Title)
	require.Equal(t, "A & B", values.ArtistName)
	require.Equal(t, "Release Artist", values.AlbumArtistName)
	require.Equal(t, "month", values.ReleasePrecision)
	require.Equal(t, int64(2), *values.DiscNumber)
	require.Equal(t, int64(3), *values.TrackNumber)
	require.Equal(t, int64(1), values.Compilation)
}

func TestApplyMusicOverridesDistinguishesExplicitBlankFromInheritance(t *testing.T) {
	releaseDate := "2024"
	values := musicTrackValues{
		Title: "Source title", ArtistName: "Source artist", ReleaseDate: &releaseDate,
		ReleasePrecision: "year", DiscNumber: int64Pointer(2), Compilation: 1,
	}
	overrides := []repo.MusicTrackOverride{
		{Field: "title", IsPresent: 0},
		{Field: "release_date", IsPresent: 0},
		{Field: "disc_number", IsPresent: 1, Value: stringPointer("4")},
		{Field: "compilation", IsPresent: 1, Value: stringPointer("0")},
	}

	applyOverrides(&values, overrides)
	require.Empty(t, values.Title)
	require.Nil(t, values.ReleaseDate)
	require.Equal(t, int64(4), *values.DiscNumber)
	require.Zero(t, values.Compilation)
}

func TestApplyMusicOverridesKeepsManualAlbumAssignmentAcrossExtraction(t *testing.T) {
	manualAlbumID := "11111111-1111-1111-1111-111111111111"
	values := musicTrackValues{AlbumID: uuidNull(nil)}

	applyOverrides(&values, []repo.MusicTrackOverride{{Field: "album_id", IsPresent: 1, Value: &manualAlbumID}})

	require.True(t, values.AlbumID.Valid)
	require.Equal(t, manualAlbumID, values.AlbumID.UUID.String())

	applyOverrides(&values, []repo.MusicTrackOverride{{Field: "album_id", IsPresent: 0}})
	require.False(t, values.AlbumID.Valid)
}

func int64Pointer(value int64) *int64 {
	return &value
}
