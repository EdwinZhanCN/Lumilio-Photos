package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"server/internal/agent/ref"
	"server/internal/db/repo"
	"server/internal/llm"
	"server/internal/settings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type LLMConfigProvider interface {
	GetLLMConfig(ctx context.Context) (settings.LLM, error)
}

type AgentService interface {
	// AskAgent 执行 Agent 查询或开启新会话。userID 与 threadID 共同构成 ref 作用域（INV-4）。
	// instructionExtras is appended to the agent instruction (context/mention bindings).
	// mode selects a tool subset: "review" | "organize" | "analyze" | "curate" | "" (full).
	AskAgent(ctx context.Context, userID int32, threadID, query, instructionExtras, mode string, sideChannels ...chan<- *SideChannelEvent) *adk.AsyncIterator[*adk.AgentEvent]

	// ResumeAgent 恢复中断的会话
	ResumeAgent(ctx context.Context, userID int32, threadID string, params *adk.ResumeParams, sideChannels ...chan<- *SideChannelEvent) (*adk.AsyncIterator[*adk.AgentEvent], error)

	// GetAvailableTools 列出所有可用工具
	GetAvailableTools() []*schema.ToolInfo

	// GetToolsByMode 列出某个 quick-action mode 下可见的工具；空/未知 mode 返回全量。
	GetToolsByMode(mode string) []*schema.ToolInfo
}

type agentService struct {
	queries        *repo.Queries
	registry       *ToolRegistry
	configProvider LLMConfigProvider
	store          *PostgresStore
	refStore       ref.Store
	search         RetrieverSearch
	conversations  *ConversationStore
}

func NewAgentService(queries *repo.Queries, configProvider LLMConfigProvider, refStore ref.Store, search RetrieverSearch, conversations *ConversationStore) AgentService {
	return &agentService{
		queries:        queries,
		registry:       GetRegistry(),
		configProvider: configProvider,
		store:          NewPostgresStore(queries),
		refStore:       refStore,
		search:         search,
		conversations:  conversations,
	}
}

func (s *agentService) GetAvailableTools() []*schema.ToolInfo {
	return s.registry.GetAllToolInfos()
}

func (s *agentService) GetToolsByMode(mode string) []*schema.ToolInfo {
	return s.registry.GetToolInfosByMode(mode)
}

func (s *agentService) newChatModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	cfg, err := s.configProvider.GetLLMConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load llm settings: %w", err)
	}

	return llm.NewChatModel(ctx, cfg)
}

// buildAgent 构建 Agent 实例。mode 为空时返回全量工具集（自由模式）；非空时
// 只实例化该 mode 允许的工具子集（渐进式披露）。checkpoint 恢复用当前请求的
// mode 重建 agent——旧工具调用结果已在消息历史中，不影响恢复。
func (s *agentService) buildAgent(ctx context.Context, userID int32, threadID string, instructionExtras, mode string, sideChannel chan<- *SideChannelEvent) (*adk.ChatModelAgent, error) {
	deps := &ToolDependencies{
		Queries:     s.queries,
		SideChannel: sideChannel,
		RefStore:    s.refStore,
		Search:      s.search,
		UserID:      userID,
		ThreadID:    threadID,
	}

	tools, err := s.registry.GetToolsByMode(ctx, deps, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to get tools: %w", err)
	}

	chatModel, err := s.newChatModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat model: %w", err)
	}

	// Summarization compacts the conversation in place once it crosses the
	// token budget; the session middleware then writes the (possibly
	// compacted) state back to the conversation store, so long threads stay
	// bounded without losing the recent exchange. Ref handles survive
	// compaction by construction — the ledger is rebuilt into the
	// instruction every turn from the ref store, not from messages.
	summarizer, err := summarization.New(ctx, &summarization.Config{
		Model:   chatModel,
		Trigger: &summarization.TriggerCondition{ContextTokens: summarizeTriggerTokens},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create summarization middleware: %w", err)
	}

	session := &sessionMiddleware{
		store:    s.conversations,
		userID:   userID,
		threadID: threadID,
		onUsage: func(usage *schema.TokenUsage) {
			if sideChannel == nil || usage == nil {
				return
			}
			deps.Send(&SideChannelEvent{
				Type:      EventTypeTokenUsage,
				Timestamp: time.Now().UnixMilli(),
				Usage: &TokenUsageInfo{
					PromptTokens:     usage.PromptTokens,
					CompletionTokens: usage.CompletionTokens,
					TotalTokens:      usage.TotalTokens,
				},
			})
		},
	}

	t := time.Now()
	today := fmt.Sprintf("%s, %s", t.Weekday().String(), t.Format("2006-01-02"))
	ledger := s.refStore.List(ref.Scope{UserID: userID, ThreadID: threadID})
	return adk.NewChatModelAgent(
		ctx,
		&adk.ChatModelAgentConfig{
			Name:        "Photo Asset Assistant",
			Description: "Agent for managing photo assets with filtering and search capabilities",
			Instruction: buildInstruction(today, ledger, mode) + instructionExtras,
			Model:       chatModel,
			ToolsConfig: adk.ToolsConfig{
				ToolsNodeConfig: compose.ToolsNodeConfig{
					Tools: tools,
				},
			},
			Handlers: []adk.ChatModelAgentMiddleware{summarizer, session},
		},
	)
}

