package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"server/internal/api/dto"
)

// Helpers shared by the #223 acceptance tests in this package. Each reads or
// writes only through the public HTTP API, like the Web client.

// try sends one request and returns a transport error instead of failing the
// test, for polling across an in-process runtime restart.
func (s *blackboxServer) try(method, pathname string, requestBody, responseBody any) (int, error) {
	var body io.Reader
	contentType := ""
	if requestBody != nil {
		encoded, err := json.Marshal(requestBody)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(encoded)
		contentType = "application/json"
	}
	request, err := http.NewRequest(method, s.baseURL+pathname, body)
	if err != nil {
		return 0, err
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if s.token != "" {
		request.Header.Set("Authorization", "Bearer "+s.token)
	}
	response, err := s.client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return response.StatusCode, err
	}
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusAccepted {
		return response.StatusCode, fmt.Errorf("%s %s: %d %s", method, pathname, response.StatusCode, payload)
	}
	if responseBody != nil {
		if err := json.Unmarshal(payload, responseBody); err != nil {
			return response.StatusCode, fmt.Errorf("%s %s: decode %s: %w", method, pathname, payload, err)
		}
	}
	return response.StatusCode, nil
}

// asset reads the Asset detail the viewer shows.
func (s *blackboxServer) asset(assetID string) dto.AssetDetailDTO {
	s.t.Helper()
	var detail dto.AssetDetailDTO
	s.mustJSON(http.MethodGet, "/api/v1/assets/"+url.PathEscape(assetID), nil, &detail)
	return detail
}

// searchByFilename runs the search the Web search box issues and returns the
// filenames of the result items.
func (s *blackboxServer) searchByFilename(filename string) []string {
	s.t.Helper()
	var response struct {
		TopItems    []blackboxBrowseItem `json:"top_items"`
		ResultItems []blackboxBrowseItem `json:"result_items"`
	}
	s.mustJSON(http.MethodPost, "/api/v1/assets/search", map[string]any{
		"query": filename, "pagination": map[string]any{"limit": 50, "offset": 0},
	}, &response)
	var names []string
	for _, item := range append(response.TopItems, response.ResultItems...) {
		if name := item.filename(); name == filename {
			names = append(names, name)
		}
	}
	return names
}

type blackboxBrowseItem struct {
	MediaItem *struct {
		PrimaryAsset blackboxAsset `json:"primary_asset"`
	} `json:"media_item"`
	Stack *struct {
		Cover *struct {
			PrimaryAsset blackboxAsset `json:"primary_asset"`
		} `json:"cover"`
	} `json:"stack"`
}

func (item blackboxBrowseItem) filename() string {
	switch {
	case item.MediaItem != nil:
		return item.MediaItem.PrimaryAsset.OriginalFilename
	case item.Stack != nil && item.Stack.Cover != nil:
		return item.Stack.Cover.PrimaryAsset.OriginalFilename
	}
	return ""
}

// activeAssetCount is the Storage page's Asset count for a repository.
func (s *blackboxServer) activeAssetCount(repository blackboxRepository) int64 {
	s.t.Helper()
	var view dto.StorageViewResponseDTO
	s.mustJSON(http.MethodGet, "/api/v1/storage/view", nil, &view)
	for _, candidate := range view.Repositories {
		if candidate.ID == repository.ID && candidate.AssetCount != nil {
			return *candidate.AssetCount
		}
	}
	s.t.Fatalf("repository %s is not in the storage view", repository.Name)
	return -1
}

func (s *blackboxServer) setRatingAndLike(assetID string, rating int, liked bool) {
	s.t.Helper()
	s.mustJSON(http.MethodPut, "/api/v1/assets/"+url.PathEscape(assetID)+"/rating-and-like",
		dto.UpdateRatingAndLikeRequestDTO{Rating: rating, Liked: liked}, nil)
}

func (s *blackboxServer) setDescription(assetID, description string) {
	s.t.Helper()
	s.mustJSON(http.MethodPut, "/api/v1/assets/"+url.PathEscape(assetID)+"/description",
		dto.UpdateDescriptionRequestDTO{Description: description}, nil)
}

func (s *blackboxServer) addTag(assetID, name string) {
	s.t.Helper()
	s.mustJSON(http.MethodPost, "/api/v1/assets/"+url.PathEscape(assetID)+"/tags", dto.AddAssetTagRequestDTO{TagName: name}, nil)
}

// tags lists an Asset's tags as name → source.
func (s *blackboxServer) tags(assetID string) map[string]string {
	s.t.Helper()
	var response dto.AssetTagsResponseDTO
	s.mustJSON(http.MethodGet, "/api/v1/assets/"+url.PathEscape(assetID)+"/tags", nil, &response)
	out := map[string]string{}
	for _, tag := range response.Tags {
		source := ""
		if tag.Source != nil {
			source = *tag.Source
		}
		out[tag.TagName] = source
	}
	return out
}

// thumbnail returns the bytes of one thumbnail size, or nil while it is not
// generated yet.
func (s *blackboxServer) thumbnail(assetID, size string) []byte {
	s.t.Helper()
	status, payload := s.do(http.MethodGet, "/api/v1/assets/"+url.PathEscape(assetID)+"/thumbnail?size="+size, nil, "")
	if status != http.StatusOK {
		return nil
	}
	return payload
}

// trashedFilesOf lists the files in a repository's trash directory.
func trashedFilesOf(repository blackboxRepository) []string {
	files, _ := filepath.Glob(filepath.Join(repository.Path, ".lumilio", "trash", "files", "*", "*"))
	return files
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
