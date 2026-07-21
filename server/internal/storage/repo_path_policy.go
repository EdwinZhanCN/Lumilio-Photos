package storage

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"server/internal/db/dbtypes"
)

// ErrPathNotAllowed reports that a requested repository location is not
// permitted by the active path policy.
var ErrPathNotAllowed = errors.New("repository path is not allowed")

// PathPolicy decides where a new repository may live. It is the one place where
// the server and desktop deployments genuinely differ: a server administers a
// storage root it owns, while a desktop user picks a folder on their own disk.
//
// The policy answers only "may a repository live here". Whether a repository
// already exists at the path, or is nested inside another, stays with the
// repository manager, which already enforces both on every create path.
type PathPolicy interface {
	// ResolveCreatePath returns the on-disk path for a new repository, plus any
	// non-fatal warnings the caller should surface to the user.
	ResolveCreatePath(spec CreateRepositorySpec) (string, []string, error)
}

// RootedPolicy confines repositories to folders under the configured storage
// root, named after the repository. This is the server deployment: the
// administrator configures one root and the server owns everything below it.
type RootedPolicy struct{}

func (RootedPolicy) ResolveCreatePath(spec CreateRepositorySpec) (string, []string, error) {
	// A caller that supplies a path is asking for something this deployment does
	// not offer. Ignoring it silently would put the repository somewhere other
	// than where the caller asked.
	if strings.TrimSpace(spec.Path) != "" {
		return "", nil, fmt.Errorf("%w: this server places repositories under its storage root, not at a caller-supplied path", ErrPathNotAllowed)
	}
	path, err := resolveRepositoryCreatePath(spec.Root, spec.Name, normalizeRepoRole(spec.Role))
	if err != nil {
		return "", nil, err
	}
	return path, nil, nil
}

// FreePolicy lets a repository live at any absolute path the user names. This is
// the desktop deployment, where the user picks a folder with a native directory
// picker and the storage root remains only the application data directory.
type FreePolicy struct{}

func (FreePolicy) ResolveCreatePath(spec CreateRepositorySpec) (string, []string, error) {
	requested := strings.TrimSpace(spec.Path)
	if requested == "" {
		return "", nil, fmt.Errorf("%w: a repository path is required", ErrPathNotAllowed)
	}
	if !filepath.IsAbs(requested) {
		return "", nil, fmt.Errorf("%w: %s is not an absolute path", ErrPathNotAllowed, requested)
	}

	path, err := CanonicalizeRepositoryPath(requested)
	if err != nil {
		return "", nil, fmt.Errorf("%w: %v", ErrPathNotAllowed, err)
	}

	// An Apple Photos library is a bundle with its own internal database. Writing
	// a Lumilio repository into it corrupts both.
	if isInsidePhotosLibrary(path) {
		return "", nil, fmt.Errorf("%w: %s is inside a Photos library bundle", ErrPathNotAllowed, path)
	}

	var warnings []string
	if provider := cloudSyncProvider(path); provider != "" {
		warnings = append(warnings, fmt.Sprintf(
			"%s is inside %s. Sync clients may evict originals to the cloud or duplicate files, which Lumilio cannot detect.",
			path, provider))
	}

	return path, warnings, nil
}

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

// pathPolicyForRole keeps the primary repository rooted even under FreePolicy:
// the primary repository is the application's own default storage and belongs
// with the rest of the application data.
func pathPolicyForRole(policy PathPolicy, role dbtypes.RepoRole, spec CreateRepositorySpec) PathPolicy {
	if normalizeRepoRole(role) == dbtypes.RepoRolePrimary && strings.TrimSpace(spec.Path) == "" {
		return RootedPolicy{}
	}
	return policy
}