// summarizeTriggerTokens is the context budget before the conversation is
// compacted. Conservative relative to common local-model windows; promote to
// an LLM settings knob when per-deployment tuning is needed.
const summarizeTriggerTokens = 60000

// buildInstruction states the ref discipline the model must follow: act
// through refs, verify counts, never recite internal ids or asset data.
// The ref ledger (one line per active ref) is rebuilt every turn from the
// store, so resumed conversations keep their handles without the model
// having to remember them.
//
// The instruction deliberately gives NO concrete ref id example: models will
// parrot a literal like "r3_kyoto" straight back as a fabricated ref. Instead
// it states the contract — refs come only from tool receipts and must never be
// invented — and the rule that a producer tool (filter/search) must run before
// any consumer tool (describe/rank/show/…) can act on a ref.
func buildInstruction(today string, ledger []*ref.Ref, mode string) string {
	hasRefs := len(ledger) > 0
	refAvailability := "At the start of a conversation you hold no refs."
	if hasRefs {
		refAvailability = "The refs you currently hold are listed under \"Active refs\" below."
	}

	organizing := ""
	if ModeHasTool(mode, "tag_assets") {
		organizing = "ORGANIZING:\n" +
			"- When the user wants to organize or label photos, use the tag_assets tool to add or remove tags on a ref.\n" +
			"- After showing a result the user seems interested in, offer to pin it to their board so they can revisit it later.\n\n"
	}

	instruction := fmt.Sprintf(
		"You are a helpful assistant for managing the user's photo library. Today is %s.\n\n"+
			"WORKING WITH PHOTOS (refs):\n"+
			"- A \"ref\" is an opaque server-side handle (an id plus a count) that stands for a set of photos. "+
			"You receive a ref only inside a tool's result.\n"+
			"- Never invent, guess, hand-write, recall from memory, or edit a ref id. "+
			"Only use a ref id that a tool returned earlier in THIS conversation, or one listed under \"Active refs\" below. "+
			"%s\n"+
			"- To obtain a ref, first call a producer tool (filter_assets, or a search_* tool). "+
			"Only after a producer returns a ref can the other tools (describe, rank, sample, show, …) act on it. "+
			"If you need photos and hold no suitable ref, call filter_assets or a search tool first — "+
			"never call describe/show/rank/etc. with a ref you were not given.\n"+
			"- Pass refs between tools instead of describing photos in text. "+
			"Read the count in each tool receipt before acting on a ref, and tell the user when a result is empty.\n\n"+
			"TALKING TO THE USER:\n"+
			"- Use the show tool to display photos — never enumerate or list photos in text.\n"+
			"- Never mention ref ids or other internal identifiers to the user; speak about results in plain language.\n"+
			"- Respond in the user's language.\n\n"+
			"%s"+
			"CHOOSING A SHOW WIDGET (by intent): number_card for a pure count or statistic, "+
			"cover_card for browsing a collection by its cover photo, spark_card for a time distribution, "+
			"mosaic_card for a visual thumbnail collage. When in doubt, use cover_card."+
			"%s",
		today,
		refAvailability,
		organizing,
		ModeInstruction(mode),
	)

	if hasRefs {
		var b strings.Builder
		b.WriteString(instruction)
		b.WriteString("\n\nActive refs from earlier in this conversation (use these exact ids; do not alter them):\n")
		for _, r := range ledger {
			summary := r.Summary
			if summary == "" {
				summary = r.Plan.Op
			}
			fmt.Fprintf(&b, "- %s: %d assets — %s\n", r.ID, r.Count(), summary)
		}
		return b.String()
	}
	return instruction
}

func (s *agentService) AskAgent(ctx context.Context, userID int32, threadID, query, instructionExtras, mode string, sideChannels ...chan<- *SideChannelEvent) *adk.AsyncIterator[*adk.AgentEvent] {
	var sideChannel chan<- *SideChannelEvent
	if len(sideChannels) > 0 && sideChannels[0] != nil {
		sideChannel = sideChannels[0]
	}

	agent, err := s.buildAgent(ctx, userID, threadID, instructionExtras, mode, sideChannel)
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

	// Multi-turn: replay the thread's history plus the new user message.
	// Query() would start a blank conversation every turn — the model would
	// never see prior exchanges. History comes back via sessionMiddleware.
	messages := append(s.conversations.Messages(userID, threadID), schema.UserMessage(query))
	return runner.Run(ctx, messages, adk.WithCheckPointID(threadID))
}

func (s *agentService) ResumeAgent(ctx context.Context, userID int32, threadID string, params *adk.ResumeParams, sideChannels ...chan<- *SideChannelEvent) (*adk.AsyncIterator[*adk.AgentEvent], error) {
	var sideChannel chan<- *SideChannelEvent
	if len(sideChannels) > 0 && sideChannels[0] != nil {
		sideChannel = sideChannels[0]
	}

	agent, err := s.buildAgent(ctx, userID, threadID, "", "", sideChannel)
	if err != nil {
		return nil, fmt.Errorf("failed to build agent for resume: %w", err)
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
		CheckPointStore: s.store,
	})

	// Eino 会自动从 Postgres 加载 data -> 反序列化 -> 填充 Session
	iter, err := runner.ResumeWithParams(ctx, threadID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to resume agent: %w", err)
	}
	return iter, nil
}
