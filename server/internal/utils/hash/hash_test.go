package hash

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/zeebo/blake3"
)

func TestCalculateLayeredBLAKE3SmallFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "small.bin")
	content := []byte("authoritative content")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := CalculateLayeredBLAKE3(path)
	if err != nil {
		t.Fatal(err)
	}
	want := blake3.Sum256(content)
	wantHex := hex.EncodeToString(want[:])
	if result.ContentHash != wantHex {
		t.Fatalf("content hash = %q, want %q", result.ContentHash, wantHex)
	}
	if result.QuickFingerprint != nil || result.QuickFingerprintVersion != nil {
		t.Fatal("small files must not receive a quick fingerprint")
	}
}

func TestCalculateLayeredBLAKE3LargeFileKeepsSeparateFingerprint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.bin")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(QuickHashThreshold + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte("first"), 0); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte("last"), QuickHashThreshold-3); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	result, err := CalculateLayeredBLAKE3(path)
	if err != nil {
		t.Fatal(err)
	}
	if result.QuickFingerprint == nil || result.QuickFingerprintVersion == nil {
		t.Fatal("large files must receive a versioned quick fingerprint")
	}
	if *result.QuickFingerprintVersion != QuickFingerprintVersion {
		t.Fatalf("quick fingerprint version = %q", *result.QuickFingerprintVersion)
	}
	if result.ContentHash == *result.QuickFingerprint {
		t.Fatal("authoritative content hash must be distinct from the sampled fingerprint")
	}
}
