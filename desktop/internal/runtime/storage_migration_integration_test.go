package runtime_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"server/config"

	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"desktop/internal/platform"
	desktopruntime "desktop/internal/runtime"
	"desktop/internal/runtime/runtimeconfig"
	"desktop/internal/state"

	_ "github.com/mattn/go-sqlite3"
)

type embeddedRuntimeHarness struct {
	t             *testing.T
	paths         platform.Paths
	config        *runtimeconfig.Store
	state         *state.Store
	operations    *operation.Registry
	runtime       *desktopruntime.Controller
	oldPath       string
	oldConfigPath string
	oldPort       string
}

func newEmbeddedRuntimeHarness(t *testing.T) *embeddedRuntimeHarness {
	t.Helper()
	paths, err := platform.NewPaths(filepath.Join(t.TempDir(), "app-data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	installMediaToolFixtures(t)

	configStore := runtimeconfig.NewStore(paths)
	draft, err := configStore.ReadDraft()
	if err != nil {
		t.Fatalf("read initial config draft: %v", err)
	}
	oldPath := filepath.Join(t.TempDir(), "default-storage")
	settings := draft.Settings
	settings.NetworkMode = "custom"
	// The manifest contract requires a non-zero port, but this storage-focused
	// harness must not reserve and release a process-global port before startup.
	// The test factory below replaces this persisted placeholder with :0 only
	// for the embedded generation it starts.
	settings.Listen = "127.0.0.1:1"
	settings.StoragePath = oldPath
	settings.HardwareAcceleration = "none"
	draft, err = configStore.PatchDraft(draft.TOML, settings)
	if err != nil {
		t.Fatalf("patch initial config draft: %v", err)
	}
	validation, err := configStore.Validate(filepath.Join(paths.RuntimeIntents, "initial.toml"), []byte(draft.TOML))
	if err != nil {
		t.Fatalf("validate initial config: %v", err)
	}
	if err := configStore.WriteIntent(validation); err != nil {
		t.Fatal(err)
	}
	if err := configStore.WritePointer(paths.RuntimeCurrent, validation.Fingerprint); err != nil {
		t.Fatal(err)
	}
	if err := configStore.WritePointer(paths.RuntimeLKG, validation.Fingerprint); err != nil {
		t.Fatal(err)
	}

	snapshotStore := state.NewWithInstanceID("embedded-storage-migration")
	operations := operation.New()
	loadConfig := func() (config.AppConfig, error) {
		cfg, loadErr := configStore.LoadCurrentConfig()
		if loadErr == nil {
			cfg.ServerConfig.Listen = "127.0.0.1:0"
		}
		return cfg, loadErr
	}
	controller := desktopruntime.NewController(desktopruntime.Options{
		Store: snapshotStore, Operations: operations,
		Desired:    desktopruntime.NewMemoryDesiredState(dto.DesiredStopped),
		Factory:    desktopruntime.ServerFactory{Load: loadConfig},
		Configured: true, OnReady: configStore.PromoteCurrentToLastKnownGood,
		ReadyBudget: 20 * time.Second, StopBudget: 20 * time.Second,
	})
	ctx, cancel := context.WithCancel(context.Background())
	controller.StartActor(ctx)
	t.Cleanup(func() {
		if snapshot := controller.RuntimeSnapshot(); snapshot.Ownership == dto.OwnershipHeld {
			_, _ = controller.Quiesce("test-cleanup", snapshot.Version)
			waitEmbeddedRuntime(t, controller, func(runtime dto.RuntimeSnapshot) bool {
				return runtime.Ownership == dto.OwnershipNone
			})
		}
		cancel()
		controller.Close()
		snapshotStore.Close()
	})

	harness := &embeddedRuntimeHarness{
		t: t, paths: paths, config: configStore, state: snapshotStore,
		operations: operations, runtime: controller, oldPath: validation.Config.StorageConfig.Path,
		oldConfigPath: validation.Config.StorageConfig.Path, oldPort: settings.Listen,
	}
	harness.start("initial-start")
	harness.stop("seed-stop")
	harness.seedPrimaryRepository()
	return harness
}

func (h *embeddedRuntimeHarness) start(requestID string) dto.RuntimeSnapshot {
	h.t.Helper()
	if _, err := h.runtime.Start(requestID, h.runtime.RuntimeSnapshot().Version); err != nil {
		h.t.Fatalf("start embedded runtime: %v", err)
	}
	return waitEmbeddedRuntime(h.t, h.runtime, func(runtime dto.RuntimeSnapshot) bool {
		return runtime.Phase == dto.RuntimeRunning && runtime.Ownership == dto.OwnershipHeld
	})
}

func (h *embeddedRuntimeHarness) stop(requestID string) dto.RuntimeSnapshot {
	h.t.Helper()
	if _, err := h.runtime.Quiesce(requestID, h.runtime.RuntimeSnapshot().Version); err != nil {
		h.t.Fatalf("stop embedded runtime: %v", err)
	}
	return waitEmbeddedRuntime(h.t, h.runtime, func(runtime dto.RuntimeSnapshot) bool {
		return runtime.Phase == dto.RuntimeStopped && runtime.Ownership == dto.OwnershipNone
	})
}

func (h *embeddedRuntimeHarness) seedPrimaryRepository() {
	h.t.Helper()
	cfg, err := h.config.LoadCurrentConfig()
	if err != nil {
		h.t.Fatal(err)
	}
	database, err := sql.Open("sqlite3", cfg.DatabaseConfig.Path)
	if err != nil {
		h.t.Fatal(err)
	}
	defer database.Close()
	var rootID, rootPath string
	if err := database.QueryRow(`SELECT root_id, path FROM repository_roots WHERE kind = 'default'`).Scan(&rootID, &rootPath); err != nil {
		h.t.Fatalf("read default root: %v", err)
	}
	// The Server canonicalizes macOS' /var symlink before cataloging paths;
	// retain that disk-authoritative spelling for the physical move.
	h.oldPath = rootPath
	repositoryID := "3dbf2738-a2ed-47bc-899a-a935769538e8"
	repositoryPath := filepath.Join(rootPath, "primary")
	if err := os.MkdirAll(repositoryPath, 0o755); err != nil {
		h.t.Fatal(err)
	}
	created := time.Now().UTC()
	marker := fmt.Sprintf("version: \"1.0\"\nid: %s\nname: Primary Storage\ncreated_at: %s\nstorage_strategy: date\nlocal_settings:\n  handle_duplicate_filenames: rename\n", repositoryID, created.Format(time.RFC3339Nano))
	if err := os.WriteFile(filepath.Join(repositoryPath, ".lumiliorepo"), []byte(marker), 0o644); err != nil {
		h.t.Fatal(err)
	}
	configJSON, err := json.Marshal(map[string]any{
		"version": "1.0", "id": repositoryID, "name": "Primary Storage", "created_at": created,
		"storage_strategy": "date", "local_settings": map[string]string{"handle_duplicate_filenames": "rename"},
	})
	if err != nil {
		h.t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	_, err = database.Exec(`INSERT INTO repositories
		(repo_id, name, path, config, reachability, activity, pause_reason, created_at, updated_at, role, root_id)
		VALUES (?, ?, ?, ?, 'active', 'idle', '', ?, ?, 'primary', ?)`,
		repositoryID, "Primary Storage", repositoryPath, string(configJSON), now, now, rootID)
	if err != nil {
		h.t.Fatalf("insert primary repository: %v", err)
	}
}

func (h *embeddedRuntimeHarness) moveStorage() string {
	h.t.Helper()
	newPath := filepath.Join(h.t.TempDir(), "moved-default-storage")
	if err := os.Rename(h.oldPath, newPath); err != nil {
		h.t.Fatalf("physically move default storage: %v", err)
	}
	if _, err := os.Stat(h.oldPath); !errors.Is(err, os.ErrNotExist) {
		h.t.Fatalf("old storage path remained online: %v", err)
	}
	return newPath
}

func (h *embeddedRuntimeHarness) candidate(newPath, listen string) dto.RuntimeConfigDraft {
	h.t.Helper()
	draft, err := h.config.ReadDraft()
	if err != nil {
		h.t.Fatal(err)
	}
	settings := draft.Settings
	settings.StoragePath = newPath
	settings.NetworkMode = "custom"
	settings.Listen = listen
	candidate, err := h.config.PatchDraft(draft.TOML, settings)
	if err != nil {
		h.t.Fatalf("patch moved storage candidate: %v", err)
	}
	return candidate
}

func TestEmbeddedRuntimeAppliesMovedDefaultStorageAndCommitsLKG(t *testing.T) {
	harness := newEmbeddedRuntimeHarness(t)
	newPath := harness.moveStorage()
	harness.start("degraded-old-path-start")
	candidate := harness.candidate(newPath, harness.oldPort)
	transactions := runtimeconfig.NewTransactionController(harness.config, harness.state, harness.operations, harness.runtime)
	if _, err := transactions.Apply("apply-moved-storage", harness.runtime.RuntimeSnapshot().Version, candidate.BaseFingerprint, candidate.TOML); err != nil {
		t.Fatalf("apply moved storage: %v", err)
	}
	current, _ := harness.config.CurrentPointer()
	lkg, _ := harness.config.LastKnownGoodPointer()
	if current.Fingerprint != candidate.CandidateFingerprint || lkg.Fingerprint != candidate.CandidateFingerprint {
		t.Fatalf("candidate was not committed to current and LKG: current=%q lkg=%q candidate=%q", current.Fingerprint, lkg.Fingerprint, candidate.CandidateFingerprint)
	}
	cfg, err := harness.config.LoadCurrentConfig()
	if err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite3", cfg.DatabaseConfig.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var rootPath, repositoryPath string
	if err := database.QueryRow(`SELECT path FROM repository_roots WHERE kind = 'default'`).Scan(&rootPath); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT path FROM repositories WHERE role = 'primary'`).Scan(&repositoryPath); err != nil {
		t.Fatal(err)
	}
	canonicalNewPath, err := filepath.EvalSymlinks(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if rootPath != canonicalNewPath || repositoryPath != filepath.Join(canonicalNewPath, "primary") {
		t.Fatalf("catalog paths were not migrated: root=%q repository=%q", rootPath, repositoryPath)
	}
}

func TestEmbeddedRuntimeFailedMovedStorageCandidateRollsBackToDegradedLKG(t *testing.T) {
	harness := newEmbeddedRuntimeHarness(t)
	newPath := harness.moveStorage()
	harness.start("degraded-old-path-start")
	releaseRootLock, err := holdRootLock(newPath)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseRootLock()
	candidate := harness.candidate(newPath, harness.oldPort)
	previous, _ := harness.config.CurrentPointer()
	transactions := runtimeconfig.NewTransactionController(harness.config, harness.state, harness.operations, harness.runtime)
	if _, err := transactions.Apply("apply-failing-moved-storage", harness.runtime.RuntimeSnapshot().Version, candidate.BaseFingerprint, candidate.TOML); err == nil {
		t.Fatal("candidate with externally locked moved root unexpectedly started")
	}
	running := waitEmbeddedRuntime(t, harness.runtime, func(runtime dto.RuntimeSnapshot) bool {
		return runtime.Phase == dto.RuntimeRunning && runtime.Ownership == dto.OwnershipHeld
	})
	if !running.PendingConfigValidation {
		t.Fatalf("degraded LKG rollback did not retain recovery signal: %#v", running)
	}
	current, _ := harness.config.CurrentPointer()
	lkg, _ := harness.config.LastKnownGoodPointer()
	if current.Fingerprint != previous.Fingerprint || lkg.Fingerprint != previous.Fingerprint {
		t.Fatalf("failed candidate changed persisted config: current=%q lkg=%q previous=%q", current.Fingerprint, lkg.Fingerprint, previous.Fingerprint)
	}
	cfg, err := harness.config.LoadCurrentConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StorageConfig.Path != harness.oldConfigPath {
		t.Fatalf("rollback did not restore offline LKG path: %q", cfg.StorageConfig.Path)
	}
}

func installMediaToolFixtures(t *testing.T) {
	t.Helper()
	resources := t.TempDir()
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	for _, relative := range []string{
		filepath.Join("exiftool", "exiftool"+suffix),
		filepath.Join("ffmpeg", "ffmpeg"+suffix),
		filepath.Join("ffmpeg", "ffprobe"+suffix),
	} {
		path := filepath.Join(resources, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("LUMILIO_RESOURCES_DIR", resources)
}

func waitEmbeddedRuntime(t *testing.T, controller *desktopruntime.Controller, predicate func(dto.RuntimeSnapshot) bool) dto.RuntimeSnapshot {
	t.Helper()
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		snapshot := controller.RuntimeSnapshot()
		if predicate(snapshot) {
			return snapshot
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("embedded runtime did not reach expected state; got %#v", controller.RuntimeSnapshot())
	return dto.RuntimeSnapshot{}
}
