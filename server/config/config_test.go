package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMLConfig_RespectsExplicitFlags(t *testing.T) {
	t.Setenv("SERVER_ENV", "development")
	t.Setenv("ML_SEMANTIC_ENABLED", "true")
	t.Setenv("ML_BIOCLIP_ENABLED", "false")
	t.Setenv("ML_OCR_ENABLED", "false")
	t.Setenv("ML_FACE_ENABLED", "false")

	cfg := LoadMLConfig()

	if !cfg.SemanticEnabled {
		t.Fatalf("expected semantic enabled, got %+v", cfg)
	}
	if cfg.BioCLIPEnabled || cfg.OCREnabled || cfg.FaceEnabled {
		t.Fatalf("expected non-semantic tasks disabled, got %+v", cfg)
	}
}

func TestMLConfig_HasRuntimeDemandReflectsTaskFlags(t *testing.T) {
	cfg := MLConfig{
		SemanticEnabled:    false,
		BioCLIPEnabled: false,
		OCREnabled:         false,
		FaceEnabled:        false,
	}

	if cfg.HasRuntimeDemand() {
		t.Fatalf("expected no runtime demand when all ML tasks are disabled, got %+v", cfg)
	}
	cfg.FaceEnabled = true
	if !cfg.HasRuntimeDemand() {
		t.Fatalf("expected runtime demand when a task is enabled, got %+v", cfg)
	}
}

func TestLLMConfig_IsConfigured_APIKeyProviders(t *testing.T) {
	cfg := LLMConfig{
		Provider:  "openai",
		APIKey:    "sk-test",
		ModelName: "gpt-4.1-mini",
	}

	if !cfg.IsConfigured() {
		t.Fatalf("expected api-key provider config to be configured, got %+v", cfg)
	}
}

func TestLLMConfig_IsConfigured_OllamaRequiresBaseURL(t *testing.T) {
	cfg := LLMConfig{
		Provider:  "ollama",
		ModelName: "qwen3:latest",
	}

	if cfg.IsConfigured() {
		t.Fatalf("expected ollama without base url to be unconfigured, got %+v", cfg)
	}
}

func TestLoadRepositoryScanConfig_Defaults(t *testing.T) {
	t.Setenv("REPOSITORY_SCAN_ENABLED", "")
	t.Setenv("REPOSITORY_SCAN_INTERVAL_SECONDS", "")
	t.Setenv("REPOSITORY_SCAN_SETTLE_SECONDS", "")
	t.Setenv("REPOSITORY_SCAN_MAX_CONCURRENT_REPOS", "")
	t.Setenv("REPOSITORY_SCAN_BATCH_SIZE", "")

	cfg := LoadRepositoryScanConfig()

	if !cfg.Enabled {
		t.Fatalf("expected repository scan enabled by default")
	}
	if cfg.IntervalSeconds != 300 {
		t.Fatalf("expected default interval of 300 seconds, got %d", cfg.IntervalSeconds)
	}
	if cfg.SettleSeconds != 5 {
		t.Fatalf("expected default settle of 5 seconds, got %d", cfg.SettleSeconds)
	}
	if cfg.MaxConcurrentRepos != 1 {
		t.Fatalf("expected default max concurrent repos of 1, got %d", cfg.MaxConcurrentRepos)
	}
	if cfg.BatchSize != 500 {
		t.Fatalf("expected default batch size of 500, got %d", cfg.BatchSize)
	}
}

func TestLoadRepositoryScanConfig_EnvOverrides(t *testing.T) {
	t.Setenv("REPOSITORY_SCAN_ENABLED", "false")
	t.Setenv("REPOSITORY_SCAN_INTERVAL_SECONDS", "60")
	t.Setenv("REPOSITORY_SCAN_SETTLE_SECONDS", "2")
	t.Setenv("REPOSITORY_SCAN_MAX_CONCURRENT_REPOS", "3")
	t.Setenv("REPOSITORY_SCAN_BATCH_SIZE", "25")

	cfg := LoadRepositoryScanConfig()

	if cfg.Enabled {
		t.Fatalf("expected repository scan disabled")
	}
	if cfg.IntervalSeconds != 60 || cfg.SettleSeconds != 2 || cfg.MaxConcurrentRepos != 3 || cfg.BatchSize != 25 {
		t.Fatalf("unexpected config overrides: %+v", cfg)
	}
}

