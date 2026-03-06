package llm

import (
	"context"
	"errors"
	"strings"

	"server/config"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const (
	arkProvider      = "ark"
	openAIProvider   = "openai"
	deepseekProvider = "deepseek"
	ollamaProvider   = "ollama"
)

func NewChatModel(ctx context.Context, cfg config.LLMConfig) (model.ToolCallingChatModel, error) {
	provider := cfg.EffectiveProvider()
	modelName := strings.TrimSpace(cfg.ModelName)
	baseURL := strings.TrimSpace(cfg.BaseURL)
	apiKey := strings.TrimSpace(cfg.APIKey)

	switch provider {
	case openAIProvider:
		return openai.NewChatModel(ctx, &openai.ChatModelConfig{
			APIKey:  apiKey,
			Model:   modelName,
			BaseURL: baseURL,
		})
	case deepseekProvider:
		return deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
			APIKey:  apiKey,
			Model:   modelName,
			BaseURL: baseURL,
		})
	case arkProvider:
		return ark.NewChatModel(ctx, &ark.ChatModelConfig{
			APIKey:  apiKey,
			Model:   modelName,
			BaseURL: baseURL,
		})
	case ollamaProvider:
		return ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
			BaseURL: baseURL,
			Model:   modelName,
		})
	default:
		return ark.NewChatModel(ctx, &ark.ChatModelConfig{
			APIKey:  apiKey,
			Model:   modelName,
			BaseURL: baseURL,
		})
	}
}

func ValidateChatModel(ctx context.Context, cfg config.LLMConfig) error {
	if !cfg.IsConfigured() {
		return errors.New("llm settings are incomplete")
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
