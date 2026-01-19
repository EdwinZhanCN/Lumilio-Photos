package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"server/internal/agent/core"
	"server/internal/api"
	"server/internal/api/dto"

	"github.com/cloudwego/eino/adk"
	"github.com/gin-gonic/gin"
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
	Query     string   `json:"query" binding:"required"`
	ToolNames []string `json:"tool_names,omitempty"`
}

// Chat handles agent chat requests with SSE streaming
// @Summary Chat with Agent
// @Description Send a query to agent and receive streaming responses via SSE
// @Tags agent
// @Accept json
// @Produce text/event-stream
// @Param request body AgentChatRequest true "Chat request"
// @Success 200 {string} string "SSE stream"
// @Failure 400 {object} api.Result "Invalid request"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /agent/chat [post]
func (h *AgentHandler) Chat(c *gin.Context) {
	var req AgentChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	// Set SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Header().Set("X-Accel-Buffering", "no") // Disable Nginx buffering

	// Get flush writer
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		api.GinInternalError(c, fmt.Errorf("streaming not supported"))
		return
	}

	// Create UI side channel for tool-generated UI events
	// 增加缓冲区容量以防止工具发送事件时阻塞
	uiChannel := make(chan *core.SideChannelEvent, 100)

	// Create agent iterator with UI channel
	iter := h.agentService.AskAgent(c.Request.Context(), req.Query, req.ToolNames, uiChannel)

	// 用于通知 goroutine 退出（防止 goroutine 泄漏）
	done := make(chan struct{})
	defer close(done)

	// Start goroutine to handle UI events from tools
	go func() {
		defer close(uiChannel)
		for {
			select {
			case <-done:
				// 请求结束，立即退出
				return

			case <-c.Request.Context().Done():
				// Client disconnected, stop listening
				return

			case event, ok := <-uiChannel:
				if !ok {
					// Channel closed
					return
				}

				// 非阻塞发送（防止客户端断开后阻塞）
				select {
				case <-c.Request.Context().Done():
					return
				default:
					// Send UI event through SSE
					h.sendSSE(c, flusher, "ui_event", event)
				}
			}
		}
	}()

	// 创建心跳定时器（每 30 秒发送一次心跳，保持连接活跃）
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Stream events
	for {
		// Check if client disconnected or heartbeat tick
		select {
		case <-c.Request.Context().Done():
			// Client disconnected
			return

		case <-ticker.C:
			// Send heartbeat to keep connection alive
			h.sendSSE(c, flusher, "heartbeat", map[string]interface{}{
				"timestamp": time.Now().Unix(),
			})
			continue

		default:
			// Continue to process events
		}

		// Get next event
		event, ok := iter.Next()
		if !ok {
			// Stream ended
			h.sendSSE(c, flusher, "done", nil)
			return
		}

		// Check for errors
		if event.Err != nil {
			h.sendSSE(c, flusher, "error", map[string]interface{}{
				"error": event.Err.Error(),
			})
			return
		}

		// Handle streaming output specially
		if event.Output != nil && event.Output.MessageOutput != nil && event.Output.MessageOutput.IsStreaming {
			// Process streaming messages
			h.handleStreamingOutput(c, flusher, event)
		} else {
			// Send regular event data
			eventData := h.formatAgentEvent(event)
			h.sendSSE(c, flusher, "message", eventData)
		}

	}
}

// sendSSE sends a Server-Sent Event
func (h *AgentHandler) sendSSE(c *gin.Context, flusher http.Flusher, eventType string, data interface{}) {
	var jsonData []byte
	var err error

	if data == nil {
		jsonData = []byte("{}")
	} else {
		jsonData, err = json.Marshal(data)
		if err != nil {
			// Send error event
			fmt.Fprintf(c.Writer, "event: error\ndata: {\"error\":\"failed to marshal data\"}\n\n")
			flusher.Flush()
			return
		}
	}

	// Format: event: <type>\ndata: <json>\n\n
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
		// Extract text content from AgentOutput
		// AgentOutput contains MessageOutput which may be streaming
		if event.Output.MessageOutput != nil {
			messageVariant := event.Output.MessageOutput

			// Handle non-streaming output
			message := messageVariant.Message
			if message != nil {
				// Check for reasoning content first
				if message.ReasoningContent != "" {
					result["reasoning"] = message.ReasoningContent
				}

				// First, try to use Content field (plain text)
				if message.Content != "" {
					result["output"] = message.Content
				} else if len(message.AssistantGenMultiContent) > 0 {
					// If Content is empty, try AssistantGenMultiContent (multimodal output)
					var textParts []string
					for _, part := range message.AssistantGenMultiContent {
						if part.Type == "text" {
							textParts = append(textParts, part.Text)
						}
						// Ignore other types like image/tool calls for streaming output
					}
					if len(textParts) > 0 {
						result["output"] = strings.Join(textParts, "")
					}
				}
			}
		} else if event.Output.CustomizedOutput != nil {
			// Handle custom output
			outputData, err := json.Marshal(event.Output.CustomizedOutput)
			if err != nil {
				result["output"] = fmt.Sprintf("%v", event.Output.CustomizedOutput)
			} else {
				result["output"] = string(outputData)
			}
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

	// Process all messages from stream
	for {
		msg, err := messageVariant.MessageStream.Recv()
		if err != nil {
			if err == io.EOF {
				// Stream ended
				break
			}
			// Send error event
			h.sendSSE(c, flusher, "error", map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		if msg == nil {
			continue
		}

		// Extract text content and reasoning
		var outputText string
		var reasoningText string

		// Check for reasoning content first
		if msg.ReasoningContent != "" {
			reasoningText = msg.ReasoningContent
		}

		if msg.Content != "" {
			outputText = msg.Content
		} else if len(msg.AssistantGenMultiContent) > 0 {
			// Extract text from multimodal content
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

		// Send event if we have either reasoning or text content
		if reasoningText != "" || outputText != "" {
			eventData := map[string]interface{}{
				"agent_name": event.AgentName,
			}

			// Include reasoning content if present
			if reasoningText != "" {
				eventData["reasoning"] = reasoningText
			}

			// Include regular output text if present
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
// @Router /agent/tools [get]
func (h *AgentHandler) GetTools(c *gin.Context) {
	registry := core.GetRegistry()
	tools := registry.GetAllToolInfos()

	// Convert ToolInfo to a serializable format
	result := make([]ToolInfoResponse, 0, len(tools))
	for _, tool := range tools {
		toolData := ToolInfoResponse{
			Name: tool.Name,
			Desc: tool.Desc,
		}

		// Include extra information if present
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
// @Router /agent/schemas [get]
func (h *AgentHandler) GetToolSchemas(c *gin.Context) {
	// This endpoint exists solely to register DTOs with Swagger
	// These DTOs are used in agent tool SideChannel events
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
