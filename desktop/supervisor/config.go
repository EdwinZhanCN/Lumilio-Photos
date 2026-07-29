package supervisor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
)

const (
	desktopSettingsVersion       = 2
	lanHTTPWarningCurrentVersion = 1
)

// NetworkMode selects whether the embedded server is local-only or reachable
// over the LAN. Public HTTPS deployment belongs to the Server distribution.
type NetworkMode string

const (
	NetworkLocal   NetworkMode = "local"
	NetworkLANHTTP NetworkMode = "lan_http"
)

// DesktopSettings are host/control-plane choices that persist across launches.
// Runtime network policy lives in runtime.toml; NetworkMode and Listen are
// transient values used while applying a structured runtime patch.
type DesktopSettings struct {
	Version                       int         `json:"version"`
	NetworkMode                   NetworkMode `json:"network_mode"`
	Listen                        string      `json:"listen"`
	LANHTTPWarningAcceptedVersion int         `json:"lan_http_warning_accepted_version,omitempty"`

	// StoragePath is the onboarding/legacy location choice. New runtimes always
	// use <appdata>/storage as the default root and migrate a different value to
	// the repository_roots registry as an external Storage Location.
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

type desktopSettingsV2 struct {
	Version                       int    `json:"version"`
	LANHTTPWarningAcceptedVersion int    `json:"lan_http_warning_accepted_version,omitempty"`
	StoragePath                   string `json:"storage_path,omitempty"`
	OnboardingCompleted           bool   `json:"onboarding_completed,omitempty"`
	TOSAcceptedVersion            string `json:"tos_accepted_version,omitempty"`
	Language                      string `json:"language,omitempty"`
	Region                        string `json:"region,omitempty"`
	LumenEnabled                  bool   `json:"lumen_enabled,omitempty"`
	LumenPreset                   string `json:"lumen_preset,omitempty"`
	LumenBackend                  string `json:"lumen_backend,omitempty"`
	LumenProfile                  string `json:"lumen_profile,omitempty"`
	LumenCacheDir                 string `json:"lumen_cache_dir,omitempty"`
	LumenPreviousCacheDir         string `json:"lumen_previous_cache_dir,omitempty"`
	LumenInstalledVersion         string `json:"lumen_installed_version,omitempty"`
	LumenInstalledProfile         string `json:"lumen_installed_profile,omitempty"`
}

// LoadSettings reads desktop-settings.json using the current disk schema.
// A missing file yields an empty v2 host configuration.
func LoadSettings(path string) (DesktopSettings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return DesktopSettings{Version: desktopSettingsVersion}, nil
		}
		return DesktopSettings{}, fmt.Errorf("read desktop settings: %w", err)
	}
	var header struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return DesktopSettings{}, fmt.Errorf("parse desktop settings: %w", err)
	}
	switch header.Version {
	case desktopSettingsVersion:
		var disk desktopSettingsV2
		if err := decodeSettingsJSON(data, &disk); err != nil {
			return DesktopSettings{}, err
		}
		return desktopSettingsFromV2(disk), nil
	default:
		return DesktopSettings{}, fmt.Errorf("unsupported desktop settings version %d", header.Version)
	}
}

func decodeSettingsJSON(data []byte, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("parse desktop settings: %w", err)
	}
	return nil
}

// SaveSettings persists only the v2 host/control-plane schema atomically.
func SaveSettings(path string, s DesktopSettings) error {
	disk := desktopSettingsToV2(s)
	data, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal desktop settings: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create desktop settings directory: %w", err)
	}
	if err := applyPrivateDirectoryMode(filepath.Dir(path)); err != nil {
		return fmt.Errorf("protect desktop settings directory: %w", err)
	}
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write desktop settings: %w", err)
	}
	if err := applyPrivateFileMode(tmp); err != nil {
		return fmt.Errorf("protect desktop settings: %w", err)
	}
	defer os.Remove(tmp)
	if err := replaceFile(tmp, path); err != nil {
		return fmt.Errorf("replace desktop settings: %w", err)
	}
	return nil
}

func desktopSettingsToV2(s DesktopSettings) desktopSettingsV2 {
	return desktopSettingsV2{
		Version:                       desktopSettingsVersion,
		LANHTTPWarningAcceptedVersion: s.LANHTTPWarningAcceptedVersion,
		StoragePath:                   s.StoragePath,
		OnboardingCompleted:           s.OnboardingCompleted,
		TOSAcceptedVersion:            s.TOSAcceptedVersion,
		Language:                      s.Language,
		Region:                        s.Region,
		LumenEnabled:                  s.LumenEnabled,
		LumenPreset:                   s.LumenPreset,
		LumenBackend:                  s.LumenBackend,
		LumenProfile:                  s.LumenProfile,
		LumenCacheDir:                 s.LumenCacheDir,
		LumenPreviousCacheDir:         s.LumenPreviousCacheDir,
		LumenInstalledVersion:         s.LumenInstalledVersion,
		LumenInstalledProfile:         s.LumenInstalledProfile,
	}
}

func desktopSettingsFromV2(d desktopSettingsV2) DesktopSettings {
	return DesktopSettings{
		Version:                       desktopSettingsVersion,
		LANHTTPWarningAcceptedVersion: d.LANHTTPWarningAcceptedVersion,
		StoragePath:                   d.StoragePath,
		OnboardingCompleted:           d.OnboardingCompleted,
		TOSAcceptedVersion:            d.TOSAcceptedVersion,
		Language:                      d.Language,
		Region:                        d.Region,
		LumenEnabled:                  d.LumenEnabled,
		LumenPreset:                   d.LumenPreset,
		LumenBackend:                  d.LumenBackend,
		LumenProfile:                  d.LumenProfile,
		LumenCacheDir:                 d.LumenCacheDir,
		LumenPreviousCacheDir:         d.LumenPreviousCacheDir,
		LumenInstalledVersion:         d.LumenInstalledVersion,
		LumenInstalledProfile:         d.LumenInstalledProfile,
	}
}

// normalizeNetworkSettings validates the complete Desktop network profile.
func normalizeNetworkSettings(s DesktopSettings) (DesktopSettings, error) {
	s.Version = desktopSettingsVersion
	if s.NetworkMode == "" {
		s.NetworkMode = NetworkLocal
	}

	switch s.NetworkMode {
	case NetworkLocal:
		s.Listen = "127.0.0.1:6680"
	case NetworkLANHTTP:
		if s.LANHTTPWarningAcceptedVersion < lanHTTPWarningCurrentVersion {
			return DesktopSettings{}, fmt.Errorf(
				"LAN HTTP requires warning acceptance version %d",
				lanHTTPWarningCurrentVersion,
			)
		}
		s.Listen = "0.0.0.0:6680"
	default:
		return DesktopSettings{}, fmt.Errorf("unsupported desktop network mode %q", s.NetworkMode)
	}
	return s, nil
}

func validateDesktopListen(value string) error {
	host, port, err := net.SplitHostPort(strings.TrimSpace(value))
	if err != nil || port == "" {
		return fmt.Errorf("invalid desktop listen address %q", value)
	}
	if host != "" {
		if _, err := netip.ParseAddr(host); err != nil {
			return fmt.Errorf("desktop listen host must be an IP address: %q", host)
		}
	}
	return nil
}
