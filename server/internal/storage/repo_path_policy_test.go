package storage

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"server/internal/db/dbtypes"
)

func TestRootedPolicyPlacesRepositoryUnderRoot(t *testing.T) {
	root := canonicalTempDir(t)

	got, warnings, err := RootedPolicy{}.ResolveCreatePath(CreateRepositorySpec{
		Name: "Family Photos",
		Role: dbtypes.RepoRoleRegular,
		Root: root,
	})
	if err != nil {
		t.Fatalf("ResolveCreatePath returned error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if want := filepath.Join(root, "family-photos"); got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

// Silently ignoring a caller-supplied path would put the repository somewhere
// other than where the caller asked for it.
func TestRootedPolicyRejectsCallerSuppliedPath(t *testing.T) {
	_, _, err := RootedPolicy{}.ResolveCreatePath(CreateRepositorySpec{
		Name: "Family Photos",
		Role: dbtypes.RepoRoleRegular,
		Root: canonicalTempDir(t),
		Path: "/Volumes/Media/Photos",
	})
	if !errors.Is(err, ErrPathNotAllowed) {
		t.Fatalf("err = %v, want ErrPathNotAllowed", err)
	}
}

func TestFreePolicyAcceptsAbsolutePath(t *testing.T) {
	target := filepath.Join(canonicalTempDir(t), "library")

	got, warnings, err := FreePolicy{}.ResolveCreatePath(CreateRepositorySpec{
		Name: "Library",
		Role: dbtypes.RepoRoleRegular,
		Path: target,
	})
	if err != nil {
		t.Fatalf("ResolveCreatePath returned error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if got != target {
		t.Fatalf("path = %q, want %q", got, target)
	}
}

func TestFreePolicyRequiresAbsolutePath(t *testing.T) {
	for _, path := range []string{"", "   ", "relative/library"} {
		_, _, err := FreePolicy{}.ResolveCreatePath(CreateRepositorySpec{Name: "Library", Path: path})
		if !errors.Is(err, ErrPathNotAllowed) {
			t.Fatalf("path %q: err = %v, want ErrPathNotAllowed", path, err)
		}
	}
}

// Writing into an Apple Photos bundle corrupts both libraries.
func TestFreePolicyRejectsPhotosLibraryBundle(t *testing.T) {
	base := canonicalTempDir(t)
	for _, path := range []string{
		filepath.Join(base, "Pictures.photoslibrary"),
		filepath.Join(base, "Pictures.photoslibrary", "Masters"),
		filepath.Join(base, "Old.aplibrary", "inner"),
	} {
		_, _, err := FreePolicy{}.ResolveCreatePath(CreateRepositorySpec{Name: "Library", Path: path})
		if !errors.Is(err, ErrPathNotAllowed) {
			t.Fatalf("path %q: err = %v, want ErrPathNotAllowed", path, err)
		}
	}
}

// A synced folder is a legitimate if risky choice, so it warns rather than
// rejects.
func TestFreePolicyWarnsAboutCloudSyncedLocations(t *testing.T) {
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
		got, warnings, err := FreePolicy{}.ResolveCreatePath(CreateRepositorySpec{Name: "Library", Path: path})
		if err != nil {
			t.Fatalf("path %q returned error: %v", path, err)
		}
		if got == "" {
			t.Fatalf("path %q resolved to empty", path)
		}
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

// The primary repository is the application's own default storage and belongs
// with the rest of the application data, even on a desktop using FreePolicy.
func TestPathPolicyForRoleKeepsUnpositionedPrimaryRooted(t *testing.T) {
	spec := CreateRepositorySpec{Name: "Primary Storage", Root: canonicalTempDir(t)}

	policy := pathPolicyForRole(FreePolicy{}, dbtypes.RepoRolePrimary, spec)
	if _, ok := policy.(RootedPolicy); !ok {
		t.Fatalf("policy = %T, want RootedPolicy", policy)
	}

	spec.Path = filepath.Join(canonicalTempDir(t), "chosen")
	policy = pathPolicyForRole(FreePolicy{}, dbtypes.RepoRolePrimary, spec)
	if _, ok := policy.(FreePolicy); !ok {
		t.Fatalf("policy = %T, want FreePolicy when the user picked a path", policy)
	}
}
