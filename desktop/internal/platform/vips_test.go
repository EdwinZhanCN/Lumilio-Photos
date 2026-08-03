package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBundledVipsModuleDir(t *testing.T) {
	tests := []struct {
		name       string
		goos       string
		executable func(root string) string
		libDir     func(root string) string
	}{
		{
			name: "macOS app bundle",
			goos: "darwin",
			executable: func(root string) string {
				return filepath.Join(root, "Lumilio Photos.app", "Contents", "MacOS", "lumilio-photos")
			},
			libDir: func(root string) string {
				return filepath.Join(root, "Lumilio Photos.app", "Contents", "Resources", "lib")
			},
		},
		{
			name: "Windows portable bundle",
			goos: "windows",
			executable: func(root string) string {
				return filepath.Join(root, "lumilio-photos.exe")
			},
			libDir: func(root string) string {
				return filepath.Join(root, "resources", "lib")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			moduleDir := filepath.Join(tt.libDir(root), "vips-modules-8.18")
			if err := os.MkdirAll(moduleDir, 0o755); err != nil {
				t.Fatalf("create module directory: %v", err)
			}

			got := bundledVipsModuleDir(tt.executable(root), tt.goos)
			if got != moduleDir {
				t.Fatalf("bundledVipsModuleDir() = %q, want %q", got, moduleDir)
			}

			wantHome := filepath.Dir(filepath.Dir(moduleDir))
			if got := bundledVipsHome(tt.executable(root), tt.goos); got != wantHome {
				t.Fatalf("bundledVipsHome() = %q, want %q", got, wantHome)
			}
		})
	}
}

func TestBundledVipsModuleDirIgnoresNonDesktopLayouts(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "lib", "vips-modules-8.18"), 0o755); err != nil {
		t.Fatalf("create module directory: %v", err)
	}

	if got := bundledVipsModuleDir(filepath.Join(root, "lumilio-photos"), "linux"); got != "" {
		t.Fatalf("bundledVipsModuleDir() = %q for Linux, want empty", got)
	}
}

func TestConfigureBundledVipsModulesPreservesExplicitVIPSHOME(t *testing.T) {
	resources := t.TempDir()
	moduleDir := filepath.Join(resources, "lib", "vips-modules-8.18")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("create module directory: %v", err)
	}
	t.Setenv("LUMILIO_RESOURCES_DIR", resources)
	t.Setenv("VIPSHOME", filepath.Join(resources, "explicit"))

	if err := ConfigureBundledVipsModules(); err != nil {
		t.Fatalf("ConfigureBundledVipsModules() error = %v", err)
	}
	if got := os.Getenv("VIPSHOME"); got != filepath.Join(resources, "explicit") {
		t.Fatalf("VIPSHOME = %q, want explicit value", got)
	}
}
