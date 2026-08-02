package preferences

import (
	"path/filepath"
	"testing"

	"desktop/internal/control/dto"
	"desktop/internal/runtime/runtimeconfig"
	"desktop/internal/state"
)

type updateChannelRecorder struct{ channel string }

func (r *updateChannelRecorder) SetChannel(channel string) error {
	r.channel = channel
	return nil
}

func TestSavePreservesLifecycleSettingsAndPublishesPreferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.v1.json")
	settings := runtimeconfig.DefaultSettings()
	settings.OnboardingComplete = true
	settings.RuntimeDesiredState = dto.DesiredRunning
	if err := runtimeconfig.SaveSettings(path, settings); err != nil {
		t.Fatal(err)
	}
	store := state.NewWithInstanceID("preferences-test")
	updates := &updateChannelRecorder{}
	controller := NewController(Options{Path: path, Settings: settings, Store: store, Updates: updates})

	candidate := store.Get().Host.Preferences
	candidate.Locale = "zh-CN"
	candidate.Region = "china"
	candidate.UpdateChannel = "beta"
	candidate.OpenProductOnLaunch = true
	saved, err := controller.Save(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Version != candidate.Version+1 || updates.channel != "beta" {
		t.Fatalf("preferences were not published: saved=%+v channel=%q", saved, updates.channel)
	}
	persisted, err := runtimeconfig.LoadSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if !persisted.OnboardingComplete || persisted.RuntimeDesiredState != dto.DesiredRunning {
		t.Fatalf("lifecycle settings were overwritten: %+v", persisted)
	}
	if persisted.Locale != "zh-CN" || persisted.Region != "china" || persisted.UpdateChannel != "beta" || !persisted.OpenProductOnLaunch {
		t.Fatalf("preferences were not saved: %+v", persisted)
	}
}

func TestSaveRejectsStalePreferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.v1.json")
	settings := runtimeconfig.DefaultSettings()
	if err := runtimeconfig.SaveSettings(path, settings); err != nil {
		t.Fatal(err)
	}
	controller := NewController(Options{Path: path, Settings: settings, Store: state.NewWithInstanceID("preferences-test")})
	candidate := controller.store.Get().Host.Preferences
	candidate.Version++
	if _, err := controller.Save(candidate); err == nil {
		t.Fatal("stale preferences were accepted")
	}
}
