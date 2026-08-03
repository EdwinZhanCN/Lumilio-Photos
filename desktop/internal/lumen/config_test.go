package lumen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultSetupSelectionFollowsReleaseProfile(t *testing.T) {
	tests := []struct {
		profile  string
		platform string
		backend  string
	}{
		{profile: "darwin-arm64-metal", platform: "darwin-arm64", backend: "metal"},
		{profile: "darwin-arm64-cpu", platform: "darwin-arm64", backend: "cpu"},
		{profile: "windows-x64-gpu", platform: "windows-x64", backend: "gpu"},
		{profile: "windows-x64-cpu", platform: "windows-x64", backend: "cpu"},
	}
	for _, test := range tests {
		t.Run(test.profile, func(t *testing.T) {
			selection, err := DefaultSetupSelection("/private/config.yaml", "/private/models", "china", test.profile)
			if err != nil {
				t.Fatal(err)
			}
			if selection.Version != "0.1.0" || selection.Region != "cn" || selection.Preset.Name != "basic" {
				t.Fatalf("unexpected defaults: %+v", selection)
			}
			if selection.Platform.Name != test.platform || selection.Backend.Name != test.backend || selection.Backend.ReleaseProfile != test.profile {
				t.Fatalf("selection does not follow profile: %+v", selection)
			}
		})
	}
}

func TestEnsureSetupConfigUsesLauncherBasicPresetAndDesktopLoopback(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "lumen", "config.yaml")
	cache := filepath.Join(root, "models'cache")
	selection, err := DefaultSetupSelection(path, cache, "china", "darwin-arm64-metal")
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureSetupConfig(selection); err != nil {
		t.Fatalf("ensure config: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	config := string(data)
	for _, expected := range []string{
		"# Preset: basic (siglip, face, ocr, bioclip)",
		"# Resource guidance: RAM 6 GB, GPU/Unified memory 3 GB, disk 6 GB.",
		"region: cn", `host: "127.0.0.1"`, "port: 50051", "mdns:", "TreeOfLife200MCore", "models''cache",
	} {
		if !strings.Contains(config, expected) {
			t.Fatalf("config does not contain %q:\n%s", expected, config)
		}
	}
	if strings.Contains(config, "siglip2-so400m-patch14-384") {
		t.Fatalf("basic preset received brave SigLIP model:\n%s", config)
	}
	if err := os.WriteFile(path, []byte("user config\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSetupConfig(selection); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "user config\n" {
		t.Fatalf("existing config was overwritten: %q", data)
	}
}

func TestRenderSetupConfigFollowsPresetComponents(t *testing.T) {
	selection, err := NewSetupSelection("/private/config.yaml", "/private/models", "other", "windows-x64-gpu", "minimal")
	if err != nil {
		t.Fatal(err)
	}
	minimalConfig := renderSetupConfig(selection)
	for _, omitted := range []string{"  ocr:\n", "  bioclip:\n"} {
		if strings.Contains(minimalConfig, omitted) {
			t.Fatalf("minimal preset contains %q:\n%s", omitted, minimalConfig)
		}
	}
	if err := validateRenderedSetupConfig(minimalConfig, selection); err != nil {
		t.Fatalf("validate minimal config: %v", err)
	}

	brave, _ := setupPresetByName("brave")
	selection.Preset = brave
	braveConfig := renderSetupConfig(selection)
	for _, expected := range []string{"siglip2-so400m-patch14-384", "dataset: TreeOfLife200M\n"} {
		if !strings.Contains(braveConfig, expected) {
			t.Fatalf("brave preset does not contain %q:\n%s", expected, braveConfig)
		}
	}
	if strings.Contains(braveConfig, "dataset: TreeOfLife200MCore") {
		t.Fatalf("brave preset received Core dataset:\n%s", braveConfig)
	}
}

func TestSetupChoiceCatalogsMatchOfficialDesktopArtifacts(t *testing.T) {
	if got := strings.Join(ReleaseProfiles("darwin", "arm64"), ","); got != "darwin-arm64-metal,darwin-arm64-cpu" {
		t.Fatalf("darwin profiles = %q", got)
	}
	if got := strings.Join(ReleaseProfiles("windows", "amd64"), ","); got != "windows-x64-cpu,windows-x64-gpu" {
		t.Fatalf("windows profiles = %q", got)
	}
	if got := strings.Join(SetupPresetNames(), ","); got != "minimal,basic,brave" {
		t.Fatalf("presets = %q", got)
	}
	if profiles := ReleaseProfiles("linux", "amd64"); len(profiles) != 0 {
		t.Fatalf("unsupported platform profiles = %v", profiles)
	}
}

func TestSetupSelectionRejectsMismatchedBackend(t *testing.T) {
	selection, err := DefaultSetupSelection("/private/config.yaml", "/private/models", "other", "darwin-arm64-metal")
	if err != nil {
		t.Fatal(err)
	}
	selection.Backend.Name = "cpu"
	if err := EnsureSetupConfig(selection); err == nil || !strings.Contains(err.Error(), "does not match release profile") {
		t.Fatalf("expected backend mismatch, got %v", err)
	}
	selection, err = DefaultSetupSelection("/private/config.yaml", "/private/models", "other", "darwin-arm64-metal")
	if err != nil {
		t.Fatal(err)
	}
	selection.Version = "9.9.9"
	if err := EnsureSetupConfig(selection); err == nil || !strings.Contains(err.Error(), "does not match release") {
		t.Fatalf("expected release version mismatch, got %v", err)
	}
	if _, err := DefaultSetupSelection("/private/config.yaml", "/private/models", "other", "linux-x64-cpu"); err == nil {
		t.Fatal("unsupported release profile was accepted")
	}
}
