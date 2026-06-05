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
// source of truth for those choices because server.local.toml is only a
// regenerated debug copy and therefore cannot hold persisted state.
type DesktopSettings struct {
	// StoragePath is the user-chosen media library location. Empty means "use
	// the default" (<appdata>/storage), resolved at startup.
	StoragePath string `json:"storage_path,omitempty"`
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
