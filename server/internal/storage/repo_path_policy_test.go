package storage

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectsPhotosLibraryBundle(t *testing.T) {
	base := canonicalTempDir(t)
	for _, path := range []string{
		filepath.Join(base, "Pictures.photoslibrary"),
		filepath.Join(base, "Pictures.photoslibrary", "Masters"),
		filepath.Join(base, "Old.aplibrary", "inner"),
	} {
		if !isInsidePhotosLibrary(path) {
			t.Fatalf("path %q was not detected as a Photos library bundle", path)
		}
	}
}

// A synced folder is a legitimate if risky choice, so it warns rather than
// rejects.
func TestCloudSyncProviderRecognizesKnownLocations(t *testing.T) {
	// Built from the platform's own root so these stay absolute on Windows,
	// where filepath.IsAbs rejects a driveless path like "/Users/someone".
	root := filepath.VolumeName(canonicalTempDir(t)) + string(filepath.Separator)
	cases := map[string]string{
		filepath.Join(root, "Users", "someone", "Library", "Mobile Documents", "com~apple~CloudDocs", "Photos"): "iCloud Drive",
		filepath.Join(root, "Users", "someone", "Dropbox", "Photos"):                                            "Dropbox",
		filepath.Join(root, "Users", "someone", "Google Drive", "Photos"):                                       "Google Drive",
		filepath.Join(root, "Users", "someone", "OneDrive", "Photos"):                                           "OneDrive",
		// OneDrive for Business appends the tenant name to the folder.
		filepath.Join(root, "Users", "someone", "OneDrive - Contoso", "Photos"): "OneDrive",
	}

	for path, provider := range cases {
		warnings := RepositoryRootWarnings(path)
		if len(warnings) != 1 || !strings.Contains(warnings[0], provider) {
			t.Fatalf("path %q warnings = %v, want one mentioning %s", path, warnings, provider)
		}
	}
}

// The markers must survive the platform separator. A separator-sensitive match
// would silently never warn on Windows, where OneDrive is on by default.
func TestCloudSyncProviderMatchesBothSeparators(t *testing.T) {
	cases := map[string]string{
		`C:\Users\someone\OneDrive\Photos`:           "OneDrive",
		`C:\Users\someone\OneDrive - Contoso\Photos`: "OneDrive",
		`C:\Users\someone\Dropbox\Photos`:            "Dropbox",
		`G:\My Drive\Photos`:                         "Google Drive",
		"/Users/someone/OneDrive/Photos":             "OneDrive",
		"/Users/someone/Photos":                      "",
		`C:\Users\someone\Photos`:                    "",
	}

	for path, want := range cases {
		if got := cloudSyncProvider(path); got != want {
			t.Fatalf("cloudSyncProvider(%q) = %q, want %q", path, got, want)
		}
	}
}
