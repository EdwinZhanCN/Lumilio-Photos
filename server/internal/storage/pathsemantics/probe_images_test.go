package pathsemantics_test

import (
	"os"
	"path/filepath"
	"testing"

	"server/internal/storage/pathsemantics"
	"server/internal/storage/testfixture"
)

// The qualification table in testfixture is proven against the real images
// the filesystem-image CI jobs mount: a FAT32, exFAT, plain ext4 and
// case-folding ext4 loop image on Linux; APFS, case-sensitive APFS, HFS+ and
// ExFAT hdiutil images on macOS. Extending the supported volume list means
// adding an image and a row, never guessing a rule.
func TestVolumeImageProbeMatchesTheDocumentedRule(t *testing.T) {
	for _, fixture := range testfixture.VolumeFixtures(t, os.Getenv(testfixture.VolumeFixturesEnv)) {
		t.Run(fixture.Kind, func(t *testing.T) {
			dir, err := os.MkdirTemp(fixture.Path, "probe-*")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)
			measured, err := pathsemantics.Probe(dir)
			if err != nil {
				t.Fatalf("probe %s image: %v", fixture.Kind, err)
			}
			if !measured.Equal(fixture.Expected) {
				t.Fatalf("%s image measured %s, documented %s", fixture.Kind, measured, fixture.Expected)
			}
			// A repository tree on the image qualifies as a whole: the root
			// and its private directory agree.
			private := filepath.Join(dir, ".lumilio")
			if err := os.Mkdir(private, 0o755); err != nil {
				t.Fatal(err)
			}
			agreed, err := pathsemantics.Qualify(dir, private)
			if err != nil || !agreed.Equal(fixture.Expected) {
				t.Fatalf("qualify %s tree = %s err=%v", fixture.Kind, agreed, err)
			}
		})
	}
}
