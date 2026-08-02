// Package update contains the platform-neutral trust and state policy for
// Desktop updates. Platform helpers may download or apply artifacts, but
// they must pass this verifier before quiescing the application.
package update

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const ManifestSchemaVersion = 1

type Manifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	Channel       string `json:"channel"`
	Version       string `json:"version"`
	URL           string `json:"url"`
	SHA256        string `json:"sha256"`
	Signature     string `json:"signature"`
}

func Parse(data []byte) (Manifest, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode update manifest: %w", err)
	}
	if manifest.SchemaVersion != ManifestSchemaVersion || strings.TrimSpace(manifest.Channel) == "" || strings.TrimSpace(manifest.Version) == "" || strings.TrimSpace(manifest.URL) == "" {
		return Manifest{}, fmt.Errorf("update manifest is incomplete or unsupported")
	}
	parsedURL, err := url.Parse(manifest.URL)
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" || parsedURL.User != nil {
		return Manifest{}, fmt.Errorf("update manifest URL must be an HTTPS origin")
	}
	if len(manifest.SHA256) != sha256.Size*2 {
		return Manifest{}, fmt.Errorf("update manifest has invalid SHA-256")
	}
	if _, err := hex.DecodeString(manifest.SHA256); err != nil {
		return Manifest{}, fmt.Errorf("update manifest has invalid SHA-256: %w", err)
	}
	return manifest, nil
}

func VerifyManifest(data []byte, publicKey ed25519.PublicKey) (Manifest, error) {
	manifest, err := Parse(data)
	if err != nil {
		return Manifest{}, err
	}
	signature, err := base64.StdEncoding.DecodeString(manifest.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return Manifest{}, fmt.Errorf("invalid update signature")
	}
	if len(publicKey) != ed25519.PublicKeySize || !ed25519.Verify(publicKey, signingPayload(manifest), signature) {
		return Manifest{}, fmt.Errorf("invalid update signature")
	}
	return manifest, nil
}

func VerifyArtifact(manifest Manifest, artifact []byte) error {
	hash := sha256.Sum256(artifact)
	if !strings.EqualFold(hex.EncodeToString(hash[:]), manifest.SHA256) {
		return fmt.Errorf("update artifact digest does not match manifest")
	}
	return nil
}

func signingPayload(manifest Manifest) []byte {
	copy := manifest
	copy.Signature = ""
	data, _ := json.Marshal(copy)
	return data
}

// SignForTest is intentionally small and useful for unit tests and release
// tooling tests; production signing keys never belong in the Desktop module.
func SignForTest(manifest Manifest, privateKey ed25519.PrivateKey) (Manifest, error) {
	manifest.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, signingPayload(manifest)))
	return manifest, nil
}
