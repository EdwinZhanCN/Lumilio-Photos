package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"server/internal/api/dto"
	"server/internal/api/problem"
)

// Phase 5 proves the public commands against the real Server, catalog, and files.
func TestAssetLifecycleAPI(t *testing.T) {
	s := startBlackboxServer(t)
	const name = "lifecycle-api.jpg"
	original := s.writeFile(s.primary, name, blackboxJPEG(t, 75, "lifecycle-api"))
	s.scan(s.primary, 2*time.Minute)
	var assets []blackboxAsset
	require.True(t, waitFor(time.Minute, func() bool { assets = s.listByFilename(name); return len(assets) == 1 }))
	id := assets[0].AssetID
	selection := map[string]any{"asset_ids": []string{id}}
	album := s.createAlbum("Lifecycle API album")
	s.addToAlbum(album, id)
	var impact dto.AssetDeleteImpactDTO
	s.mustJSON(http.MethodPost, "/api/v1/assets/delete-impact", selection, &impact)
	require.Equal(t, 1, impact.Assets)
	require.Equal(t, 1, impact.Files)
	info, err := os.Stat(original)
	require.NoError(t, err)
	require.Equal(t, info.Size(), impact.Bytes)
	require.Equal(t, 30, impact.RetentionDays)
	require.Equal(t, s.primary.ID, impact.Repositories[0].ID)
	s.mustJSON(http.MethodPost, "/api/v1/assets/trash", selection, nil)
	_, err = os.Stat(original)
	require.ErrorIs(t, err, os.ErrNotExist)
	assertProblem := func(path string, descriptor problem.Descriptor) {
		status, body := s.do(http.MethodGet, path, nil, "")
		require.Equal(t, http.StatusConflict, status, string(body))
		var details problem.Details
		require.NoError(t, json.Unmarshal(body, &details))
		require.Equal(t, descriptor.Type, details.Type)
	}
	assertProblem("/api/v1/assets/"+id+"/availability", problem.AssetTrashed)
	assertProblem("/api/v1/assets/"+id+"/original", problem.AssetTrashed)
	var view dto.StorageViewResponseDTO
	s.mustJSON(http.MethodGet, "/api/v1/storage/view", nil, &view)
	for _, repository := range view.Repositories {
		if repository.ID == s.primary.ID {
			require.Equal(t, int64(1), repository.TrashCount)
			require.Equal(t, info.Size(), repository.TrashBytes)
		}
	}
	// Unconfirmed permanent delete leaves both the catalog and the trash file.
	encoded, err := json.Marshal(selection)
	require.NoError(t, err)
	status, _ := s.do(http.MethodPost, "/api/v1/assets/delete-permanently", bytes.NewReader(encoded), "application/json")
	require.Equal(t, http.StatusBadRequest, status)
	files, err := filepath.Glob(filepath.Join(s.primary.Path, ".lumilio", "trash", "files", "*", "*"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	// An occupied destination is preserved and the API reports the new path.
	const occupied = "an unrelated file"
	require.NoError(t, os.WriteFile(original, []byte(occupied), 0o644))
	var restored dto.AssetLifecycleResultDTO
	s.mustJSON(http.MethodPost, "/api/v1/assets/restore", selection, &restored)
	require.Len(t, restored.Renamed, 1)
	data, err := os.ReadFile(original)
	require.NoError(t, err)
	require.Equal(t, occupied, string(data))
	restoredPath := filepath.Join(s.primary.Path, filepath.FromSlash(restored.Renamed[0].RestoredPath))
	_, err = os.Stat(restoredPath)
	require.NoError(t, err)
	require.Len(t, s.albumAssets(album), 1)
	// Missing metadata can be read, but originals carry the dedicated Problem.
	require.NoError(t, os.Remove(restoredPath))
	s.scan(s.primary, 2*time.Minute)
	assertProblem("/api/v1/assets/"+id+"/availability", problem.AssetMissing)
	assertProblem("/api/v1/assets/"+id+"/original", problem.AssetMissing)
	var missing dto.QueryAssetsResponseDTO
	s.mustJSON(http.MethodPost, "/api/v1/assets/list", map[string]any{"filter": map[string]any{"lifecycle_state": "missing", "repository_id": s.primary.ID}, "pagination": map[string]any{"limit": 20, "offset": 0}}, &missing)
	require.Len(t, missing.Items, 1)
	require.Equal(t, id, missing.Items[0].MediaItem.PrimaryAsset.AssetID)
	s.mustJSON(http.MethodGet, "/api/v1/storage/view", nil, &view)
	for _, repository := range view.Repositories {
		if repository.ID == s.primary.ID {
			require.Equal(t, int64(1), repository.MissingCount)
			require.Zero(t, repository.TrashCount)
		}
	}
	s.mustJSON(http.MethodPost, "/api/v1/assets/remove-missing", map[string]any{"repository_id": s.primary.ID, "confirm": true}, nil)
	status, _ = s.do(http.MethodGet, "/api/v1/assets/"+id, nil, "")
	require.Equal(t, http.StatusNotFound, status)
	data, err = os.ReadFile(original)
	require.NoError(t, err)
	require.Equal(t, occupied, string(data))
}
