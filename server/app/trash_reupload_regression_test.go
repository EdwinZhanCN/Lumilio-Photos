package app

import (
	"net/http"
	"testing"
	"time"
)

// #223 defect 3: uploading a photo whose Asset is in the Trash must show it
// in the library again instead of binding the new file to the trashed Asset.
func TestLifecycleRegressionReuploadOfTrashedPhotoIsVisible(t *testing.T) {
	server := startBlackboxServer(t)
	const filename = "trash-regression.jpg"
	contents := blackboxJPEG(t, 20, "trash-regression")
	server.upload(server.primary, filename, contents)
	var listed []blackboxAsset
	if !waitFor(time.Minute, func() bool { listed = server.listByFilename(filename); return len(listed) == 1 }) {
		t.Fatalf("uploaded photo never appeared in library browse: %+v", listed)
	}

	server.waitProcessingSettled()
	server.mustJSON(http.MethodDelete, "/api/v1/assets/"+listed[0].AssetID, nil, nil)
	if !waitFor(15*time.Second, func() bool { return len(server.listByFilename(filename)) == 0 }) {
		t.Fatalf("deleted photo is still listed in library browse: %+v", server.listByFilename(filename))
	}

	server.upload(server.primary, filename, contents)
	if !waitFor(15*time.Second, func() bool { return len(server.listByFilename(filename)) == 1 }) {
		t.Fatalf("re-uploading a trashed photo did not show it in library browse: %+v (asset %s)",
			server.listByFilename(filename), listed[0].AssetID)
	}
}
