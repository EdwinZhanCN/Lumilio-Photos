package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"server/config"
	"server/internal/db/repo"

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
	// AskAgent 执行 Agent 查询
	// uiChannel: 可选，用于接收工具产生的 UI 指令/数据
	AskAgent(ctx context.Context, query string, toolNames []string, uiChannel ...chan<- *SideChannelEvent) *adk.AsyncIterator[*adk.AgentEvent]

	// GetAvailableTools 列出所有可用工具
	GetAvailableTools() []*schema.ToolInfo

	// AskLLM 直接与 LLM 交互，不使用任何工具
	AskLLM(ctx context.Context, query string) (resp string, err error)
}

type agentService struct {
	queries  *repo.Queries
	registry *ToolRegistry
	config   config.LLMConfig
}

func NewAgentService(queries *repo.Queries, llmConfig config.LLMConfig) AgentService {
	// 注册核心工具
	// 注意：在实际启动时应该统一注册，这里为了 demo 确保注册
	// tools.RegisterFilterAsset() // 假设在 main 或 init 中已调用

	return &agentService{
		queries:  queries,
		registry: GetRegistry(),
		config:   llmConfig,
	}
}

func (s *agentService) GetAvailableTools() []*schema.ToolInfo {
	return s.registry.GetAllToolInfos()
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

func (s *agentService) AskAgent(ctx context.Context, query string, toolNames []string, uiChannel ...chan<- *SideChannelEvent) *adk.AsyncIterator[*adk.AgentEvent] {
	// 1. 准备工具依赖
	// Handle optional uiChannel parameter
	var sideChannel chan<- *SideChannelEvent
	if len(uiChannel) > 0 && uiChannel[0] != nil {
		sideChannel = uiChannel[0]
	}

	// 2. 创建 ReferenceManager（用于跨工具存储和引用数据）
	deps := &ToolDependencies{
		Queries:     s.queries,
		SideChannel: sideChannel,
	}
	deps.ReferenceManager = NewReferenceManager(deps)

	// 3. 创建 ToolInputExtractor（用于自动转换 ref_id）
	deps.InputExtractor = NewToolInputExtractor(deps.ReferenceManager)

	// 2. 默认使用所有工具，或者根据请求过滤
	if len(toolNames) == 0 {
		// 默认加载常用工具
		toolNames = []string{"filter_assets"}
	}

	// 3. 获取工具实例 (请求级隔离)
	tools, err := s.registry.GetTools(ctx, toolNames, deps)
	if err != nil {
		panic(fmt.Sprintf("failed to get tools: %v", err))
	}

	// 4. 创建 ChatModel
	chatModel, err := s.newChatModel(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to create chat model: %v", err))
	}

	// 5. 构建 Agent
	today := time.Now().Format("2006-01-02")
	agent, err := adk.NewChatModelAgent(
		ctx,
		&adk.ChatModelAgentConfig{
			Name:        "Photo Asset Assistant",
			Description: "Agent for managing photo assets with filtering and search capabilities",
			Instruction: fmt.Sprintf("You are a helpful assistant for managing photo assets. Today is %s", today),
			Model:       chatModel,
			ToolsConfig: adk.ToolsConfig{
				ToolsNodeConfig: compose.ToolsNodeConfig{
					Tools: tools,
				},
			},
		},
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create agent: %v", err))
	}

	// 6. 创建 Runner
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	})

	// 7. 执行查询
	return runner.Query(ctx, query)
}
