package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"desktop/internal/platform"
	"desktop/internal/state"
)

func TestShortcutCacheIsDiscardableAndOpenRevalidatesMarker(t *testing.T) {
	root := t.TempDir()
	paths, err := platform.NewPaths(filepath.Join(root, "app-data"))
	if err != nil {
		t.Fatalf("paths: %v", err)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatalf("ensure paths: %v", err)
	}
	location := filepath.Join(root, "photos")
	if err := os.Mkdir(location, 0o755); err != nil {
		t.Fatalf("mkdir location: %v", err)
	}
	if err := os.WriteFile(filepath.Join(location, ".lumilioroot"), []byte("marker"), 0o644); err != nil {
		t.Fatalf("marker: %v", err)
	}
	controller := NewController(Options{Paths: paths, Store: state.NewWithInstanceID("storage-test")})
	if err := controller.saveCache(cacheFile{
		SchemaVersion: cacheSchemaVersion,
		Version:       4,
		Items:         []dto.StorageShortcut{{ID: "root-1", Name: "Photos", Path: location, Status: "active", CanOpen: true}},
	}); err != nil {
		t.Fatalf("save cache: %v", err)
	}
	var opened string
	controller.SetOpenFileManager(func(path string, _ bool) error {
		opened = path
		return nil
	})
	items, err := controller.ListShortcuts(context.Background())
	if err != nil || len(items) != 1 {
		t.Fatalf("list shortcuts = %#v, err = %v", items, err)
	}
	if err := controller.OpenLocation(context.Background(), "root-1"); err != nil {
		t.Fatalf("open location: %v", err)
	}
	wantLocation, _ := filepath.EvalSymlinks(location)
	if opened != wantLocation {
		t.Fatalf("opened path = %q, want %q", opened, wantLocation)
	}
	if err := os.Remove(filepath.Join(location, ".lumilioroot")); err != nil {
		t.Fatalf("remove marker: %v", err)
	}
	if err := controller.OpenLocation(context.Background(), "root-1"); err == nil || operation.ErrorCodeOf(err) != dto.ErrorStorageLocationOffline {
		t.Fatalf("offline open error = %v", err)
	}
}

func TestCorruptShortcutCacheDoesNotBlockRead(t *testing.T) {
	root := t.TempDir()
	paths, err := platform.NewPaths(filepath.Join(root, "app-data"))
	if err != nil {
		t.Fatalf("paths: %v", err)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatalf("ensure paths: %v", err)
	}
	if err := os.WriteFile(paths.ShortcutsFile, []byte("not-json"), 0o600); err != nil {
		t.Fatalf("write corrupt cache: %v", err)
	}
	controller := NewController(Options{Paths: paths, Store: state.New()})
	items, err := controller.ListShortcuts(context.Background())
	if err != nil || len(items) != 0 {
		t.Fatalf("corrupt cache list = %#v, err = %v", items, err)
	}
	matches, err := filepath.Glob(paths.ShortcutsFile + ".corrupt.*")
	if err != nil || len(matches) != 1 {
		t.Fatalf("quarantine files = %#v, err = %v", matches, err)
	}
}

func TestPickLocationCanonicalizesBeforeReturning(t *testing.T) {
	root := t.TempDir()
	paths, err := platform.NewPaths(filepath.Join(root, "app-data"))
	if err != nil {
		t.Fatalf("paths: %v", err)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatalf("ensure paths: %v", err)
	}
	location := filepath.Join(root, "photos")
	if err := os.Mkdir(location, 0o755); err != nil {
		t.Fatalf("mkdir location: %v", err)
	}
	if err := os.WriteFile(filepath.Join(location, ".lumilioroot"), []byte("marker"), 0o644); err != nil {
		t.Fatalf("marker: %v", err)
	}
	controller := NewController(Options{
		Paths: paths,
		Store: state.NewWithInstanceID("storage-picker-test"),
		PickDirectory: func(string) (string, error) {
			return filepath.Join(location, "."), nil
		},
	})
	selected, err := controller.PickLocation("Choose photos")
	if err != nil {
		t.Fatalf("pick location: %v", err)
	}
	want, _ := filepath.EvalSymlinks(location)
	if selected != want {
		t.Fatalf("selected path = %q, want canonical %q", selected, want)
	}
}
