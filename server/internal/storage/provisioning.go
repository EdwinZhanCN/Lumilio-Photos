package storage

import (
	"fmt"
	"os"
	"strings"

	"server/config"
)

// EnsureRootLayout creates the explicitly separate private cloud/backup
// directories. It is idempotent.
//
// It creates:
//   - storage.cloud_state_path cloud provider sessions/artifacts (owner-only)
//   - storage.backups_path     database backups (owner-only)
//
// The Default Storage Location (storage.path) is NOT created here: a first
// run creates it when it registers the default, and a registered default
// whose folder is gone (an unplugged disk, an unmounted volume) must stay
// absent so startup reports it offline instead of shadowing the mount point
// with an empty folder. The primary repository directory (<storage.path>/primary)
// is built with its full repository structure when the primary repository is
// initialized.
func EnsureRootLayout(cfg config.StorageConfig) error {
	if strings.TrimSpace(cfg.Path) == "" {
		return fmt.Errorf("storage location path is empty")
	}

	layout := []struct {
		path string
		mode os.FileMode
	}{
		{cfg.CloudDir(), 0o700},
		{cfg.BackupsDir(), 0o700},
	}
	for _, d := range layout {
		if err := os.MkdirAll(d.path, d.mode); err != nil {
			return fmt.Errorf("create storage layout dir %s: %w", d.path, err)
		}
		// MkdirAll's mode is advisory on Windows and subject to umask on Unix, so
		// the policy is applied explicitly rather than trusted from creation.
		if err := applyDirectoryMode(d.path, d.mode); err != nil {
			return fmt.Errorf("protect storage layout dir %s: %w", d.path, err)
		}
	}
	return nil
}
