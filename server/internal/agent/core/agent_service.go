package core

import (
	"context"
	"fmt"
	"server/config"
	pkgagent "server/internal/agent"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

const (
	arkProvider      = "ark"
	openAIProvider   = "openai"
	deepseekProvider = "deepseek"
)

type AgentService interface {
	AskLLM(ctx context.Context, query string) (resp string, err error)
	AskAgent(ctx context.Context, query string, toolNames []string) *adk.AsyncIterator[*adk.AgentEvent]
}

type agentService struct {
	config config.LLMConfig
	deps   *pkgagent.ToolDependencies
}

func NewLLMService(llmConfig config.LLMConfig, deps *pkgagent.ToolDependencies) (*agentService, error) {
	return &agentService{
		config: llmConfig,
		deps:   deps,
	}, nil
}

func (s *agentService) newChatModel(ctx context.Context) (model.ToolCallingChatModel, error) {
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

func (s *agentService) newDefaultChatModelAgent(ctx context.Context) (adk.Agent, error) {
	// 获取所有已注册的工具名称
	registry := pkgagent.GetRegistry()
	toolNames := registry.GetAllToolNames()

	// 从 Registry 构建所有工具
	tools, err := registry.BuildTools(ctx, toolNames, s.deps)
	if err != nil {
		return nil, fmt.Errorf("build tools error: %w", err)
	}

	// 创建 ChatModel
	chatModel, err := s.newChatModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("create chat model error: %w", err)
	}

	// 构建默认 Agent,包含所有工具
	agent, err := adk.NewChatModelAgent(
		ctx,
		&adk.ChatModelAgentConfig{
			Name:        "Default",
			Description: "Default Agent with all available tools",
			Instruction: "You are a helpful assistant for managing photo assets. You have access to various tools to filter, search, and manage photo assets. Use these tools to help users with their requests.",
			Model:       chatModel,
			ToolsConfig: adk.ToolsConfig{
				ToolsNodeConfig: compose.ToolsNodeConfig{
					Tools: tools,
				},
			},
		},
	)

	if err != nil {
		return nil, fmt.Errorf("create agent error: %w", err)
	}

	return agent, nil
}

func (s *agentService) AskLLM(ctx context.Context, query string) (resp string, err error) {

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

func (s *agentService) newAgentWithTools(ctx context.Context, toolNames []string) (adk.Agent, error) {
	registry := pkgagent.GetRegistry()

	// 从 Registry 构建指定的工具
	tools, err := registry.BuildTools(ctx, toolNames, s.deps)
	if err != nil {
		return nil, fmt.Errorf("build tools error: %w", err)
	}

	// 创建 ChatModel
	chatModel, err := s.newChatModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("create chat model error: %w", err)
	}

	// 构建自定义 Agent,只包含指定的工具
	agent, err := adk.NewChatModelAgent(
		ctx,
		&adk.ChatModelAgentConfig{
			Name:        "Custom",
			Description: "Custom Agent with selected tools",
			Instruction: "You are a helpful assistant for managing photo assets. Use the available tools to help users with their requests.",
			Model:       chatModel,
			ToolsConfig: adk.ToolsConfig{
				ToolsNodeConfig: compose.ToolsNodeConfig{
					Tools: tools,
				},
			},
		},
	)

	if err != nil {
		return nil, fmt.Errorf("create agent error: %w", err)
	}

	return agent, nil
}

func (s *agentService) newAgentRunner(ctx context.Context, toolNames []string) (*adk.Runner, error) {
	var agent adk.Agent
	var err error

	// 如果没有指定工具列表，使用默认 Agent(包含所有工具)
	if len(toolNames) == 0 {
		agent, err = s.newDefaultChatModelAgent(ctx)
		if err != nil {
			return nil, fmt.Errorf("create default agent: %w", err)
		}
	} else {
		// 使用指定的工具列表创建 Agent
		agent, err = s.newAgentWithTools(ctx, toolNames)
		if err != nil {
			return nil, fmt.Errorf("create agent with tools: %w", err)
		}
	}

	// 创建 Runner
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	})

	return runner, nil
}

func (s *agentService) AskAgent(ctx context.Context, query string, toolNames []string) *adk.AsyncIterator[*adk.AgentEvent] {
	// 创建 Runner
	// 注意: 这里如果失败会 panic,因为 AskAgent 接口无法返回 error
	// 调用方应该确保配置正确,或者在调用前先验证配置
	runner, err := s.newAgentRunner(ctx, toolNames)
	if err != nil {
		panic(fmt.Sprintf("failed to create agent runner: %v", err))
	}

	// 使用 Query 方法运行 Agent (接受字符串查询)
	// Query 会立即返回 AsyncIterator,事件会异步生成
	// 错误会通过 AgentEvent.Err 字段传递
	return runner.Query(ctx, query)
}
