package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/facets"
	"server/internal/agent/inject"
	"server/internal/agent/pins"
	"server/internal/agent/ref"
	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/db/repo"
	"server/internal/service"

	"github.com/cloudwego/eino/adk"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	refAssetsDefaultLimit = 50
	refAssetsMaxLimit     = 200
)

// AgentHandler handles agent-related HTTP requests
type AgentHandler struct {
	agentService core.AgentService
	refStore     ref.Store
	queries      *repo.Queries
	pins         *pins.Service
	assetService service.AssetService
}

// NewAgentHandler creates a new agent handler
func NewAgentHandler(agentService core.AgentService, refStore ref.Store, queries *repo.Queries, pinService *pins.Service, assetService service.AssetService) *AgentHandler {
	return &AgentHandler{
		agentService: agentService,
		refStore:     refStore,
		queries:      queries,
		pins:         pinService,
		assetService: assetService,
	}
}

// AgentChatRequest represents request body for agent chat
type AgentChatRequest struct {
	ThreadID string               `json:"thread_id,omitempty"`
	Query    string               `json:"query" binding:"required"`
	Mode     string               `json:"mode,omitempty" enums:"review,organize,analyze,curate"`
	Context  []inject.ContextItem `json:"context,omitempty"`
	Mentions []inject.MentionItem `json:"mentions,omitempty"`
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
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/agent/chat [post]
func (h *AgentHandler) Chat(c *gin.Context) {
	var req AgentChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	user, ok := requireCurrentUser(c)
	if !ok {
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

	// 创建工具侧信道
	sideChannel := make(chan *core.SideChannelEvent, 100)

	prepared, err := inject.Prepare(c.Request.Context(), inject.Dependencies{
		Queries:  h.queries,
		RefStore: h.refStore,
		Pins:     h.pins,
		UserID:   int32(user.UserID),
		ThreadID: threadID,
	}, req.Context, req.Mentions)
	if err != nil {
		api.GinInternalError(c, err, "Failed to prepare agent context")
		return
	}

	// 获取 Agent 迭代器
	iter := h.agentService.AskAgent(c.Request.Context(), int32(user.UserID), threadID, req.Query, prepared.InstructionExtras, req.Mode, sideChannel)

	// 发送会话信息事件
	sessionInfo := map[string]any{"thread_id": threadID}
	if len(prepared.DroppedMentions) > 0 {
		sessionInfo["dropped_mentions"] = prepared.DroppedMentions
	}
	h.sendSSE(c, flusher, "session_info", sessionInfo)

	// 开始流式传输事件
	h.streamAgentEvents(c, flusher, iter, sideChannel)
}

// ResumeChat handles resuming an interrupted agent execution
// @Summary Resume Agent Chat
// @Description Resume a conversation from an interrupt point (e.g., user confirmation for a tool call)
// @Tags agent
// @Accept json
// @Produce text/event-stream
// @Param request body AgentResumeRequest true "Resume request"
// @Success 200 {string} string "SSE stream"
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/agent/chat/resume [post]
func (h *AgentHandler) ResumeChat(c *gin.Context) {
	var req AgentResumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	flusher, err := h.prepareSSE(c)
	if err != nil {
		api.GinInternalError(c, err)
		return
	}

	sideChannel := make(chan *core.SideChannelEvent, 100)

	// 构建 Resume 参数
	params := &adk.ResumeParams{
		Targets: req.Targets,
	}

	iter, err := h.agentService.ResumeAgent(c.Request.Context(), int32(user.UserID), req.ThreadID, params, sideChannel)
	if err != nil {
		h.sendSSE(c, flusher, "error", map[string]interface{}{"error": err.Error()})
		return
	}

	h.sendSSE(c, flusher, "session_info", map[string]string{"thread_id": req.ThreadID})
	h.streamAgentEvents(c, flusher, iter, sideChannel)
}

// GetRef returns ref metadata with facets (hydration: control plane handle →
// data plane summary). Cross-scope, missing and expired refs are all 404 —
// existence never leaks (INV-4).
// @Summary Get Agent Ref Metadata
// @Description Get metadata and facet summary for an agent ref. Refs are scoped to the requesting user and thread.
// @Tags agent
// @Produce json
// @Param id path string true "Ref ID"
// @Param thread_id query string true "Thread (conversation) the ref belongs to"
// @Success 200 {object} dto.AgentRefDTO
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 404 {object} api.ErrorResponse "Ref not found"
// @Router /api/v1/agent/refs/{id} [get]
func (h *AgentHandler) GetRef(c *gin.Context) {
	r, ok := h.resolveRef(c)
	if !ok {
		return
	}

	facetSummary, err := facets.Build(c.Request.Context(), h.queries, r)
	if err != nil {
		api.GinInternalError(c, err, "Failed to compute ref facets")
		return
	}

	api.JSONOK(c, dto.AgentRefDTO{
		RefID:     r.ID,
		Count:     r.Count(),
		Truncated: r.Truncated,
		Op:        r.Plan.Op,
		CreatedAt: r.CreatedAt,
		Facets:    dto.ToAgentRefFacetsDTO(facetSummary),
	})
}

// GetRefAssets returns one hydration page of a ref in snapshot order. This
// is the data plane: asset data flows here, never through the LLM (INV-1).
// @Summary Get Agent Ref Assets
// @Description Get a page of assets for an agent ref, in snapshot order.
// @Tags agent
// @Produce json
// @Param id path string true "Ref ID"
// @Param thread_id query string true "Thread (conversation) the ref belongs to"
// @Param limit query int false "Page size (default 50, max 200)"
// @Param offset query int false "Page offset (default 0)"
// @Success 200 {object} dto.AgentRefAssetsDTO
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 404 {object} api.ErrorResponse "Ref not found"
// @Router /api/v1/agent/refs/{id}/assets [get]
func (h *AgentHandler) GetRefAssets(c *gin.Context) {
	r, ok := h.resolveRef(c)
	if !ok {
		return
	}

	limit, offset := refAssetsDefaultLimit, 0
	if v, err := strconv.Atoi(c.DefaultQuery("limit", "")); err == nil && v > 0 {
		limit = min(v, refAssetsMaxLimit)
	}
	if v, err := strconv.Atoi(c.DefaultQuery("offset", "")); err == nil && v >= 0 {
		offset = v
	}

	page := r.Slice(offset, limit)
	assets := make([]dto.AssetDTO, 0, len(page))
	if len(page) > 0 {
		pgIDs := make([]pgtype.UUID, len(page))
		for i, id := range page {
			pgIDs[i] = pgtype.UUID{Bytes: id, Valid: true}
		}
		rows, err := h.queries.GetAssetsByIDs(c.Request.Context(), pgIDs)
		if err != nil {
			api.GinInternalError(c, err, "Failed to load ref assets")
			return
		}
		// GetAssetsByIDs has no order guarantee; restore snapshot order.
		byID := make(map[uuid.UUID]repo.Asset, len(rows))
		for _, row := range rows {
			byID[uuid.UUID(row.AssetID.Bytes)] = row
		}
		for _, id := range page {
			if row, found := byID[id]; found {
				assets = append(assets, dto.ToAssetDTO(row))
			}
		}
	}

	api.JSONOK(c, dto.AgentRefAssetsDTO{
		Assets:     assets,
		Total:      r.Count(),
		Pagination: dto.PaginationDTO{Limit: limit, Offset: offset},
	})
}

// resolveRef authenticates the caller and dereferences the ref within their
// (user, thread) scope, answering 404 for every failure mode.
func (h *AgentHandler) resolveRef(c *gin.Context) (*ref.Ref, bool) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return nil, false
	}

	threadID := c.Query("thread_id")
	refID := c.Param("id")
	if threadID == "" || refID == "" {
		api.GinNotFound(c, errors.New("ref not found"), "Ref not found")
		return nil, false
	}

	scope := ref.Scope{UserID: int32(user.UserID), ThreadID: threadID}
	r, refErr := h.refStore.Get(scope, refID)
	if refErr != nil {
		api.GinNotFound(c, errors.New("ref not found"), "Ref not found")
		return nil, false
	}
	return r, true
}

