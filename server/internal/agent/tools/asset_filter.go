package tools

import (
	"context"
	"fmt"
	"server/internal/agent/core"
	"server/internal/api/dto"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

// AssetFilterInput defines how the LLM calls this tool
type AssetFilterInput struct {
	DateFrom string `json:"date_from,omitempty" jsonschema:"description=Start date in YYYY-MM-DD format"`
	DateTo   string `json:"date_to,omitempty" jsonschema:"description=End date in YYYY-MM-DD format"`
	Type     string `json:"type,omitempty" jsonschema:"description=Asset type (PHOTO | VIDEO | AUDIO)"`
	Filename string `json:"filename,omitempty" jsonschema:"description=Filename pattern to search for"`
	Raw      *bool  `json:"raw,omitempty" jsonschema:"description=Filter for RAW photos only"`
	Rating   *int   `json:"rating,omitempty" jsonschema:"description=Filter by rating (0-5)"`
	Liked    *bool  `json:"liked,omitempty" jsonschema:"description=Filter for liked/favorited assets"`
}

// AssetFilterOutput tool execution result
type AssetFilterOutput struct {
	Message  string              `json:"message" jsonschema:"description=A human-readable message describing the outcome."`
	Filter   *dto.AssetFilterDTO `json:"filter,omitempty" jsonschema:"description=The applied filter configuration."`
	RefID    string              `json:"ref_id,omitempty" jsonschema:"description=A reference ID for the stored filter configuration."`
	Duration int64               `json:"duration_ms,omitempty" jsonschema:"description=The duration of the tool execution in milliseconds."`
}

func RegisterFilterAsset() {
	info := &schema.ToolInfo{
		Name: "filter_assets",
		Desc: "Constructs a filter configuration for assets. You should set Type per user request correctly. Use this to help the user view specific assets. " +
			"Returns a ref_id that can be used by other tools (like bulk_like_assets) to act on these criteria.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		t, err := utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *AssetFilterInput) (*AssetFilterOutput, error) {
			startTime := time.Now()
			executionID := fmt.Sprintf("%d", startTime.UnixNano())

			sendPendingEvent(ctx, deps, executionID, startTime, input)
			sendRunningEvent(ctx, deps, executionID, "Configuring asset filter...")

			// Build the DTO directly from input
			filterDTO := buildFilterDTO(input)

			// Store the filter DTO in ReferenceManager
			refID := ""
			if deps.ReferenceManager != nil {
				// We store the DTO itself, not the assets
				refID = deps.ReferenceManager.StoreWithID(ctx, filterDTO, "Asset Filter Configuration")
			}

			// Send the filter configuration to the frontend via side channel
			sendFilterSuccessEvent(ctx, deps, executionID, startTime.UnixMilli(), input, filterDTO, refID)

			duration := time.Since(startTime).Milliseconds()

			return &AssetFilterOutput{
				Message: fmt.Sprintf("Filter applied. RefID: %s. "+
					"Guide the user to view the results in the gallery by clicking the button below.", refID),
				Filter:   &filterDTO,
				RefID:    refID,
				Duration: duration,
			}, nil
		})
		if err != nil {
			return nil, err
		}
		return t, nil
	})
}

// --- Helper Functions ---

func buildFilterDTO(input *AssetFilterInput) dto.AssetFilterDTO {
	filter := dto.AssetFilterDTO{}

	// Date Range
	if input.DateFrom != "" || input.DateTo != "" {
		dateRange := &dto.DateRangeDTO{}
		if input.DateFrom != "" {
			if t, err := time.Parse("2006-01-02", input.DateFrom); err == nil {
				dateRange.From = &t
			}
		}
		if input.DateTo != "" {
			if t, err := time.Parse("2006-01-02", input.DateTo); err == nil {
				dateRange.To = &t
			}
		}
		filter.Date = dateRange
	}

	// Type
	if input.Type != "" {
		filter.Type = &input.Type
	}

	// Filename
	if input.Filename != "" {
		filter.Filename = &dto.FilenameFilterDTO{
			Value: input.Filename,
			Mode:  "contains", // Default mode
		}
	}

	// Boolean/Int flags
	if input.Raw != nil {
		filter.RAW = input.Raw
	}
	if input.Rating != nil {
		filter.Rating = input.Rating
	}
	if input.Liked != nil {
		filter.Liked = input.Liked
	}

	return filter
}

func sendPendingEvent(ctx context.Context, deps *core.ToolDependencies, execID string, startTime time.Time, input *AssetFilterInput) {
	if deps.SideChannel == nil {
		return
	}
	deps.SideChannel <- &core.SideChannelEvent{
		Type:      "tool_execution",
		Timestamp: startTime.UnixMilli(),
		Tool:      core.ToolIdentity{Name: "filter_assets", ExecutionID: execID},
		Execution: core.ExecutionInfo{Status: core.ExecutionStatusPending, Message: "Preparing filter configuration...", Parameters: input},
	}
}

func sendRunningEvent(ctx context.Context, deps *core.ToolDependencies, execID string, message string) {
	if deps.SideChannel == nil {
		return
	}
	deps.SideChannel <- &core.SideChannelEvent{
		Type:      "tool_execution",
		Timestamp: time.Now().UnixMilli(),
		Tool:      core.ToolIdentity{Name: "filter_assets", ExecutionID: execID},
		Execution: core.ExecutionInfo{Status: core.ExecutionStatusRunning, Message: message},
	}
}

func sendFilterSuccessEvent(ctx context.Context, deps *core.ToolDependencies, execID string, startTimeMs int64, input *AssetFilterInput, filterDTO dto.AssetFilterDTO, refID string) {
	if deps.SideChannel == nil {
		return
	}
	deps.SideChannel <- &core.SideChannelEvent{
		Type:      "tool_execution",
		Timestamp: time.Now().UnixMilli(),
		Tool:      core.ToolIdentity{Name: "filter_assets", ExecutionID: execID},
		Execution: core.ExecutionInfo{
			Status:     core.ExecutionStatusSuccess,
			Message:    "Filter applied successfully",
			Duration:   time.Now().UnixMilli() - startTimeMs,
			Parameters: input,
		},
		Data: &core.DataPayload{
			RefID:       refID,
			PayloadType: "AssetFilterDTO",
			Payload:     filterDTO,
			Rendering: &core.RenderingConfig{
				Component: core.ComponentJustifiedGallery,
				Config:    &core.JustifiedGalleryConfig{GroupBy: "date"},
			},
		},
	}
}
