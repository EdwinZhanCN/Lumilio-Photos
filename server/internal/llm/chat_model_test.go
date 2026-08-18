package llm

import (
	"context"
	"testing"

	"server/internal/settings"
)

func TestNewChatModelRejectsAmbientCredentials(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "ambient-openai")
	t.Setenv("DEEPSEEK_API_KEY", "ambient-deepseek")
	t.Setenv("ANTHROPIC_API_KEY", "ambient-anthropic")
	t.Setenv("GEMINI_API_KEY", "ambient-gemini")
	t.Setenv("GOOGLE_API_KEY", "ambient-google")

	_, err := NewChatModel(context.Background(), settings.LLM{
		Provider:  "deepseek",
		ModelName: "deepseek-chat",
		BaseURL:   "https://api.deepseek.example/v1",
	})
	if err == nil {
		t.Fatal("NewChatModel() accepted an ambient credential instead of rejecting an empty stored API key")
	}
}

func TestDeepSeekUsesNativeAdapter(t *testing.T) {
	chatModel, err := newProviderChatModel(context.Background(), settings.LLM{
		Provider:  "deepseek",
		APIKey:    "stored-secret",
		ModelName: "deepseek-chat",
		BaseURL:   "https://api.deepseek.example/v1",
	})
	if err != nil {
		t.Fatalf("newProviderChatModel() error = %v", err)
	}

	typed, ok := chatModel.(interface{ GetType() string })
	if !ok {
		t.Fatalf("DeepSeek model type %T does not expose component identity", chatModel)
	}
	if got := typed.GetType(); got != "DeepSeek" {
		t.Fatalf("DeepSeek component identity = %q, want %q", got, "DeepSeek")
	}
}

func TestSupportedProvidersUseTheirNativeAdapters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		provider string
		wantType string
		baseURL  string
	}{
		{provider: settings.LLMProviderArk, wantType: "Ark"},
		{provider: settings.LLMProviderOpenAI, wantType: "OpenAI"},
		{provider: settings.LLMProviderDeepSeek, wantType: "DeepSeek", baseURL: "https://deepseek.example/v1"},
		{provider: settings.LLMProviderOllama, wantType: "Ollama", baseURL: "http://ollama.example"},
		{provider: settings.LLMProviderClaude, wantType: "Claude"},
		{provider: settings.LLMProviderGemini, wantType: "Gemini"},
		{provider: settings.LLMProviderQwen, wantType: "Qwen", baseURL: "https://dashscope.example/v1"},
		{provider: settings.LLMProviderOpenRouter, wantType: "OpenRouter"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			t.Parallel()
			apiKey := "stored-secret"
			if tt.provider == settings.LLMProviderOllama {
				apiKey = ""
			}
			chatModel, err := NewChatModel(context.Background(), settings.LLM{
				Provider:  tt.provider,
				APIKey:    apiKey,
				ModelName: "test-model",
				BaseURL:   tt.baseURL,
			})
			if err != nil {
				t.Fatalf("NewChatModel() error = %v", err)
			}
			typed, ok := chatModel.(interface{ GetType() string })
			if !ok {
				t.Fatalf("model type %T does not expose component identity", chatModel)
			}
			if got := typed.GetType(); got != tt.wantType {
				t.Fatalf("component identity = %q, want %q", got, tt.wantType)
			}
		})
	}
}
