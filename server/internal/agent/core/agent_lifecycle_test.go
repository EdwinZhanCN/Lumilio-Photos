package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/settings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func TestADKLifecycleStreamsToolsUsageAndPersistsSession(t *testing.T) {
	usage := &schema.TokenUsage{PromptTokens: 1200, CompletionTokens: 80, TotalTokens: 1280}
	finalChunk := schema.AssistantMessage("answer", nil)
	finalChunk.ResponseMeta = &schema.ResponseMeta{Usage: usage}
	chatModel := &scriptedLifecycleModel{streamResponses: [][]*schema.Message{
		{schema.AssistantMessage("", []schema.ToolCall{{
			ID: "call-1", Type: "function",
			Function: schema.FunctionCall{Name: "fixture_lookup", Arguments: `{"query":"cats"}`},
		}})},
		{schema.AssistantMessage("final ", nil), finalChunk},
	}}
	conversation := NewConversationStore(0)
	var reportedUsage *schema.TokenUsage
	session := &sessionMiddleware{
		store: conversation, userID: 7, threadID: "stream-thread",
		onUsage: func(got *schema.TokenUsage) { reportedUsage = got },
	}

	agent, err := adk.NewChatModelAgent(context.Background(), &adk.ChatModelAgentConfig{
		Name: "lifecycle", Description: "lifecycle compatibility seam", Model: chatModel,
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{
			Tools: []tool.BaseTool{fixtureLifecycleTool{}},
		}},
		Handlers: []adk.ChatModelAgentMiddleware{session},
	})
	if err != nil {
		t.Fatalf("NewChatModelAgent() error = %v", err)
	}
	runner := adk.NewRunner(context.Background(), adk.RunnerConfig{Agent: agent, EnableStreaming: true})
	result := drainLifecycleEvents(t, runner.Run(context.Background(), []*schema.Message{
		schema.UserMessage("find cats"),
	}))
	if len(result.errs) > 0 {
		t.Fatalf("runner errors = %v", result.errs)
	}
	if !messagesContain(result.messages, "final answer") {
		t.Fatalf("streamed output = %#v, want final answer", result.messages)
	}

	history := conversation.Messages(7, "stream-thread")
	if !messagesHaveToolCall(history, "fixture_lookup") {
		t.Fatalf("persisted history lacks streamed tool call: %#v", history)
	}
	if !messagesContainRole(history, schema.Tool, "fixture tool result") {
		t.Fatalf("persisted history lacks tool result: %#v", history)
	}
	if !messagesContainRole(history, schema.Assistant, "final answer") {
		t.Fatalf("persisted history lacks final streamed answer: %#v", history)
	}
	if reportedUsage == nil || reportedUsage.TotalTokens != usage.TotalTokens {
		t.Fatalf("reported usage = %#v, want %#v", reportedUsage, usage)
	}
}

func TestADKLifecycleSummarizesBeforeSessionPersistence(t *testing.T) {
	chatModel := &scriptedLifecycleModel{
		generateResponses: []*schema.Message{schema.AssistantMessage("compact summary", nil)},
		streamResponses:   [][]*schema.Message{{schema.AssistantMessage("post-summary answer", nil)}},
	}
	summarizer, err := summarization.New(context.Background(), &summarization.Config{
		Model: chatModel, Trigger: &summarization.TriggerCondition{ContextTokens: summarizeTriggerTokens},
	})
	if err != nil {
		t.Fatalf("summarization.New() error = %v", err)
	}
	conversation := NewConversationStore(0)
	session := &sessionMiddleware{store: conversation, userID: 7, threadID: "summary-thread"}
	agent, err := adk.NewChatModelAgent(context.Background(), &adk.ChatModelAgentConfig{
		Name: "summarizing", Description: "summarization compatibility seam", Model: chatModel,
		Handlers: []adk.ChatModelAgentMiddleware{summarizer, session},
	})
	if err != nil {
		t.Fatalf("NewChatModelAgent() error = %v", err)
	}

	largeContext := schema.AssistantMessage("old answer", nil)
	largeContext.ResponseMeta = &schema.ResponseMeta{Usage: &schema.TokenUsage{TotalTokens: summarizeTriggerTokens + 1}}
	runner := adk.NewRunner(context.Background(), adk.RunnerConfig{Agent: agent, EnableStreaming: true})
	result := drainLifecycleEvents(t, runner.Run(context.Background(), []*schema.Message{
		schema.UserMessage("old question"), largeContext, schema.UserMessage("new question"),
	}))
	if len(result.errs) > 0 {
		t.Fatalf("runner errors = %v", result.errs)
	}
	if got := chatModel.generateCallCount(); got != 1 {
		t.Fatalf("summary Generate() calls = %d, want 1", got)
	}
	if !chatModel.firstStreamInputContains("compact summary") {
		t.Fatalf("post-summary model input did not contain compact summary: %#v", chatModel.firstStreamInput())
	}
	history := conversation.Messages(7, "summary-thread")
	if !messagesContain(history, "compact summary") || !messagesContain(history, "post-summary answer") {
		t.Fatalf("persisted summarized history = %#v", history)
	}
	if messagesContain(history, "old answer") {
		t.Fatalf("persisted history retained pre-summary context: %#v", history)
	}
}

