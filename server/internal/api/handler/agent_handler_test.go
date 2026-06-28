package handler

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"server/internal/agent/core"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type flushableResponseWriter struct {
	*httptest.ResponseRecorder
}

func (w *flushableResponseWriter) Flush() {}

func TestStreamAgentEvents_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	w := &flushableResponseWriter{ResponseRecorder: recorder}
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("POST", "/api/v1/agent/chat", nil)
	c.Request = req

	handler := NewAgentHandler(nil, nil, nil, nil, nil)

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
	handler.streamAgentEvents(c, w, iter, sideChannel)

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

	handler := NewAgentHandler(nil, nil, nil, nil, nil)

	// Create Eino iterator pair
	iter, _ := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	sideChannel := make(chan *core.SideChannelEvent, 10)

	// Cancel context immediately to simulate disconnect
	cancel()

	// Run streaming, it should exit immediately without hanging
	doneChan := make(chan struct{})
	go func() {
		handler.streamAgentEvents(c, w, iter, sideChannel)
		close(doneChan)
	}()

	select {
	case <-doneChan:
		// Success: it returned
	case <-time.After(2 * time.Second):
		t.Fatal("streamAgentEvents did not exit after context cancellation")
	}
}
