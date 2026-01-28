package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"server/internal/agent/core"
	"server/internal/api"
	"server/internal/api/dto"

	"github.com/cloudwego/eino/adk"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AgentHandler handles agent-related HTTP requests
type AgentHandler struct {
	agentService core.AgentService
}

// NewAgentHandler creates a new agent handler
func NewAgentHandler(agentService core.AgentService) *AgentHandler {
	return &AgentHandler{
		agentService: agentService,
	}
}

// AgentChatRequest represents request body for agent chat
type AgentChatRequest struct {
	ThreadID  string   `json:"thread_id,omitempty"`
	Query     string   `json:"query" binding:"required"`
	ToolNames []string `json:"tool_names,omitempty"`
}

// AgentResumeRequest represents request body for resuming agent chat
type AgentResumeRequest struct {
	ThreadID string         `json:"thread_id" binding:"required"`
	Targets  map[string]any `json:"targets" binding:"required"`
}

// Chat handles agent chat requests with SSE streaming
// @Summary Chat with Agent
// @Description Send a query to agent and receive streaming responses via SSE. Manages conversation threads.
// @Tags agent
// @Accept json
// @Produce text/event-stream
// @Param request body AgentChatRequest true "Chat request"
// @Success 200 {string} string "SSE stream"
// @Failure 400 {object} api.Result "Invalid request"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/agent/chat [post]
func (h *AgentHandler) Chat(c *gin.Context) {
	var req AgentChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	// 准备 SSE
	flusher, err := h.prepareSSE(c)
	if err != nil {
		api.GinInternalError(c, err)
		return
	}

	// 管理会话 ID
	threadID := req.ThreadID
	if threadID == "" {
		threadID = uuid.NewString()
	}

	// 创建 UI 侧信道
	uiChannel := make(chan *core.SideChannelEvent, 100)

	// 获取 Agent 迭代器
	iter := h.agentService.AskAgent(c.Request.Context(), threadID, req.Query, req.ToolNames, uiChannel)

	// 发送会话信息事件
	h.sendSSE(c, flusher, "session_info", map[string]string{"thread_id": threadID})

	// 开始流式传输事件
	h.streamAgentEvents(c, flusher, iter, uiChannel)
}

// ResumeChat handles resuming an interrupted agent execution
// @Summary Resume Agent Chat
// @Description Resume a conversation from an interrupt point (e.g., user confirmation for a tool call)
// @Tags agent
// @Accept json
// @Produce text/event-stream
// @Param request body AgentResumeRequest true "Resume request"
// @Success 200 {string} string "SSE stream"
// @Failure 400 {object} api.Result "Invalid request"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/agent/chat/resume [post]
func (h *AgentHandler) ResumeChat(c *gin.Context) {
	var req AgentResumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	flusher, err := h.prepareSSE(c)
	if err != nil {
		api.GinInternalError(c, err)
		return
	}

	uiChannel := make(chan *core.SideChannelEvent, 100)

	// 构建 Resume 参数
	params := &adk.ResumeParams{
		Targets: req.Targets,
	}

	iter, err := h.agentService.ResumeAgent(c.Request.Context(), req.ThreadID, params, uiChannel)
	if err != nil {
		h.sendSSE(c, flusher, "error", map[string]interface{}{"error": err.Error()})
		return
	}

	h.sendSSE(c, flusher, "session_info", map[string]string{"thread_id": req.ThreadID})
	h.streamAgentEvents(c, flusher, iter, uiChannel)
}

// prepareSSE sets the necessary headers for Server-Sent Events.
func (h *AgentHandler) prepareSSE(c *gin.Context) (http.Flusher, error) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	// c.Writer.Header().Set("Transfer-Encoding", "chunked") // Removed to prevent ERR_INVALID_CHUNKED_ENCODING
	c.Writer.Header().Set("X-Accel-Buffering", "no") // Disable Nginx buffering

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}
	return flusher, nil
}

