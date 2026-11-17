package testfixture

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"server/internal/storage/pathsemantics"
)

// VolumeFixturesEnv names the filesystem images a CI job mounted for the
// volume-semantics evidence: "kind=path;kind=path". The kinds are the rows
// of [ExpectedVolumeSemantics]. Tests that need a real foreign volume read
// it through [VolumeFixtures] and skip when it is unset, the way GPU suites
// skip without a GPU; the filesystem-image jobs always set it.
const VolumeFixturesEnv = "LUMILIO_VOLUME_FIXTURES"

// VolumeFixture is one mounted filesystem image.
type VolumeFixture struct {
	Kind string
	Path string
	// Expected is the rule the image is documented to follow; the probe
	// test proves it and the reconnect tests build on it.
	Expected pathsemantics.Semantics
}

// expectedVolumeSemantics is the qualification table: the rule each image
// kind follows, as measured by the Linux loop-mount and macOS hdiutil jobs.
// Extending support to another filesystem means adding its image to a job
// and its row here, never guessing.
var expectedVolumeSemantics = map[string]pathsemantics.Semantics{
	// Linux loop images.
	"vfat":     {Case: pathsemantics.CaseInsensitive, Normalization: pathsemantics.NormalizationNone},
	"exfat":    {Case: pathsemantics.CaseInsensitive, Normalization: pathsemantics.NormalizationNone},
	"ext4":     {Case: pathsemantics.CaseSensitive, Normalization: pathsemantics.NormalizationNone},
	"casefold": {Case: pathsemantics.CaseInsensitive, Normalization: pathsemantics.NormalizationNFC},
	// macOS hdiutil images. Unlike the Linux driver, macOS treats canonically
	// equivalent spellings on ExFAT as one name (the first image run measured
	// it), so the same on-disk format carries a different rule per host and
	// the rule is stored per repository, never derived from the format name.
	"apfs":           {Case: pathsemantics.CaseInsensitive, Normalization: pathsemantics.NormalizationNFC},
	"apfs-sensitive": {Case: pathsemantics.CaseSensitive, Normalization: pathsemantics.NormalizationNFC},
	"hfsplus":        {Case: pathsemantics.CaseInsensitive, Normalization: pathsemantics.NormalizationNFC},
	"exfat-darwin":   {Case: pathsemantics.CaseInsensitive, Normalization: pathsemantics.NormalizationNFC},
}

// ExpectedVolumeSemantics returns the documented rule of an image kind.
func ExpectedVolumeSemantics(kind string) (pathsemantics.Semantics, bool) {
	expected, ok := expectedVolumeSemantics[kind]
	return expected, ok
}

// VolumeFixtures parses the value of VolumeFixturesEnv, which the test reads
// from its own environment, and skips the test when it is empty. Every named
// path must exist and every kind must have a documented rule; a job that
// mounts an image this table does not know fails loudly.
func VolumeFixtures(t testing.TB, raw string) []VolumeFixture {
	t.Helper()
	raw = strings.TrimSpace(raw)
	if raw == "" {
		t.Skipf("%s is unset; the filesystem-image CI jobs provide the volumes this test needs", VolumeFixturesEnv)
	}
	fixtures, err := ParseVolumeFixtures(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		info, err := os.Stat(fixture.Path)
		if err != nil || !info.IsDir() {
			t.Fatalf("volume fixture %s at %s is not a directory: %v", fixture.Kind, fixture.Path, err)
		}
	}
	return fixtures
}

// ParseVolumeFixtures decodes the "kind=path;kind=path" form.
func ParseVolumeFixtures(raw string) ([]VolumeFixture, error) {
	var fixtures []VolumeFixture
	for _, item := range strings.Split(raw, ";") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		kind, path, ok := strings.Cut(item, "=")
		kind, path = strings.TrimSpace(kind), strings.TrimSpace(path)
		if !ok || kind == "" || path == "" {
			return nil, fmt.Errorf("%s entry %q is not kind=path", VolumeFixturesEnv, item)
		}
		expected, known := expectedVolumeSemantics[kind]
		if !known {
			return nil, fmt.Errorf("%s names an image kind %q without a documented rule", VolumeFixturesEnv, kind)
		}
		fixtures = append(fixtures, VolumeFixture{Kind: kind, Path: path, Expected: expected})
	}
	if len(fixtures) == 0 {
		return nil, fmt.Errorf("%s names no volume", VolumeFixturesEnv)
	}
	return fixtures, nil
}
