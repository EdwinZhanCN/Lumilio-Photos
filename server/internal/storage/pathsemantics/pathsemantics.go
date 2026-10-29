package pathsemantics

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/unicode/norm"
)

type CaseMode string

const (
	CaseSensitive   CaseMode = "sensitive"
	CaseInsensitive CaseMode = "insensitive"
)

type Normalization string

const (
	NormalizationNone    Normalization = "none"
	NormalizationNFC     Normalization = "nfc"
	NormalizationNFD     Normalization = "nfd"
	NormalizationUnknown Normalization = "unknown"
)

type Semantics struct {
	Case          CaseMode
	Normalization Normalization
}

func (s Semantics) Validate() error {
	if s.Case != CaseSensitive && s.Case != CaseInsensitive {
		return fmt.Errorf("unsupported path case mode %q", s.Case)
	}
	switch s.Normalization {
	case NormalizationNone, NormalizationNFC, NormalizationNFD, NormalizationUnknown:
		return nil
	default:
		return fmt.Errorf("unsupported path normalization %q", s.Normalization)
	}
}

// NameKey returns the persisted comparison key for one on-disk component.
// Unknown normalization canonicalizes to NFC so canonically equivalent input
// cannot create two logical children while an adapter is still being probed.
func (s Semantics) NameKey(name string) (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}
	if name == "" || strings.ContainsRune(name, '/') || strings.ContainsRune(name, '\x00') {
		return "", fmt.Errorf("invalid repository node name %q", name)
	}
	key := name
	switch s.Normalization {
	case NormalizationNFD:
		key = norm.NFD.String(key)
	case NormalizationNFC, NormalizationUnknown:
		key = norm.NFC.String(key)
	}
	if s.Case == CaseInsensitive {
		key = strings.ToLower(key)
	}
	return key, nil
}

// PathKey maps a '/'-separated repository-relative path to its comparison
// key, component by component. The empty path (the repository root) keys as
// the empty string.
func (s Semantics) PathKey(relative string) (string, error) {
	if relative == "" {
		return "", nil
	}
	components := strings.Split(relative, "/")
	for index, component := range components {
		key, err := s.NameKey(component)
		if err != nil {
			return "", err
		}
		components[index] = key
	}
	return strings.Join(components, "/"), nil
}

// ErrUnqualified reports that the name comparison rule of a directory could
// not be measured, or that directories of one tree disagree. A repository
// whose rule is unqualified is never registered or reconnected: the scan
// index would key it under a rule the volume does not follow.
var ErrUnqualified = errors.New("volume name rules could not be qualified")

// ErrMixed is the ErrUnqualified case where the directories of one tree
// follow different rules (a per-directory case-folding flag, for example),
// so no single rule can key the tree.
var ErrMixed = fmt.Errorf("%w: directories of the tree follow different rules", ErrUnqualified)

// Equal reports whether two rules key every name identically.
func (s Semantics) Equal(other Semantics) bool {
	return s.Case == other.Case && s.Normalization == other.Normalization
}

func (s Semantics) String() string {
	return fmt.Sprintf("case %s, normalization %s", s.Case, s.Normalization)
}

const probePrefix = ".lumilio-volume-probe-"

// Probe measures the name comparison rule of the filesystem that holds dir
// by creating two hidden temporary files in it and looking them up under a
// case-changed and a normalization-changed spelling. The files are removed
// before it returns. It is a write probe: callers run it only inside a
// mutation that already writes to the tree, never from a read-only
// assessment (R7).
//
// A lookup that fails for any reason other than "no such file" leaves the
// rule unqualified; a denied or unknown stat is never read as "sensitive".
func Probe(dir string) (Semantics, error) {
	sensitive, err := probeDistinct(dir, "-a", func(name string) string {
		return strings.ToUpper(name)
	})
	if err != nil {
		return Semantics{}, err
	}
	// e + U+0301 (combining acute) is the NFD spelling of U+00E9.
	preservesNormalization, err := probeDistinct(dir, "-é", func(name string) string {
		return norm.NFC.String(name)
	})
	if err != nil {
		return Semantics{}, err
	}
	result := Semantics{Case: CaseInsensitive, Normalization: NormalizationNFC}
	if sensitive {
		result.Case = CaseSensitive
	}
	if preservesNormalization {
		result.Normalization = NormalizationNone
	}
	return result, nil
}

// probeDistinct creates a temporary file whose name ends in suffix and
// reports whether the filesystem treats alternate(name) as a different name.
func probeDistinct(dir, suffix string, alternate func(string) string) (bool, error) {
	file, err := os.CreateTemp(dir, probePrefix+"*"+suffix)
	if err != nil {
		return false, fmt.Errorf("%w: create probe in %s: %v", ErrUnqualified, dir, err)
	}
	name := file.Name()
	defer os.Remove(name)
	if err := file.Close(); err != nil {
		return false, fmt.Errorf("%w: close probe %s: %v", ErrUnqualified, name, err)
	}
	created, err := os.Lstat(name)
	if err != nil {
		return false, fmt.Errorf("%w: stat probe %s: %v", ErrUnqualified, name, err)
	}
	other := filepath.Join(filepath.Dir(name), alternate(filepath.Base(name)))
	if other == name {
		return false, fmt.Errorf("%w: probe name %s has no alternate spelling", ErrUnqualified, name)
	}
	found, err := os.Lstat(other)
	switch {
	case err == nil && os.SameFile(created, found):
		return false, nil
	case err == nil:
		return false, fmt.Errorf("%w: %s exists beside the probe %s", ErrUnqualified, other, name)
	case errors.Is(err, fs.ErrNotExist):
		return true, nil
	default:
		return false, fmt.Errorf("%w: stat %s: %v", ErrUnqualified, other, err)
	}
}

// Qualify measures every directory and returns the one rule they share. A
// tree whose directories disagree is ErrMixed; a directory that cannot be
// probed is ErrUnqualified. At least one directory is required.
func Qualify(dirs ...string) (Semantics, error) {
	if len(dirs) == 0 {
		return Semantics{}, fmt.Errorf("%w: no directory to probe", ErrUnqualified)
	}
	var agreed Semantics
	for index, dir := range dirs {
		measured, err := Probe(dir)
		if err != nil {
			return Semantics{}, err
		}
		if index == 0 {
			agreed = measured
			continue
		}
		if !measured.Equal(agreed) {
			return Semantics{}, fmt.Errorf("%w: %s follows %s while %s follows %s", ErrMixed, dirs[0], agreed, dir, measured)
		}
	}
	return agreed, nil
}
