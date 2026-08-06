package settings

import "testing"

func TestLLMConfigurationRequiresExplicitSupportedProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  LLM
		ok   bool
	}{
		{name: "empty provider", cfg: LLM{APIKey: "secret", ModelName: "model"}},
		{name: "unknown provider", cfg: LLM{Provider: "other", APIKey: "secret", ModelName: "model"}},
		{name: "openai", cfg: LLM{Provider: "openai", APIKey: "secret", ModelName: "model"}, ok: true},
		{name: "deepseek requires URL", cfg: LLM{Provider: "deepseek", APIKey: "secret", ModelName: "model"}},
		{name: "deepseek", cfg: LLM{Provider: "deepseek", APIKey: "secret", ModelName: "model", BaseURL: "https://deepseek.example/v1"}, ok: true},
		{name: "ollama requires URL", cfg: LLM{Provider: "ollama", ModelName: "model"}},
		{name: "ollama", cfg: LLM{Provider: " OLLAMA ", ModelName: "model", BaseURL: "http://localhost:11434"}, ok: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.ValidateConfiguration()
			if tt.ok && err != nil {
				t.Fatalf("ValidateConfiguration() error = %v", err)
			}
			if !tt.ok && err == nil {
				t.Fatal("ValidateConfiguration() unexpectedly succeeded")
			}
			if got := tt.cfg.IsConfigured(); got != tt.ok {
				t.Fatalf("IsConfigured() = %t, want %t", got, tt.ok)
			}
		})
	}
}
