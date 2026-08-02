package runtimeconfig

import (
	"os"
	"path/filepath"
	"testing"

	"desktop/internal/control/dto"
	"desktop/internal/platform"
)

func TestFingerprintAndPointerRoundTrip(t *testing.T) {
	paths, err := platform.NewPaths(filepath.Join(t.TempDir(), "app-data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	store := NewStore(paths)
	fingerprint := Fingerprint([]byte("candidate"))
	if err := store.WritePointer(paths.RuntimeCurrent, fingerprint); err != nil {
		t.Fatal(err)
	}
	got, err := store.CurrentPointer()
	if err != nil {
		t.Fatal(err)
	}
	if got.Fingerprint != fingerprint || got.SchemaVersion != PointerSchemaVersion {
		t.Fatalf("unexpected pointer: %+v", got)
	}
}

func TestReconcileRejectsUnknownPointer(t *testing.T) {
	paths, err := platform.NewPaths(filepath.Join(t.TempDir(), "app-data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	store := NewStore(paths)
	previous := Fingerprint([]byte("previous"))
	candidate := Fingerprint([]byte("candidate"))
	unknown := Fingerprint([]byte("unknown"))
	if err := store.WritePointer(paths.RuntimeCurrent, unknown); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteJournal(Journal{
		OperationID: "op-1", Mode: "apply", Phase: PhasePrepared,
		PreviousFingerprint: previous, CandidateFingerprint: candidate,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Reconcile(); err == nil {
		t.Fatal("reconcile accepted pointer outside journal")
	}
}

func TestSettingsCorruptionIsNotReset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.v1.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSettings(path); err == nil {
		t.Fatal("corrupt settings were accepted")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("corrupt settings were discarded: %v", err)
	}
}

func TestReadDraftGeneratesCompleteFirstRunIntent(t *testing.T) {
	paths, err := platform.NewPaths(filepath.Join(t.TempDir(), "app-data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	store := NewStore(paths)
	draft, err := store.ReadDraft()
	if err != nil {
		t.Fatal(err)
	}
	if !draft.Validation.Valid || draft.Source != "default" || draft.BaseFingerprint != "" {
		t.Fatalf("unexpected first-run draft: %#v", draft)
	}
	if draft.Settings.NetworkMode != "local" || draft.Settings.Listen != "127.0.0.1:6680" {
		t.Fatalf("unexpected network defaults: %#v", draft.Settings)
	}
	if draft.Settings.StoragePath == "" || draft.TOML == "" {
		t.Fatal("first-run draft did not include a usable storage path and complete TOML")
	}
	if pointer, err := store.CurrentPointer(); err != nil || pointer.Fingerprint != "" {
		t.Fatalf("reading the first-run draft persisted configuration: %#v, %v", pointer, err)
	}
}

func TestPatchDraftUpdatesStructuredSettingsThroughStrictLoader(t *testing.T) {
	paths, err := platform.NewPaths(filepath.Join(t.TempDir(), "app-data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	store := NewStore(paths)
	draft, err := store.ReadDraft()
	if err != nil {
		t.Fatal(err)
	}
	settings := draft.Settings
	settings.NetworkMode = "lan"
	settings.StoragePath = filepath.Join(t.TempDir(), "photos")
	settings.LoggingLevel = "debug"
	settings.RepositoryScanEnabled = false
	settings.HardwareAcceleration = "none"

	patched, err := store.PatchDraft(draft.TOML, settings)
	if err != nil {
		t.Fatal(err)
	}
	if !patched.Validation.Valid || patched.Settings.NetworkMode != "lan" || patched.Settings.Listen != "0.0.0.0:6680" {
		t.Fatalf("unexpected patched draft: %#v", patched)
	}
	if patched.Settings != (dto.RuntimeConfigSettings{
		NetworkMode: "lan", Listen: "0.0.0.0:6680", StoragePath: settings.StoragePath,
		LoggingLevel: "debug", RepositoryScanEnabled: false, HardwareAcceleration: "none",
	}) {
		t.Fatalf("structured settings did not round-trip: %#v", patched.Settings)
	}
}

func TestPatchDraftRejectsDefaultStorageChangeAfterSetup(t *testing.T) {
	paths, err := platform.NewPaths(filepath.Join(t.TempDir(), "app-data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	store := NewStore(paths)
	draft, err := store.ReadDraft()
	if err != nil {
		t.Fatal(err)
	}
	validation, err := store.Validate(filepath.Join(paths.RuntimeIntents, "candidate.toml"), []byte(draft.TOML))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WriteIntent(validation); err != nil {
		t.Fatal(err)
	}
	if err := store.WritePointer(paths.RuntimeCurrent, validation.Fingerprint); err != nil {
		t.Fatal(err)
	}

	draft.Settings.StoragePath = filepath.Join(t.TempDir(), "moved")
	if _, err := store.PatchDraft(draft.TOML, draft.Settings); err == nil {
		t.Fatal("configured default storage location was allowed to move")
	}
}
