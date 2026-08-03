package lumen

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"desktop/internal/platform"

	"go.yaml.in/yaml/v3"
)

const DefaultEndpoint = "127.0.0.1:50051"

type SetupPreset struct {
	Name       string
	Components []string
	MinRAMGB   uint64
	MinVRAMGB  uint64
	MinDiskGB  uint64
}

type SetupPlatform struct {
	Name string
}

type SetupBackend struct {
	Name              string
	ReleaseProfile    string
	CVRuntime         string
	SemanticRuntime   string
	SemanticPrecision string
}

// SetupSelection mirrors the non-interactive result of lumen-cli init. Desktop
// owns the choices, while rendering and validation follow the upstream setup
// pipeline.
type SetupSelection struct {
	Version    string
	Region     string
	Preset     SetupPreset
	Platform   SetupPlatform
	Backend    SetupBackend
	CacheDir   string
	ConfigPath string
}

var setupPresets = []SetupPreset{
	{Name: "minimal", Components: []string{"siglip", "face"}, MinRAMGB: 4, MinVRAMGB: 2, MinDiskGB: 2},
	{Name: "basic", Components: []string{"siglip", "face", "ocr", "bioclip"}, MinRAMGB: 6, MinVRAMGB: 3, MinDiskGB: 6},
	{Name: "brave", Components: []string{"siglip", "face", "ocr", "bioclip"}, MinRAMGB: 8, MinVRAMGB: 4, MinDiskGB: 10},
}

// DefaultSetupSelection applies the choices currently owned by Desktop: the
// Basic preset, the configured region and cache, and the backend encoded by
// the exact release profile selected by the installer.
func DefaultSetupSelection(configPath, cacheDir, region, releaseProfile string) (SetupSelection, error) {
	return NewSetupSelection(configPath, cacheDir, region, releaseProfile, "basic")
}

// NewSetupSelection builds the same complete selection produced by the
// interactive CLI/Launcher, using choices supplied by the Desktop UI.
func NewSetupSelection(configPath, cacheDir, region, releaseProfile, presetName string) (SetupSelection, error) {
	artifact, ok := officialReleaseArtifacts[releaseProfile]
	if !ok {
		return SetupSelection{}, fmt.Errorf("unsupported Lumen release profile %q", releaseProfile)
	}
	platformProfile, err := setupPlatformForReleaseProfile(releaseProfile)
	if err != nil {
		return SetupSelection{}, err
	}
	backend, err := setupBackendForReleaseProfile(releaseProfile)
	if err != nil {
		return SetupSelection{}, err
	}
	preset, ok := setupPresetByName(presetName)
	if !ok {
		return SetupSelection{}, fmt.Errorf("unsupported Lumen setup preset %q", presetName)
	}
	selection := SetupSelection{
		Version:    strings.TrimPrefix(artifact.Version, "v"),
		Region:     setupRegion(region),
		Preset:     preset,
		Platform:   platformProfile,
		Backend:    backend,
		CacheDir:   cacheDir,
		ConfigPath: configPath,
	}
	if err := validateSetupSelection(selection); err != nil {
		return SetupSelection{}, err
	}
	return selection, nil
}

func SetupPresetNames() []string {
	names := make([]string, 0, len(setupPresets))
	for _, preset := range setupPresets {
		names = append(names, preset.Name)
	}
	return names
}

// CanonicalCacheDirectory validates the untrusted result of the native
// directory picker before it can be persisted or written into config.yaml.
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

// EnsureSetupConfig materialises the Desktop-owned Lumen Hub intent once.
// Subsequent starts preserve the user's file; future structured configuration
// changes must update this file through an explicit transaction.
func EnsureSetupConfig(selection SetupSelection) error {
	if err := validateSetupSelection(selection); err != nil {
		return err
	}
	if _, err := os.Stat(selection.ConfigPath); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	config := renderSetupConfig(selection)
	if err := validateRenderedSetupConfig(config, selection); err != nil {
		return fmt.Errorf("validate generated Lumen configuration: %w", err)
	}
	if err := platform.EnsurePrivateDirectory(filepath.Dir(selection.ConfigPath)); err != nil {
		return err
	}
	if err := platform.EnsurePrivateDirectory(selection.CacheDir); err != nil {
		return err
	}
	return platform.WriteAtomic(selection.ConfigPath, []byte(config), 0o600)
}

func setupPresetByName(name string) (SetupPreset, bool) {
	for _, preset := range setupPresets {
		if preset.Name == name {
			preset.Components = slices.Clone(preset.Components)
			return preset, true
		}
	}
	return SetupPreset{}, false
}

func setupPlatformForReleaseProfile(profile string) (SetupPlatform, error) {
	switch {
	case strings.HasPrefix(profile, "darwin-arm64-"):
		return SetupPlatform{Name: "darwin-arm64"}, nil
	case strings.HasPrefix(profile, "windows-x64-"):
		return SetupPlatform{Name: "windows-x64"}, nil
	default:
		return SetupPlatform{}, fmt.Errorf("release profile %q has no supported Lumen platform", profile)
	}
}

