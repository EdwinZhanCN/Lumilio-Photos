package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Paths struct {
	Root string

	SettingsFile  string
	ShortcutsFile string
	SecretsDir    string
	LogsDir       string

	RuntimeDir         string
	RuntimeIntents     string
	RuntimeCurrent     string
	RuntimeLKG         string
	RuntimeApply       string
	RuntimeGenerations string

	ResourcesDir      string
	ResourcesVersions string
	ResourcesCurrent  string
	ResourcesInstall  string

	LumenDir      string
	LumenVersions string
	LumenCurrent  string
	LumenInstall  string
	LumenConfig   string
	LumenModels   string
	LumenOwner    string

	UpdatesDir     string
	UpdatesStaging string
	UpgradeFile    string

	// WebRoot is the product SPA directory served by the in-process server
	// (server.web_root). It is resolved from the packaged bundle layout at
	// startup; empty means API-only mode and RegisterSPA stays a no-op.
	WebRoot string
}

func ResolvePaths() (Paths, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve OS app-data directory: %w", err)
	}
	paths, err := NewPaths(filepath.Join(root, "Lumilio Photos"))
	if err != nil {
		return Paths{}, err
	}
	paths.WebRoot = bundleWebRoot()
	return paths, nil
}

// bundleWebRoot returns the product SPA directory shipped next to the
// executable when running from a packaged bundle (macOS .app or the Windows
// portable layout), or "" in API-only mode (dev runs, unpackaged binaries).
// The empty case is intentional: server RegisterSPA stays a no-op and the
// runtime remains usable as a pure API host.
func bundleWebRoot() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	var web string
	if runtime.GOOS == "darwin" {
		// .../Lumilio Photos.app/Contents/MacOS/lumilio-photos
		marker := ".app" + string(os.PathSeparator) + "Contents" + string(os.PathSeparator) + "MacOS"
		if idx := strings.LastIndex(exe, marker); idx > 0 {
			web = filepath.Join(exe[:idx+len(".app")], "Contents", "Resources", "web")
		}
	} else {
		// Windows portable layout: <app dir>/resources/web beside the exe.
		web = filepath.Join(filepath.Dir(exe), "resources", "web")
	}
	if _, err := os.Stat(filepath.Join(web, "index.html")); err == nil {
		return web
	}
	return ""
}

func NewPaths(root string) (Paths, error) {
	if root == "" {
		return Paths{}, fmt.Errorf("app-data root is required")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return Paths{}, fmt.Errorf("resolve app-data root: %w", err)
	}
	runtimeDir := filepath.Join(root, "runtime")
	resourcesDir := filepath.Join(root, "resources")
	lumenDir := filepath.Join(root, "lumen")
	updatesDir := filepath.Join(root, "updates")
	return Paths{
		Root:               root,
		SettingsFile:       filepath.Join(root, "settings.v1.json"),
		ShortcutsFile:      filepath.Join(root, "storage-shortcuts.v1.json"),
		SecretsDir:         filepath.Join(root, "secrets"),
		LogsDir:            filepath.Join(root, "logs"),
		RuntimeDir:         runtimeDir,
		RuntimeIntents:     filepath.Join(runtimeDir, "intents"),
		RuntimeCurrent:     filepath.Join(runtimeDir, "current.json"),
		RuntimeLKG:         filepath.Join(runtimeDir, "lkg.json"),
		RuntimeApply:       filepath.Join(runtimeDir, "apply.json"),
		RuntimeGenerations: filepath.Join(runtimeDir, "generations"),
		ResourcesDir:       resourcesDir,
		ResourcesVersions:  filepath.Join(resourcesDir, "versions"),
		ResourcesCurrent:   filepath.Join(resourcesDir, "current.json"),
		ResourcesInstall:   filepath.Join(resourcesDir, "install.json"),
		LumenDir:           lumenDir,
		LumenVersions:      filepath.Join(lumenDir, "versions"),
		LumenCurrent:       filepath.Join(lumenDir, "current.json"),
		LumenInstall:       filepath.Join(lumenDir, "install.json"),
		LumenConfig:        filepath.Join(lumenDir, "config.yaml"),
		LumenModels:        filepath.Join(lumenDir, "models"),
		LumenOwner:         filepath.Join(lumenDir, "owner.lock"),
		UpdatesDir:         updatesDir,
		UpdatesStaging:     filepath.Join(updatesDir, "staging"),
		UpgradeFile:        filepath.Join(updatesDir, "upgrade.json"),
	}, nil
}

func (p Paths) Ensure() error {
	for _, path := range []string{
		p.Root, p.SecretsDir, p.LogsDir,
		p.RuntimeDir, p.RuntimeIntents, p.RuntimeGenerations,
		p.ResourcesDir, p.ResourcesVersions,
		p.LumenDir, p.LumenVersions, p.LumenModels,
		p.UpdatesDir, p.UpdatesStaging,
	} {
		if err := EnsurePrivateDirectory(path); err != nil {
			return err
		}
	}
	return nil
}
