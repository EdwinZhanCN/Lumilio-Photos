package memory

import (
	"testing"
	"time"

	"server/internal/agent/core"

	"github.com/stretchr/testify/require"
)

func TestBuildRetrievalTextUsesDenseRetrievalSections(t *testing.T) {
	t.Parallel()

	episode := Episode{
		Scenario: "cleanup_duplicate_shoot",
		Goal:     "clean up duplicate travel photos while keeping the sharpest version",
		Intent:   "cleanup_duplicates",
		Summary:  "The agent tightened the duplicate threshold after a false-positive match and finished the cleanup.",
		Status:   EpisodeStatusRecovered,
		Tags:     []string{"media", "duplicates"},
		Entities: []EntityRef{
			{Type: "location", Name: "Tokyo"},
		},
		Metadata: map[string]string{
			"time_window": "spring_2024",
			"liked_state": "false",
			"cluster_id":  "cluster_should_not_leak",
		},
		ToolTrace: []ToolTraceStep{
			{Tool: core.ToolIdentity{Name: "mock_find_duplicate_assets"}},
			{Tool: core.ToolIdentity{Name: "mock_inspect_asset_metadata"}},
			{Tool: core.ToolIdentity{Name: "mock_bulk_archive_assets"}},
		},
	}

	text := episode.BuildRetrievalText()
	require.Contains(t, text, "what:\nscenario=cleanup_duplicate_shoot\nintent=cleanup_duplicates")
	require.Contains(t, text, "summary=The agent tightened the duplicate threshold after a false-positive match and finished the cleanup.")
	require.Contains(t, text, "goal:\nclean up duplicate travel photos while keeping the sharpest version")
	require.Contains(t, text, "task_content:\nlocation=Tokyo")
	require.Contains(t, text, "procedure:\nmock_find_duplicate_assets -> mock_inspect_asset_metadata -> mock_bulk_archive_assets")
	require.NotContains(t, text, "slots:")
	require.NotContains(t, text, "tags:")
	require.NotContains(t, text, "status:")
	require.NotContains(t, text, "cluster_should_not_leak")
	require.NotContains(t, text, "liked_state=false")
}

func TestDefaultWritePolicy(t *testing.T) {
	t.Parallel()

	policy := DefaultWritePolicy()
	episode := Episode{
		Summary: "A valid summary",
		Status:  EpisodeStatusSucceeded,
		ToolTrace: []ToolTraceStep{
			{Tool: core.ToolIdentity{Name: "mock_filter_assets"}, StartedAt: time.Now(), FinishedAt: time.Now()},
		},
	}

	require.True(t, policy.ShouldWrite(episode))
	episode.Status = EpisodeStatusAborted
	require.False(t, policy.ShouldWrite(episode))
}