func TestADKLifecycleConfirmationCheckpointAndResume(t *testing.T) {
	ctx := context.Background()
	checkpointStore := newLifecycleCheckpointStore(t)
	conversation := NewConversationStore(0)
	chatModel := &scriptedLifecycleModel{streamResponses: [][]*schema.Message{
		{schema.AssistantMessage("", []schema.ToolCall{{
			ID: "call-confirm", Type: "function",
			Function: schema.FunctionCall{Name: "confirm_fixture", Arguments: `{}`},
		}})},
		{schema.AssistantMessage("confirmed completion", nil)},
	}}
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name: "confirmation", Description: "checkpoint compatibility seam", Model: chatModel,
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{
			Tools: []tool.BaseTool{confirmationLifecycleTool{}},
		}},
		Handlers: []adk.ChatModelAgentMiddleware{&sessionMiddleware{
			store: conversation, userID: 7, threadID: "confirm-thread",
		}},
	})
	if err != nil {
		t.Fatalf("NewChatModelAgent() error = %v", err)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent: agent, EnableStreaming: true, CheckPointStore: checkpointStore,
	})
	checkpointID := CheckpointKey(7, "confirm-thread")
	interrupted := drainLifecycleEvents(t, runner.Run(ctx, []*schema.Message{
		schema.UserMessage("perform confirmed action"),
	}, adk.WithCheckPointID(checkpointID)))
	if len(interrupted.errs) > 0 {
		t.Fatalf("initial runner errors = %v", interrupted.errs)
	}
	rootID := rootInterruptID(interrupted.interrupts)
	if rootID == "" {
		t.Fatalf("interrupt contexts = %#v, want root cause", interrupted.interrupts)
	}
	if data, exists, getErr := checkpointStore.Get(ctx, checkpointID); getErr != nil || !exists || len(data) == 0 {
		t.Fatalf("checkpoint Get() = %d bytes, %v, %v", len(data), exists, getErr)
	}

	resumed, err := runner.ResumeWithParams(ctx, checkpointID, &adk.ResumeParams{
		Targets: map[string]any{rootID: "approved"},
	})
	if err != nil {
		t.Fatalf("ResumeWithParams() error = %v", err)
	}
	completed := drainLifecycleEvents(t, resumed)
	if len(completed.errs) > 0 {
		t.Fatalf("resume runner errors = %v", completed.errs)
	}
	if !messagesContain(completed.messages, "confirmed completion") {
		t.Fatalf("resume output = %#v", completed.messages)
	}
	history := conversation.Messages(7, "confirm-thread")
	if !messagesContainRole(history, schema.Tool, "approved") || !messagesContain(history, "confirmed completion") {
		t.Fatalf("resumed history = %#v", history)
	}
}

