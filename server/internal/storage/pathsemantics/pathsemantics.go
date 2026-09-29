// Package pathsemantics turns filesystem-specific name comparison rules into
// durable repository name keys. It owns one path component at a time; the
// scan index joins components into path keys.
package pathsemantics

import (
	"fmt"
	"runtime"
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

// HostDefault is the comparison rule of the host's default filesystem:
// case-insensitive on macOS and Windows, case-sensitive elsewhere, NFC on all.
// It is not a probe of any particular volume.
func HostDefault() Semantics {
	switch runtime.GOOS {
	case "windows", "darwin":
		return Semantics{Case: CaseInsensitive, Normalization: NormalizationNFC}
	default:
		return Semantics{Case: CaseSensitive, Normalization: NormalizationNFC}
	}
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
