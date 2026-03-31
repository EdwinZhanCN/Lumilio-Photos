package memory

import (
	"testing"
	"time"

	"server/internal/agent/core"

	"github.com/stretchr/testify/require"
)

func TestBuildRetrievalTextIncludesCoreFields(t *testing.T) {
	t.Parallel()

	episode := Episode{
		Goal:    "clean up duplicate travel photos while keeping the sharpest version",
		Intent:  "cleanup_duplicates",
		Summary: "The agent tightened the duplicate threshold after a false-positive match and finished the cleanup.",
		Status:  EpisodeStatusRecovered,
		Tags:    []string{"media", "duplicates"},
		Entities: []EntityRef{
			{Type: "location", Name: "Tokyo"},
		},
		ToolTrace: []ToolTraceStep{
			{Tool: core.ToolIdentity{Name: "mock_find_duplicate_assets"}},
			{Tool: core.ToolIdentity{Name: "mock_inspect_asset_metadata"}},
			{Tool: core.ToolIdentity{Name: "mock_bulk_archive_assets"}},
		},
	}

	text := episode.BuildRetrievalText()
	require.Contains(t, text, "goal: clean up duplicate travel photos while keeping the sharpest version")
	require.Contains(t, text, "location=Tokyo")
	require.Contains(t, text, "mock_find_duplicate_assets -> mock_inspect_asset_metadata -> mock_bulk_archive_assets")
	require.Contains(t, text, "status: recovered")
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
