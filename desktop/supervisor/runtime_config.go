package supervisor

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"strings"

	"desktop/lumen"

	serverconfig "server/config"

	"github.com/pelletier/go-toml/v2"
)

var ErrStaleRuntimeConfig = errors.New("runtime configuration changed since this draft was opened")

var hostManagedRuntimePaths = []string{
	"schema_version",
	"environment",
	"database.path",
	"server.web_root",
	"server.cors_allowed_origins",
	"server.tls.http_listen",
	"server.tls.email",
	"server.tls.storage_path",
	"logging.dir",
	"storage.path",
	"storage.cloud_state_path",
	"storage.backups_path",
	"auth.secret_key_file",
	"tools.exiftool_path",
	"tools.ffmpeg_path",
	"tools.ffprobe_path",
	"lumen.discovery_static_nodes",
	"lumen.deployment_id",
}

type ConfigIssue struct {
	Field   string `json:"field,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type SemanticChange struct {
	Field  string `json:"field"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type RuntimeConfigView struct {
	CurrentTOML            string           `json:"currentToml"`
	CandidateTOML          string           `json:"candidateToml"`
	BaseFingerprint        string           `json:"baseFingerprint"`
	LastKnownGoodAvailable bool             `json:"lastKnownGoodAvailable"`
	HostManagedPaths       []string         `json:"hostManagedPaths"`
	Network                NetworkSummary   `json:"network"`
	Issues                 []ConfigIssue    `json:"issues"`
	SemanticChanges        []SemanticChange `json:"semanticChanges"`
}

type RuntimeConfigValidation struct {
	Valid           bool             `json:"valid"`
	CandidateTOML   string           `json:"candidateToml"`
	BaseFingerprint string           `json:"baseFingerprint"`
	Network         NetworkSummary   `json:"network"`
	Issues          []ConfigIssue    `json:"issues"`
	SemanticChanges []SemanticChange `json:"semanticChanges"`
	RequiresRestart bool             `json:"requiresRestart"`
}

type NetworkCandidatePatch struct {
	Mode              NetworkMode `json:"mode"`
	PrimaryOrigin     string      `json:"primaryOrigin"`
	Listen            string      `json:"listen"`
	ProxyLocation     string      `json:"proxyLocation"`
	TrustedProxyCIDRs []string    `json:"trustedProxyCIDRs"`
	AcceptLANWarning  bool        `json:"acceptLANWarning"`
}

type hostProjection struct {
	WebRoot, LogDir, StoragePath              string
	CloudStatePath, BackupsPath, DatabasePath string
	SecretKeyFile                             string
	ExifToolPath, FFmpegPath, FFprobePath     string
	LumenStaticNode                           string
}

func runtimeFingerprint(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", sum)
}

func (s *Supervisor) hostProjection() (hostProjection, error) {
	if err := s.ensurePaths(); err != nil {
		return hostProjection{}, err
	}
	resources, err := ResourcesDir()
	if err != nil {
		return hostProjection{}, fmt.Errorf("resolve resources dir: %w", err)
	}
	return hostProjection{
		WebRoot:         bundledWebRoot(resources),
		LogDir:          s.paths.Logs,
		StoragePath:     s.paths.DefaultLib,
		CloudStatePath:  s.paths.Cloud,
		BackupsPath:     s.paths.Backups,
		DatabasePath:    s.paths.Database,
		SecretKeyFile:   s.paths.SecretKeyFile(),
		ExifToolPath:    bundledExifTool(resources),
		FFmpegPath:      bundledFFmpeg(resources),
		FFprobePath:     bundledFFprobe(resources),
		LumenStaticNode: lumen.GRPCEndpoint,
	}, nil
}

func (p hostProjection) initialBindings(network DesktopSettings) serverManifestBindings {
	tlsMode := "off"
	proxyMode := "disabled"
	if network.NetworkMode == NetworkExternalHTTPS {
		tlsMode = "external"
		proxyMode = "required"
	}
	return serverManifestBindings{
		Listen:            network.Listen,
		PrimaryOrigin:     network.PrimaryOrigin,
		TLSMode:           tlsMode,
		ProxyMode:         proxyMode,
		TrustedProxyCIDRs: network.TrustedProxyCIDRs,
		WebRoot:           p.WebRoot,
		LogDir:            p.LogDir,
		StoragePath:       p.StoragePath,
		CloudStatePath:    p.CloudStatePath,
		BackupsPath:       p.BackupsPath,
		DatabasePath:      p.DatabasePath,
		SecretKeyFile:     p.SecretKeyFile,
		ExifToolPath:      p.ExifToolPath,
		FFmpegPath:        p.FFmpegPath,
		FFprobePath:       p.FFprobePath,
		LumenStaticNode:   p.LumenStaticNode,
	}
}

func (s *Supervisor) ensureRuntimeIntent(settings DesktopSettings) error {
	projection, err := s.hostProjection()
	if err != nil {
		return err
	}
	path := s.paths.RuntimeConfigFile()
	data, readErr := os.ReadFile(path)
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		return fmt.Errorf("read runtime intent: %w", readErr)
	}
	if errors.Is(readErr, fs.ErrNotExist) {
		network := settings
		if !settings.legacyNetwork {
			network = DesktopSettings{}
		}
		network, err = normalizeNetworkSettings(network)
		if err != nil {
			return fmt.Errorf("migrate desktop network settings: %w", err)
		}
		data, err = renderServerManifest(projection.initialBindings(network))
		if err != nil {
			return err
		}
		if _, err := serverconfig.LoadAppConfigBytes(path, data); err != nil {
			return fmt.Errorf("validate initial runtime intent: %w", err)
		}
		if err := writeAtomicPrivate(path, data); err != nil {
			return fmt.Errorf("write runtime intent: %w", err)
		}
		if err := writeAtomicPrivate(s.paths.RuntimeLastKnownGoodFile(), data); err != nil {
			return fmt.Errorf("write initial last-known-good runtime: %w", err)
		}
	}
	if _, _, err := s.materializeRuntimeBytes(data); err != nil {
		return fmt.Errorf("validate runtime intent: %w", err)
	}
	if settings.Version != desktopSettingsVersion || settings.legacyNetwork {
		if err := SaveSettings(s.paths.DesktopSettingsFile(), settings); err != nil {
			return fmt.Errorf("migrate desktop settings to v2: %w", err)
		}
	}
	return nil
}

