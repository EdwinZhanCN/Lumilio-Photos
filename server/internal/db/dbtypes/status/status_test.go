package status

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrackedStatusCompletesWhenAllTasksComplete(t *testing.T) {
	current := NewTrackedProcessingStatus("Asset ingestion started", []string{
		"metadata_asset",
		"thumbnail_asset",
	})

	require.Equal(t, StateProcessing, current.State)
	require.Len(t, current.Tasks, 2)

	current.MarkTaskProcessing("metadata_asset", "Extracting metadata")
	require.Equal(t, StateProcessing, current.State)
	require.Equal(t, "Extracting metadata", current.Message)

	current.MarkTaskComplete("metadata_asset", "Metadata extracted")
	require.Equal(t, StateProcessing, current.State)

	current.MarkTaskProcessing("thumbnail_asset", "Generating thumbnails")
	current.MarkTaskComplete("thumbnail_asset", "Thumbnails generated")

	require.Equal(t, StateComplete, current.State)
	require.Equal(t, "Asset processed successfully", current.Message)
	require.Empty(t, current.Errors)
}

func TestTrackedStatusFailedTaskCanRecoverAfterRetry(t *testing.T) {
	current := NewTrackedProcessingStatus("Asset ingestion started", []string{
		"metadata_asset",
		"thumbnail_asset",
	})

	current.MarkTaskComplete("metadata_asset", "Metadata extracted")
	current.MarkTaskFailed("thumbnail_asset", "thumbnail generation failed", "disk full")

	require.Equal(t, StateWarning, current.State)
	require.Equal(t, "Asset processed with 1 failed task(s)", current.Message)
	require.Len(t, current.Errors, 1)
	require.Equal(t, "thumbnail_asset", current.Errors[0].Task)

	current.MarkTaskPending("thumbnail_asset", "Retry queued for thumbnail_asset")
	require.Equal(t, StateProcessing, current.State)

	current.MarkTaskProcessing("thumbnail_asset", "Generating thumbnails")
	current.MarkTaskComplete("thumbnail_asset", "Thumbnails generated")

	require.Equal(t, StateComplete, current.State)
	require.Equal(t, "Asset processed successfully", current.Message)
	require.Empty(t, current.Errors)
}
