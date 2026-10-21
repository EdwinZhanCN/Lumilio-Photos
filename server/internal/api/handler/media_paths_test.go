package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"server/internal/artifact"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/pipeline"
	"server/internal/storage"
	"server/internal/storage/repocfg"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAssetAudioContentTypeUsesAssetMimeAndFilenameFallback(t *testing.T) {
	require.Equal(t, "audio/flac", assetAudioContentType(&repo.Asset{MimeType: "audio/flac", OriginalFilename: "track.mp3"}))
	require.Equal(t, "audio/mpeg", assetAudioContentType(&repo.Asset{OriginalFilename: "track.mp3"}))
	require.Equal(t, "application/octet-stream", assetAudioContentType(&repo.Asset{OriginalFilename: "track.unknown"}))
	require.Equal(t, "application/octet-stream", assetAudioContentType(nil))
}

// A media element keeps range-requesting the URL it started with. When the
// transcoded MP3 is published mid-playback, a URL that silently switches from
// the original to the MP3 answers the next range with the wrong file (or a 416
// past the MP3's end) and playback hangs. Each representation must therefore
// live at its own URL, and the unpinned URL only redirects.
func TestServePinnedWebMediaKeepsRangesOnTheRepresentationPlaybackStartedWith(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := bytes.Repeat([]byte("F"), 4096)
	transcoded := bytes.Repeat([]byte("M"), 1024)

	tempDir := t.TempDir()
	repositoryID := uuid.New()
	config := repocfg.NewRepositoryConfig("web media test")
	config.ID = repositoryID.String()
	require.NoError(t, config.SaveConfigToFile(tempDir))
	repository := repo.Repository{RepoID: repositoryID, Path: tempDir, Reachability: dbtypes.RepositoryReachabilityActive, Config: *config}
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "track.flac"), original, 0o644))
	mediaPath, err := storage.ParseUserMediaPath("track.flac")
	require.NoError(t, err)
	factory := storage.NewRepositoryFSFactory(nil, nil)
	asset := &repo.Asset{AssetID: uuid.New(), ContentID: uuid.New(), OriginalFilename: "track.flac", MimeType: "audio/flac"}
	resolver := &zipTestResolver{factory: factory, repository: repository, paths: map[uuid.UUID]storage.RepositoryPath{asset.AssetID: mediaPath}}

	serve := func(target, rangeHeader string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, target, nil)
		if rangeHeader != "" {
			c.Request.Header.Set("Range", rangeHeader)
		}
		servePinnedWebMedia(c, resolver, asset, "_web.mp3", "public, max-age=86400", func(variant webMediaVariant) string {
			if variant == webMediaVariantWeb {
				return "audio/mpeg"
			}
			return assetAudioContentType(asset)
		})
		return recorder
	}
	const base = "/api/v1/assets/id/audio/web"

	// Before the transcode exists the unpinned URL pins the original.
	first := serve(base+"?mt=token", "bytes=0-")
	require.Equal(t, http.StatusTemporaryRedirect, first.Code)
	require.Equal(t, "no-store", first.Header().Get("Cache-Control"))
	require.Equal(t, "web?mt=token&variant=original", first.Header().Get("Location"))
	require.Equal(t, http.StatusNotFound, serve(base+"?variant=web", "").Code)

	// The MP3 is published while the original is playing.
	privatePath, err := (artifact.Identity{SourceFence: asset.ContentID.String(), Stage: "transcode", PipelineVersion: pipeline.AssetPipelineVersion, Name: "web.mp3"}).Path()
	require.NoError(t, err)
	published := filepath.Join(tempDir, filepath.FromSlash(privatePath.String()))
	require.NoError(t, os.MkdirAll(filepath.Dir(published), 0o755))
	require.NoError(t, os.WriteFile(published, transcoded, 0o644))

	// The playing element's next range still reads the original, including
	// offsets past the end of the MP3.
	resumed := serve(base+"?mt=token&variant=original", "bytes=3000-")
	require.Equal(t, http.StatusPartialContent, resumed.Code)
	require.Equal(t, "audio/flac", resumed.Header().Get("Content-Type"))
	require.Equal(t, "bytes 3000-4095/4096", resumed.Header().Get("Content-Range"))
	require.Equal(t, original[3000:], resumed.Body.Bytes())

	// A new playback is pinned to the MP3.
	next := serve(base+"?mt=token", "bytes=0-")
	require.Equal(t, http.StatusTemporaryRedirect, next.Code)
	require.Equal(t, "web?mt=token&variant=web", next.Header().Get("Location"))
	web := serve(base+"?mt=token&variant=web", "bytes=0-")
	require.Equal(t, http.StatusPartialContent, web.Code)
	require.Equal(t, "audio/mpeg", web.Header().Get("Content-Type"))
	require.Equal(t, transcoded, web.Body.Bytes())

	require.Equal(t, http.StatusBadRequest, serve(base+"?variant=best", "").Code)
}
