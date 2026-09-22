package handler

import (
	"testing"

	"server/internal/db/repo"

	"github.com/stretchr/testify/require"
)

func TestAssetAudioContentTypeUsesAssetMimeAndFilenameFallback(t *testing.T) {
	require.Equal(t, "audio/flac", assetAudioContentType(&repo.Asset{MimeType: "audio/flac", OriginalFilename: "track.mp3"}))
	require.Equal(t, "audio/mpeg", assetAudioContentType(&repo.Asset{OriginalFilename: "track.mp3"}))
	require.Equal(t, "application/octet-stream", assetAudioContentType(&repo.Asset{OriginalFilename: "track.unknown"}))
	require.Equal(t, "application/octet-stream", assetAudioContentType(nil))
}
