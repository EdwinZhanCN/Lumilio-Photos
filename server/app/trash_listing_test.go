package app

import (
	"net/http"
	"testing"
	"time"
)

// listTrashed lists the Trash the way the Web Trash view does, in one stack
// mode and optionally scoped to one Repository, and returns the filenames.
func (s *blackboxServer) listTrashed(stackMode, repositoryID string) []string {
	s.t.Helper()
	var response struct {
		Items []struct {
			Asset     *blackboxAsset `json:"asset"`
			MediaItem *struct {
				PrimaryAsset *blackboxAsset `json:"primary_asset"`
			} `json:"media_item"`
			Stack *struct {
				Cover *struct {
					PrimaryAsset *blackboxAsset `json:"primary_asset"`
				} `json:"cover"`
			} `json:"stack"`
		} `json:"items"`
	}
	filter := map[string]any{"lifecycle_state": "trashed"}
	if repositoryID != "" {
		filter["repository_id"] = repositoryID
	}
	body := map[string]any{
		"filter":     filter,
		"pagination": map[string]any{"limit": 50, "offset": 0},
	}
	if stackMode != "" {
		body["stack_mode"] = stackMode
	}
	s.mustJSON(http.MethodPost, "/api/v1/assets/list", body, &response)
	var names []string
	for _, item := range response.Items {
		switch {
		case item.Asset != nil:
			names = append(names, item.Asset.OriginalFilename)
		case item.MediaItem != nil && item.MediaItem.PrimaryAsset != nil:
			names = append(names, item.MediaItem.PrimaryAsset.OriginalFilename)
		case item.Stack != nil && item.Stack.Cover != nil && item.Stack.Cover.PrimaryAsset != nil:
			names = append(names, item.Stack.Cover.PrimaryAsset.OriginalFilename)
		}
	}
	return names
}

// A deleted photo is listed in the Trash in every stack mode, also when the
// view is scoped to its Repository, and Restore returns it to browse.
func TestTrashListsADeletedPhotoAndRestoreReturnsIt(t *testing.T) {
	server := startBlackboxServer(t)
	const filename = "trash-listing.jpg"
	server.upload(server.primary, filename, blackboxJPEG(t, 30, "trash-listing"))
	var listed []blackboxAsset
	if !waitFor(time.Minute, func() bool { listed = server.listByFilename(filename); return len(listed) == 1 }) {
		t.Fatalf("uploaded photo never appeared in browse: %+v", listed)
	}
	server.mustJSON(http.MethodDelete, "/api/v1/assets/"+listed[0].AssetID, nil, nil)
	for _, mode := range []string{"", "expanded", "collapsed"} {
		for _, repositoryID := range []string{"", server.primary.ID} {
			if names := server.listTrashed(mode, repositoryID); len(names) != 1 || names[0] != filename {
				t.Fatalf("Trash in stack mode %q, Repository %q lists %v, want [%s]", mode, repositoryID, names, filename)
			}
		}
	}
	server.mustJSON(http.MethodPost, "/api/v1/assets/"+listed[0].AssetID+"/restore", nil, nil)
	if names := server.listTrashed("", server.primary.ID); len(names) != 0 {
		t.Fatalf("Trash after restore lists %v", names)
	}
	if got := server.listByFilename(filename); len(got) != 1 {
		t.Fatalf("restored photo is not in browse: %+v", got)
	}
}