func setupBackendForReleaseProfile(profile string) (SetupBackend, error) {
	backend := SetupBackend{
		ReleaseProfile: profile, CVRuntime: "burn", SemanticRuntime: "burn", SemanticPrecision: "fp16q8",
	}
	switch {
	case strings.HasSuffix(profile, "-metal"):
		backend.Name = "metal"
	case strings.HasSuffix(profile, "-gpu"):
		backend.Name = "gpu"
	case strings.HasSuffix(profile, "-cpu"):
		backend.Name = "cpu"
	default:
		return SetupBackend{}, fmt.Errorf("release profile %q has no supported Lumen backend", profile)
	}
	return backend, nil
}

func setupRegion(region string) string {
	region = strings.ToLower(strings.TrimSpace(region))
	if region == "cn" || region == "china" {
		return "cn"
	}
	return "other"
}

func validateSetupSelection(selection SetupSelection) error {
	if strings.TrimSpace(selection.Version) == "" {
		return errors.New("Lumen setup version is required")
	}
	if selection.Region != "cn" && selection.Region != "other" {
		return fmt.Errorf("invalid Lumen setup region %q", selection.Region)
	}
	preset, ok := setupPresetByName(selection.Preset.Name)
	if !ok || !slices.Equal(selection.Preset.Components, preset.Components) ||
		selection.Preset.MinRAMGB != preset.MinRAMGB || selection.Preset.MinVRAMGB != preset.MinVRAMGB || selection.Preset.MinDiskGB != preset.MinDiskGB {
		return fmt.Errorf("invalid Lumen setup preset %q", selection.Preset.Name)
	}
	platformProfile, err := setupPlatformForReleaseProfile(selection.Backend.ReleaseProfile)
	if err != nil {
		return err
	}
	if selection.Platform != platformProfile {
		return fmt.Errorf("Lumen platform %q does not match release profile %q", selection.Platform.Name, selection.Backend.ReleaseProfile)
	}
	backend, err := setupBackendForReleaseProfile(selection.Backend.ReleaseProfile)
	if err != nil {
		return err
	}
	if selection.Backend != backend {
		return fmt.Errorf("Lumen backend %q does not match release profile %q", selection.Backend.Name, selection.Backend.ReleaseProfile)
	}
	artifact, ok := officialReleaseArtifacts[selection.Backend.ReleaseProfile]
	if !ok {
		return fmt.Errorf("unsupported Lumen release profile %q", selection.Backend.ReleaseProfile)
	}
	if selection.Version != strings.TrimPrefix(artifact.Version, "v") {
		return fmt.Errorf("Lumen setup version %q does not match release %q", selection.Version, artifact.Version)
	}
	if strings.TrimSpace(selection.CacheDir) == "" {
		return errors.New("Lumen cache directory is required")
	}
	cleanCacheDir := filepath.Clean(selection.CacheDir)
	if cleanCacheDir == "." || filepath.Dir(cleanCacheDir) == "." || filepath.Dir(cleanCacheDir) == cleanCacheDir {
		return errors.New("Lumen cache directory must not be a filesystem root or single path component")
	}
	if strings.TrimSpace(selection.ConfigPath) == "" || filepath.Dir(selection.ConfigPath) == "." {
		return errors.New("Lumen configuration path must include a private parent directory")
	}
	return nil
}

