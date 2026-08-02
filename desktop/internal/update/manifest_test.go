package update

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestSignedManifestAndArtifactVerification(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("desktop update")
	hash := sha256.Sum256(artifact)
	manifest, err := SignForTest(Manifest{
		SchemaVersion: ManifestSchemaVersion, Channel: "stable", Version: "1.2.3",
		URL: "https://updates.example.test/lumilio.zip", SHA256: hex.EncodeToString(hash[:]),
	}, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := VerifyManifest(data, publicKey)
	if err != nil {
		t.Fatalf("verify manifest: %v", err)
	}
	if err := VerifyArtifact(verified, artifact); err != nil {
		t.Fatalf("verify artifact: %v", err)
	}
	artifact[0] = 'X'
	if err := VerifyArtifact(verified, artifact); err == nil {
		t.Fatal("tampered artifact was accepted")
	}
}

func TestSignedManifestRejectsWrongKeyAndUnknownFields(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	otherPublicKey, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	manifest, _ := SignForTest(Manifest{SchemaVersion: ManifestSchemaVersion, Channel: "stable", Version: "1.0.0", URL: "https://example.test/app", SHA256: "0000000000000000000000000000000000000000000000000000000000000000"}, privateKey)
	data, _ := json.Marshal(manifest)
	if _, err := VerifyManifest(data, otherPublicKey); err == nil {
		t.Fatal("manifest verified with the wrong key")
	}
	if _, err := Parse([]byte(`{"schemaVersion":1,"channel":"stable","version":"1.0.0","url":"https://example.test/app","sha256":"0000000000000000000000000000000000000000000000000000000000000000","unexpected":true}`)); err == nil {
		t.Fatal("unknown manifest field was accepted")
	}
}