func TestLoadAppConfigWithError_LoadsExplicitTOML(t *testing.T) {
	configFile := writeTestConfig(t, `
environment = "development"

[server]
port = "7777"
log_level = "debug"
cors_allowed_origins = ["http://localhost:6657"]

[database]
host = "db-from-toml"
port = "5432"
user = "postgres"
password = "postgres"
password_file = "/tmp/lumilio-db-password"
name = "lumiliophotos"
ssl = "disable"

[storage]
path = "/tmp/lumilio"
strategy = "date"
preserve_filename = true
duplicate_handling = "rename"

[repository_scan]
enabled = false
interval_seconds = 120
settle_seconds = 3
max_concurrent_repos = 2
batch_size = 250

[geocoding]
provider = "nominatim"
nominatim_endpoint = "https://example.invalid/reverse"
language = "zh"
user_agent = "Lumilio-Test/1.0"

[ml]
semantic_enabled = true
bioclip_enabled = false
ocr_enabled = false
face_enabled = true

[auth]
secret_key_path = "/tmp/secret"
access_token_ttl = "10m"
refresh_token_ttl = "24h"
media_token_ttl = "5m"

[transcode]
hardware_accel = "auto"
`)
	t.Setenv("SERVER_CONFIG_FILE", configFile)
	t.Setenv("SERVER_ENV", "")

	cfg, err := LoadAppConfigWithError()
	if err != nil {
		t.Fatalf("expected config to load: %v", err)
	}

	if cfg.ServerConfig.Port != "7777" {
		t.Fatalf("expected server port from toml, got %+v", cfg.ServerConfig)
	}
	if cfg.DatabaseConfig.Host != "db-from-toml" {
		t.Fatalf("expected database host from toml, got %+v", cfg.DatabaseConfig)
	}
	if cfg.DatabaseConfig.PasswordFile != "/tmp/lumilio-db-password" {
		t.Fatalf("expected database password_file from toml, got %+v", cfg.DatabaseConfig)
	}
	if cfg.RepositoryScan.Enabled || cfg.RepositoryScan.IntervalSeconds != 120 {
		t.Fatalf("expected repository scan from toml, got %+v", cfg.RepositoryScan)
	}
	if cfg.Geocoding.Provider != "nominatim" || cfg.Geocoding.Language != "zh" {
		t.Fatalf("expected geocoding from toml, got %+v", cfg.Geocoding)
	}
	if !cfg.MLConfig.SemanticEnabled || !cfg.MLConfig.FaceEnabled {
		t.Fatalf("expected ml flags from toml, got %+v", cfg.MLConfig)
	}
}

func TestLoadAppConfigWithError_EnvOverridesTOML(t *testing.T) {
	configFile := writeTestConfig(t, `
[server]
port = "7777"
log_level = "info"

[database]
host = "db-from-toml"

[storage]
path = "/toml/storage"

[repository_scan]
enabled = true
interval_seconds = 300
settle_seconds = 5
max_concurrent_repos = 1
batch_size = 500

[ml]
semantic_enabled = false
`)
	t.Setenv("SERVER_CONFIG_FILE", configFile)
	t.Setenv("SERVER_PORT", "9999")
	t.Setenv("DB_HOST", "db-from-env")
	t.Setenv("DB_PASSWORD_FILE", "/tmp/env-db-password")
	t.Setenv("STORAGE_PATH", "/env/storage")
	t.Setenv("REPOSITORY_SCAN_ENABLED", "false")
	t.Setenv("ML_SEMANTIC_ENABLED", "true")

	cfg, err := LoadAppConfigWithError()
	if err != nil {
		t.Fatalf("expected config to load: %v", err)
	}

	if cfg.ServerConfig.Port != "9999" {
		t.Fatalf("expected env server port override, got %s", cfg.ServerConfig.Port)
	}
	if cfg.DatabaseConfig.Host != "db-from-env" {
		t.Fatalf("expected env database host override, got %s", cfg.DatabaseConfig.Host)
	}
	if cfg.DatabaseConfig.PasswordFile != "/tmp/env-db-password" {
		t.Fatalf("expected env database password file override, got %s", cfg.DatabaseConfig.PasswordFile)
	}
	if cfg.StorageConfig.Path != "/env/storage" {
		t.Fatalf("expected env storage path override, got %s", cfg.StorageConfig.Path)
	}
	if cfg.RepositoryScan.Enabled {
		t.Fatalf("expected env repository scan override, got %+v", cfg.RepositoryScan)
	}
	if !cfg.MLConfig.SemanticEnabled {
		t.Fatalf("expected env ML override, got %+v", cfg.MLConfig)
	}
}

func TestLoadAppConfigWithError_LoadsExampleTOML(t *testing.T) {
	t.Setenv("SERVER_CONFIG_FILE", "server.example.toml")
	t.Setenv("SERVER_ENV", "development")

	cfg, err := LoadAppConfigWithError()
	if err != nil {
		t.Fatalf("expected example config to load: %v", err)
	}

	if cfg.ServerConfig.Port != "6680" {
		t.Fatalf("expected example server port, got %s", cfg.ServerConfig.Port)
	}
	if cfg.DatabaseConfig.DBName != "lumiliophotos" {
		t.Fatalf("expected example database name, got %s", cfg.DatabaseConfig.DBName)
	}
	if cfg.Auth.SecretKeyPath == "" {
		t.Fatalf("expected example auth secret path")
	}
}

func TestConfigFilePathPrefersLocalConfig(t *testing.T) {
	t.Setenv("SERVER_CONFIG_FILE", "")
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	localPath := filepath.Join(configDir, "server.local.toml")
	if err := os.WriteFile(localPath, []byte("[server]\nport = \"6680\"\n"), 0o600); err != nil {
		t.Fatalf("write local config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "server.development.toml"), []byte("[server]\nport = \"7777\"\n"), 0o600); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	path, explicit := configFilePath("development")

	if explicit {
		t.Fatalf("expected discovered config, got explicit")
	}
	if path != filepath.Join("config", "server.local.toml") {
		t.Fatalf("expected local config path, got %s", path)
	}
}

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "server.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write test config: %v", err)
	}
	return path
}
