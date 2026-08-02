package lumen

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPackageInstallerVerifiesProbeAndPromotes(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	artifact := zipArtifact(t, "bin/lumen-hub", []byte("signed hub"))
	hash := sha256.Sum256(artifact)
	manifest := ArtifactManifest{
		SchemaVersion: InstallSchemaVersion,
		Version:       "1.2.3",
		Profile:       "darwin-arm64-metal",
		Binary:        "bin/lumen-hub",
		SHA256:        hex.EncodeToString(hash[:]),
	}
	unsigned, _ := json.Marshal(manifest)
	manifest.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, unsigned))
	manifestBytes, _ := json.Marshal(manifest)
	root := filepath.Join(t.TempDir(), "lumen")
	probed := false
	installer := PackageInstaller{
		Root: root, Key: publicKey,
		Fetch: func(context.Context, string) ([]byte, []byte, error) { return manifestBytes, artifact, nil },
		Probe: func(_ context.Context, binary, profile, token string) error {
			probed = binary != "" && profile == manifest.Profile && token != ""
			return nil
		},
	}
	version, err := installer.Install(context.Background(), manifest.Profile)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if version != manifest.Version || !probed {
		t.Fatalf("install result = %q, probed = %v", version, probed)
	}
	current, err := LoadCurrent(root)
	if err != nil {
		t.Fatalf("load current: %v", err)
	}
	if current.Version != manifest.Version || current.Profile != manifest.Profile {
		t.Fatalf("current pointer = %#v", current)
	}
	if _, err := os.Stat(filepath.Join(root, "install.json")); !os.IsNotExist(err) {
		t.Fatalf("install journal remains: %v", err)
	}
}

func TestArtifactManifestRejectsTraversal(t *testing.T) {
	manifest := []byte(`{"schemaVersion":1,"version":"1","profile":"test","binary":"../lumen-hub","sha256":"` + hex.EncodeToString(make([]byte, sha256.Size)) + `","signature":""}`)
	if _, err := VerifyArtifactManifest(manifest, make(ed25519.PublicKey, ed25519.PublicKeySize)); err == nil {
		t.Fatal("traversal binary path was accepted")
	}
}

func zipArtifact(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	header := &zip.FileHeader{Name: name, Method: zip.Store}
	header.SetMode(0o700)
	writer, err := archive.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
