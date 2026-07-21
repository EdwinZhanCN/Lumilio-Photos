package storage

import (
	"errors"
	"path/filepath"
	"strings"
)

// ErrPathNotAllowed reports that a requested repository location is not
// permitted by the active path policy.
var ErrPathNotAllowed = errors.New("repository path is not allowed")

// isInsidePhotosLibrary reports whether any component of the path is a
// .photoslibrary (or .aplibrary) bundle.
func isInsidePhotosLibrary(path string) bool {
	for _, component := range strings.Split(path, string(filepath.Separator)) {
		lowered := strings.ToLower(component)
		if strings.HasSuffix(lowered, ".photoslibrary") || strings.HasSuffix(lowered, ".aplibrary") {
			return true
		}
	}
	return false
}

// cloudSyncProvider names the sync provider whose directory contains path, or
// "" when there is none. This is a warning, not a rejection: putting a library
// in a synced folder is a legitimate if risky choice.
//
// Backslashes are folded to forward slashes unconditionally rather than with
// filepath.ToSlash, which is a no-op off Windows. The markers therefore behave
// identically on every platform and can be tested on the Linux/macOS CI that is
// the only CI running these tests. This matters most for OneDrive: it is
// enabled by default on Windows and Files On-Demand actively evicts originals,
// so it is the single most valuable warning there — and exactly the one a
// separator-sensitive match would silently never emit. A literal backslash in a
// Unix directory name is legal but vanishingly rare, and the cost of a false
// match is one extra warning, never a rejection.
func cloudSyncProvider(path string) string {
	known := []struct {
		marker string
		name   string
	}{
		{"library/mobile documents", "iCloud Drive"}, // macOS
		{"/icloud drive", "iCloud Drive"},
		{"/icloudphotos", "iCloud Drive"}, // Windows
		{"/iclouddrive", "iCloud Drive"},  // Windows
		{"/dropbox", "Dropbox"},
		{"/google drive", "Google Drive"},
		{"/googledrive", "Google Drive"},
		{"/my drive", "Google Drive"}, // Google Drive for desktop mounts a volume
		{"/onedrive", "OneDrive"},     // also matches "OneDrive - <Tenant>"
		{"/creative cloud files", "Creative Cloud"},
	}

	lowered := strings.ToLower(strings.ReplaceAll(path, `\`, "/"))
	for _, candidate := range known {
		if strings.Contains(lowered, candidate.marker) {
			return candidate.name
		}
	}
	return ""
}
