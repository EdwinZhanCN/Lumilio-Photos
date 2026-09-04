package llm

import (
	"context"
	"errors"
	"strings"

	"server/internal/settings"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/openrouter"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"google.golang.org/genai"
)

const claudeMaxTokens = 4096

var (
	_ model.ToolCallingChatModel = (*ark.ChatModel)(nil)
	_ model.ToolCallingChatModel = (*openai.ChatModel)(nil)
	_ model.ToolCallingChatModel = (*deepseek.ChatModel)(nil)
	_ model.ToolCallingChatModel = (*ollama.ChatModel)(nil)
	_ model.ToolCallingChatModel = (*claude.ChatModel)(nil)
	_ model.ToolCallingChatModel = (*gemini.ChatModel)(nil)
	_ model.ToolCallingChatModel = (*qwen.ChatModel)(nil)
	_ model.ToolCallingChatModel = (*openrouter.ChatModel)(nil)
)

func NewChatModel(ctx context.Context, cfg settings.LLM, auditPaths ...string) (model.ToolCallingChatModel, error) {
	// Validate before constructing an SDK client. Several upstream SDKs accept
	// environment credentials when a key is empty; Lumilio credentials are
	// runtime settings and must never fall through to ambient process state.
	if err := cfg.ValidateConfiguration(); err != nil {
		return nil, err
	}
	inner, err := newProviderChatModel(ctx, cfg)
	if err != nil {
		return nil, sanitizeProviderError("construct", err)
	}
	auditPath := ""
	if len(auditPaths) > 0 {
		auditPath = strings.TrimSpace(auditPaths[0])
	}
	return maybeWrapAudit(sanitizeProviderErrors(inner), auditPath), nil
}

func newProviderChatModel(ctx context.Context, cfg settings.LLM) (model.ToolCallingChatModel, error) {
	provider := cfg.EffectiveProvider()
	modelName := strings.TrimSpace(cfg.ModelName)
	baseURL := strings.TrimSpace(cfg.BaseURL)
	apiKey := strings.TrimSpace(cfg.APIKey)

	switch provider {
	case settings.LLMProviderOpenAI:
		return openai.NewChatModel(ctx, &openai.ChatModelConfig{
			APIKey:  apiKey,
			Model:   modelName,
			BaseURL: baseURL,
		})
	case settings.LLMProviderDeepSeek:
		return deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
			APIKey:  apiKey,
			Model:   modelName,
			BaseURL: baseURL,
		})
	case settings.LLMProviderArk:
		return ark.NewChatModel(ctx, &ark.ChatModelConfig{
			APIKey:  apiKey,
			Model:   modelName,
			BaseURL: baseURL,
		})
	case settings.LLMProviderOllama:
		return ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
			BaseURL: baseURL,
			Model:   modelName,
		})
	case settings.LLMProviderClaude:
		var endpoint *string
		if baseURL != "" {
			endpoint = &baseURL
		}
		return claude.NewChatModel(ctx, &claude.Config{
			APIKey:    apiKey,
			Model:     modelName,
			BaseURL:   endpoint,
			MaxTokens: claudeMaxTokens,
		})
	case settings.LLMProviderGemini:
		client, err := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  apiKey,
			Backend: genai.BackendGeminiAPI,
			HTTPOptions: genai.HTTPOptions{
				BaseURL: baseURL,
			},
		})
		if err != nil {
			return nil, err
		}
		return gemini.NewChatModel(ctx, &gemini.Config{
			Client: client,
			Model:  modelName,
		})
	case settings.LLMProviderQwen:
		return qwen.NewChatModel(ctx, &qwen.ChatModelConfig{
			APIKey:  apiKey,
			Model:   modelName,
			BaseURL: baseURL,
		})
	case settings.LLMProviderOpenRouter:
		return openrouter.NewChatModel(ctx, &openrouter.Config{
			APIKey:  apiKey,
			Model:   modelName,
			BaseURL: baseURL,
		})
	default:
		return nil, errors.New("unsupported or missing llm provider")
	}
}

func ValidateChatModel(ctx context.Context, cfg settings.LLM) error {
	if err := cfg.ValidateConfiguration(); err != nil {
		return err
	}

	chatModel, err := NewChatModel(ctx, cfg)
	if err != nil {
		return err
	}

	_, err = chatModel.Generate(ctx, []*schema.Message{
		schema.SystemMessage("Reply with OK only."),
		schema.UserMessage("OK"),
	})
	return err
}
