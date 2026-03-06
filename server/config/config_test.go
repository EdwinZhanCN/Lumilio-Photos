package config

import "testing"

func TestLoadMLConfig_AutoEnableKeepsRawTaskFlags(t *testing.T) {
	t.Setenv("SERVER_ENV", "development")
	t.Setenv("ML_AUTO", "enable")
	t.Setenv("ML_CLIP_ENABLED", "false")
	t.Setenv("ML_OCR_ENABLED", "false")
	t.Setenv("ML_CAPTION_ENABLED", "false")
	t.Setenv("ML_FACE_ENABLED", "false")

	cfg := LoadMLConfig()

	if !cfg.IsAutoEnabled() {
		t.Fatalf("expected auto mode enabled, got %q", cfg.AutoMode)
	}
	if cfg.CLIPEnabled || cfg.OCREnabled || cfg.CaptionEnabled || cfg.FaceEnabled {
		t.Fatalf("expected raw ML task switches preserved in auto mode, got %+v", cfg)
	}
}

func TestLoadMLConfig_DisableRespectsExplicitFlags(t *testing.T) {
	t.Setenv("SERVER_ENV", "development")
	t.Setenv("ML_AUTO", "disable")
	t.Setenv("ML_CLIP_ENABLED", "true")
	t.Setenv("ML_OCR_ENABLED", "false")
	t.Setenv("ML_CAPTION_ENABLED", "false")
	t.Setenv("ML_FACE_ENABLED", "false")

	cfg := LoadMLConfig()

	if cfg.IsAutoEnabled() {
		t.Fatalf("expected auto mode disabled, got %q", cfg.AutoMode)
	}
	if !cfg.CLIPEnabled {
		t.Fatalf("expected clip enabled, got %+v", cfg)
	}
	if cfg.OCREnabled || cfg.CaptionEnabled || cfg.FaceEnabled {
		t.Fatalf("expected non-clip tasks disabled, got %+v", cfg)
	}
}

func TestLoadMLConfig_InvalidAutoFallsBackToDisable(t *testing.T) {
	t.Setenv("SERVER_ENV", "development")
	t.Setenv("ML_AUTO", "unexpected-value")
	t.Setenv("ML_CLIP_ENABLED", "false")
	t.Setenv("ML_OCR_ENABLED", "false")
	t.Setenv("ML_CAPTION_ENABLED", "false")
	t.Setenv("ML_FACE_ENABLED", "false")

	cfg := LoadMLConfig()
	if cfg.IsAutoEnabled() {
		t.Fatalf("expected invalid ML_AUTO to fall back to disable, got %q", cfg.AutoMode)
	}
}

func TestMLConfig_EffectiveRuntimeConfig_AutoEnableIgnoresManualFlags(t *testing.T) {
	cfg := MLConfig{
		AutoMode:       MLAutoModeEnable,
		CLIPEnabled:    false,
		OCREnabled:     false,
		CaptionEnabled: false,
		FaceEnabled:    false,
	}

	effective := cfg.EffectiveRuntimeConfig()

	if !effective.CLIPEnabled || !effective.OCREnabled || !effective.CaptionEnabled || !effective.FaceEnabled {
		t.Fatalf("expected runtime config to enable all ML tasks in auto mode, got %+v", effective)
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
