package supervisor

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// appDirName is the per-user app-data directory name. On macOS this lands
	// under ~/Library/Application Support/Lumilio Photos.
	appDirName = "Lumilio Photos"

	// pgMajorVersion encodes the bundled PostgreSQL major version in the data
	// directory layout (postgres/16/...) so a future major upgrade can detect a
	// version mismatch and run a dump/restore. Bump alongside the bundled PG.
	pgMajorVersion = "16"

	// pgPort only affects the Unix socket filename (.s.PGSQL.<port>); no TCP
	// port is opened (listen_addresses is empty). It must still be unique enough
	// to avoid colliding with a system PostgreSQL socket in the fallback dir.
	pgPort = "5487"

	// maxUnixSocketPath is the conservative ceiling for a Unix domain socket
	// path on macOS (sun_path is ~104 bytes). When the natural socket path would
	// exceed this (very long usernames), the socket directory falls back to /tmp.
	maxUnixSocketPath = 95
)

// Paths holds the resolved on-disk locations the desktop app uses. Everything
// except the user-selectable media library lives under a single per-user
// app-data directory so the database and secrets stay on local disk even when
// the library is relocated to an external drive.
type Paths struct {
	AppData    string // root app-data directory
	PGData     string // PostgreSQL data directory (initdb target)
	PGRun      string // preferred Unix socket directory
	PGLogs     string // PostgreSQL log directory
	Logs       string // API/application log directory
	Secrets    string // db_password + lumilio_secret_key
	Config     string // generated server.local.toml + desktop-settings.json
	Backups    string // pg_dump auto-backups (upgrades)
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
	pgVersionDir := filepath.Join(root, "postgres", pgMajorVersion)
	return &Paths{
		AppData:    root,
		PGData:     filepath.Join(pgVersionDir, "data"),
		PGRun:      filepath.Join(pgVersionDir, "run"),
		PGLogs:     filepath.Join(pgVersionDir, "logs"),
		Logs:       filepath.Join(root, "logs"),
		Secrets:    filepath.Join(root, "secrets"),
		Config:     filepath.Join(root, "config"),
		Backups:    filepath.Join(root, "backups"),
		DefaultLib: filepath.Join(root, "storage"),
	}, nil
}

func resolveAppDataRoot() (string, error) {
	if override := os.Getenv("LUMILIO_APP_DATA"); override != "" {
		return override, nil
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
	for _, dir := range []string{p.AppData, p.PGData, p.PGRun, p.PGLogs, p.Logs, p.Secrets, p.Config, p.Backups} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return nil
}

// LockFile is the flock single-instance guard path.
func (p *Paths) LockFile() string { return filepath.Join(p.AppData, "lumilio.lock") }

// DesktopSettingsFile persists user choices that must survive relaunch (e.g. the
// storage path). It is NOT regenerated, unlike ServerConfigFile.
func (p *Paths) DesktopSettingsFile() string {
	return filepath.Join(p.Config, "desktop-settings.json")
}

// ServerConfigFile is the generated server.local.toml. It is rewritten on every
// launch, so it must never be the source of truth for persisted user choices.
func (p *Paths) ServerConfigFile() string {
	return filepath.Join(p.Config, "server.local.toml")
}

// DBPasswordFile holds the generated PostgreSQL password (referenced by the
// server config's password_file).
func (p *Paths) DBPasswordFile() string { return filepath.Join(p.Secrets, "db_password") }

// SecretKeyFile holds the app root secret used to derive JWT/MFA/media keys.
func (p *Paths) SecretKeyFile() string { return filepath.Join(p.Secrets, "lumilio_secret_key") }

// SocketDir returns the directory PostgreSQL should place its Unix socket in.
// It prefers PGRun but falls back to a short /tmp path when the natural socket
// path would exceed the platform limit (long usernames).
func (p *Paths) SocketDir() string {
	full := filepath.Join(p.PGRun, ".s.PGSQL."+pgPort)
	if len(full) <= maxUnixSocketPath {
		return p.PGRun
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("lumilio-%d", os.Getuid()))
}