func (s *Supervisor) materializeRuntimeBytes(intent []byte) ([]byte, serverconfig.AppConfig, error) {
	projection, err := s.hostProjection()
	if err != nil {
		return nil, serverconfig.AppConfig{}, err
	}
	document, err := parseRuntimeDocument(intent)
	if err != nil {
		return nil, serverconfig.AppConfig{}, err
	}
	projectRuntimeHostFields(document, projection)
	materialized, err := toml.Marshal(document)
	if err != nil {
		return nil, serverconfig.AppConfig{}, fmt.Errorf("encode materialized runtime manifest: %w", err)
	}
	cfg, err := serverconfig.LoadAppConfigBytes(s.paths.ServerConfigFile(), materialized)
	if err != nil {
		return nil, serverconfig.AppConfig{}, err
	}
	return materialized, cfg, nil
}

func projectRuntimeHostFields(document map[string]any, p hostProjection) {
	setRuntimePath(document, "schema_version", serverconfig.SchemaVersion)
	setRuntimePath(document, "environment", "production")
	setRuntimePath(document, "database.path", p.DatabasePath)
	setRuntimePath(document, "server.web_root", p.WebRoot)
	setRuntimePath(document, "server.cors_allowed_origins", []string{})
	setRuntimePath(document, "server.tls.http_listen", "")
	setRuntimePath(document, "server.tls.email", "")
	setRuntimePath(document, "server.tls.storage_path", "")
	setRuntimePath(document, "logging.dir", p.LogDir)
	setRuntimePath(document, "storage.path", p.StoragePath)
	setRuntimePath(document, "storage.cloud_state_path", p.CloudStatePath)
	setRuntimePath(document, "storage.backups_path", p.BackupsPath)
	setRuntimePath(document, "auth.secret_key_file", p.SecretKeyFile)
	setRuntimePath(document, "tools.exiftool_path", p.ExifToolPath)
	setRuntimePath(document, "tools.ffmpeg_path", p.FFmpegPath)
	setRuntimePath(document, "tools.ffprobe_path", p.FFprobePath)
	setRuntimePath(document, "lumen.discovery_static_nodes", []string{p.LumenStaticNode})
	setRuntimePath(document, "lumen.deployment_id", "desktop")
}

func parseRuntimeDocument(data []byte) (map[string]any, error) {
	var document map[string]any
	if err := toml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse runtime TOML: %w", err)
	}
	return document, nil
}

