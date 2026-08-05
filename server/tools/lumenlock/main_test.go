package main

import (
	"strings"
	"testing"
)

func schema2Manifest() []byte {
	return []byte(`{
		"schemaVersion": 2,
		"version": "v0.1.2",
		"presets": [
			{"id": "minimal"},
			{"id": "basic"},
			{"id": "brave"}
		],
		"hub": [{
			"profile": "darwin-arm64-metal",
			"file_name": "lumen-hub-darwin-arm64-metal.zip",
			"url": "https://example.com/v0.1.2/lumen-hub-darwin-arm64-metal.zip",
			"sha256": "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
		}]
	}`)
}

func TestParseManifestRequiresManagedConfigSchema(t *testing.T) {
	parsed, err := parseManifest(schema2Manifest())
	if err != nil {
		t.Fatalf("parseManifest(schemaVersion 2): %v", err)
	}
	if parsed.SchemaVersion == nil || *parsed.SchemaVersion != 2 || len(parsed.Presets) != 3 {
		t.Fatalf("parsed fields incomplete: %+v", parsed)
	}

	legacy := []byte(`{"version":"v0.1.1","hub":[{"profile":"darwin-arm64-metal","file_name":"x.zip","url":"https://example.com/x.zip","sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}]}`)
	if _, err := parseManifest(legacy); err == nil || !strings.Contains(err.Error(), "managed config") {
		t.Fatalf("legacy manifest was not rejected with a migration error: %v", err)
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
		"schemaVersion": 2,
		"repository": "https://github.com/EdwinZhanCN/Lumen-Hub.git",
		"release": "v0.1",
		"revision": "cd67719660056244c405835d07786110fc7c1223",
		"manifestSha256": "4b8a42ca5137bc631b6d60cdca13ea08d0e2b62a035e74c5202d79d6e2842c56",
		"catalogSha256": "4b8a42ca5137bc631b6d60cdca13ea08d0e2b62a035e74c5202d79d6e2842c56",
		"controlProtoSha256": "4b8a42ca5137bc631b6d60cdca13ea08d0e2b62a035e74c5202d79d6e2842c56"
	}`))
	if err == nil {
		t.Fatal("invalid release was accepted")
	}
}

func TestManifestChecksumMustBePresentAndMatch(t *testing.T) {
	hash := strings.Repeat("a", 64)
	if err := verifyManifestChecksum(map[string]string{}, hash); err == nil || !strings.Contains(err.Error(), "missing manifest.json") {
		t.Fatalf("missing manifest entry was accepted: %v", err)
	}
	if err := verifyManifestChecksum(map[string]string{"manifest.json": strings.Repeat("b", 64)}, hash); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched manifest entry was accepted: %v", err)
	}
	if err := verifyManifestChecksum(map[string]string{"manifest.json": hash}, hash); err != nil {
		t.Fatalf("matching manifest entry was rejected: %v", err)
	}
}

func TestManifestPresetNamesRejectsMissingAndDuplicates(t *testing.T) {
	parsed, err := parseManifest(schema2Manifest())
	if err != nil {
		t.Fatal(err)
	}
	presets, err := manifestPresetNames(parsed)
	if err != nil || strings.Join(presets, ",") != "minimal,basic,brave" {
		t.Fatalf("presets = %v, err = %v", presets, err)
	}
	parsed.Presets = append(parsed.Presets, parsed.Presets[0])
	if _, err := manifestPresetNames(parsed); err == nil {
		t.Fatal("duplicate preset was accepted")
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

func TestGenerateCatalogIncludesCanonicalPresetAllowList(t *testing.T) {
	artifacts := map[string]artifact{}
	for _, profile := range desktopProfiles {
		artifacts[profile] = artifact{profile: profile, fileName: profile + ".zip", sha256: strings.Repeat("a", 64)}
	}
	catalog, err := generateCatalog("v0.2.0", desktopProfiles, artifacts, []string{"minimal", "basic", "brave"})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`const OfficialReleaseVersion = "v0.2.0"`, `var OfficialSetupPresets`, "\t\"basic\","} {
		if !strings.Contains(catalog, expected) {
			t.Fatalf("catalog missing %q:\n%s", expected, catalog)
		}
	}
}

func TestBytesEqual(t *testing.T) {
	if !bytesEqual([]byte("same"), []byte("same")) {
		t.Fatal("identical bytes reported different")
	}
	if bytesEqual([]byte("same"), []byte("drift")) {
		t.Fatal("different bytes reported identical")
	}
}

func TestParseReleaseArg(t *testing.T) {
	got, err := parseReleaseArg([]string{"--release", "v0.2.0"})
	if err != nil || got != "v0.2.0" {
		t.Fatalf("parseReleaseArg = %q, %v", got, err)
	}
	for _, args := range [][]string{{"--release"}, {"--release", "0.2.0"}, {"--future", "v0.2.0"}} {
		if _, err := parseReleaseArg(args); err == nil {
			t.Fatalf("parseReleaseArg(%q) succeeded", args)
		}
	}
}
