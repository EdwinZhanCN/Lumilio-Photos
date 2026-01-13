package service

import (
	"context"
	"fmt"
	"server/config"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const (
	arkProvider      = "ark"
	openAIProvider   = "openai"
	deepseekProvider = "deepseek"
)

type LLMService interface {
	AskLLM(ctx context.Context, query string) (resp string, err error)
}

type LLMChatModel interface {
	Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error)
}

type LLmService struct {
	config config.LLMConfig
}

func NewLLMService(llmConfig config.LLMConfig) (*LLmService, error) {
	return &LLmService{
		config: llmConfig,
	}, nil
}

func (s *LLmService) newChatModel(ctx context.Context) (LLMChatModel, error) {
	switch strings.ToLower(s.config.Provider) {
	case openAIProvider:
		return openai.NewChatModel(ctx, &openai.ChatModelConfig{
			APIKey: s.config.APIKey,
			Model:  s.config.ModelName,
		})
	case deepseekProvider:
		return deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
			APIKey: s.config.APIKey,
			Model:  s.config.ModelName,
		})
	case arkProvider:
		return ark.NewChatModel(ctx, &ark.ChatModelConfig{
			APIKey: s.config.APIKey,
			Model:  s.config.ModelName,
		})
	default:
		// default fallback to ark
		return ark.NewChatModel(ctx, &ark.ChatModelConfig{
			APIKey: s.config.APIKey,
			Model:  s.config.ModelName,
		})
	}
}

func (s *LLmService) AskLLM(ctx context.Context, query string) (resp string, err error) {

	cm, err := s.newChatModel(ctx)
	if err != nil {
		return "", fmt.Errorf("create chat model error: %w", err)
	}

	input := []*schema.Message{
		schema.SystemMessage("You are a helpful assistant that should respond to user with their language unless specified."),
		schema.UserMessage(query),
	}

	response, err := cm.Generate(ctx, input)
	if err != nil {
		return "", fmt.Errorf("ask llm error: %w", err)
	}
	return response.Content, nil
}
