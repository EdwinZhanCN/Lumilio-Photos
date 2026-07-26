package supervisor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// appDirName is the per-user app-data directory name. On macOS this lands
// under ~/Library/Application Support/Lumilio Photos.
const appDirName = "Lumilio Photos"

// Paths holds the resolved on-disk locations the desktop app uses. Everything
// except the user-selectable media library lives under a single per-user
// app-data directory so the database and secrets stay on local disk even when
// the library is relocated to an external drive.
type Paths struct {
	AppData    string // root machine-local app-state directory
	Database   string // active SQLite catalog
	Logs       string // API/application log directory
	Secrets    string // lumilio_secret_key and other private credentials
	Config     string // runtime intent, generated manifest, journal + desktop-settings.json
	Backups    string // consistent SQLite snapshots and manifests
	Cloud      string // cloud provider sessions and credential artifacts
	DefaultLib string // default media library location (<appdata>/storage)
}

// NewPaths resolves the app-data directory tree. LUMILIO_APP_DATA overrides the
// root (used for tests and for running multiple isolated instances); otherwise
// os.UserConfigDir provides the platform-native location (Application Support on
// macOS, %AppData% on Windows, ~/.config on Linux).
func NewPaths() (*Paths, error) {
	root, err := resolveAppDataRoot()
	if err != nil {
		return nil, err
	}
	return &Paths{
		AppData:    root,
		Database:   filepath.Join(root, "library.sqlite3"),
		Logs:       filepath.Join(root, "logs"),
		Secrets:    filepath.Join(root, "secrets"),
		Config:     filepath.Join(root, "config"),
		Backups:    filepath.Join(root, "backups"),
		Cloud:      filepath.Join(root, "cloud"),
		DefaultLib: filepath.Join(root, "storage"),
	}, nil
}

func resolveAppDataRoot() (string, error) {
	if override := os.Getenv("LUMILIO_APP_DATA"); override != "" {
		return override, nil
	}
	// On Windows os.UserConfigDir is %AppData% (Roaming); the SQLite catalog and
	// credentials must not roam between machines, so prefer %LocalAppData%.
	if runtime.GOOS == "windows" {
		if base := os.Getenv("LOCALAPPDATA"); base != "" {
			return filepath.Join(base, appDirName), nil
		}
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(base, appDirName), nil
}

// EnsureDirs creates the full app-data directory tree. The media library is
// created separately once its (possibly user-chosen) location is resolved.
func (p *Paths) EnsureDirs() error {
	for _, dir := range []string{p.AppData, p.Logs, p.Secrets, p.Config, p.Backups, p.Cloud} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
		if err := applyPrivateDirectoryMode(dir); err != nil {
			return fmt.Errorf("protect %s: %w", dir, err)
		}
	}
	return nil
}

// LockFile is the flock single-instance guard path.
func (p *Paths) LockFile() string { return filepath.Join(p.AppData, "lumilio.lock") }

// LumenDir holds the optional supervised Lumen Hub: the unpacked build,
// its generated config, and the model cache. Always on local disk.
func (p *Paths) LumenDir() string { return filepath.Join(p.AppData, "lumen") }

// DesktopSettingsFile persists user choices that must survive relaunch (e.g. the
// storage path). It is NOT regenerated, unlike ServerConfigFile.
func (p *Paths) DesktopSettingsFile() string {
	return filepath.Join(p.Config, "desktop-settings.json")
}

// ServerConfigFile is the generated, authoritative manifest consumed on this
// launch. Persisted user choices remain in desktop-settings.json.
func (p *Paths) ServerConfigFile() string {
	return filepath.Join(p.Config, "server.toml")
}

// RuntimeConfigFile is the persistent, complete schema-v3 user/runtime intent.
func (p *Paths) RuntimeConfigFile() string {
	return filepath.Join(p.Config, "runtime.toml")
}

// RuntimeLastKnownGoodFile is the most recent intent that passed readiness.
func (p *Paths) RuntimeLastKnownGoodFile() string {
	return filepath.Join(p.Config, "runtime.last-known-good.toml")
}

// RuntimeCandidateFile is the staged apply input and normally does not exist.
func (p *Paths) RuntimeCandidateFile() string {
	return filepath.Join(p.Config, "runtime.candidate.toml")
}

// RuntimeApplyJournalFile records crash-recoverable apply progress.
func (p *Paths) RuntimeApplyJournalFile() string {
	return filepath.Join(p.Config, "runtime-apply.json")
}

// SecretKeyFile holds the app root secret used to derive JWT/MFA/media keys.
func (p *Paths) SecretKeyFile() string { return filepath.Join(p.Secrets, "lumilio_secret_key") }
