package supervisor

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// DesktopSettings are user choices that must persist across launches. It is the
// source of truth for those choices. The generated server.toml is rebuilt from
// these choices and is the authoritative immutable input for one launch.
type DesktopSettings struct {
	// StoragePath is the user-chosen media library location. Empty means "use
	// the default" (<appdata>/storage), resolved at startup.
	StoragePath string `json:"storage_path,omitempty"`

	// OnboardingCompleted gates the first-run native onboarding window. Once the
	// user finishes onboarding (accepts the license, picks a storage location) it
	// is set true and the window is never shown again.
	OnboardingCompleted bool `json:"onboarding_completed,omitempty"`

	// TOSAcceptedVersion records which version of the terms/open-source-license
	// notice the user accepted, so a future revision can re-prompt.
	TOSAcceptedVersion string `json:"tos_accepted_version,omitempty"`

	// Language is the desktop-native UI language ("en" or "zh") for tray/dialog/
	// onboarding chrome only. It is independent of the in-browser app language the
	// web BootstrapWizard owns. Empty means "follow the OS locale".
	Language string `json:"language,omitempty"`

	// Region is the desktop download/network region: "cn" (mainland China) or
	// "other". It controls app-update mirrors and Lumen model download region.
	// Independent of the in-browser preference "region" (maps / OSM). Empty means
	// derive a default from Language / OS at read time.
	Region string `json:"region,omitempty"`

	// LumenEnabled records that the user turned on local AI: the supervised
	// Lumen Hub is started on every launch until it is disabled from the tray.
	LumenEnabled bool `json:"lumen_enabled,omitempty"`

	// LumenPreset, LumenBackend and LumenProfile are the launcher-compatible
	// local AI choices. Empty values are migrated to the recommended defaults.
	LumenPreset           string `json:"lumen_preset,omitempty"`
	LumenBackend          string `json:"lumen_backend,omitempty"`
	LumenProfile          string `json:"lumen_profile,omitempty"`
	LumenCacheDir         string `json:"lumen_cache_dir,omitempty"`
	LumenPreviousCacheDir string `json:"lumen_previous_cache_dir,omitempty"`
	LumenInstalledVersion string `json:"lumen_installed_version,omitempty"`
	LumenInstalledProfile string `json:"lumen_installed_profile,omitempty"`
}

// LoadSettings reads desktop-settings.json. A missing file yields zero-value
// settings (first run) rather than an error.
func LoadSettings(path string) (DesktopSettings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return DesktopSettings{}, nil
		}
		return DesktopSettings{}, fmt.Errorf("read desktop settings: %w", err)
	}
	var s DesktopSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return DesktopSettings{}, fmt.Errorf("parse desktop settings: %w", err)
	}
	return s, nil
}

// SaveSettings persists desktop-settings.json atomically.
func SaveSettings(path string, s DesktopSettings) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal desktop settings: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write desktop settings: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace desktop settings: %w", err)
	}
	return nil
}

// ensureSecret writes a fresh 32-byte cryptographically random hex secret to
// path if the file is missing or empty. Existing secrets are preserved so keys
// remain stable across launches.
func ensureSecret(path string) error {
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		return nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Errorf("generate secret: %w", err)
	}
	if err := os.WriteFile(path, []byte(hex.EncodeToString(buf)), 0o600); err != nil {
		return fmt.Errorf("write secret %s: %w", path, err)
	}
	return nil
}