func renderSetupConfig(selection SetupSelection) string {
	var config strings.Builder
	fmt.Fprintln(&config, "# Generated by Lumilio Photos Desktop using the lumen-cli init setup model.")
	fmt.Fprintf(&config, "# Preset: %s (%s)\n", selection.Preset.Name, strings.Join(selection.Preset.Components, ", "))
	fmt.Fprintf(&config, "# Resource guidance: RAM %d GB, GPU/Unified memory %d GB, disk %d GB.\n", selection.Preset.MinRAMGB, selection.Preset.MinVRAMGB, selection.Preset.MinDiskGB)
	fmt.Fprintln(&config, "# Weights and BioCLIP catalogs are memory-mapped: they load on demand and")
	fmt.Fprintln(&config, "# do not all count as resident RAM. A brief warmup spike is reclaimed after startup.")
	fmt.Fprintln(&config)
	fmt.Fprintln(&config, "metadata:")
	fmt.Fprintf(&config, "  version: %q\n", selection.Version)
	fmt.Fprintf(&config, "  region: %s\n", selection.Region)
	fmt.Fprintf(&config, "  cache_dir: %s\n\n", yamlSingleQuoted(selection.CacheDir))
	fmt.Fprintln(&config, "deployment:")
	fmt.Fprintln(&config, "  mode: hub")
	fmt.Fprintln(&config, "  services:")
	for _, service := range selection.Preset.Components {
		fmt.Fprintf(&config, "    - %s\n", service)
	}
	fmt.Fprintln(&config)
	fmt.Fprintln(&config, "server:")
	fmt.Fprintln(&config, "  host: \"127.0.0.1\"")
	fmt.Fprintln(&config, "  port: 50051")
	fmt.Fprintln(&config, "  mdns:")
	fmt.Fprintln(&config, "    enabled: true")
	fmt.Fprintln(&config, "  batching:")
	fmt.Fprintln(&config, "    enabled: false")
	fmt.Fprintln(&config, "    max_batch_size: 8")
	fmt.Fprintln(&config, "    queue_latency_ms: 2")
	fmt.Fprintln(&config)
	fmt.Fprintln(&config, "services:")
	siglipModel := "siglip2-base-patch16-224"
	if selection.Preset.Name == "brave" {
		siglipModel = "siglip2-so400m-patch14-384"
	}
	renderSetupService(&config, "SigLIP: semantic image + text embeddings.", "siglip", "siglip", siglipModel, selection.Backend.SemanticRuntime, selection.Backend.SemanticPrecision, "")
	fmt.Fprintln(&config)
	renderSetupService(&config, "InsightFace antelopev2: face detection + recognition.", "face", "insightface", "antelopev2", selection.Backend.CVRuntime, "fp16q8", "")
	if slices.Contains(selection.Preset.Components, "ocr") {
		fmt.Fprintln(&config)
		renderSetupService(&config, "PP-OCRv6 small: in-image text detection + recognition.", "ocr", "ppocr", "pp-ocrv6-small", selection.Backend.CVRuntime, "fp16q8", "")
	}
	if slices.Contains(selection.Preset.Components, "bioclip") {
		dataset := "TreeOfLife200MCore"
		if selection.Preset.Name == "brave" {
			dataset = "TreeOfLife200M"
		}
		fmt.Fprintln(&config)
		renderSetupService(&config, "BioCLIP-2: species classification over the Tree of Life catalog.", "bioclip", "clip", "bioclip-2", selection.Backend.SemanticRuntime, selection.Backend.SemanticPrecision, dataset)
	}
	return config.String()
}

func renderSetupService(config *strings.Builder, comment, name, packageName, model, runtimeName, precision, dataset string) {
	fmt.Fprintf(config, "  # %s\n", comment)
	fmt.Fprintf(config, "  %s:\n", name)
	fmt.Fprintln(config, "    enabled: true")
	fmt.Fprintf(config, "    package: %s\n", packageName)
	fmt.Fprintln(config, "    models:")
	fmt.Fprintln(config, "      default:")
	fmt.Fprintf(config, "        model: %s\n", model)
	fmt.Fprintf(config, "        runtime: %s\n", runtimeName)
	fmt.Fprintf(config, "        precision: %s\n", precision)
	if dataset != "" {
		fmt.Fprintf(config, "        dataset: %s\n", dataset)
	}
}

func yamlSingleQuoted(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

type renderedSetupConfig struct {
	Metadata struct {
		Version  string `yaml:"version"`
		Region   string `yaml:"region"`
		CacheDir string `yaml:"cache_dir"`
	} `yaml:"metadata"`
	Deployment struct {
		Mode     string   `yaml:"mode"`
		Services []string `yaml:"services"`
	} `yaml:"deployment"`
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
		MDNS struct {
			Enabled bool `yaml:"enabled"`
		} `yaml:"mdns"`
		Batching struct {
			Enabled bool `yaml:"enabled"`
		} `yaml:"batching"`
	} `yaml:"server"`
	Services map[string]struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"services"`
}

func validateRenderedSetupConfig(config string, selection SetupSelection) error {
	var rendered renderedSetupConfig
	decoder := yaml.NewDecoder(strings.NewReader(config))
	if err := decoder.Decode(&rendered); err != nil {
		return err
	}
	if rendered.Metadata.Version != selection.Version || rendered.Metadata.Region != selection.Region || rendered.Metadata.CacheDir != selection.CacheDir {
		return errors.New("generated Lumen metadata does not match setup selection")
	}
	if rendered.Deployment.Mode != "hub" || !slices.Equal(rendered.Deployment.Services, selection.Preset.Components) {
		return errors.New("generated Lumen deployment does not match setup preset")
	}
	if rendered.Server.Host != "127.0.0.1" || rendered.Server.Port != 50051 || !rendered.Server.MDNS.Enabled || rendered.Server.Batching.Enabled {
		return errors.New("generated Lumen server configuration is not Desktop-safe")
	}
	if len(rendered.Services) != len(selection.Preset.Components) {
		return errors.New("generated Lumen service configuration does not match setup preset")
	}
	for _, component := range selection.Preset.Components {
		service, ok := rendered.Services[component]
		if !ok || !service.Enabled {
			return fmt.Errorf("generated Lumen service %q is missing or disabled", component)
		}
	}
	return nil
}
