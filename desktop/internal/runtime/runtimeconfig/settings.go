package runtimeconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"desktop/internal/control/dto"
	"desktop/internal/platform"
)

const SettingsSchemaVersion = 1

type Settings struct {
	SchemaVersion       int              `json:"schemaVersion"`
	OnboardingComplete  bool             `json:"onboardingComplete"`
	Locale              string           `json:"locale"`
	Region              string           `json:"region"`
	UpdateChannel       string           `json:"updateChannel"`
	Theme               string           `json:"theme"`
	OpenProductOnLaunch bool             `json:"openProductOnLaunch"`
	RuntimeDesiredState dto.DesiredState `json:"runtimeDesiredState"`
	LumenDesiredState   dto.DesiredState `json:"lumenDesiredState"`
}

func DefaultSettings() Settings {
	return Settings{
		SchemaVersion:       SettingsSchemaVersion,
		Locale:              "en",
		Region:              "global",
		UpdateChannel:       "stable",
		Theme:               "system",
		RuntimeDesiredState: dto.DesiredStopped,
		LumenDesiredState:   dto.DesiredDisabled,
	}
}

func LoadSettings(path string) (Settings, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return DefaultSettings(), nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("read settings: %w", err)
	}
	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return Settings{}, fmt.Errorf("decode settings: %w", err)
	}
	if settings.SchemaVersion != SettingsSchemaVersion {
		return Settings{}, fmt.Errorf("unsupported settings schema version %d", settings.SchemaVersion)
	}
	if settings.RuntimeDesiredState == "" {
		settings.RuntimeDesiredState = dto.DesiredStopped
	}
	if settings.LumenDesiredState == "" {
		settings.LumenDesiredState = dto.DesiredDisabled
	}
	if settings.Locale == "" {
		settings.Locale = "en"
	}
	if settings.Region == "" {
		settings.Region = "global"
	}
	if settings.UpdateChannel == "" {
		settings.UpdateChannel = "stable"
	}
	if settings.Theme == "" {
		settings.Theme = "system"
	}
	if settings.RuntimeDesiredState != dto.DesiredStopped && settings.RuntimeDesiredState != dto.DesiredRunning {
		return Settings{}, fmt.Errorf("unsupported runtime desired state %q", settings.RuntimeDesiredState)
	}
	if settings.LumenDesiredState != dto.DesiredDisabled && settings.LumenDesiredState != dto.DesiredRunning {
		return Settings{}, fmt.Errorf("unsupported Lumen desired state %q", settings.LumenDesiredState)
	}
	if settings.UpdateChannel != "" && settings.UpdateChannel != "stable" && settings.UpdateChannel != "beta" {
		return Settings{}, fmt.Errorf("unsupported update channel %q", settings.UpdateChannel)
	}
	if settings.Theme != "light" && settings.Theme != "dark" && settings.Theme != "system" {
		return Settings{}, fmt.Errorf("unsupported theme %q", settings.Theme)
	}
	return settings, nil
}

func SaveSettings(path string, settings Settings) error {
	if settings.SchemaVersion == 0 {
		settings.SchemaVersion = SettingsSchemaVersion
	}
	if settings.SchemaVersion != SettingsSchemaVersion {
		return fmt.Errorf("unsupported settings schema version %d", settings.SchemaVersion)
	}
	if settings.RuntimeDesiredState != dto.DesiredStopped && settings.RuntimeDesiredState != dto.DesiredRunning {
		return fmt.Errorf("unsupported runtime desired state %q", settings.RuntimeDesiredState)
	}
	if settings.LumenDesiredState != dto.DesiredDisabled && settings.LumenDesiredState != dto.DesiredRunning {
		return fmt.Errorf("unsupported Lumen desired state %q", settings.LumenDesiredState)
	}
	if settings.UpdateChannel != "stable" && settings.UpdateChannel != "beta" {
		return fmt.Errorf("unsupported update channel %q", settings.UpdateChannel)
	}
	if settings.Theme != "light" && settings.Theme != "dark" && settings.Theme != "system" {
		return fmt.Errorf("unsupported theme %q", settings.Theme)
	}
	if settings.Locale == "" {
		return errors.New("Desktop locale is required")
	}
	if settings.Region == "" {
		return errors.New("Desktop region is required")
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return platform.WriteAtomic(path, append(data, '\n'), 0o600)
}
