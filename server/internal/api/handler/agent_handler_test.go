package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"server/internal/agent/core"
	"server/internal/db/repo"
	"server/internal/service"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type flushableResponseWriter struct {
	*httptest.ResponseRecorder
}

func (w *flushableResponseWriter) Flush() {}

type streamTestAgentService struct{}

func (streamTestAgentService) EnsureThread(context.Context, int32, string, string, core.ThreadBindings) (repo.AgentThread, error) {
	return repo.AgentThread{}, nil
}
func (streamTestAgentService) AskAgent(context.Context, int32, string, string, string, ...chan<- *core.SideChannelEvent) (*core.AgentRun, error) {
	return nil, nil
}
func (streamTestAgentService) ResumeAgent(context.Context, int32, string, *adk.ResumeParams, ...chan<- *core.SideChannelEvent) (*core.AgentRun, error) {
	return nil, nil
}
func (streamTestAgentService) CancelRun(context.Context, int32, string, uuid.UUID) (string, error) {
	return "cancelled", nil
}
func (streamTestAgentService) FinishRun(_ context.Context, _ int32, _ string, _ uuid.UUID, status string) (string, error) {
	return status, nil
}
func (streamTestAgentService) GetEffectReceipt(context.Context, int32, string, uuid.UUID) (core.EffectReceipt, string, error) {
	return core.EffectReceipt{}, "", nil
}
func (streamTestAgentService) GetAvailableTools() []*schema.ToolInfo { return nil }
func (streamTestAgentService) GetToolsByMode(string) []*schema.ToolInfo {
	return nil
}

func TestChatRequiresKnownExplicitMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, body := range map[string]string{
		"missing": `{"query":"hello"}`,
		"unknown": `{"query":"hello","mode":"untrusted"}`,
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/agent/chat", bytes.NewBufferString(body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set("current_user", &service.UserResponse{UserID: 1, Username: "tester"})

			NewAgentHandler(nil, nil, nil, nil, nil).Chat(c)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}

func TestFormatAgentEventNeverEmitsProviderReasoning(t *testing.T) {
	event := &adk.AgentEvent{
		AgentName: "agent",
		Output: &adk.AgentOutput{MessageOutput: &adk.MessageVariant{Message: &schema.Message{
			Role: schema.Assistant, Content: "answer", ReasoningContent: "private chain",
		}}},
	}
	data := (&AgentHandler{}).formatAgentEvent(event)
	require.Equal(t, "answer", data["output"])
	require.NotContains(t, data, "reasoning")
	require.NotContains(t, data, "reasoning_content")
}

func TestStreamAgentEvents_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	w := &flushableResponseWriter{ResponseRecorder: recorder}
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("POST", "/api/v1/agent/chat", nil)
	c.Request = req

	handler := NewAgentHandler(streamTestAgentService{}, nil, nil, nil, nil)

	// Create Eino iterator pair
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	sideChannel := make(chan *core.SideChannelEvent, 10)

	// Start feeding events in a separate goroutine
	go func() {
		// Send side-channel event
		sideChannel <- &core.SideChannelEvent{
			Type:      "tool_execution",
			Timestamp: 12345,
			Tool:      core.ToolIdentity{Name: "test_tool"},
		}

		// Send agent message event
		gen.Send(&adk.AgentEvent{
			AgentName: "TestAgent",
			Output: &adk.AgentOutput{
				MessageOutput: &adk.MessageVariant{
					Message: &schema.Message{
						Role:    schema.Assistant,
						Content: "Hello from agent!",
					},
				},
			},
		})

		// Send another side-channel event
		sideChannel <- &core.SideChannelEvent{
			Type:      "tool_execution",
			Timestamp: 67890,
			Tool:      core.ToolIdentity{Name: "test_tool_2"},
		}

		// Close agent event iterator
		gen.Close()
	}()

	// Run streaming
	handler.streamAgentEvents(c, w, &core.AgentRun{
		RunID: uuid.New(), ThreadID: "thread-1", Iterator: iter,
	}, sideChannel, 1)

	body := recorder.Body.String()

	// Verify events are in the output
	require.Contains(t, body, "event: side_event")
	require.Contains(t, body, "test_tool")
	require.Contains(t, body, "test_tool_2")
	require.Contains(t, body, "event: message")
	require.Contains(t, body, "Hello from agent!")
	require.Contains(t, body, "event: done")
}

func TestStreamAgentEvents_ClientDisconnect(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	w := &flushableResponseWriter{ResponseRecorder: recorder}
	c, _ := gin.CreateTestContext(w)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("POST", "/api/v1/agent/chat", nil)
	req = req.WithContext(ctx)
	c.Request = req

	handler := NewAgentHandler(streamTestAgentService{}, nil, nil, nil, nil)

	// Create Eino iterator pair
	iter, _ := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	sideChannel := make(chan *core.SideChannelEvent, 10)

	// Cancel context immediately to simulate disconnect
	cancel()

	// Run streaming, it should exit immediately without hanging
	doneChan := make(chan struct{})
	go func() {
		handler.streamAgentEvents(c, w, &core.AgentRun{
			RunID: uuid.New(), ThreadID: "thread-1", Iterator: iter,
		}, sideChannel, 1)
		close(doneChan)
	}()

	select {
	case <-doneChan:
		// Success: it returned
	case <-time.After(2 * time.Second):
		t.Fatal("streamAgentEvents did not exit after context cancellation")
	}
}
