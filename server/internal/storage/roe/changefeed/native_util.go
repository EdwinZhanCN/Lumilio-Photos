package changefeed

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"server/internal/db/repo"
)

func repositoryVolume(repository repo.Repository) (identity, kind string) {
	identity, kind, err := platformRepositoryVolume(repository.Path)
	if err != nil || strings.TrimSpace(identity) == "" {
		identity = "repository:" + repository.RepoID.String()
	}
	switch kind {
	case "local", "network", "removable", "unsupported":
	default:
		kind = "unknown"
	}
	return identity, kind
}

func relativeUserPath(repository repo.Repository, nativePath string) (string, error) {
	cleanedNativePath := filepath.Clean(nativePath)
	roots := nativeRepositoryRoots(repository.Path)
	for _, root := range roots {
		relative, err := filepath.Rel(root, cleanedNativePath)
		if err != nil {
			continue
		}
		relative = filepath.ToSlash(relative)
		if relative == "." {
			return "", nil
		}
		if relative == ".." || strings.HasPrefix(relative, "../") {
			continue
		}
		if !validUserRelativePath(relative) {
			return "", fmt.Errorf("native event path %q is not portable user media", nativePath)
		}
		return relative, nil
	}
	return "", fmt.Errorf("native event path %q is outside repository", nativePath)
}

func nativeRepositoryRoots(repositoryPath string) []string {
	roots := make([]string, 0, 3)
	appendRoot := func(candidate string) {
		candidate = filepath.Clean(candidate)
		for _, existing := range roots {
			if existing == candidate {
				return
			}
		}
		roots = append(roots, candidate)
	}
	appendRoot(repositoryPath)
	absolute, err := filepath.Abs(repositoryPath)
	if err == nil {
		appendRoot(absolute)
		if evaluated, evalErr := filepath.EvalSymlinks(absolute); evalErr == nil {
			appendRoot(evaluated)
		}
	}
	return roots
}

func validUserRelativePath(value string) bool {
	if value == "" || !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 ||
		strings.ContainsRune(value, '\\') || !fs.ValidPath(value) || path.Clean(value) != value || value == "." {
		return false
	}
	first, _, _ := strings.Cut(value, "/")
	switch first {
	case ".lumilio", ".lumiliorepo", ".lumiliorepo.lock", ".lumilioroot", ".lumilioroot.lock":
		return false
	}
	for _, prefix := range []string{
		".lumilio_permission_test-",
		".lumilio_case_probe-",
		".lumilio-write-test-",
	} {
		if strings.HasPrefix(first, prefix) {
			return false
		}
	}
	return true
}
