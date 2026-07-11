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
	Widget string         `json:"widget,omitempty"` // e.g. WidgetCoverCard
	Params map[string]any `json:"params,omitempty"`
}

const (
	WidgetCoverCard  = "cover_card"
	WidgetNumberCard = "number_card"
	WidgetSparkCard  = "spark_card"
	WidgetMosaicCard = "mosaic_card"
)

// knownWidgets is the set of valid view identifiers a pin may render through.
// A widget is just the currently selected view over a pinned ref, so callers
// validate user-supplied view choices against this set before persisting.
var knownWidgets = map[string]bool{
	WidgetCoverCard:  true,
	WidgetNumberCard: true,
	WidgetSparkCard:  true,
	WidgetMosaicCard: true,
}

// IsKnownWidget reports whether view is a registered widget/view identifier.
func IsKnownWidget(view string) bool {
	return knownWidgets[view]
}

// modeToolSets maps a quick-action mode to the set of tool names the agent
// may see in that mode. An empty or unknown mode yields the full toolset
// (free / default mode). Tools outside the set are never instantiated, so
// the model cannot call or even observe them — progressive disclosure at
// the registry level.
var modeToolSets = map[string]map[string]bool{
	"review": {
		"filter_assets": true,
		"search_people": true,
		"lookup_people": true,
		"describe":      true,
		"sample":        true,
		"rank":          true,
		"show":          true,
	},
	"organize": {
		"filter_assets":   true,
		"search_semantic": true,
		"search_people":   true,
		"lookup_people":   true,
		"lookup_albums":   true,
		"describe":        true,
		"peek":            true,
		"combine":         true,
		"tag_assets":      true,
		"create_album":    true,
		"add_to_album":    true,
		"show":            true,
	},
	"analyze": {
		"filter_assets":   true,
		"search_semantic": true,
		"search_text":     true,
		"search_people":   true,
		"lookup_people":   true,
		"describe":        true,
		"inspect":         true,
		"peek":            true,
		"combine":         true,
		"show":            true,
	},
	"curate": {
		"filter_assets":   true,
		"search_semantic": true,
		"search_people":   true,
		"lookup_people":   true,
		"lookup_albums":   true,
		"combine":         true,
		"rank":            true,
		"top":             true,
		"sample":          true,
		"dedupe":          true,
		"describe":        true,
		"create_album":    true,
		"add_to_album":    true,
		"show":            true,
	},
}

// modeInstructionExtras appends a mode-specific behaviour prompt after the
// base instruction. Empty for free mode.
var modeInstructionExtras = map[string]string{
	"review": "You are in REVIEW MODE. Help the user review a period of their photography: " +
		"use filter to scope the time range, describe to understand the overall picture, " +
		"sample(spread_over_time) to pick representative photos, then show and give a narrative summary. " +
		"Focus on timeline and memory narrative. Do not organize or modify photos.",
	"organize": "You are in ORGANIZE MODE. Help the user group and archive photos: " +
		"use filter/describe to understand the full picture, reason about sensible groupings " +
		"(by place/time/people), then use tag_assets to label and create_album to organize. " +
		"Proactively suggest an organization plan and state your intent before each step.",
	"analyze": "You are in ANALYZE MODE. Help the user discover shooting habits and trends: " +
		"use filter to scope, describe for aggregate distributions (cameras, focal lengths, lenses, " +
		"places, time histogram), inspect for per-photo gear details. " +
		"Give data-driven insights (most-used focal length, peak shooting periods, how gear/lens and " +
		"location choices shift over time) and use show to present supporting photos.",
	"curate": "You are in CURATE MODE. Help the user pick the best photos: " +
		"use filter to build a candidate pool (min_quality_percentile drops the lower aesthetic tier " +
		"of the matched set — e.g. 75 keeps scores at/above that set's p75), rank(quality) to sort " +
		"(quality is based on the SigLIP aesthetic model score 1-10; scores cluster in the 5-7 range " +
		"with median ~5.5-6, 7+ is already a good photo and 8+ is extremely rare), " +
		"dedupe to collapse bursts (keep_per_cluster>1 when the user wants a few frames per burst), " +
		"top to trim. Watch for diversity — avoid burst/duplicate picks. " +
		"Give brief selection rationale, show the result, and offer create_album or add_to_album " +
		"so the shortlist becomes a durable delivery (user must confirm).",
}

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

// GetToolsByMode instantiates only the tools allowed in the given mode. An
// empty or unknown mode returns the full toolset (free mode).
func (r *ToolRegistry) GetToolsByMode(ctx context.Context, deps *ToolDependencies, mode string) ([]tool.BaseTool, error) {
	if mode == "" {
		return r.GetAllTools(ctx, deps)
	}
	toolSet, ok := modeToolSets[mode]
	if !ok {
		return r.GetAllTools(ctx, deps)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(toolSet))
	for name := range r.factories {
		if toolSet[name] {
			names = append(names, name)
		}
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

// GetToolInfosByMode returns metadata for the tools allowed in the given
// mode. Empty or unknown mode returns all tool infos.
func (r *ToolRegistry) GetToolInfosByMode(mode string) []*schema.ToolInfo {
	if mode == "" {
		return r.GetAllToolInfos()
	}
	toolSet, ok := modeToolSets[mode]
	if !ok {
		return r.GetAllToolInfos()
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var infos []*schema.ToolInfo
	for name, info := range r.infos {
		if toolSet[name] {
			infos = append(infos, info)
		}
	}
	return infos
}

// ModeHasTool reports whether a tool is visible in the given mode. An empty or
// unknown mode is the full toolset, so every tool is available. Used by the
// instruction builder so static guidance never names a tool the current mode
// has filtered out.
func ModeHasTool(mode, name string) bool {
	if mode == "" {
		return true
	}
	toolSet, ok := modeToolSets[mode]
	if !ok {
		return true
	}
	return toolSet[name]
}

// ModeInstruction returns the mode-specific prompt fragment, or "" for
// free mode.
func ModeInstruction(mode string) string {
	if mode == "" {
		return ""
	}
	extra, ok := modeInstructionExtras[mode]
	if !ok {
		return ""
	}
	return "\n" + extra
}
