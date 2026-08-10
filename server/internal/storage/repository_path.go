package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

var (
	ErrRepositoryPathInvalid   = errors.New("invalid repository-relative path")
	ErrRepositoryPathNamespace = errors.New("repository path is outside the requested namespace")
)

type repositoryNamespace uint8

const (
	namespaceUserMedia repositoryNamespace = iota + 1
	namespacePrivate
)

// RepositoryPath is a validated, canonical, slash-separated path relative to
// one repository root. Values can only be constructed by the parsers below.
type RepositoryPath struct {
	value     string
	namespace repositoryNamespace
}

func (p RepositoryPath) String() string { return p.value }

func (p RepositoryPath) local() (string, error) {
	if p.value == "" || (p.namespace != namespaceUserMedia && p.namespace != namespacePrivate) {
		return "", fmt.Errorf("%w: zero path", ErrRepositoryPathInvalid)
	}
	localized, err := filepath.Localize(p.value)
	if err != nil {
		return "", fmt.Errorf("%w: %q: %v", ErrRepositoryPathInvalid, p.value, err)
	}
	return localized, nil
}

func (p RepositoryPath) isUserMedia() bool { return p.namespace == namespaceUserMedia }
func (p RepositoryPath) isPrivate() bool   { return p.namespace == namespacePrivate }

// ParseUserMediaPath validates a path in the user-controlled repository tree.
// inbox is an ordinary user directory; only application markers and .lumilio
// are excluded.
func ParseUserMediaPath(value string) (RepositoryPath, error) {
	parsed, err := parseRepositoryPath(value)
	if err != nil {
		return RepositoryPath{}, err
	}
	first, _, _ := strings.Cut(parsed, "/")
	if first == ".lumilio" || first == ".lumiliorepo" || first == ".lumilioroot" {
		return RepositoryPath{}, fmt.Errorf("%w: %q is application-owned", ErrRepositoryPathNamespace, value)
	}
	return RepositoryPath{value: parsed, namespace: namespaceUserMedia}, nil
}

// ParsePrivateRepositoryPath validates a path owned by Lumilio under
// .lumilio/. Ordinary media consumers must not receive this capability.
func ParsePrivateRepositoryPath(value string) (RepositoryPath, error) {
	parsed, err := parseRepositoryPath(value)
	if err != nil {
		return RepositoryPath{}, err
	}
	if parsed != ".lumilio" && !strings.HasPrefix(parsed, ".lumilio/") {
		return RepositoryPath{}, fmt.Errorf("%w: %q is not under .lumilio", ErrRepositoryPathNamespace, value)
	}
	return RepositoryPath{value: parsed, namespace: namespacePrivate}, nil
}

func parseRepositoryPath(value string) (string, error) {
	if value == "" || !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 {
		return "", fmt.Errorf("%w: empty, invalid UTF-8, or NUL-containing path", ErrRepositoryPathInvalid)
	}
	if strings.ContainsRune(value, '\\') || IsRootedPath(value) {
		return "", fmt.Errorf("%w: rooted or non-canonical path %q", ErrRepositoryPathInvalid, value)
	}
	if !fs.ValidPath(value) || path.Clean(value) != value || value == "." {
		return "", fmt.Errorf("%w: %q", ErrRepositoryPathInvalid, value)
	}
	for _, component := range strings.Split(value, "/") {
		if isPortableReservedComponent(component) {
			return "", fmt.Errorf("%w: reserved component %q", ErrRepositoryPathInvalid, component)
		}
	}
	return value, nil
}

// isPortableReservedComponent applies Windows' filename restrictions on every
// platform so a repository-relative path has one meaning when media moves
// between supported hosts.
func isPortableReservedComponent(component string) bool {
	if component == "" || strings.HasSuffix(component, " ") || strings.HasSuffix(component, ".") {
		return true
	}
	for _, r := range component {
		if r < 0x20 || strings.ContainsRune(`<>:"|?*`, r) {
			return true
		}
	}
	base := component
	if before, _, ok := strings.Cut(component, "."); ok {
		base = before
	}
	base = strings.ToUpper(base)
	switch base {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
		return true
	}
	return false
}