func TestADKLifecycleResumesEinoV096ConfirmationCheckpoint(t *testing.T) {
	ctx := context.Background()
	checkpointStore := newLifecycleCheckpointStore(t)
	checkpointData, err := os.ReadFile(filepath.Join("testdata", "eino-v0.9.6-confirmation-checkpoint.bin"))
	if err != nil {
		t.Fatalf("read v0.9.6 checkpoint fixture: %v", err)
	}
	rootIDData, err := os.ReadFile(filepath.Join("testdata", "eino-v0.9.6-confirmation-checkpoint.bin.root-id"))
	if err != nil {
		t.Fatalf("read v0.9.6 checkpoint root ID: %v", err)
	}
	rootID := strings.TrimSpace(string(rootIDData))
	const checkpointID = "u:7:t:pre-upgrade-confirm"
	if err := checkpointStore.Set(ctx, checkpointID, checkpointData); err != nil {
		t.Fatalf("install v0.9.6 checkpoint fixture: %v", err)
	}

	conversation := NewConversationStore(0)
	chatModel := &scriptedLifecycleModel{streamResponses: [][]*schema.Message{
		{schema.AssistantMessage("resumed pre-upgrade checkpoint", nil)},
	}}
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name: "confirmation", Description: "checkpoint compatibility seam", Model: chatModel,
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{
			Tools: []tool.BaseTool{confirmationLifecycleTool{}},
		}},
		Handlers: []adk.ChatModelAgentMiddleware{&sessionMiddleware{
			store: conversation, userID: 7, threadID: "pre-upgrade-confirm",
		}},
	})
	if err != nil {
		t.Fatalf("NewChatModelAgent() error = %v", err)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent: agent, EnableStreaming: true, CheckPointStore: checkpointStore,
	})
	resumed, err := runner.ResumeWithParams(ctx, checkpointID, &adk.ResumeParams{
		Targets: map[string]any{rootID: "approved from v0.9.6"},
	})
	if err != nil {
		t.Fatalf("ResumeWithParams(v0.9.6 fixture) error = %v", err)
	}
	completed := drainLifecycleEvents(t, resumed)
	if len(completed.errs) > 0 {
		t.Fatalf("v0.9.6 resume runner errors = %v", completed.errs)
	}
	if !messagesContain(completed.messages, "resumed pre-upgrade checkpoint") {
		t.Fatalf("v0.9.6 resume output = %#v", completed.messages)
	}
	history := conversation.Messages(7, "pre-upgrade-confirm")
	if !messagesContainRole(history, schema.Tool, "approved from v0.9.6") ||
		!messagesContain(history, "resumed pre-upgrade checkpoint") {
		t.Fatalf("v0.9.6 resumed history = %#v", history)
	}
}

func TestADKLifecycleCancellationDoesNotPersistPartialTurn(t *testing.T) {
	ctx := context.Background()
	chatModel := &cancellingLifecycleModel{started: make(chan struct{})}
	conversation := NewConversationStore(0)
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name: "cancellation", Description: "cancellation compatibility seam", Model: chatModel,
		Handlers: []adk.ChatModelAgentMiddleware{&sessionMiddleware{
			store: conversation, userID: 7, threadID: "cancel-thread",
		}},
	})
	if err != nil {
		t.Fatalf("NewChatModelAgent() error = %v", err)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: true})
	cancelOpt, cancel := adk.WithCancel()
	iter := runner.Run(ctx, []*schema.Message{schema.UserMessage("slow request")}, cancelOpt)
	select {
	case <-chatModel.started:
	case <-time.After(5 * time.Second):
		t.Fatal("model did not start")
	}
	handle, ok := cancel(adk.WithAgentCancelMode(adk.CancelImmediate), adk.WithRecursive())
	if !ok || handle == nil {
		t.Fatal("cancel handle was not created")
	}
	if err := handle.Wait(); err != nil {
		t.Fatalf("cancel handle Wait() error = %v", err)
	}
	result := drainLifecycleEvents(t, iter)
	var cancelErr *adk.CancelError
	for _, eventErr := range result.errs {
		if errors.As(eventErr, &cancelErr) {
			break
		}
	}
	if cancelErr == nil {
		t.Fatalf("runner errors = %v, want CancelError", result.errs)
	}
	if history := conversation.Messages(7, "cancel-thread"); len(history) != 0 {
		t.Fatalf("cancelled turn persisted: %#v", history)
	}
}

