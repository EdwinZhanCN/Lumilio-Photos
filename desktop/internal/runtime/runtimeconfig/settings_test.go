package runtimeconfig

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsLumenPresetDefaultsAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.v1.json")
	legacy := `{"schemaVersion":1,"locale":"en","region":"global","updateChannel":"stable","theme":"system","runtimeDesiredState":"stopped","lumenDesiredState":"disabled"}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := LoadSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if settings.LumenPreset != "basic" {
		t.Fatalf("legacy preset = %q, want basic", settings.LumenPreset)
	}
	settings.LumenPreset = "brave"
	settings.LumenCacheDir = filepath.Join(t.TempDir(), "lumen-cache")
	if err := SaveSettings(path, settings); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.LumenPreset != "brave" {
		t.Fatalf("persisted preset = %q, want brave", loaded.LumenPreset)
	}
	if loaded.LumenCacheDir != settings.LumenCacheDir {
		t.Fatalf("persisted cache dir = %q, want %q", loaded.LumenCacheDir, settings.LumenCacheDir)
	}
}

func TestSettingsRejectsUnsafeLumenCacheDirectory(t *testing.T) {
	for _, cacheDir := range []string{"relative/cache", string(filepath.Separator)} {
		settings := DefaultSettings()
		settings.LumenCacheDir = cacheDir
		err := SaveSettings(filepath.Join(t.TempDir(), "settings.v1.json"), settings)
		if err == nil || !strings.Contains(err.Error(), "invalid Lumen cache directory") {
			t.Fatalf("cache %q error = %v", cacheDir, err)
		}
	}
}

func TestSettingsRejectsUnknownLumenPreset(t *testing.T) {
	settings := DefaultSettings()
	settings.LumenPreset = "huge"
	err := SaveSettings(filepath.Join(t.TempDir(), "settings.v1.json"), settings)
	if err == nil || !strings.Contains(err.Error(), "unsupported Lumen preset") {
		t.Fatalf("error = %v", err)
	}
}

func TestLumenSetupStorePersistsPresetAndCacheTogether(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.v1.json")
	if err := SaveSettings(path, DefaultSettings()); err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(t.TempDir(), "models")
	if err := NewLumenSetupStore(path).SaveSetup(context.Background(), "minimal", cacheDir); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.LumenPreset != "minimal" || loaded.LumenCacheDir != cacheDir {
		t.Fatalf("persisted setup = %+v", loaded)
	}
}
