package layout

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
	ErrPathInvalid   = errors.New("invalid repository-relative path")
	ErrPathNamespace = errors.New("repository path is outside the requested namespace")
)

type namespace uint8

const (
	namespaceMedia namespace = iota + 1
	namespacePrivate
)

// Path is a validated, canonical, slash-separated path relative to one
// Repository root, in one of two namespaces: the user's media tree, or the
// private tree under .lumilio/. A media Path comes from [ParseMedia]; a
// private Path comes from this package's constructors, or from [Parse] for
// a private path read back from storage. The zero Path is invalid.
type Path struct {
	value string
	space namespace
}

func (p Path) String() string { return p.value }

// IsZero reports whether p is the zero Path.
func (p Path) IsZero() bool { return p.value == "" }

// IsMedia reports whether p is in the user's media tree.
func (p Path) IsMedia() bool { return p.space == namespaceMedia }

// IsPrivate reports whether p is under .lumilio/.
func (p Path) IsPrivate() bool { return p.space == namespacePrivate }

// Local returns p in the host's separator form, for os.Root operations.
func (p Path) Local() (string, error) {
	if p.value == "" || (p.space != namespaceMedia && p.space != namespacePrivate) {
		return "", fmt.Errorf("%w: zero path", ErrPathInvalid)
	}
	localized, err := filepath.Localize(p.value)
	if err != nil {
		return "", fmt.Errorf("%w: %q: %v", ErrPathInvalid, p.value, err)
	}
	return localized, nil
}

// Dir returns the parent of a private Path, which is private too. The
// parent of the private root itself is not a Path.
func (p Path) Dir() (Path, error) {
	if !p.IsPrivate() || p.value == PrivateRoot {
		return Path{}, fmt.Errorf("%w: %q has no private parent", ErrPathNamespace, p.value)
	}
	return private(path.Dir(p.value))
}

// ParseMedia validates a path in the user's media tree. inbox is an
// ordinary user directory; the names [IsReserved] reports are not media.
func ParseMedia(value string) (Path, error) {
	parsed, err := parse(value)
	if err != nil {
		return Path{}, err
	}
	if IsReserved(parsed) {
		return Path{}, fmt.Errorf("%w: %q is application-owned", ErrPathNamespace, value)
	}
	return Path{value: parsed, space: namespaceMedia}, nil
}

// Parse validates a private path that was stored as text: a recovery
// receipt item, or a path a catalog row still holds. New paths come from
// the constructors in paths.go.
func Parse(value string) (Path, error) {
	parsed, err := parse(value)
	if err != nil {
		return Path{}, err
	}
	return private(parsed)
}

// private wraps an already canonical path under .lumilio/.
func private(value string) (Path, error) {
	if value != PrivateRoot && !strings.HasPrefix(value, PrivateRoot+"/") {
		return Path{}, fmt.Errorf("%w: %q is not under %s", ErrPathNamespace, value, PrivateRoot)
	}
	return Path{value: value, space: namespacePrivate}, nil
}

func parse(value string) (string, error) {
	if value == "" || !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 {
		return "", fmt.Errorf("%w: empty, invalid UTF-8, or NUL-containing path", ErrPathInvalid)
	}
	if strings.ContainsRune(value, '\\') || IsRooted(value) {
		return "", fmt.Errorf("%w: rooted or non-canonical path %q", ErrPathInvalid, value)
	}
	if !fs.ValidPath(value) || path.Clean(value) != value || value == "." {
		return "", fmt.Errorf("%w: %q", ErrPathInvalid, value)
	}
	for _, component := range strings.Split(value, "/") {
		if isPortableReservedComponent(component) {
			return "", fmt.Errorf("%w: reserved component %q", ErrPathInvalid, component)
		}
	}
	return value, nil
}

// segment reports whether value is one valid path component.
func segment(value string) bool {
	return value != "" && value != "." && value != ".." && !strings.ContainsAny(value, `/\`) && !isPortableReservedComponent(value)
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

// IsRooted reports whether path is rooted on any supported platform.
//
// filepath.IsAbs is not enough on Windows: it reports false for a
// rooted-but-driveless path like "/etc/passwd" or "\Windows", because Go
// requires a volume for a path to be absolute there. Every containment check
// written as `if filepath.IsAbs(p) { reject }` therefore lets those through on
// Windows, which is exactly the class of path such a check exists to stop.
//
// The rules are applied uniformly rather than switching on runtime.GOOS, so the
// behaviour is identical everywhere and can be tested on any platform. The cost
// is that a relative file literally named `\foo` is rejected on Unix, where that
// is a legal name. Failing closed on a pathological filename is the right bias
// for a containment check.
func IsRooted(path string) bool {
	if path == "" {
		return false
	}
	if filepath.IsAbs(path) {
		return true
	}
	// "/foo" and "\foo": rooted on Windows, which IsAbs does not report.
	// "\\server\share" (UNC) is covered by the same leading separator.
	if path[0] == '/' || path[0] == '\\' {
		return true
	}
	// "C:\foo" and "C:foo" (drive-relative). Detected explicitly rather than via
	// filepath.VolumeName, which returns "" off Windows and would make this
	// answer differ by platform, and so be untestable on the Linux and macOS
	// runners that execute these tests.
	return len(path) >= 2 && path[1] == ':' && isASCIILetter(path[0])
}

func isASCIILetter(c byte) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}
