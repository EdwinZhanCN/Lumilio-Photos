package config

import "testing"

func TestLoadMLConfig_RespectsExplicitFlags(t *testing.T) {
	t.Setenv("SERVER_ENV", "development")
	t.Setenv("ML_CLIP_ENABLED", "true")
	t.Setenv("ML_BIOCLIP_ENABLED", "false")
	t.Setenv("ML_OCR_ENABLED", "false")
	t.Setenv("ML_CAPTION_ENABLED", "false")
	t.Setenv("ML_FACE_ENABLED", "false")

	cfg := LoadMLConfig()

	if !cfg.CLIPEnabled {
		t.Fatalf("expected clip enabled, got %+v", cfg)
	}
	if cfg.BioCLIPEnabled || cfg.OCREnabled || cfg.CaptionEnabled || cfg.FaceEnabled {
		t.Fatalf("expected non-clip tasks disabled, got %+v", cfg)
	}
}

func TestMLConfig_HasRuntimeDemandReflectsTaskFlags(t *testing.T) {
	cfg := MLConfig{
		CLIPEnabled:    false,
		BioCLIPEnabled: false,
		OCREnabled:     false,
		CaptionEnabled: false,
		FaceEnabled:    false,
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
