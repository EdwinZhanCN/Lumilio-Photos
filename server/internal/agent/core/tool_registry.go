package core

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"server/internal/db/repo"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// SideChannelEvent represents an event sent through the side channel to frontend
// This provides real-time execution status and data without passing through LLM
type SideChannelEvent struct {
	// Event type identifier
	Type string `json:"type"`

	// Event timestamp (Unix milliseconds)
	Timestamp int64 `json:"timestamp"`

	// Tool identity information
	Tool ToolIdentity `json:"tool"`

	// Execution status and lifecycle
	Execution ExecutionInfo `json:"execution"`

	// Data payload for frontend rendering
	Data *DataPayload `json:"data,omitempty"`

	// Extra type-specific information
	Extra *ExtraInfo `json:"extra,omitempty"`

	// Optional metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ToolIdentity identifies the tool instance
type ToolIdentity struct {
	Name              string `json:"name"`                        // Tool identifier (e.g., "filter_assets")
	ExecutionID       string `json:"executionId"`                 // Unique ID for this execution
	ParentExecutionID string `json:"parentExecutionId,omitempty"` // Parent execution ID for chained calls
}

// ExecutionStatus represents the current execution state
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusSuccess   ExecutionStatus = "success"
	ExecutionStatusError     ExecutionStatus = "error"
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
)

// ExecutionInfo tracks tool execution lifecycle
type ExecutionInfo struct {
	Status     ExecutionStatus `json:"status"`               // Current execution status
	Message    string          `json:"message,omitempty"`    // Status description
	Error      *ErrorInfo      `json:"error,omitempty"`      // Error details if status is "error"
	Parameters interface{}     `json:"parameters,omitempty"` // Tool input parameters (from LLM)
	Duration   int64           `json:"duration,omitempty"`   // Execution duration in milliseconds
}

// ErrorInfo captures structured error information
type ErrorInfo struct {
	Code    string      `json:"code"`              // Error code for programmatic handling
	Message string      `json:"message"`           // Human-readable error message
	Details interface{} `json:"details,omitempty"` // Additional error context
}

// DataPayload contains data for frontend rendering
type DataPayload struct {
	RefID       string           `json:"refId"`                  // Reference ID for this data
	PayloadType string           `json:"payload_type,omitempty"` // DTO type name (e.g., "AssetDTO[]")
	Payload     interface{}      `json:"payload,omitempty"`      // Actual DTO data
	Rendering   *RenderingConfig `json:"rendering,omitempty"`    // Rendering configuration
}

// RenderingConfig defines how data should be displayed
type RenderingConfig struct {
	Component ComponentType `json:"component"` // Component type
	Config    interface{}   `json:"config,omitempty"`
}

// ComponentType represents available frontend components
type ComponentType string

const (
	ComponentJustifiedGallery ComponentType = "justified_gallery"
	// ComponentDataTable    ComponentType = "data_table"
	// ComponentChart        ComponentType = "chart"
	// ComponentCustom       ComponentType = "custom"
)

type JustifiedGalleryConfig struct {
	GroupBy string `json:"groupBy"` // "date", "type", "flat"
}

// ExtraInfo provides tool-specific extension data
type ExtraInfo struct {
	ExtraType string      `json:"extra_type"` // OpenAPI DTO type name (e.g., "FilterAssetsRequestDTO")
	Data      interface{} `json:"data"`       // Extra data structure
}

// FilterConfirmationInfo is the information sent to the user when a tool interrupt occurs
type FilterConfirmationInfo struct {
	Count          int    `json:"count"`
	ConfirmationID string `json:"confirmationId"` // Used on resume to identify the confirmation
	Message        string `json:"message"`
}

// FilterInterruptState is the state saved during a tool interrupt
type FilterInterruptState struct {
	RefID       string `json:"ref_id"`
	Count       int    `json:"count"`
	ExecutionID string `json:"execution_id"`
	StartTime   int64  `json:"start_time"`
}

// EventDispatcher defines the interface for sending side channel events
type EventDispatcher interface {
	Dispatch(event *SideChannelEvent)
}

// sideChannelDispatcher implements EventDispatcher with non-blocking send
type sideChannelDispatcher struct {
	ch chan<- *SideChannelEvent
}

func NewEventDispatcher(ch chan<- *SideChannelEvent) EventDispatcher {
	if ch == nil {
		return nil
	}
	return &sideChannelDispatcher{ch: ch}
}

func (d *sideChannelDispatcher) Dispatch(event *SideChannelEvent) {
	if d == nil || d.ch == nil {
		return
	}

	select {
	case d.ch <- event:
		// Sent successfully
	case <-time.After(200 * time.Millisecond):
		// Timeout, drop event to avoid blocking the agent
		log.Printf("Warning: Side channel blocked, dropping event of type %s", event.Type)
	}
}

// ToolDependencies Defines the dependencies like database queries that tool execution needs
type ToolDependencies struct {
	Queries          *repo.Queries
	Dispatcher       EventDispatcher   // Dispatcher sends DTO data to frontend, bypassing LLM
	ReferenceManager *ReferenceManager // ReferenceManager stores and manages tool outputs across session
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

// GetTools 根据工具名称列表获取工具实例
// 关键点：这里接收 deps，并在创建工具时注入，实现了请求级别的隔离
//
// 自动包装：如果 deps.InputExtractor 存在，所有工具都会被 ToolWrapper 包装，
// 以实现 Reference[T] 的自动解析功能
func (r *ToolRegistry) GetTools(ctx context.Context, toolNames []string, deps *ToolDependencies) ([]tool.BaseTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tools []tool.BaseTool
	for _, name := range toolNames {
		if factory, ok := r.factories[name]; ok {
			t, err := factory(ctx, deps)
			if err != nil {
				return nil, fmt.Errorf("failed to create tool %s: %w", name, err)
			}

			tools = append(tools, t)

		}
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
