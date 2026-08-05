package main

import (
	"strings"
	"testing"
)

const validLock = `{
  "schemaVersion": 1,
  "repository": "https://github.com/example/assets.git",
  "revision": "0123456789abcdef0123456789abcdef01234567",
  "release": "assets-v1.2.3",
  "profile": "smoke",
  "manifestSha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
}`

func TestParseLockAcceptsCanonicalLock(t *testing.T) {
	parsed, err := parseLock([]byte(validLock))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Release != "assets-v1.2.3" || parsed.Profile != "smoke" {
		t.Fatalf("parsed lock = %+v", parsed)
	}
}

func TestParseLockRejectsUnknownFieldsAndMalformedIdentity(t *testing.T) {
	for name, raw := range map[string]string{
		"unknown field": strings.Replace(validLock, "\n}", ",\n  \"future\": true\n}", 1),
		"schema":        strings.Replace(validLock, `"schemaVersion": 1`, `"schemaVersion": 2`, 1),
		"repository":    strings.Replace(validLock, "https://github.com/example/assets.git", "git@example.com:assets.git", 1),
		"release":       strings.Replace(validLock, "assets-v1.2.3", "v1.2.3", 1),
		"revision":      strings.Replace(validLock, "0123456789abcdef0123456789abcdef01234567", "deadbeef", 1),
		"manifest":      strings.Replace(validLock, strings.Repeat("a", 64), strings.Repeat("A", 64), 1),
		"profile":       strings.Replace(validLock, `"smoke"`, `"Smoke Profile"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseLock([]byte(raw)); err == nil {
				t.Fatalf("parseLock accepted %s", raw)
			}
		})
	}
}

func TestParseReleaseArg(t *testing.T) {
	got, err := parseReleaseArg([]string{"--release", "assets-v2.0.1"})
	if err != nil || got != "assets-v2.0.1" {
		t.Fatalf("parseReleaseArg = %q, %v", got, err)
	}
	for _, args := range [][]string{{"--release"}, {"--release", "v2.0.1"}, {"--future"}} {
		if _, err := parseReleaseArg(args); err == nil {
			t.Fatalf("parseReleaseArg(%q) succeeded", args)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	for _, test := range []struct {
		left, right string
		want        int
	}{
		{"assets-v1.2.3", "assets-v1.2.3", 0},
		{"assets-v1.3.0", "assets-v1.2.9", 1},
		{"assets-v1.2.3", "assets-v2.0.0", -1},
	} {
		got, err := compareVersions(test.left, test.right)
		if err != nil || got != test.want {
			t.Fatalf("compareVersions(%q, %q) = %d, %v; want %d", test.left, test.right, got, err, test.want)
		}
	}
}
