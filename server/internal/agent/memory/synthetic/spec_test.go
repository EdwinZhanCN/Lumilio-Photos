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
	}

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
