package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteInfoPlistSeparatesProductAndBuildVersions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Info.plist")
	if err := writeInfoPlist(path, "26.1.0", "17"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "<key>CFBundleShortVersionString</key><string>26.1.0</string>") {
		t.Fatalf("marketing version missing from plist: %s", text)
	}
	if !strings.Contains(text, "<key>CFBundleVersion</key><string>17</string>") {
		t.Fatalf("build number missing from plist: %s", text)
	}
	if strings.Contains(text, "beta") {
		t.Fatalf("pre-release label leaked into numeric bundle metadata: %s", text)
	}
}

func TestWriteWindowsVersionInfoKeepsProductLabelOutOfNumericVersion(t *testing.T) {
	path, err := writeWindowsVersionInfo(t.TempDir(), "26.1.0", "26.1.0-beta.1")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Fixed struct {
			FileVersion string `json:"file_version"`
		} `json:"fixed"`
		Info map[string]map[string]string `json:"info"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Fixed.FileVersion != "26.1.0" {
		t.Fatalf("file version = %q, want 26.1.0", payload.Fixed.FileVersion)
	}
	if payload.Info["0000"]["ProductVersion"] != "26.1.0-beta.1" {
		t.Fatalf("product version = %q, want 26.1.0-beta.1", payload.Info["0000"]["ProductVersion"])
	}
}