func TestConstructedAgentModelIsIsolatedFromProviderSwitch(t *testing.T) {
	oldServer := newOpenAILifecycleServer(t, "old provider response")
	defer oldServer.Close()
	newServer := newOpenAILifecycleServer(t, "new provider response")
	defer newServer.Close()

	provider := &mutableLifecycleConfigProvider{cfg: settings.LLM{
		Provider: settings.LLMProviderOpenAI, APIKey: "old-key", ModelName: "fixture-model", BaseURL: oldServer.URL,
	}}
	service := &agentService{configProvider: provider}
	oldModel, err := service.newChatModel(context.Background())
	if err != nil {
		t.Fatalf("construct old model: %v", err)
	}

	provider.set(settings.LLM{
		Provider: settings.LLMProviderOpenAI, APIKey: "new-key", ModelName: "fixture-model", BaseURL: newServer.URL,
	})
	newModel, err := service.newChatModel(context.Background())
	if err != nil {
		t.Fatalf("construct new model: %v", err)
	}

	oldResponse, err := oldModel.Generate(context.Background(), []*schema.Message{schema.UserMessage("old run")})
	if err != nil {
		t.Fatalf("old model Generate() error = %v", err)
	}
	newResponse, err := newModel.Generate(context.Background(), []*schema.Message{schema.UserMessage("new run")})
	if err != nil {
		t.Fatalf("new model Generate() error = %v", err)
	}
	if oldResponse.Content != "old provider response" || newResponse.Content != "new provider response" {
		t.Fatalf("provider switch changed constructed models: old=%q new=%q", oldResponse.Content, newResponse.Content)
	}
}

type scriptedLifecycleModel struct {
	mu                sync.Mutex
	generateResponses []*schema.Message
	streamResponses   [][]*schema.Message
	generateCalls     int
	streamCalls       int
	streamInputs      [][]*schema.Message
}

func (m *scriptedLifecycleModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.generateCalls >= len(m.generateResponses) {
		return nil, fmt.Errorf("unexpected Generate call %d with %d messages", m.generateCalls+1, len(input))
	}
	response := m.generateResponses[m.generateCalls]
	m.generateCalls++
	return response, nil
}

func (m *scriptedLifecycleModel) Stream(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.streamCalls >= len(m.streamResponses) {
		return nil, fmt.Errorf("unexpected Stream call %d with %d messages", m.streamCalls+1, len(input))
	}
	m.streamInputs = append(m.streamInputs, append([]*schema.Message(nil), input...))
	response := m.streamResponses[m.streamCalls]
	m.streamCalls++
	return schema.StreamReaderFromArray(response), nil
}

func (m *scriptedLifecycleModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func (m *scriptedLifecycleModel) generateCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.generateCalls
}

func (m *scriptedLifecycleModel) firstStreamInput() []*schema.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.streamInputs) == 0 {
		return nil
	}
	return append([]*schema.Message(nil), m.streamInputs[0]...)
}

func (m *scriptedLifecycleModel) firstStreamInputContains(content string) bool {
	return messagesContain(m.firstStreamInput(), content)
}

type fixtureLifecycleTool struct{}

func (fixtureLifecycleTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "fixture_lookup", Desc: "returns a deterministic result"}, nil
}

func (fixtureLifecycleTool) InvokableRun(context.Context, string, ...tool.Option) (string, error) {
	return "fixture tool result", nil
}

type confirmationLifecycleTool struct{}

func (confirmationLifecycleTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "confirm_fixture", Desc: "requires confirmation"}, nil
}

func (confirmationLifecycleTool) InvokableRun(ctx context.Context, _ string, _ ...tool.Option) (string, error) {
	wasInterrupted, _, _ := compose.GetInterruptState[string](ctx)
	if !wasInterrupted {
		return "", compose.StatefulInterrupt(ctx, "approval required", "pending action")
	}
	isTarget, hasData, data := compose.GetResumeContext[string](ctx)
	if !isTarget || !hasData {
		return "", compose.StatefulInterrupt(ctx, "approval required", "pending action")
	}
	return data, nil
}

