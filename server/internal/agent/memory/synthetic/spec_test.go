package synthetic

import (
	"encoding/json"
	"os"
	"testing"

	"server/internal/agent/memory"

	"github.com/stretchr/testify/require"
)

func TestExampleSpecBundleValidates(t *testing.T) {
	t.Parallel()

	bundle := ExampleSpecBundle()
	require.NoError(t, bundle.Validate())
}

func TestCompileEpisodeSpecs(t *testing.T) {
	t.Parallel()

	bundle := ExampleSpecBundle()
	episodes, err := CompileEpisodeSpecs(bundle.Episodes, 42, "spec-user")
	require.NoError(t, err)
	require.Len(t, episodes, len(bundle.Episodes))

	for _, episode := range episodes {
		require.Equal(t, "spec-user", episode.UserID)
		require.NotEmpty(t, episode.RetrievalText)
		require.NotEmpty(t, episode.ToolTrace)
		require.Contains(t, episode.Route, "/mock/media/")
		require.NotEmpty(t, episode.ID)
	}

	require.Equal(t, bundle.Episodes[0].EpisodeID, episodes[0].ID)
	require.Equal(t, bundle.Episodes[1].EpisodeID, episodes[1].ID)
	require.Equal(t, memory.EpisodeStatusRecovered, episodes[1].Status)
	require.NotNil(t, episodes[1].ToolTrace[0].Error)
}

func TestEpisodeSpecValidateRejectsMissingSteps(t *testing.T) {
	t.Parallel()

	err := EpisodeSpec{
		Scenario: "missing_steps",
		Goal:     "test invalid spec",
		Intent:   "invalid_case",
	}.Validate()
	require.Error(t, err)
}

func TestLoadSpecBundle(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	path := tempDir + "/bundle.json"
	bundle := ExampleSpecBundle()
	raw, err := json.Marshal(bundle)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o644))

	loaded, err := LoadSpecBundle(path)
	require.NoError(t, err)
	require.Equal(t, SpecSchemaVersion, loaded.SchemaVersion)
	require.Len(t, loaded.Episodes, len(bundle.Episodes))
}

func TestLoadSpecBundleNormalizesLegacyTargets(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	path := tempDir + "/legacy-bundle.json"
	bundle := ExampleSpecBundle()
	for index := range bundle.Episodes {
		bundle.Episodes[index].EpisodeID = ""
	}
	for index := range bundle.Queries {
		bundle.Queries[index].TargetEpisodeIDs = nil
	}

	raw, err := json.Marshal(bundle)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o644))

	loaded, err := LoadSpecBundle(path)
	require.NoError(t, err)
	require.NotEmpty(t, loaded.Episodes[0].EpisodeID)
	require.NotEmpty(t, loaded.Episodes[1].EpisodeID)
	require.NotEmpty(t, loaded.Queries[0].TargetEpisodeIDs)
	require.NotEmpty(t, loaded.Queries[1].TargetEpisodeIDs)
}
