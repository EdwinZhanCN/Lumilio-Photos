package storage

import (
	"fmt"
	"os"
	"strings"

	"server/config"
)

// EnsureRootLayout creates the well-known non-repository directories under the
// immutable storage root, deriving their locations from StorageConfig. It is
// idempotent and is the filesystem producer of the bootstrap "dirs_ready" gate.
//
// It creates:
//   - <root>          the storage root itself
//   - <root>/.secrets db_password and the app secret key (owner-only)
//   - <root>/.cloud   cloud sync working area (owner-only)
//
// The primary repository directory (<root>/primary) is NOT created here; it is
// built with its full repository structure when the primary repository is
// initialized.
func EnsureRootLayout(cfg config.StorageConfig) error {
	if strings.TrimSpace(cfg.Path) == "" {
		return fmt.Errorf("storage root path is empty")
	}

	layout := []struct {
		path string
		mode os.FileMode
	}{
		{cfg.Path, 0o755},
		{cfg.SecretsDir(), 0o700},
		{cfg.CloudDir(), 0o700},
	}
	for _, d := range layout {
		if err := os.MkdirAll(d.path, d.mode); err != nil {
			return fmt.Errorf("create storage layout dir %s: %w", d.path, err)
		}
	}
	return nil
}
