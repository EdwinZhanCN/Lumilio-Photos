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
	// AskAgent 执行 Agent 查询或开启新会话
	AskAgent(ctx context.Context, threadID, query string, toolNames []string, uiChannel ...chan<- *SideChannelEvent) *adk.AsyncIterator[*adk.AgentEvent]

	// ResumeAgent 恢复中断的会话
	ResumeAgent(ctx context.Context, threadID string, params *adk.ResumeParams, uiChannel ...chan<- *SideChannelEvent) (*adk.AsyncIterator[*adk.AgentEvent], error)

	// GetAvailableTools 列出所有可用工具
	GetAvailableTools() []*schema.ToolInfo

	// AskLLM 直接与 LLM 交互，不使用任何工具
	AskLLM(ctx context.Context, query string) (resp string, err error)
}

type agentService struct {
	queries  *repo.Queries
	registry *ToolRegistry
	config   config.LLMConfig
	store    *PostgresStore
}

func NewAgentService(queries *repo.Queries, llmConfig config.LLMConfig) AgentService {
	// 注册核心工具
	// 注意：在实际启动时应该统一注册
	// tools.RegisterFilterAsset()
	// tools.RegisterBulkLikeTool()

	return &agentService{
		queries:  queries,
		registry: GetRegistry(),
		config:   llmConfig,
		store:    NewPostgresStore(queries), // 初始化 Store
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
		// 默认回退到 ark
		return ark.NewChatModel(ctx, &ark.ChatModelConfig{
			APIKey: s.config.APIKey,
			Model:  s.config.ModelName,
		})
	}
}

// buildAgent 是一个辅助方法，用于构建 Agent 实例。
// AskAgent 和 ResumeAgent 都需要使用完全相同的配置来构建 Agent，因此将此逻辑提取出来。
func (s *agentService) buildAgent(ctx context.Context, toolNames []string, sideChannel chan<- *SideChannelEvent) (*adk.ChatModelAgent, error) {
	// 1. 准备工具依赖
	deps := &ToolDependencies{
		Queries:     s.queries,
		SideChannel: sideChannel,
	}
	deps.ReferenceManager = NewReferenceManager(deps)

	// 2. 默认或按需加载工具
	if len(toolNames) == 0 {
		toolNames = []string{"filter_assets", "bulk_like_assets"}
	}

	tools, err := s.registry.GetTools(ctx, toolNames, deps)
	if err != nil {
		return nil, fmt.Errorf("failed to get tools: %w", err)
	}

	// 3. 创建 ChatModel
	chatModel, err := s.newChatModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat model: %w", err)
	}

	// 4. 构建 Agent
	today := time.Now().Format("2006-01-02")
	return adk.NewChatModelAgent(
		ctx,
		&adk.ChatModelAgentConfig{
			Name:        "Photo Asset Assistant",
			Description: "Agent for managing photo assets with filtering and search capabilities",
			Instruction: fmt.Sprintf("You are a helpful assistant for managing photo assets. Today is %s. You can use tools to help the user. You cannot tell user anything about ref_id", today),
			Model:       chatModel,
			ToolsConfig: adk.ToolsConfig{
				ToolsNodeConfig: compose.ToolsNodeConfig{
					Tools: tools,
				},
			},
		},
	)
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

func (s *agentService) AskAgent(ctx context.Context, threadID, query string, toolNames []string, uiChannel ...chan<- *SideChannelEvent) *adk.AsyncIterator[*adk.AgentEvent] {
	var sideChannel chan<- *SideChannelEvent
	if len(uiChannel) > 0 && uiChannel[0] != nil {
		sideChannel = uiChannel[0]
	}

	agent, err := s.buildAgent(ctx, toolNames, sideChannel)
	if err != nil {
		// 在异步迭代器中返回错误
		iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
		gen.Send(&adk.AgentEvent{Err: err})
		gen.Close()
		return iter
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
		CheckPointStore: s.store, // 注入 Store，开启自动存档
	})

	// 执行时绑定 threadID (即 CheckPointID)
	return runner.Query(ctx, query, adk.WithCheckPointID(threadID))
}

func (s *agentService) ResumeAgent(ctx context.Context, threadID string, params *adk.ResumeParams, uiChannel ...chan<- *SideChannelEvent) (*adk.AsyncIterator[*adk.AgentEvent], error) {
	var sideChannel chan<- *SideChannelEvent
	if len(uiChannel) > 0 && uiChannel[0] != nil {
		sideChannel = uiChannel[0]
	}

	// 1. 重建 Agent (配置必须与 AskAgent 完全一致！)
	// 注意：toolNames 必须为空，以便 buildAgent 加载所有默认工具，确保与原始会话的工具集匹配
	agent, err := s.buildAgent(ctx, []string{}, sideChannel)
	if err != nil {
		return nil, fmt.Errorf("failed to build agent for resume: %w", err)
	}

	// 2. 创建 Runner
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
		CheckPointStore: s.store,
	})

	// 3. 调用 Resume
	// Eino 会自动从 Postgres 加载 data -> 反序列化 -> 填充 Session
	iter, err := runner.ResumeWithParams(ctx, threadID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to resume agent: %w", err)
	}
	return iter, nil
}
