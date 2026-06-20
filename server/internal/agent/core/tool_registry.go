package core

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"server/internal/agent/ref"
	"server/internal/db/repo"
	"server/internal/search"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// SideChannelEvent is the control-plane→frontend event envelope. It carries
// tool lifecycle state and ref handles only — asset data never rides the
// side channel; the frontend hydrates refs over the hydration API (INV-1).
type SideChannelEvent struct {
	// Type is "tool_execution" for lifecycle updates or "widget_show" for
	// explicit show-terminal render requests.
	Type string `json:"type"`

	// Event timestamp (Unix milliseconds)
	Timestamp int64 `json:"timestamp"`

	// Tool identity information
	Tool ToolIdentity `json:"tool"`

	// Execution status and lifecycle
	Execution ExecutionInfo `json:"execution"`

	// Ref handle and rendering hints; never inline asset data.
	Data *DataPayload `json:"data,omitempty"`

	// Usage carries token accounting for token_usage events.
	Usage *TokenUsageInfo `json:"usage,omitempty"`
}

const (
	EventTypeToolExecution = "tool_execution"
	EventTypeWidgetShow    = "widget_show"
	EventTypeTokenUsage    = "token_usage"
)

// TokenUsageInfo reports the last model call's token accounting; prompt
// tokens are effectively the current context size of the conversation.
type TokenUsageInfo struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
}

// ToolIdentity identifies the tool instance
type ToolIdentity struct {
	Name        string `json:"name"`        // Tool identifier (e.g., "filter_assets")
	ExecutionID string `json:"executionId"` // Unique ID for this execution
}

// ExecutionStatus represents the current execution state
type ExecutionStatus string

const (
	ExecutionStatusRunning ExecutionStatus = "running"
	ExecutionStatusSuccess ExecutionStatus = "success"
	ExecutionStatusError   ExecutionStatus = "error"
)

// ExecutionInfo tracks tool execution lifecycle
type ExecutionInfo struct {
	Status     ExecutionStatus `json:"status"`               // Current execution status
	Message    string          `json:"message,omitempty"`    // Status description
	Error      *ErrorInfo      `json:"error,omitempty"`      // Error details if status is "error"
	Parameters interface{}     `json:"parameters,omitempty"` // Tool input parameters (from LLM)
	Duration   int64           `json:"duration,omitempty"`   // Execution duration in milliseconds
}

// ErrorInfo mirrors ref.Error for the frontend.
type ErrorInfo struct {
	Code    string `json:"code"`           // ref.Code value
	Message string `json:"message"`        // Human-readable error message
	Hint    string `json:"hint,omitempty"` // Recovery hint
}

// DataPayload references a ref and how to render it. The frontend fetches
// the actual assets from GET /api/v1/agent/refs/{id}/assets.
type DataPayload struct {
	RefID  string         `json:"refId"`
	Count  int            `json:"count"`
	Widget string         `json:"widget,omitempty"` // e.g. WidgetAssetGrid
	Params map[string]any `json:"params,omitempty"`
}

const (
	WidgetAssetGrid      = "asset_grid"
	WidgetFacetDashboard = "facet_dashboard"
	WidgetTimeline       = "timeline"
	WidgetStoryline      = "storyline"
)

// RetrieverSearch is the single-retriever search surface the producer tools
// wrap: the retriever's own ranking becomes the ref snapshot order (no RRF
// fusion). The semantic channel is a set retrieval — a per-query calibrated
// cutoff decides membership, not a fixed TopK. Implemented by
// service.AssetService.
type RetrieverSearch interface {
	SearchAssetIDsSemantic(ctx context.Context, query string, strictness search.SetStrictness, maxResults int) ([]uuid.UUID, search.SetMeta, error)
	SearchAssetIDsOCR(ctx context.Context, query string, maxResults int) ([]uuid.UUID, error)
}

// ToolDependencies carries the per-request state injected into tool
// factories: DB access, the side channel, the ref store and the scope that
// pins every created ref to (user, thread) (INV-4).
type ToolDependencies struct {
	Queries     *repo.Queries
	SideChannel chan<- *SideChannelEvent
	RefStore    ref.Store
	Search      RetrieverSearch
	UserID      int32
	ThreadID    string
}

// Scope returns the ref scope for this request.
func (d *ToolDependencies) Scope() ref.Scope {
	return ref.Scope{UserID: d.UserID, ThreadID: d.ThreadID}
}

// Send emits a side channel event if the channel is attached.
func (d *ToolDependencies) Send(event *SideChannelEvent) {
	if d.SideChannel != nil {
		d.SideChannel <- event
	}
}

type ToolFactory func(ctx context.Context, deps *ToolDependencies) (tool.BaseTool, error)

type ToolRegistry struct {
	mu        sync.RWMutex
	factories map[string]ToolFactory
	infos     map[string]*schema.ToolInfo
}

var (
	registry *ToolRegistry
	once     sync.Once
)

func GetRegistry() *ToolRegistry {
	once.Do(func() {
		registry = &ToolRegistry{
			factories: make(map[string]ToolFactory),
			infos:     make(map[string]*schema.ToolInfo),
		}
	})
	return registry
}

// Register 注册一个工具工厂
func (r *ToolRegistry) Register(info *schema.ToolInfo, factory ToolFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[info.Name] = factory
	r.infos[info.Name] = info
}

// GetAllTools instantiates every registered tool with the request-scoped
// dependencies, in deterministic name order. The agent always runs with the
// full toolset so checkpoint resume sees an identical configuration.
func (r *ToolRegistry) GetAllTools(ctx context.Context, deps *ToolDependencies) ([]tool.BaseTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	sort.Strings(names)

	tools := make([]tool.BaseTool, 0, len(names))
	for _, name := range names {
		t, err := r.factories[name](ctx, deps)
		if err != nil {
			return nil, fmt.Errorf("failed to create tool %s: %w", name, err)
		}
		tools = append(tools, t)
	}
	return tools, nil
}

// GetAllToolInfos 获取所有注册工具的元数据 (用于 /api/agent/tools 接口)
func (r *ToolRegistry) GetAllToolInfos() []*schema.ToolInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var infos []*schema.ToolInfo
	for _, info := range r.infos {
		infos = append(infos, info)
	}
	return infos
}