// streamAgentEvents handles the main loop for streaming agent and UI events via SSE.
func (h *AgentHandler) streamAgentEvents(c *gin.Context, flusher http.Flusher, iter *adk.AsyncIterator[*adk.AgentEvent], uiChannel chan *core.SideChannelEvent) {
	done := make(chan struct{})
	defer close(done)

	// Goroutine to handle UI events from tools
	go func() {
		defer close(uiChannel)
		for {
			select {
			case <-done:
				return
			case <-c.Request.Context().Done():
				return
			case event, ok := <-uiChannel:
				if !ok {
					return
				}
				h.sendSSE(c, flusher, "ui_event", event)
			}
		}
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			h.sendSSE(c, flusher, "heartbeat", map[string]interface{}{"timestamp": time.Now().Unix()})
			continue
		default:
		}

		event, ok := iter.Next()
		if !ok {
			h.sendSSE(c, flusher, "done", nil)
			return
		}

		// Guardrail: Skip internal-only events that are not meant for the user.
		// An event with only `CustomizedOutput` is a raw tool result intended for the next LLM reasoning step.
		if event.Output != nil && event.Output.MessageOutput == nil && event.Output.CustomizedOutput != nil {
			log.Printf("[AgentHandler] Skipping internal tool output event")
			continue
		}

		if event.Err != nil {
			log.Printf("[AgentHandler] Error event: %v", event.Err)
			h.sendSSE(c, flusher, "error", map[string]interface{}{"error": event.Err.Error()})
			return
		}

		if event.Output != nil && event.Output.MessageOutput != nil && event.Output.MessageOutput.IsStreaming {
			log.Printf("[AgentHandler] Handling streaming output")
			h.handleStreamingOutput(c, flusher, event)
		} else {
			log.Printf("[AgentHandler] Handling non-streaming output")
			eventData := h.formatAgentEvent(event)
			if len(eventData) > 0 {
				h.sendSSE(c, flusher, "message", eventData)
			} else {
				log.Printf("[AgentHandler] Skipped empty event data")
			}
		}
	}
}

// sendSSE sends a Server-Sent Event
func (h *AgentHandler) sendSSE(c *gin.Context, flusher http.Flusher, eventType string, data interface{}) {
	if c.Request.Context().Err() != nil {
		return // 客户端已断开，不发送
	}
	var jsonData []byte
	var err error

	if data == nil {
		jsonData = []byte("{}")
	} else {
		jsonData, err = json.Marshal(data)
		if err != nil {
			fmt.Fprintf(c.Writer, "event: error\ndata: {\"error\":\"failed to marshal data\"}\n\n")
			flusher.Flush()
			return
		}
	}

	fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", eventType, jsonData)
	flusher.Flush()
}

// formatAgentEvent formats an AgentEvent for JSON serialization
func (h *AgentHandler) formatAgentEvent(event *adk.AgentEvent) map[string]interface{} {
	result := make(map[string]interface{})
	result["agent_name"] = event.AgentName
	if len(event.RunPath) > 0 {
		result["run_path"] = event.RunPath
	}

	if event.Output != nil {
		if event.Output.MessageOutput != nil {
			message := event.Output.MessageOutput.Message
			// Guardrail: Only process messages with the 'assistant' role as user-facing output.
			// Messages with a 'tool' role are internal results for the LLM and should not be displayed.
			if message != nil && message.Role == "assistant" {
				if message.ReasoningContent != "" {
					result["reasoning"] = message.ReasoningContent
				}
				if message.Content != "" {
					result["output"] = message.Content
				} else if len(message.AssistantGenMultiContent) > 0 {
					var textParts []string
					for _, part := range message.AssistantGenMultiContent {
						if part.Type == "text" {
							textParts = append(textParts, part.Text)
						}
					}
					if len(textParts) > 0 {
						result["output"] = strings.Join(textParts, "")
					}
				}
			} else if message != nil {
				log.Printf("[AgentHandler] Skipped message with role: %s", message.Role)
			}
		} else if event.Output.CustomizedOutput != nil {
			// This block handles outputs that are not from the LLM. Based on our analysis,
			// these are internal data and should not be sent to the user.
			// By not processing this, we filter out raw tool results.
			log.Printf("[AgentHandler] Skipped CustomizedOutput")
		}
	}

	if event.Action != nil {
		result["action"] = event.Action
	}
	if event.Err != nil {
		result["error"] = event.Err.Error()
	}

	return result
}

