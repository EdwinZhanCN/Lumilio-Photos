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
	var storageLocationID, rootPath string
	if err := database.QueryRow(`SELECT storage_location_id, path FROM storage_locations WHERE kind = 'default'`).Scan(&storageLocationID, &rootPath); err != nil {
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
	// The seeded row keys under the rule of the directory it lives in, as a
	// registered primary would; the Desktop module cannot import the Server's
	// probe, so the test measures the two facts the same way.
	pathCase, pathNormalization := measureVolumeRule(h.t, repositoryPath)
	now := time.Now().UnixMilli()
	_, err = database.Exec(`INSERT INTO repositories
		(repo_id, name, path, config, reachability, activity, pause_reason, created_at, updated_at, role, storage_location_id, path_case, path_normalization)
		VALUES (?, ?, ?, ?, 'active', 'idle', '', ?, ?, 'primary', ?, ?, ?)`,
		repositoryID, "Primary Storage", repositoryPath, string(configJSON), now, now, storageLocationID, pathCase, pathNormalization)
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
	if err := database.QueryRow(`SELECT path FROM storage_locations WHERE kind = 'default'`).Scan(&rootPath); err != nil {
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

// catalogDefaultRoot reads the Default Storage Location path the Server
// catalog holds, to compare with the configuration that is current.
func (h *embeddedRuntimeHarness) catalogDefaultRoot() string {
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
	var rootPath string
	if err := database.QueryRow(`SELECT path FROM storage_locations WHERE kind = 'default'`).Scan(&rootPath); err != nil {
		h.t.Fatal(err)
	}
	return rootPath
}

// crashedApply leaves the runtime files exactly as the transaction
// controller has them at one journal phase of an Apply that moves the
// Default Storage Location: previous is the last known-good generation,
// the candidate intent is written, and the runtime is stopped.
func (h *embeddedRuntimeHarness) crashedApply(phase runtimeconfig.JournalPhase, candidate dto.RuntimeConfigDraft, currentIsCandidate bool) (previous string) {
	h.t.Helper()
	previousPointer, err := h.config.CurrentPointer()
	if err != nil {
		h.t.Fatal(err)
	}
	validation, err := h.config.Validate(filepath.Join(h.paths.RuntimeIntents, "crash-candidate.toml"), []byte(candidate.TOML))
	if err != nil {
		h.t.Fatal(err)
	}
	if err := h.config.WriteIntent(validation); err != nil {
		h.t.Fatal(err)
	}
	current := previousPointer.Fingerprint
	if currentIsCandidate {
		current = validation.Fingerprint
	}
	if err := h.config.WritePointer(h.paths.RuntimeCurrent, current); err != nil {
		h.t.Fatal(err)
	}
	if err := h.config.WriteJournal(runtimeconfig.Journal{
		OperationID: "crashed-" + string(phase), Mode: "apply", Phase: phase,
		PreviousFingerprint: previousPointer.Fingerprint, CandidateFingerprint: validation.Fingerprint,
	}); err != nil {
		h.t.Fatal(err)
	}
	return previousPointer.Fingerprint
}

// A Desktop that crashed after selecting the moved-storage candidate but
// before proving it ready restarts into recovery, never starts the unproven
// candidate on its own, and restores the previous generation through the
// same control the Settings window drives: current, last known-good, the
// loaded configuration and the catalog's Default Storage Location agree,
// and the previous generation runs again.
func TestEmbeddedRuntimeCrashAfterCandidateSelectedRestoresThePreviousGeneration(t *testing.T) {
	harness := newEmbeddedRuntimeHarness(t)
	newPath := harness.moveStorage()
	candidate := harness.candidate(newPath, harness.oldPort)
	previous := harness.crashedApply(runtimeconfig.PhaseCandidateSelected, candidate, true)

	// Restart: a fresh store reads the same files the way main.go does.
	restarted := runtimeconfig.NewStore(harness.paths)
	verdict, err := restarted.Reconcile()
	if err != nil {
		t.Fatal(err)
	}
	if !verdict.NeedsRollback || verdict.SafeToStart || verdict.Current.Fingerprint != candidate.CandidateFingerprint || verdict.LastKnownGood.Fingerprint != previous {
		t.Fatalf("boot verdict after a crash at candidate_selected = %+v", verdict)
	}
	transactions := runtimeconfig.NewTransactionController(restarted, harness.state, harness.operations, harness.runtime)
	if _, err := transactions.RestoreLastKnownGood("restore-after-crash", harness.runtime.RuntimeSnapshot().Version); err != nil {
		t.Fatalf("restore last known-good: %v", err)
	}
	current, _ := restarted.CurrentPointer()
	lkg, _ := restarted.LastKnownGoodPointer()
	if current.Fingerprint != previous || lkg.Fingerprint != previous {
		t.Fatalf("restore left current=%q lkg=%q, want both %q", current.Fingerprint, lkg.Fingerprint, previous)
	}
	if again, err := restarted.Reconcile(); err != nil || again.NeedsResume || again.NeedsRollback {
		t.Fatalf("journal still open after the restore: %+v err=%v", again, err)
	}
	cfg, err := restarted.LoadCurrentConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StorageConfig.Path != harness.oldConfigPath {
		t.Fatalf("restored configuration names %q, want the previous Default Storage Location %q", cfg.StorageConfig.Path, harness.oldConfigPath)
	}
	// The previous generation starts (degraded: its Default Storage
	// Location is the moved-away path) and the catalog still names that
	// same path, so configuration and catalog agree.
	harness.start("start-after-restore")
	if root := harness.catalogDefaultRoot(); root != harness.oldPath {
		t.Fatalf("catalog Default Storage Location = %q after the restore, want %q", root, harness.oldPath)
	}
	if _, err := os.Stat(filepath.Join(newPath, "primary", ".lumiliorepo")); err != nil {
		t.Fatalf("the moved tree was touched by the recovery: %v", err)
	}
}

// A Desktop that crashed while committing a candidate that had already
// reached readiness restarts straight into that candidate: the journal
// says it is safe to start, the readiness boundary promotes it to last
// known-good and closes the journal, and the Server moves the catalog's
// Default Storage Location to the candidate's path, so configuration,
// current, last known-good and catalog agree without a decision from the
// user.
func TestEmbeddedRuntimeCrashWhileCommittingKeepsTheReadyCandidate(t *testing.T) {
	harness := newEmbeddedRuntimeHarness(t)
	newPath := harness.moveStorage()
	candidate := harness.candidate(newPath, harness.oldPort)
	previous := harness.crashedApply(runtimeconfig.PhaseCommitting, candidate, true)

	restarted := runtimeconfig.NewStore(harness.paths)
	verdict, err := restarted.Reconcile()
	if err != nil {
		t.Fatal(err)
	}
	if !verdict.NeedsResume || verdict.NeedsRollback || !verdict.SafeToStart || verdict.LastKnownGood.Fingerprint != previous {
		t.Fatalf("boot verdict after a crash at committing = %+v", verdict)
	}
	// main.go starts the current generation; OnReady is the promotion.
	harness.start("start-after-committing-crash")
	current, _ := restarted.CurrentPointer()
	lkg, _ := restarted.LastKnownGoodPointer()
	if current.Fingerprint != candidate.CandidateFingerprint || lkg.Fingerprint != candidate.CandidateFingerprint {
		t.Fatalf("readiness did not promote the candidate: current=%q lkg=%q candidate=%q", current.Fingerprint, lkg.Fingerprint, candidate.CandidateFingerprint)
	}
	if again, err := restarted.Reconcile(); err != nil || again.NeedsResume || again.NeedsRollback {
		t.Fatalf("journal still open after readiness: %+v err=%v", again, err)
	}
	canonicalNewPath, err := filepath.EvalSymlinks(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if root := harness.catalogDefaultRoot(); root != canonicalNewPath {
		t.Fatalf("catalog Default Storage Location = %q, want the candidate's %q", root, canonicalNewPath)
	}
}

// measureVolumeRule reports whether the filesystem holding dir folds case
// and whether it treats canonically equivalent spellings as one name, in
// the vocabulary the Server catalog stores.
func measureVolumeRule(t *testing.T, dir string) (pathCase, pathNormalization string) {
	t.Helper()
	lower := filepath.Join(dir, ".volume-probe-a")
	if err := os.WriteFile(lower, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(lower)
	pathCase = "sensitive"
	if _, err := os.Stat(filepath.Join(dir, ".VOLUME-PROBE-A")); err == nil {
		pathCase = "insensitive"
	}
	decomposed := filepath.Join(dir, ".volume-probe-e\u0301")
	if err := os.WriteFile(decomposed, []byte("e"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(decomposed)
	pathNormalization = "none"
	if _, err := os.Stat(filepath.Join(dir, ".volume-probe-\u00e9")); err == nil {
		pathNormalization = "nfc"
	}
	return pathCase, pathNormalization
}
