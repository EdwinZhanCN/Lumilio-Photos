package config

import "testing"

func TestLoadMLConfig_RespectsExplicitFlags(t *testing.T) {
	t.Setenv("SERVER_ENV", "development")
	t.Setenv("ML_CLIP_ENABLED", "true")
	t.Setenv("ML_OCR_ENABLED", "false")
	t.Setenv("ML_CAPTION_ENABLED", "false")
	t.Setenv("ML_FACE_ENABLED", "false")

	cfg := LoadMLConfig()

	if !cfg.CLIPEnabled {
		t.Fatalf("expected clip enabled, got %+v", cfg)
	}
	if cfg.OCREnabled || cfg.CaptionEnabled || cfg.FaceEnabled {
		t.Fatalf("expected non-clip tasks disabled, got %+v", cfg)
	}
}

func TestMLConfig_HasRuntimeDemandReflectsTaskFlags(t *testing.T) {
	cfg := MLConfig{
		CLIPEnabled:    false,
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

func TestLoadWatchmanConfig_DefaultPollFallbackEnabled(t *testing.T) {
	t.Setenv("WATCHMAN_ENABLED", "")
	t.Setenv("WATCHMAN_SOCK", "")
	t.Setenv("WATCHMAN_SETTLE_SECONDS", "")
	t.Setenv("WATCHMAN_INITIAL_SCAN", "")
	t.Setenv("WATCHMAN_POLL_FALLBACK_SECONDS", "")

	cfg := LoadWatchmanConfig()

	if cfg.PollFallbackSeconds != 10 {
		t.Fatalf("expected default poll fallback of 10 seconds, got %d", cfg.PollFallbackSeconds)
	}
	if !cfg.InitialScan {
		t.Fatalf("expected initial scan enabled by default")
	}
}
