package main

import "testing"

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

func TestNewestUpdate(t *testing.T) {
	releases := []releaseItem{
		{TagName: "v1.0.0-beta.2", HTMLURL: "u2"},
		{TagName: "v1.0.0-beta.4", HTMLURL: "u4"},
		{TagName: "v1.0.0-beta.3", HTMLURL: "u3"},
		{TagName: "v2.0.0", HTMLURL: "draft", Draft: true},
	}

	// A newer prerelease is offered (and drafts are ignored even though v2.0.0
	// would otherwise be highest).
	got, ok := newestUpdate("v1.0.0-beta.3", releases)
	if !ok || got.Version != "v1.0.0-beta.4" || got.URL != "u4" {
		t.Fatalf("newestUpdate = %+v, ok=%v; want v1.0.0-beta.4/u4", got, ok)
	}

	// On the newest release, nothing is offered.
	if _, ok := newestUpdate("v1.0.0-beta.4", releases); ok {
		t.Error("no update should be offered when already on the newest release")
	}

	// A stable release outranks a prerelease of the same version.
	stable := []releaseItem{{TagName: "v1.0.0", HTMLURL: "s"}}
	got, ok = newestUpdate("v1.0.0-beta.4", stable)
	if !ok || got.Version != "v1.0.0" {
		t.Fatalf("stable over prerelease: got %+v ok=%v", got, ok)
	}

	// Invalid current version (e.g. "dev") is handled by checkForUpdate, but
	// newestUpdate with an empty cur should still not panic and offers the max.
	if _, ok := newestUpdate("", nil); ok {
		t.Error("empty release list should offer nothing")
	}
}
