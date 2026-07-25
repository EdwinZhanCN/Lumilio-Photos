package core

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

func TestRunRegistryCancelIsExactAndMarksReplayUnsafe(t *testing.T) {
	registry := NewRunRegistry()
	oldRun, newRun := uuid.New(), uuid.New()
	cancel := func(...adk.AgentCancelOption) (*adk.CancelHandle, bool) { return nil, true }
	registry.Register(7, "thread", oldRun, cancel)
	registry.Register(7, "thread", newRun, cancel)

	if _, ok := registry.RequestCancel(7, "thread", oldRun); !ok {
		t.Fatal("old run cancel handle not found")
	}
	if !registry.CancelRequested(7, "thread", oldRun) {
		t.Fatal("old run was not marked cancel requested")
	}
	if registry.CancelRequested(7, "thread", newRun) {
		t.Fatal("cancelling an old run marked the new run")
	}
	if _, ok := registry.RequestCancel(8, "thread", oldRun); ok {
		t.Fatal("cross-user cancel found a run handle")
	}
}

func TestSessionMiddlewareDoesNotPersistCancelledTurn(t *testing.T) {
	store := NewConversationStore(0)
	middleware := &sessionMiddleware{
		store: store, userID: 7, threadID: "thread",
		shouldPersist: func() bool { return false },
	}
	state := &adk.ChatModelAgentState{Messages: []*schema.Message{
		schema.UserMessage("do work"),
		schema.AssistantMessage("partial output", nil),
	}}

	if _, err := middleware.AfterAgent(context.Background(), state); err != nil {
		t.Fatalf("AfterAgent: %v", err)
	}
	if got := store.Messages(7, "thread"); len(got) != 0 {
		t.Fatalf("cancelled turn was persisted: %#v", got)
	}
}
