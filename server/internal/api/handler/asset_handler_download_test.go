package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"server/internal/api/dto"
	"server/internal/db/repo"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAssetHandlerDownloadAssets_RejectsEmptyAssetIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &AssetHandler{}
	body, err := json.Marshal(dto.DownloadAssetsRequestDTO{
		AssetIDs: []string{},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/assets/download", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.DownloadAssets(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestAssetHandlerDownloadAssets_RejectsInvalidAssetID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &AssetHandler{}
	body, err := json.Marshal(dto.DownloadAssetsRequestDTO{
		AssetIDs: []string{"not-a-uuid"},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/assets/download", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.DownloadAssets(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestUniqueZipArchiveName_DisambiguatesDuplicates(t *testing.T) {
	seen := map[string]int{}

	require.Equal(t, "IMG_0001.jpg", uniqueZipArchiveName(seen, "IMG_0001.jpg"))
	require.Equal(t, "IMG_0001 (2).jpg", uniqueZipArchiveName(seen, "IMG_0001.jpg"))
	require.Equal(t, "IMG_0001 (3).jpg", uniqueZipArchiveName(seen, "IMG_0001.jpg"))
	require.Equal(t, "asset", uniqueZipArchiveName(seen, "../"))
	require.Equal(t, "asset (2)", uniqueZipArchiveName(seen, ""))
}

func TestAssetHandlerWriteAssetToZip_UsesOriginalFilenames(t *testing.T) {
	tempDir := t.TempDir()
	firstPath := filepath.Join(tempDir, "first.bin")
	secondPath := filepath.Join(tempDir, "second.bin")
	require.NoError(t, os.WriteFile(firstPath, []byte("first"), 0o644))
	require.NoError(t, os.WriteFile(secondPath, []byte("second"), 0o644))

	var archive bytes.Buffer
	zipWriter := zip.NewWriter(&archive)
	handler := &AssetHandler{}
	archiveNames := map[string]int{}

	require.NoError(t, handler.writeAssetToZip(zipWriter, archiveNames, assetDownloadFile{
		asset: repo.Asset{OriginalFilename: "IMG_0001.jpg"},
		path:  firstPath,
	}))
	require.NoError(t, handler.writeAssetToZip(zipWriter, archiveNames, assetDownloadFile{
		asset: repo.Asset{OriginalFilename: "IMG_0001.jpg"},
		path:  secondPath,
	}))
	require.NoError(t, zipWriter.Close())

	reader, err := zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	require.NoError(t, err)
	require.Len(t, reader.File, 2)
	require.Equal(t, "IMG_0001.jpg", reader.File[0].Name)
	require.Equal(t, "IMG_0001 (2).jpg", reader.File[1].Name)

	firstEntry, err := reader.File[0].Open()
	require.NoError(t, err)
	defer firstEntry.Close()
	firstBytes, err := io.ReadAll(firstEntry)
	require.NoError(t, err)
	require.Equal(t, "first", string(firstBytes))
}
