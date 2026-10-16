package core

import (
	"github.com/stretchr/testify/require"
	"server/internal/db/repo"
	"testing"
)

func TestMusicBudgetAndArtistRotation(t *testing.T) {
	d := 60.0
	rows := []repo.SearchAgentMusicRow{{Title: "A", ArtistName: "One", Duration: &d}, {Title: "B", ArtistName: "One", Duration: &d}, {Title: "C", ArtistName: "Two", Duration: &d}, {Title: "Unknown", ArtistName: "Three"}}
	got, truncated := selectMusic(rows, MusicSearch{Limit: 3, DiverseArtists: true, DurationBudgetSeconds: 120})
	require.True(t, truncated)
	require.Len(t, got, 2)
	require.Equal(t, "A", got[0].Title)
	require.Equal(t, "C", got[1].Title)
}
