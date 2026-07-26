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

	serverconfig "server/config"
)

const (
	desktopSettingsVersion       = 1
	lanHTTPWarningCurrentVersion = 1
)

// NetworkMode selects how the embedded server is reachable. Desktop never
// owns public certificates: external HTTPS always means a trusted reverse
// proxy terminates TLS.
type NetworkMode string

const (
	NetworkLocal         NetworkMode = "local"
	NetworkLANHTTP       NetworkMode = "lan_http"
	NetworkExternalHTTPS NetworkMode = "external_https"
)

// DesktopSettings are user choices that must persist across launches. It is the
// source of truth for those choices. The generated server.toml is rebuilt from
// these choices and is the authoritative immutable input for one launch.
type DesktopSettings struct {
	Version                       int         `json:"version"`
	NetworkMode                   NetworkMode `json:"network_mode"`
	PrimaryOrigin                 string      `json:"primary_origin"`
	Listen                        string      `json:"listen"`
	TrustedProxyCIDRs             []string    `json:"trusted_proxy_cidrs,omitempty"`
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

// LoadSettings reads desktop-settings.json. A missing file yields zero-value
// settings (first run) rather than an error.
func LoadSettings(path string) (DesktopSettings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return normalizeNetworkSettings(DesktopSettings{})
		}
		return DesktopSettings{}, fmt.Errorf("read desktop settings: %w", err)
	}
	var s DesktopSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return DesktopSettings{}, fmt.Errorf("parse desktop settings: %w", err)
	}
	return normalizeNetworkSettings(s)
}

// SaveSettings persists desktop-settings.json atomically.
func SaveSettings(path string, s DesktopSettings) error {
	var err error
	s, err = normalizeNetworkSettings(s)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
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

// normalizeNetworkSettings migrates old settings and validates the complete
// network profile before it can be persisted or compiled into server.toml.
func normalizeNetworkSettings(s DesktopSettings) (DesktopSettings, error) {
	if s.Version != 0 && s.Version != desktopSettingsVersion {
		return DesktopSettings{}, fmt.Errorf("unsupported desktop settings version %d", s.Version)
	}
	s.Version = desktopSettingsVersion
	if s.NetworkMode == "" {
		s.NetworkMode = NetworkLocal
	}

	switch s.NetworkMode {
	case NetworkLocal:
		s.Listen = "127.0.0.1:6680"
		s.PrimaryOrigin = "http://localhost:6680"
		s.TrustedProxyCIDRs = nil
	case NetworkLANHTTP:
		if s.LANHTTPWarningAcceptedVersion < lanHTTPWarningCurrentVersion {
			return DesktopSettings{}, fmt.Errorf(
				"LAN HTTP requires warning acceptance version %d",
				lanHTTPWarningCurrentVersion,
			)
		}
		s.Listen = "0.0.0.0:6680"
		s.PrimaryOrigin = "http://localhost:6680"
		s.TrustedProxyCIDRs = nil
	case NetworkExternalHTTPS:
		origin, parsed, err := serverconfig.NormalizeOrigin(s.PrimaryOrigin)
		if err != nil || parsed.Scheme != "https" {
			return DesktopSettings{}, errors.New("external HTTPS requires an exact https primary origin")
		}
		s.PrimaryOrigin = origin
		if err := validateDesktopListen(s.Listen); err != nil {
			return DesktopSettings{}, err
		}
		if len(s.TrustedProxyCIDRs) == 0 {
			return DesktopSettings{}, errors.New("external HTTPS requires at least one trusted proxy CIDR")
		}
		normalizedCIDRs := make([]string, 0, len(s.TrustedProxyCIDRs))
		seen := make(map[netip.Prefix]struct{}, len(s.TrustedProxyCIDRs))
		for _, raw := range s.TrustedProxyCIDRs {
			prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
			if err != nil || prefix.Bits() == 0 {
				return DesktopSettings{}, fmt.Errorf("invalid trusted proxy CIDR %q", raw)
			}
			prefix = prefix.Masked()
			if _, exists := seen[prefix]; exists {
				continue
			}
			seen[prefix] = struct{}{}
			normalizedCIDRs = append(normalizedCIDRs, prefix.String())
		}
		s.TrustedProxyCIDRs = normalizedCIDRs
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
