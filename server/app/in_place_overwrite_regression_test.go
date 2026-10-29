package app

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// #223 defect 5: a tool that rewrites a file in place (a pixel edit, or a
// metadata-only write) must keep the Asset and its album membership.
func TestLifecycleRegressionInPlaceOverwriteKeepsAlbumMembership(t *testing.T) {
	server := startBlackboxServer(t)
	cases := []struct {
		name    string
		before  []byte
		rewrite func([]byte) []byte
	}{
		{
			name:    "pixel-edit",
			before:  blackboxJPEG(t, 40, "in-place-pixel"),
			rewrite: func([]byte) []byte { return blackboxJPEG(t, 220, "in-place-pixel") },
		},
		{
			name:    "metadata-only",
			before:  blackboxJPEG(t, 60, "in-place-metadata"),
			rewrite: func(original []byte) []byte { return blackboxWithComment(original, "rating=5") },
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			filename := fmt.Sprintf("in-place-%s.jpg", testCase.name)
			relative := filepath.ToSlash(filepath.Join("photos", filename))
			server.writeFile(server.primary, relative, testCase.before)
			server.scan(server.primary, 2*time.Minute)
			var listed []blackboxAsset
			if !waitFor(time.Minute, func() bool { listed = server.listByFilename(filename); return len(listed) == 1 }) {
				t.Fatalf("scanned file never appeared in library browse: %+v", listed)
			}
			original := listed[0]
			albumID := server.createAlbum("In place " + testCase.name)
			server.addToAlbum(albumID, original.AssetID)

			server.writeFile(server.primary, relative, testCase.rewrite(testCase.before))
			server.scan(server.primary, 2*time.Minute)
			rewritten := func(assets []blackboxAsset) bool {
				for _, asset := range assets {
					if asset.ContentID != original.ContentID {
						return true
					}
				}
				return false
			}
			if !waitFor(time.Minute, func() bool { listed = server.listByFilename(filename); return rewritten(listed) }) {
				t.Fatalf("rewritten content never reached library browse: %+v", listed)
			}
			if len(listed) != 1 || listed[0].AssetID != original.AssetID {
				t.Fatalf("in-place rewrite of Asset %s left library browse with %+v, want the same Asset with new content",
					original.AssetID, listed)
			}
			album := server.albumAssets(albumID)
			if len(album) != 1 || album[0].AssetID != original.AssetID || album[0].ContentID == original.ContentID {
				t.Fatalf("album after in-place rewrite = %+v, want Asset %s with the new content", album, original.AssetID)
			}
		})
	}
}
