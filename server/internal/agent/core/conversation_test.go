package core

import (
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

func TestConversationReplaceAndMessages(t *testing.T) {
	s := NewConversationStore(0)

	s.Replace(1, "t1", []*schema.Message{
		schema.SystemMessage("instruction with stale ledger"),
		schema.UserMessage("find cats"),
		schema.AssistantMessage("here are cats", nil),
	})

	got := s.Messages(1, "t1")
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (system message must be dropped)", len(got))
	}
	if got[0].Role != schema.User || got[1].Role != schema.Assistant {
		t.Fatalf("unexpected roles: %v, %v", got[0].Role, got[1].Role)
	}
}

func TestConversationScopeIsolation(t *testing.T) {
	s := NewConversationStore(0)
	s.Replace(1, "t1", []*schema.Message{schema.UserMessage("hello")})

	if got := s.Messages(2, "t1"); got != nil {
		t.Fatalf("cross-user history leaked: %v", got)
	}
	if got := s.Messages(1, "t2"); got != nil {
		t.Fatalf("cross-thread history leaked: %v", got)
	}
}

func TestConversationTTL(t *testing.T) {
	s := NewConversationStore(time.Minute)
	now := time.Unix(1000, 0)
	s.now = func() time.Time { return now }

	s.Replace(1, "t1", []*schema.Message{schema.UserMessage("hello")})

	now = now.Add(30 * time.Second)
	if got := s.Messages(1, "t1"); len(got) != 1 {
		t.Fatal("fresh conversation expired early")
	}

	now = now.Add(2 * time.Minute)
	if got := s.Messages(1, "t1"); got != nil {
		t.Fatal("expired conversation still served")
	}
}

// An abandoned interrupt leaves a dangling assistant tool call; replaying it
// would make providers reject the request, so the sanitized history must
// drop the dangling tail while keeping resolved tool exchanges.
func TestConversationSanitizesDanglingToolCalls(t *testing.T) {
	s := NewConversationStore(0)

	resolvedCall := schema.AssistantMessage("", []schema.ToolCall{
		{ID: "call-1", Function: schema.FunctionCall{Name: "filter_assets"}},
	})
	resolvedResult := schema.ToolMessage(`{"receipt":{}}`, "call-1")
	danglingCall := schema.AssistantMessage("", []schema.ToolCall{
		{ID: "call-2", Function: schema.FunctionCall{Name: "create_album"}},
	})

	s.Replace(1, "t1", []*schema.Message{
		schema.UserMessage("make an album"),
		resolvedCall,
		resolvedResult,
		danglingCall, // interrupted, never resumed
	})

	got := s.Messages(1, "t1")
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3 (dangling tool call dropped)", len(got))
	}
	last := got[len(got)-1]
	if last.Role != schema.Tool || last.ToolCallID != "call-1" {
		t.Fatalf("unexpected tail: role=%v toolCallID=%q", last.Role, last.ToolCallID)
	}
}

func TestLastAssistantUsage(t *testing.T) {
	withUsage := schema.AssistantMessage("answer", nil)
	withUsage.ResponseMeta = &schema.ResponseMeta{
		Usage: &schema.TokenUsage{PromptTokens: 1200, CompletionTokens: 80, TotalTokens: 1280},
	}

	messages := []*schema.Message{
		schema.UserMessage("q"),
		withUsage,
		schema.ToolMessage("result", "call-9"), // tool after assistant must not mask it
	}

	usage := lastAssistantUsage(messages)
	if usage == nil || usage.PromptTokens != 1200 {
		t.Fatalf("usage = %+v, want prompt 1200", usage)
	}

	if lastAssistantUsage([]*schema.Message{schema.UserMessage("q")}) != nil {
		t.Fatal("no assistant message must yield nil usage")
	}
}
