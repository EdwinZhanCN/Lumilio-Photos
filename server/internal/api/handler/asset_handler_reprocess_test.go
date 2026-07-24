package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsValidReprocessQueue(t *testing.T) {
	t.Parallel()

	for _, queue := range []string{
		"metadata_asset",
		"thumbnail_asset",
		"transcode_asset",
		"process_semantic",
		"process_bioclip",
		"process_ocr",
		"process_face",
		"process_video_frames",
	} {
		require.True(t, isValidReprocessQueue(queue), queue)
	}
	require.False(t, isValidReprocessQueue("unknown"))
}
