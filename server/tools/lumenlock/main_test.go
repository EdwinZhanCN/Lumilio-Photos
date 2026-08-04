package main

import "testing"

func TestParseManifestAcceptsSchema2(t *testing.T) {
	raw := []byte(`{
		"schemaVersion": 2,
		"version": "v0.1.2",
		"capabilities": [{"id": "siglip", "zhCn": "图像语义分析", "en": "Image Semantic Analysis"}],
		"presets": [{"id": "basic", "capabilities": ["siglip"], "siglipModel": "siglip2-base-patch16-224", "bioclipDataset": null, "resources": {"ramGb": 6, "vramGb": 3, "diskGb": 6}}],
		"models": {"siglip": ["siglip2-base-patch16-224"]},
		"datasets": [],
		"platforms": [{"platform": "darwin-arm64", "profile": "darwin-arm64-metal", "backend": "metal", "target": "aarch64-apple-darwin"}],
		"protocol": {"dataPlaneMajor": 1, "mlService": {"path": "x", "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, "control": {"path": "y", "sha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}},
		"hub": [{"profile": "darwin-arm64-metal", "file_name": "lumen-hub-darwin-arm64-metal.zip", "url": "https://example.com/v0.1.2/lumen-hub-darwin-arm64-metal.zip", "sha256": "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}]
	}`)
	parsed, err := parseManifest(raw)
	if err != nil {
		t.Fatalf("parseManifest(schemaVersion 2): %v", err)
	}
	if parsed.SchemaVersion == nil || *parsed.SchemaVersion != 2 {
		t.Fatalf("schemaVersion = %v", parsed.SchemaVersion)
	}
	if parsed.Version != "v0.1.2" || len(parsed.Hub) != 1 || len(parsed.Presets) != 1 {
		t.Fatalf("parsed fields incomplete: %+v", parsed)
	}
}

func TestParseManifestAcceptsLegacyV011(t *testing.T) {
	raw := []byte(`{"version":"v0.1.1","hub":[{"profile":"darwin-arm64-metal","file_name":"lumen-hub-darwin-arm64-metal.zip","url":"https://example.com/v0.1.1/lumen-hub-darwin-arm64-metal.zip","sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}]}`)
	parsed, err := parseManifest(raw)
	if err != nil {
		t.Fatalf("parseManifest(legacy): %v", err)
	}
	if parsed.SchemaVersion != nil {
		t.Fatalf("legacy schemaVersion = %v, want nil", *parsed.SchemaVersion)
	}
	if parsed.Version != "v0.1.1" || len(parsed.Hub) != 1 {
		t.Fatalf("parsed fields incomplete: %+v", parsed)
	}
}

func TestParseManifestRejectsNewerSchema(t *testing.T) {
	raw := []byte(`{"schemaVersion": 3, "version": "v0.2.0", "hub": []}`)
	if _, err := parseManifest(raw); err == nil {
		t.Fatal("schemaVersion 3 was accepted")
	}
}

func TestParseLockRejectsInvalidRelease(t *testing.T) {
	_, err := parseLock([]byte(`{
		"schemaVersion": 1,
		"repository": "https://github.com/EdwinZhanCN/Lumen-Hub.git",
		"release": "v0.1",
		"revision": "cd67719660056244c405835d07786110fc7c1223",
		"manifestSha256": "4b8a42ca5137bc631b6d60cdca13ea08d0e2b62a035e74c5202d79d6e2842c56"
	}`))
	if err == nil {
		t.Fatal("invalid release was accepted")
	}
}

func TestControlProtoRawURL(t *testing.T) {
	url, err := controlProtoRawURL("https://github.com/EdwinZhanCN/Lumen-Hub.git", "cd67719660056244c405835d07786110fc7c1223")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://raw.githubusercontent.com/EdwinZhanCN/Lumen-Hub/cd67719660056244c405835d07786110fc7c1223/crates/lumen-hub/proto/control.proto"
	if url != want {
		t.Fatalf("controlProtoRawURL() = %q, want %q", url, want)
	}
	if _, err := controlProtoRawURL("not-a-repo", "abc"); err == nil {
		t.Fatal("invalid repository was accepted")
	}
}

func TestBytesEqual(t *testing.T) {
	if !bytesEqual([]byte("same"), []byte("same")) {
		t.Fatal("identical bytes reported different")
	}
	if bytesEqual([]byte("same"), []byte("drift")) {
		t.Fatal("different bytes reported identical")
	}
	if bytesEqual(nil, []byte("x")) {
		t.Fatal("nil vs non-nil reported identical")
	}
}
