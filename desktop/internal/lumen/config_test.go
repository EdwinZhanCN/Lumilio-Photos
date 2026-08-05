package lumen

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestSetupIntentOwnsOnlyMachineIntent(t *testing.T) {
	root := t.TempDir()
	intent, err := NewSetupIntent(
		filepath.Join(root, "private", "config.yaml"),
		filepath.Join(root, "models"),
		"china",
		"basic",
	)
	if err != nil {
		t.Fatal(err)
	}
	if intent.Region != "cn" || intent.Preset != "basic" {
		t.Fatalf("unexpected intent: %+v", intent)
	}
	if got := strings.Join(SetupPresetNames(), ","); got != "minimal,basic,brave" {
		t.Fatalf("preset catalog = %q", got)
	}
}

func TestConfigRenderArgsDelegateSemanticsToHub(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, "models")
	intent, err := NewSetupIntent(
		filepath.Join(root, "private", "config.yaml"),
		cacheDir,
		"other",
		"minimal",
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"config", "render",
		"--target", "desktop",
		"--preset", "minimal",
		"--region", "other",
		"--cache-dir", cacheDir,
	}
	if got := configRenderArgs(intent); !reflect.DeepEqual(got, want) {
		t.Fatalf("configRenderArgs() = %q, want %q", got, want)
	}
}

func TestReconcileSetupConfigRegeneratesDerivedFileEveryStart(t *testing.T) {
	root := t.TempDir()
	intent, err := NewSetupIntent(
		filepath.Join(root, "private", "config.yaml"),
		filepath.Join(root, "models"),
		"other",
		"basic",
	)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	render := func(_ context.Context, binary string, got SetupIntent) ([]byte, error) {
		calls++
		if binary != "/installed/lumen-hub" || got != intent {
			t.Fatalf("renderer received binary=%q intent=%+v", binary, got)
		}
		return []byte("server:\n  host: 127.0.0.1\n"), nil
	}
	if err := reconcileSetupConfig(context.Background(), "/installed/lumen-hub", intent, render); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(intent.ConfigPath, []byte("stale user mutation\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := reconcileSetupConfig(context.Background(), "/installed/lumen-hub", intent, render); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("renderer calls = %d, want 2", calls)
	}
	data, err := os.ReadFile(intent.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "server:\n  host: 127.0.0.1\n" {
		t.Fatalf("derived config was not replaced: %q", data)
	}
	info, err := os.Stat(intent.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("config permissions = %v, want no group/other access", info.Mode().Perm())
	}
}

func TestReconcileSetupConfigDoesNotCommitFailedOrEmptyRender(t *testing.T) {
	root := t.TempDir()
	intent, err := NewSetupIntent(filepath.Join(root, "private", "config.yaml"), filepath.Join(root, "models"), "other", "basic")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(intent.ConfigPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(intent.ConfigPath, []byte("known-good\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, render := range map[string]configRenderer{
		"error": func(context.Context, string, SetupIntent) ([]byte, error) { return nil, errors.New("boom") },
		"empty": func(context.Context, string, SetupIntent) ([]byte, error) { return []byte(" \n"), nil },
	} {
		t.Run(name, func(t *testing.T) {
			if err := reconcileSetupConfig(context.Background(), "/installed/lumen-hub", intent, render); err == nil {
				t.Fatal("invalid render was accepted")
			}
			data, readErr := os.ReadFile(intent.ConfigPath)
			if readErr != nil || string(data) != "known-good\n" {
				t.Fatalf("known-good config changed: %q, err=%v", data, readErr)
			}
		})
	}
}

func TestSetupIntentRejectsUnknownPresetAndUnsafePaths(t *testing.T) {
	if _, err := NewSetupIntent("/private/config.yaml", "/private/models", "other", "future"); err == nil {
		t.Fatal("unknown preset was accepted")
	}
	if _, err := NewSetupIntent("config.yaml", "/private/models", "other", "basic"); err == nil {
		t.Fatal("config path without private parent was accepted")
	}
	if _, err := NewSetupIntent("/private/config.yaml", "models", "other", "basic"); err == nil {
		t.Fatal("relative cache directory was accepted")
	}
}

func TestSetupChoiceCatalogsMatchOfficialDesktopArtifacts(t *testing.T) {
	if got := strings.Join(ReleaseProfiles("darwin", "arm64"), ","); got != "darwin-arm64-metal,darwin-arm64-cpu" {
		t.Fatalf("darwin profiles = %q", got)
	}
	if got := strings.Join(ReleaseProfiles("windows", "amd64"), ","); got != "windows-x64-cpu,windows-x64-gpu" {
		t.Fatalf("windows profiles = %q", got)
	}
	if profiles := ReleaseProfiles("linux", "amd64"); len(profiles) != 0 {
		t.Fatalf("unsupported platform profiles = %v", profiles)
	}
}