func setRuntimePath(document map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := document
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

func runtimePathValue(document map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var current any = document
	for _, part := range parts {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func (s *Supervisor) runtimeIntent() ([]byte, serverconfig.AppConfig, error) {
	settings, err := LoadSettings(s.paths.DesktopSettingsFile())
	if err != nil {
		return nil, serverconfig.AppConfig{}, err
	}
	if err := s.ensureRuntimeIntent(settings); err != nil {
		return nil, serverconfig.AppConfig{}, err
	}
	data, err := os.ReadFile(s.paths.RuntimeConfigFile())
	if err != nil {
		return nil, serverconfig.AppConfig{}, fmt.Errorf("read runtime intent: %w", err)
	}
	_, cfg, err := s.materializeRuntimeBytes(data)
	return data, cfg, err
}

func (s *Supervisor) materializeRuntimeConfig() (serverconfig.AppConfig, error) {
	intent, _, err := s.runtimeIntent()
	if err != nil {
		return serverconfig.AppConfig{}, err
	}
	materialized, cfg, err := s.materializeRuntimeBytes(intent)
	if err != nil {
		return serverconfig.AppConfig{}, err
	}
	if err := writeAtomicPrivate(s.paths.ServerConfigFile(), materialized); err != nil {
		return serverconfig.AppConfig{}, fmt.Errorf("write desktop server manifest: %w", err)
	}
	return cfg, nil
}

func (s *Supervisor) ReadRuntimeConfig() (RuntimeConfigView, error) {
	if err := s.ensurePaths(); err != nil {
		return RuntimeConfigView{}, err
	}
	current, cfg, err := s.runtimeIntent()
	if err != nil {
		return RuntimeConfigView{}, err
	}
	_, lkgErr := os.Stat(s.paths.RuntimeLastKnownGoodFile())
	return RuntimeConfigView{
		CurrentTOML:            string(current),
		CandidateTOML:          string(current),
		BaseFingerprint:        runtimeFingerprint(current),
		LastKnownGoodAvailable: lkgErr == nil,
		HostManagedPaths:       append([]string(nil), hostManagedRuntimePaths...),
		Network:                networkSummaryFromConfig(cfg),
		Issues:                 []ConfigIssue{},
		SemanticChanges:        []SemanticChange{},
	}, nil
}

func (s *Supervisor) ValidateRuntimeConfig(baseFingerprint, candidate string) (RuntimeConfigValidation, error) {
	if err := s.ensurePaths(); err != nil {
		return RuntimeConfigValidation{}, err
	}
	current, currentConfig, err := s.runtimeIntent()
	if err != nil {
		return RuntimeConfigValidation{}, err
	}
	fingerprint := runtimeFingerprint(current)
	if baseFingerprint != fingerprint {
		return RuntimeConfigValidation{}, ErrStaleRuntimeConfig
	}
	result := RuntimeConfigValidation{
		BaseFingerprint: fingerprint,
		Issues:          []ConfigIssue{},
		SemanticChanges: []SemanticChange{},
		RequiresRestart: true,
	}
	currentDocument, err := parseRuntimeDocument(current)
	if err != nil {
		return RuntimeConfigValidation{}, err
	}
	candidateDocument, err := parseRuntimeDocument([]byte(candidate))
	if err != nil {
		result.Issues = append(result.Issues, ConfigIssue{Code: "invalid_toml", Message: err.Error()})
		return result, nil
	}
	for _, path := range hostManagedRuntimePaths {
		before, beforeOK := runtimePathValue(currentDocument, path)
		after, afterOK := runtimePathValue(candidateDocument, path)
		if beforeOK != afterOK || !reflect.DeepEqual(before, after) {
			result.Issues = append(result.Issues, ConfigIssue{
				Field: path, Code: "host_managed",
				Message: fmt.Sprintf("%s is managed by the Desktop host and cannot be changed", path),
			})
		}
	}
	if mode, _ := runtimePathValue(candidateDocument, "server.tls.mode"); mode == "acme" {
		result.Issues = append(result.Issues, ConfigIssue{
			Field: "server.tls.mode", Code: "unsupported_desktop_tls",
			Message: "Desktop supports TLS modes off and external; ACME is not available",
		})
	}
	canonical, err := toml.Marshal(candidateDocument)
	if err != nil {
		result.Issues = append(result.Issues, ConfigIssue{Code: "invalid_toml", Message: err.Error()})
		return result, nil
	}
	result.CandidateTOML = string(canonical)
	if len(result.Issues) != 0 {
		return result, nil
	}
	_, candidateConfig, err := s.materializeRuntimeBytes(canonical)
	if err != nil {
		result.Issues = append(result.Issues, ConfigIssue{Code: "invalid_manifest", Message: err.Error()})
		return result, nil
	}
	result.Valid = true
	result.Network = networkSummaryFromConfig(candidateConfig)
	result.SemanticChanges = runtimeSemanticChanges(currentConfig, candidateConfig)
	result.RequiresRestart = len(result.SemanticChanges) != 0
	return result, nil
}

func (s *Supervisor) PatchRuntimeNetwork(
	baseFingerprint, candidate string,
	patch NetworkCandidatePatch,
) (RuntimeConfigValidation, error) {
	network := DesktopSettings{
		NetworkMode: patch.Mode, PrimaryOrigin: strings.TrimSpace(patch.PrimaryOrigin),
		Listen: strings.TrimSpace(patch.Listen), TrustedProxyCIDRs: patch.TrustedProxyCIDRs,
	}
	if patch.AcceptLANWarning {
		network.LANHTTPWarningAcceptedVersion = lanHTTPWarningCurrentVersion
	}
	if patch.Mode == NetworkExternalHTTPS {
		switch patch.ProxyLocation {
		case "same_host":
			if network.Listen == "" {
				network.Listen = "127.0.0.1:6680"
			}
			network.TrustedProxyCIDRs = []string{"127.0.0.1/32", "::1/128"}
		case "remote":
			if network.Listen == "" {
				network.Listen = "0.0.0.0:6680"
			}
		default:
			return RuntimeConfigValidation{
				BaseFingerprint: baseFingerprint,
				Issues: []ConfigIssue{{
					Field: "server.proxy", Code: "invalid_proxy_location",
					Message: "external HTTPS requires a same-host or remote proxy location",
				}},
			}, nil
		}
	}
	normalized, err := normalizeNetworkSettings(network)
	if err != nil {
		return RuntimeConfigValidation{
			BaseFingerprint: baseFingerprint,
			Issues: []ConfigIssue{{
				Field: "server", Code: "invalid_network", Message: err.Error(),
			}},
		}, nil
	}
	document, err := parseRuntimeDocument([]byte(candidate))
	if err != nil {
		return RuntimeConfigValidation{
			BaseFingerprint: baseFingerprint,
			Issues:          []ConfigIssue{{Code: "invalid_toml", Message: err.Error()}},
		}, nil
	}
	tlsMode, proxyMode := "off", "disabled"
	if normalized.NetworkMode == NetworkExternalHTTPS {
		tlsMode, proxyMode = "external", "required"
	}
	setRuntimePath(document, "server.listen", normalized.Listen)
	setRuntimePath(document, "server.primary_origin", normalized.PrimaryOrigin)
	setRuntimePath(document, "server.tls.mode", tlsMode)
	setRuntimePath(document, "server.proxy.mode", proxyMode)
	setRuntimePath(document, "server.proxy.trusted_cidrs", normalized.TrustedProxyCIDRs)
	updated, err := toml.Marshal(document)
	if err != nil {
		return RuntimeConfigValidation{}, err
	}
	return s.ValidateRuntimeConfig(baseFingerprint, string(updated))
}

func runtimeSemanticChanges(before, after serverconfig.AppConfig) []SemanticChange {
	values := []struct {
		field         string
		before, after string
	}{
		{"network.mode", string(networkSummaryFromConfig(before).Mode), string(networkSummaryFromConfig(after).Mode)},
		{"server.listen", before.ServerConfig.Listen, after.ServerConfig.Listen},
		{"server.primary_origin", before.ServerConfig.PrimaryOrigin, after.ServerConfig.PrimaryOrigin},
		{"auth.passkey.rp_id", before.Auth.PasskeyIdentity.RPID, after.Auth.PasskeyIdentity.RPID},
		{"auth.passkey.enabled", fmt.Sprint(before.Auth.Passkey.Enabled), fmt.Sprint(after.Auth.Passkey.Enabled)},
		{"logging.level", before.LoggingConfig.Level, after.LoggingConfig.Level},
		{"repository_scan", fmt.Sprint(before.RepositoryScan), fmt.Sprint(after.RepositoryScan)},
		{"transcode.hardware_accel", before.Transcode.HardwareAccel, after.Transcode.HardwareAccel},
	}
	changes := make([]SemanticChange, 0, len(values))
	for _, value := range values {
		if value.before != value.after {
			changes = append(changes, SemanticChange{
				Field: value.field, Before: value.before, After: value.after,
			})
		}
	}
	return changes
}
