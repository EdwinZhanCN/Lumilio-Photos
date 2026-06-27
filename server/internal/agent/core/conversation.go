package core

import (
	"context"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
)

// ConversationStore keeps per-(user, thread) message history in memory so
// every turn replays the full conversation to the model — without it each
// AskAgent run starts blank and the model never sees prior turns.
//
// It deliberately mirrors the ref store's lifecycle philosophy: TTL-bound
// working memory, not a chat-history product. Transcripts die with the
// session (ADR in site/docs/internal/agent/exec-plans/active/agent-ref-system.md); the
// durable artifacts of a conversation are pins, not logs.
type ConversationStore struct {
	mu    sync.Mutex
	convs map[conversationKey]*conversation
	ttl   time.Duration
	now   func() time.Time
}

type conversationKey struct {
	userID   int32
	threadID string
}

type conversation struct {
	messages   []*schema.Message
	lastAccess time.Time
}

// DefaultConversationTTL matches the ref store: a conversation idle longer
// than this is gone, and with it the model's memory of the thread.
const DefaultConversationTTL = 2 * time.Hour

// NewConversationStore creates a store with the given TTL; non-positive
// falls back to the default.
func NewConversationStore(ttl time.Duration) *ConversationStore {
	if ttl <= 0 {
		ttl = DefaultConversationTTL
	}
	return &ConversationStore{
		convs: make(map[conversationKey]*conversation),
		ttl:   ttl,
		now:   time.Now,
	}
}

// Messages returns the conversation history for the scope, sanitized for
// replay: system messages are dropped (the instruction is rebuilt fresh each
// turn, ledger included) and a trailing assistant tool-call without its tool
// results is removed (an abandoned interrupt would otherwise make providers
// reject the next request).
func (s *ConversationStore) Messages(userID int32, threadID string) []*schema.Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv := s.convs[conversationKey{userID, threadID}]
	if conv == nil {
		return nil
	}
	now := s.now()
	if now.Sub(conv.lastAccess) > s.ttl {
		delete(s.convs, conversationKey{userID, threadID})
		return nil
	}
	conv.lastAccess = now
	return sanitizeForReplay(conv.messages)
}

// Replace overwrites the scope's history with the agent's end-of-run state.
// Overwrite (not append) keeps the store coherent with whatever the run
// produced — including summarization middleware compacting the messages.
func (s *ConversationStore) Replace(userID int32, threadID string, messages []*schema.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	kept := make([]*schema.Message, 0, len(messages))
	for _, message := range messages {
		if message == nil || message.Role == schema.System {
			continue
		}
		kept = append(kept, message)
	}

	s.convs[conversationKey{userID, threadID}] = &conversation{
		messages:   kept,
		lastAccess: s.now(),
	}
}

// RunJanitor sweeps expired conversations until ctx is done.
func (s *ConversationStore) RunJanitor(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweep()
		}
	}
}

func (s *ConversationStore) sweep() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for key, conv := range s.convs {
		if now.Sub(conv.lastAccess) > s.ttl {
			delete(s.convs, key)
		}
	}
}

// sanitizeForReplay returns a copy safe to send as a new request: if the
// history ends in an assistant tool call whose tool results never arrived
// (interrupted turn that was abandoned), that dangling tail is dropped.
func sanitizeForReplay(messages []*schema.Message) []*schema.Message {
	end := len(messages)
	for end > 0 {
		last := messages[end-1]
		switch {
		case last.Role == schema.Assistant && len(last.ToolCalls) > 0 && !toolCallsResolved(messages, end-1):
			end--
		case last.Role == schema.Tool && end >= 1 && !hasMatchingToolCall(messages[:end-1], last):
			end--
		default:
			return append([]*schema.Message(nil), messages[:end]...)
		}
	}
	return nil
}

// toolCallsResolved reports whether every tool call of messages[idx] has a
// tool response somewhere after it.
func toolCallsResolved(messages []*schema.Message, idx int) bool {
	pending := make(map[string]struct{}, len(messages[idx].ToolCalls))
	for _, call := range messages[idx].ToolCalls {
		pending[call.ID] = struct{}{}
	}
	for _, message := range messages[idx+1:] {
		if message.Role == schema.Tool {
			delete(pending, message.ToolCallID)
		}
	}
	return len(pending) == 0
}

// hasMatchingToolCall reports whether an orphan tool message has its
// originating assistant tool call earlier in the history.
func hasMatchingToolCall(messages []*schema.Message, toolMsg *schema.Message) bool {
	for _, message := range messages {
		if message.Role != schema.Assistant {
			continue
		}
		for _, call := range message.ToolCalls {
			if call.ID == toolMsg.ToolCallID {
				return true
			}
		}
	}
	return false
}
