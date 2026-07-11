package main

import (
	"strings"
	"testing"
)

func TestCanonicalSemver(t *testing.T) {
	cases := map[string]string{
		"1.0.0-beta.3": "v1.0.0-beta.3",
		"v1.0.0":       "v1.0.0",
		"v2.3.4":       "v2.3.4",
		"dev":          "",
		"":             "",
		"not-a-semver": "",
	}
	for in, want := range cases {
		if got := canonicalSemver(in); got != want {
			t.Errorf("canonicalSemver(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNewestUpdatePicksPlatformAsset(t *testing.T) {
	releases := []releaseItem{
		{
			TagName: "v1.0.0-beta.4",
			HTMLURL: "https://github.com/EdwinZhanCN/Lumilio-Photos/releases/tag/v1.0.0-beta.4",
			Assets: []releaseAsset{
				{Name: "Lumilio-Photos-v1.0.0-beta.4-macos-arm64.dmg", BrowserDownloadURL: "https://github.com/x/a.dmg"},
				{Name: "Lumilio-Photos-v1.0.0-beta.4-windows-amd64-setup.exe", BrowserDownloadURL: "https://github.com/x/a-setup.exe"},
				{Name: "Lumilio-Photos-v1.0.0-beta.4-windows-amd64.zip", BrowserDownloadURL: "https://github.com/x/a.zip"},
			},
		},
		{TagName: "v1.0.0-beta.3", HTMLURL: "u3"},
	}

	got, ok := newestUpdate("v1.0.0-beta.3", releases, "darwin", "arm64", "other")
	if !ok || got.Version != "v1.0.0-beta.4" || got.URL != "https://github.com/x/a.dmg" {
		t.Fatalf("darwin arm64: got %+v ok=%v", got, ok)
	}

	got, ok = newestUpdate("v1.0.0-beta.3", releases, "windows", "amd64", "other")
	if !ok || got.URL != "https://github.com/x/a-setup.exe" {
		t.Fatalf("windows prefers setup.exe: got %+v ok=%v", got, ok)
	}

	got, ok = newestUpdate("v1.0.0-beta.3", releases, "windows", "amd64", "cn")
	want := maybeMirrorGitHubURL("https://github.com/x/a-setup.exe", "cn")
	if !ok || got.URL != want {
		t.Fatalf("cn mirror: got %q want %q", got.URL, want)
	}
	if cnGitHubReleaseMirror != "" && !strings.HasPrefix(got.URL, strings.TrimRight(cnGitHubReleaseMirror, "/")) {
		t.Fatalf("expected mirrored URL, got %q", got.URL)
	}

	if _, ok := newestUpdate("v1.0.0-beta.4", releases, "darwin", "arm64", "other"); ok {
		t.Error("no update should be offered when already on the newest release")
	}
}

func TestNormalizeRegion(t *testing.T) {
	if normalizeRegion("cn") != "cn" || normalizeRegion("china") != "cn" {
		t.Fatal("cn aliases")
	}
	if normalizeRegion("") != "other" || normalizeRegion("us") != "other" {
		t.Fatal("other default")
	}
	if effectiveRegion("", "zh") != "cn" || effectiveRegion("other", "zh") != "other" {
		t.Fatal("effectiveRegion")
	}
}