// prepareSSE sets the necessary headers for Server-Sent Events.
func (h *AgentHandler) prepareSSE(c *gin.Context) (http.Flusher, error) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no") // Disable Nginx buffering

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}
	return flusher, nil
}

// streamAgentEvents handles the main loop for streaming agent and side-channel events via SSE.
func (h *AgentHandler) streamAgentEvents(c *gin.Context, flusher http.Flusher, iter *adk.AsyncIterator[*adk.AgentEvent], sideChannel chan *core.SideChannelEvent) {
	// Channel to receive agent events from the iterator in a non-blocking manner.
	type iterResult struct {
		event *adk.AgentEvent
		ok    bool
	}
	eventChan := make(chan iterResult)

	// Run iterator in a separate goroutine to avoid blocking on iter.Next()
	go func() {
		for {
			event, ok := iter.Next()
			select {
			case <-c.Request.Context().Done():
				return
			case eventChan <- iterResult{event, ok}:
				if !ok {
					return
				}
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

		case sideEvent, ok := <-sideChannel:
			if !ok {
				sideChannel = nil
				continue
			}
			h.sendSSE(c, flusher, "side_event", sideEvent)

		case res := <-eventChan:
			if !res.ok {
				// Main iterator completed.
				// Before exiting, we must drain any remaining events from sideChannel.
				if sideChannel != nil {
					draining := true
					for draining {
						select {
						case sideEvent, ok := <-sideChannel:
							if ok {
								h.sendSSE(c, flusher, "side_event", sideEvent)
							} else {
								draining = false
							}
						default:
							draining = false
						}
					}
				}
				h.sendSSE(c, flusher, "done", nil)
				return
			}

			event := res.event
			// Guardrail: Skip internal-only events that are not meant for the user.
			// An event with only `CustomizedOutput` is a raw tool result intended for the next LLM reasoning step.
			if event.Output != nil && event.Output.MessageOutput == nil && event.Output.CustomizedOutput != nil {
				continue
			}

			if event.Err != nil {
				log.Printf("[AgentHandler] Error event: %v", event.Err)
				h.sendSSE(c, flusher, "error", map[string]interface{}{"error": event.Err.Error()})
				return
			}

			if event.Output != nil && event.Output.MessageOutput != nil && event.Output.MessageOutput.IsStreaming {
				h.handleStreamingOutput(c, flusher, event)
			} else {
				eventData := h.formatAgentEvent(event)
				if len(eventData) > 0 {
					h.sendSSE(c, flusher, "message", eventData)
				}
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
// @Description Get the agent tools visible in the given quick-action mode. An empty or unknown mode returns the full toolset.
// @Tags agent
// @Produce json
// @Param mode query string false "Quick-action mode" Enums(review, organize, analyze, curate)
// @Success 200 {array} ToolInfoResponse
// @Router /api/v1/agent/tools [get]
func (h *AgentHandler) GetTools(c *gin.Context) {
	tools := h.agentService.GetToolsByMode(c.Query("mode"))

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
	api.JSONOK(c, result)
}
