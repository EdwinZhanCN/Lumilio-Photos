package pathsemantics

import "testing"

func TestNameKeyFollowsPersistedCaseAndNormalization(t *testing.T) {
	composed := "Caf\u00e9.JPG"
	decomposed := "Cafe\u0301.jpg"

	insensitive := Semantics{Case: CaseInsensitive, Normalization: NormalizationNFC}
	first, err := insensitive.NameKey(composed)
	if err != nil {
		t.Fatal(err)
	}
	second, err := insensitive.NameKey(decomposed)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("insensitive NFC keys differ: %q and %q", first, second)
	}

	sensitive := Semantics{Case: CaseSensitive, Normalization: NormalizationNFC}
	first, err = sensitive.NameKey(composed)
	if err != nil {
		t.Fatal(err)
	}
	second, err = sensitive.NameKey(decomposed)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("case-sensitive keys unexpectedly match: %q", first)
	}
}

func TestUnknownNormalizationFailsSafeToCanonicalNFC(t *testing.T) {
	semantics := Semantics{Case: CaseSensitive, Normalization: NormalizationUnknown}
	composed, err := semantics.NameKey("\u00e9")
	if err != nil {
		t.Fatal(err)
	}
	decomposed, err := semantics.NameKey("e\u0301")
	if err != nil {
		t.Fatal(err)
	}
	if composed != decomposed {
		t.Fatalf("canonically equivalent unknown-normalization keys differ: %q and %q", composed, decomposed)
	}
}