type cancellingLifecycleModel struct {
	started chan struct{}
	once    sync.Once
}

type mutableLifecycleConfigProvider struct {
	mu  sync.Mutex
	cfg settings.LLM
}

func (p *mutableLifecycleConfigProvider) GetLLMConfig(context.Context) (settings.LLM, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cfg, nil
}

func (p *mutableLifecycleConfigProvider) set(cfg settings.LLM) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cfg = cfg
}

func (m *cancellingLifecycleModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return nil, errors.New("unexpected Generate call")
}

func (m *cancellingLifecycleModel) Stream(ctx context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.once.Do(func() { close(m.started) })
	stream, writer := schema.Pipe[*schema.Message](1)
	go func() {
		defer writer.Close()
		<-ctx.Done()
		writer.Send(nil, ctx.Err())
	}()
	return stream, nil
}

func (m *cancellingLifecycleModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

type lifecycleResult struct {
	messages   []*schema.Message
	interrupts []*adk.InterruptCtx
	errs       []error
}

func drainLifecycleEvents(t *testing.T, iter *adk.AsyncIterator[*adk.AgentEvent]) lifecycleResult {
	t.Helper()
	var result lifecycleResult
	for {
		event, ok := iter.Next()
		if !ok {
			return result
		}
		if event.Err != nil {
			result.errs = append(result.errs, event.Err)
		}
		if event.Action != nil && event.Action.Interrupted != nil {
			result.interrupts = append(result.interrupts, event.Action.Interrupted.InterruptContexts...)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		output := event.Output.MessageOutput
		if output.Message != nil {
			result.messages = append(result.messages, output.Message)
		}
		if output.MessageStream == nil {
			continue
		}
		var chunks []*schema.Message
		for {
			message, err := output.MessageStream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				result.errs = append(result.errs, err)
				break
			}
			chunks = append(chunks, message)
		}
		output.MessageStream.Close()
		if len(chunks) > 0 {
			message, err := schema.ConcatMessages(chunks)
			if err != nil {
				t.Fatalf("ConcatMessages() error = %v", err)
			}
			result.messages = append(result.messages, message)
		}
	}
}

func newLifecycleCheckpointStore(t *testing.T) *CheckpointStore {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("chmod test catalog: %v", err)
	}
	database, err := db.Open(context.Background(), config.DatabaseConfig{
		Path: filepath.Join(dir, "agent-lifecycle.sqlite3"),
	})
	if err != nil {
		t.Fatalf("open test catalog: %v", err)
	}
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate test catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(context.Background()); err != nil {
			t.Errorf("close test catalog: %v", err)
		}
	})
	return NewCheckpointStore(database.Queries)
}

func newOpenAILifecycleServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"chatcmpl-lifecycle","object":"chat.completion","created":1,"model":"fixture-model","choices":[{"index":0,"message":{"role":"assistant","content":%q},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`, content)
	}))
}

func rootInterruptID(contexts []*adk.InterruptCtx) string {
	for _, interrupt := range contexts {
		if interrupt != nil && interrupt.IsRootCause {
			return interrupt.ID
		}
	}
	return ""
}

func messagesContain(messages []*schema.Message, content string) bool {
	for _, message := range messages {
		if message != nil && strings.Contains(message.Content, content) {
			return true
		}
	}
	return false
}

func messagesContainRole(messages []*schema.Message, role schema.RoleType, content string) bool {
	for _, message := range messages {
		if message != nil && message.Role == role && strings.Contains(message.Content, content) {
			return true
		}
	}
	return false
}

func messagesHaveToolCall(messages []*schema.Message, name string) bool {
	for _, message := range messages {
		if message == nil {
			continue
		}
		for _, call := range message.ToolCalls {
			if call.Function.Name == name {
				return true
			}
		}
	}
	return false
}
