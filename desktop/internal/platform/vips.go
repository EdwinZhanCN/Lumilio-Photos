package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConfigureBundledVipsModules points libvips at the dynamic loaders shipped
// with a packaged Desktop app. libvips discovers its modules below VIPSHOME,
// so this must run at the Desktop host boundary before the embedded Server
// starts libvips; standalone Server configuration remains independent of
// ambient environment variables.
func ConfigureBundledVipsModules() error {
	resources, err := ResolveBundledResourcesDir()
	if err != nil {
		return err
	}
	moduleDir := findBundledVipsModuleDir(resources)
	if moduleDir == "" || os.Getenv("VIPSHOME") != "" {
		return nil
	}
	if err := os.Setenv("VIPSHOME", filepath.Dir(filepath.Dir(moduleDir))); err != nil {
		return fmt.Errorf("set bundled libvips home: %w", err)
	}
	return nil
}

func bundledVipsHome(executable, goos string) string {
	moduleDir := bundledVipsModuleDir(executable, goos)
	if moduleDir == "" {
		return ""
	}
	return filepath.Dir(filepath.Dir(moduleDir))
}

// bundledVipsModuleDir returns the module directory used by the Desktop
// packagers. The goos argument keeps this lookup deterministic and makes the
// bundle layout testable on any host platform.
func bundledVipsModuleDir(executable, goos string) string {
	return findBundledVipsModuleDir(bundledResourcesDir(executable, goos))
}

func findBundledVipsModuleDir(resources string) string {
	if resources == "" {
		return ""
	}
	libDir := filepath.Join(resources, "lib")
	entries, err := os.ReadDir(libDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "vips-modules-") {
			return filepath.Join(libDir, entry.Name())
		}
	}
	return ""
}