// handleStreamingOutput processes streaming messages from MessageStream
func (h *AgentHandler) handleStreamingOutput(c *gin.Context, flusher http.Flusher, event *adk.AgentEvent) {
	if event.Output == nil || event.Output.MessageOutput == nil || !event.Output.MessageOutput.IsStreaming {
		return
	}
	messageVariant := event.Output.MessageOutput
	if messageVariant.MessageStream == nil {
		return
	}

	for {
		msg, err := messageVariant.MessageStream.Recv()
		if err != nil {
			if err != io.EOF {
				h.sendSSE(c, flusher, "error", map[string]interface{}{"error": err.Error()})
			}
			return
		}
		if msg == nil {
			continue
		}

		var outputText, reasoningText string
		if msg.ReasoningContent != "" {
			reasoningText = msg.ReasoningContent
		}
		if msg.Content != "" {
			outputText = msg.Content
		} else if len(msg.AssistantGenMultiContent) > 0 {
			var textParts []string
			for _, part := range msg.AssistantGenMultiContent {
				if part.Type == "text" {
					textParts = append(textParts, part.Text)
				}
			}
			if len(textParts) > 0 {
				outputText = strings.Join(textParts, "")
			}
		}

		if reasoningText != "" || outputText != "" {
			eventData := map[string]interface{}{"agent_name": event.AgentName}
			if reasoningText != "" {
				eventData["reasoning"] = reasoningText
			}
			if outputText != "" {
				eventData["output"] = outputText
			}
			if len(event.RunPath) > 0 {
				eventData["run_path"] = event.RunPath
			}
			h.sendSSE(c, flusher, "message", eventData)
		}
	}
}

// ToolInfoResponse represents tool information response
type ToolInfoResponse struct {
	Name  string                 `json:"name"`
	Desc  string                 `json:"desc"`
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// GetTools returns list of available tools
// @Summary Get Available Tools
// @Description Get list of all registered agent tools
// @Tags agent
// @Produce json
// @Success 200 {object} api.Result{data=[]ToolInfoResponse}
// @Router /api/v1/agent/tools [get]
func (h *AgentHandler) GetTools(c *gin.Context) {
	registry := core.GetRegistry()
	tools := registry.GetAllToolInfos()

	result := make([]ToolInfoResponse, 0, len(tools))
	for _, tool := range tools {
		toolData := ToolInfoResponse{
			Name: tool.Name,
			Desc: tool.Desc,
		}
		if len(tool.Extra) > 0 {
			toolData.Extra = tool.Extra
		}
		result = append(result, toolData)
	}
	api.GinSuccess(c, result)
}

// ToolSchemaResponse represents the schema response structure
type ToolSchemaResponse struct {
	BulkLikeUpdate dto.BulkLikeUpdateDTO `json:"bulk_like_update_example"`
}

// GetToolSchemas returns DTO schemas used by agent tools
// @Summary Get Agent Tool DTO Schemas
// @Description Get all DTO schemas used by agent tools for type reference
// @Tags agent
// @Produce json
// @Success 200 {object} api.Result{data=ToolSchemaResponse}
// @Router /api/v1/agent/schemas [get]
func (h *AgentHandler) GetToolSchemas(c *gin.Context) {
	schema := ToolSchemaResponse{
		BulkLikeUpdate: dto.BulkLikeUpdateDTO{
			Total:          100,
			Success:        98,
			Failed:         2,
			FailedAssetIDs: []string{"550e8400-e29b-41d4-a716-446655440000", "660e8400-e29b-41d4-a716-446655440001"},
			Liked:          true,
			Action:         "like",
			Description:    "Bulk like: 98/100 successful",
			Timestamp:      time.Now().Format(time.RFC3339),
		},
	}
	api.GinSuccess(c, schema)
}
