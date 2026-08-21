package lumen

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"desktop/internal/platform"
)

const DefaultEndpoint = "127.0.0.1:50051"

// SetupIntent is the complete state owned by Desktop. Everything else in the
// Hub config is derived by the exact Hub binary that Desktop is about to run.
type SetupIntent struct {
	Region     string
	Preset     string
	CacheDir   string
	ConfigPath string
}

func NewSetupIntent(configPath, cacheDir, region, preset string) (SetupIntent, error) {
	intent := SetupIntent{
		Region:     setupRegion(region),
		Preset:     strings.TrimSpace(preset),
		CacheDir:   filepath.Clean(cacheDir),
		ConfigPath: filepath.Clean(configPath),
	}
	if err := validateSetupIntent(intent); err != nil {
		return SetupIntent{}, err
	}
	return intent, nil
}

func SetupPresetNames() []string {
	return slices.Clone(OfficialSetupPresets)
}

// ValidateInstalledSetup asks the currently installed Hub binary to render a
// disposable candidate. It proves a reconfiguration before Desktop stops the
// running generation or persists new intent.
func ValidateInstalledSetup(ctx context.Context, root, configPath, cacheDir, region, preset string) error {
	current, err := LoadCurrent(root)
	if err != nil {
		return err
	}
	hubBinary, err := safeJoin(root, current.Binary)
	if err != nil {
		return err
	}
	candidatePath := fmt.Sprintf("%s.candidate-%d-%d", configPath, os.Getpid(), time.Now().UnixNano())
	intent, err := NewSetupIntent(candidatePath, cacheDir, region, preset)
	if err != nil {
		return err
	}
	defer os.Remove(candidatePath)
	return ReconcileSetupConfig(ctx, hubBinary, intent)
}

// ReconcileSetupConfig regenerates the derived config before every start. A
// Desktop-managed config is never treated as a second mutable source of truth.
func ReconcileSetupConfig(ctx context.Context, hubBinary string, intent SetupIntent) error {
	return reconcileSetupConfig(ctx, hubBinary, intent, executeConfigRenderer)
}

type configRenderer func(context.Context, string, SetupIntent) ([]byte, error)

func reconcileSetupConfig(ctx context.Context, hubBinary string, intent SetupIntent, render configRenderer) error {
	if err := validateSetupIntent(intent); err != nil {
		return err
	}
	if strings.TrimSpace(hubBinary) == "" {
		return errors.New("Lumen Hub binary is required to render managed config")
	}
	if err := platform.EnsurePrivateDirectory(filepath.Dir(intent.ConfigPath)); err != nil {
		return err
	}
	if err := platform.EnsurePrivateDirectory(intent.CacheDir); err != nil {
		return err
	}

	config, err := render(ctx, hubBinary, intent)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(config)) == 0 {
		return errors.New("Lumen Hub rendered an empty configuration")
	}
	if config[len(config)-1] != '\n' {
		config = append(config, '\n')
	}
	return platform.WriteAtomic(intent.ConfigPath, config, 0o600)
}

func executeConfigRenderer(ctx context.Context, hubBinary string, intent SetupIntent) ([]byte, error) {
	command := newConfigRendererCommand(ctx, hubBinary, intent)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("render managed Lumen config with %s: %s", filepath.Base(hubBinary), message)
	}
	return output, nil
}

func newConfigRendererCommand(ctx context.Context, hubBinary string, intent SetupIntent) *exec.Cmd {
	command := exec.CommandContext(ctx, hubBinary, configRenderArgs(intent)...)
	configureHiddenProcess(command)
	return command
}

func configRenderArgs(intent SetupIntent) []string {
	return []string{
		"config", "render",
		"--target", "desktop",
		"--preset", intent.Preset,
		"--region", intent.Region,
		"--cache-dir", intent.CacheDir,
	}
}

func validateSetupIntent(intent SetupIntent) error {
	if intent.Region != "cn" && intent.Region != "other" {
		return fmt.Errorf("invalid Lumen setup region %q", intent.Region)
	}
	if !slices.Contains(OfficialSetupPresets, intent.Preset) {
		return fmt.Errorf("unsupported Lumen setup preset %q", intent.Preset)
	}
	if strings.TrimSpace(intent.CacheDir) == "" || intent.CacheDir == "." {
		return errors.New("Lumen cache directory is required")
	}
	if !filepath.IsAbs(intent.CacheDir) {
		return errors.New("Lumen cache directory must be absolute")
	}
	if filepath.Dir(intent.CacheDir) == intent.CacheDir {
		return errors.New("Lumen cache directory must not be a filesystem root")
	}
	if strings.TrimSpace(intent.ConfigPath) == "" || intent.ConfigPath == "." || filepath.Dir(intent.ConfigPath) == "." {
		return errors.New("Lumen configuration path must include a private parent directory")
	}
	if !filepath.IsAbs(intent.ConfigPath) {
		return errors.New("Lumen configuration path must be absolute")
	}
	return nil
}

func setupRegion(region string) string {
	region = strings.ToLower(strings.TrimSpace(region))
	if region == "cn" || region == "china" {
		return "cn"
	}
	return "other"
}

// CanonicalCacheDirectory validates the untrusted result of the native
// directory picker before it can be persisted or passed to Hub.
func CanonicalCacheDirectory(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("Lumen cache directory is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(abs)
	if filepath.Dir(clean) == clean {
		return "", errors.New("Lumen cache directory must not be a filesystem root")
	}
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("Lumen cache path is not a directory")
	}
	return filepath.Clean(resolved), nil
}
