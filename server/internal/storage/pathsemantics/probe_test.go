package pathsemantics

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/unicode/norm"
)

// observedSemantics measures a directory the slow way, with plain files the
// test owns, so the probe has something independent to agree with.
func observedSemantics(t *testing.T, dir string) Semantics {
	t.Helper()
	lower := filepath.Join(dir, "observed-a")
	if err := os.WriteFile(lower, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := Semantics{Case: CaseSensitive, Normalization: NormalizationNone}
	if _, err := os.Stat(filepath.Join(dir, "OBSERVED-A")); err == nil {
		result.Case = CaseInsensitive
	}
	decomposed := filepath.Join(dir, "observed-é")
	if err := os.WriteFile(decomposed, []byte("e"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, norm.NFC.String("observed-é"))); err == nil {
		result.Normalization = NormalizationNFC
	}
	return result
}

func TestProbeAgreesWithObservedBehaviorAndLeavesNoFile(t *testing.T) {
	dir := t.TempDir()
	measured, err := Probe(dir)
	if err != nil {
		t.Fatal(err)
	}
	if observed := observedSemantics(t, filepath.Join(dir)); !measured.Equal(observed) {
		t.Fatalf("probe measured %s, direct observation %s", measured, observed)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if len(entry.Name()) >= len(probePrefix) && entry.Name()[:len(probePrefix)] == probePrefix {
			t.Fatalf("probe left %s behind", entry.Name())
		}
	}
}

func TestQualifyRequiresEveryDirectoryToAgree(t *testing.T) {
	root := t.TempDir()
	private := filepath.Join(root, ".lumilio")
	if err := os.Mkdir(private, 0o755); err != nil {
		t.Fatal(err)
	}
	agreed, err := Qualify(root, private)
	if err != nil {
		t.Fatal(err)
	}
	measured, err := Probe(root)
	if err != nil {
		t.Fatal(err)
	}
	if !agreed.Equal(measured) {
		t.Fatalf("Qualify = %s, Probe = %s", agreed, measured)
	}
	if _, err := Qualify(); !errors.Is(err, ErrUnqualified) {
		t.Fatalf("Qualify() = %v, want ErrUnqualified", err)
	}
}

func TestProbeOfAnUnwritableDirectoryIsUnqualifiedNotSensitive(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	_, err := Probe(missing)
	if !errors.Is(err, ErrUnqualified) {
		t.Fatalf("Probe(absent) = %v, want ErrUnqualified", err)
	}
	if errors.Is(err, ErrMixed) {
		t.Fatalf("an unprobeable directory is not a mixed tree: %v", err)
	}
}
