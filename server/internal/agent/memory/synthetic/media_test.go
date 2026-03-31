package synthetic

import (
	"testing"

	"server/internal/agent/memory"
	mocktools "server/internal/agent/mock_tools"

	"github.com/stretchr/testify/require"
)

func TestGenerateMediaEpisodes(t *testing.T) {
	t.Parallel()

	episodes := GenerateMediaEpisodes(4, 42, "mock-user")
	require.Len(t, episodes, 4)

	for _, episode := range episodes {
		require.NotEmpty(t, episode.Summary)
		require.NotEmpty(t, episode.RetrievalText)
		require.NotEmpty(t, episode.ToolTrace)
		require.Contains(t, episode.Route, "/mock/media/")
		require.True(t, episode.Status == memory.EpisodeStatusSucceeded || episode.Status == memory.EpisodeStatusRecovered)
	}
}

func TestGeneratedEpisodesOnlyUseMockMediaTools(t *testing.T) {
	t.Parallel()

	episodes := GenerateMediaEpisodes(12, 7, "mock-user")
	catalog := mocktools.Catalog()
	allowedTools := make(map[string]struct{}, len(catalog))
	for _, definition := range catalog {
		allowedTools[definition.Name] = struct{}{}
	}

	foundRecovered := false
	for _, episode := range episodes {
		for _, step := range episode.ToolTrace {
			_, ok := allowedTools[step.Tool.Name]
			require.Truef(t, ok, "unexpected tool in episode trace: %s", step.Tool.Name)
		}
		if episode.Status == memory.EpisodeStatusRecovered {
			foundRecovered = true
			require.NotEmpty(t, episode.ToolTrace)
			require.Equal(t, "mock_find_duplicate_assets", episode.ToolTrace[0].Tool.Name)
			require.NotNil(t, episode.ToolTrace[0].Error)
		}
	}

	require.True(t, foundRecovered)
}
