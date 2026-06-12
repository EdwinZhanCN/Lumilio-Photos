package core

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// sessionMiddleware persists the agent's end-of-run message state into the
// ConversationStore (multi-turn memory) and reports token usage from the
// last model call to the side channel. It runs after any summarization
// middleware, so a compacted state is written back compacted.
type sessionMiddleware struct {
	adk.BaseChatModelAgentMiddleware
	store    *ConversationStore
	userID   int32
	threadID string
	onUsage  func(usage *schema.TokenUsage)
}

func (m *sessionMiddleware) AfterAgent(ctx context.Context, state *adk.ChatModelAgentState) (context.Context, error) {
	if state == nil {
		return ctx, nil
	}
	m.store.Replace(m.userID, m.threadID, state.Messages)

	if m.onUsage != nil {
		if usage := lastAssistantUsage(state.Messages); usage != nil {
			m.onUsage(usage)
		}
	}
	return ctx, nil
}

// lastAssistantUsage returns the usage of the final model call: its prompt
// token count is the current context size, which is what the user wants to
// watch grow.
func lastAssistantUsage(messages []*schema.Message) *schema.TokenUsage {
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if message == nil || message.Role != schema.Assistant {
			continue
		}
		if message.ResponseMeta != nil && message.ResponseMeta.Usage != nil {
			return message.ResponseMeta.Usage
		}
	}
	return nil
}
