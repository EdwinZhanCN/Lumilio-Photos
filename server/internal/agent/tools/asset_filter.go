package tools

import (
	"context"
	"fmt"
	"server/internal/agent/core"
	"server/internal/api/dto"
	"server/internal/db/repo"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"

	"github.com/cloudwego/eino/schema"
)

const confirmationThreshold = 1 // If more than 1 asset is found, ask for confirmation

// AssetFilterInput 定义了 LLM 如何调用此工具
type AssetFilterInput struct {
	DateFrom string `json:"date_from,omitempty" jsonschema:"description=Start date in YYYY-MM-DD format"`
	DateTo   string `json:"date_to,omitempty" jsonschema:"description=End date in YYYY-MM-DD format"`
	Type     string `json:"type,omitempty" jsonschema:"description=Asset type (PHOTO, VIDEO, AUDIO)"`
	Filename string `json:"filename,omitempty" jsonschema:"description=Filename pattern to search for"`
	Raw      *bool  `json:"raw,omitempty" jsonschema:"description=Filter for RAW photos only"`
	Rating   *int   `json:"rating,omitempty" jsonschema:"description=Filter by rating (0-5)"`
	Liked    *bool  `json:"liked,omitempty" jsonschema:"description=Filter for liked/favorited assets"`
}

// AssetFilterOutput 工具执行结果
type AssetFilterOutput struct {
	Message  string `json:"message"`
	RefID    string `json:"ref_id,omitempty"`
	Count    int    `json:"count,omitempty"`
	Duration int64  `json:"duration_ms,omitempty"`
}

func RegisterFilterAsset() {
	info := &schema.ToolInfo{
		Name: "filter_assets",
		Desc: "Search and filter assets.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		// Use InferTool to automatically generate the parameter schema from AssetFilterInput's struct tags.
		t, err := utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *AssetFilterInput) (*AssetFilterOutput, error) {
			// Correct Interrupt/Resume Flow:
			// 1. Check if the tool's state was saved from a *previous* interrupt in this run.
			wasInterrupted, hasState, state := compose.GetInterruptState[*core.FilterInterruptState](ctx)
			if wasInterrupted {
				// This execution is the result of a resumed flow.
				// 2. Now, check if this tool is the *specific target* of the user's resume action.
				isResumeTarget, _, _ := compose.GetResumeContext[any](ctx)
				if isResumeTarget {
					// Yes, the user confirmed this tool. Use the saved state to proceed.
					if !hasState {
						return nil, fmt.Errorf("tool was resumed but required state is missing")
					}
					return handleFilterResult(ctx, deps, input, state.RefID, state.Count, state.ExecutionID, state.StartTime)
				} else {
					// No, the user is resuming a different tool. We must re-interrupt to preserve our own state.
					interruptInfo := &core.FilterConfirmationInfo{
						Count:          state.Count,
						ConfirmationID: "filter_assets_confirmation",
						Message:        fmt.Sprintf("Found %d assets. Do you want to display them?", state.Count),
					}
					return nil, compose.StatefulInterrupt(ctx, interruptInfo, state)
				}
			}

			// =====================================================
			// Initial Execution Flow (not a resume)
			// =====================================================
			startTime := time.Now()
			executionID := fmt.Sprintf("%d", time.Now().UnixNano())
			sendPendingEvent(ctx, deps, executionID, startTime, input)
			sendRunningEvent(ctx, deps, executionID, "Querying database...")

			params := buildFilterParams(input)
			assets, err := deps.Queries.FilterAssets(ctx, params)
			if err != nil {
				return handleFilterError(ctx, deps, executionID, startTime, err)
			}

			count := len(assets)
			if count == 0 {
				return handleNoResults(ctx, deps, executionID, startTime.UnixMilli())
			}

			// If results exceed threshold and not forced, trigger a stateful interrupt.
			if count > confirmationThreshold {
				refID := storeAssets(ctx, deps, assets, input)
				interruptInfo := &core.FilterConfirmationInfo{
					Count:          count,
					ConfirmationID: "filter_assets_confirmation",
					Message:        fmt.Sprintf("Found %d assets. Do you want to display them?", count),
				}
				interruptState := &core.FilterInterruptState{
					RefID:       refID,
					Count:       count,
					ExecutionID: executionID,
					StartTime:   startTime.UnixMilli(),
				}
				return nil, compose.StatefulInterrupt(ctx, interruptInfo, interruptState)
			}

			// No confirmation needed, proceed directly.
			refID := storeAssets(ctx, deps, assets, input)
			return handleFilterResult(ctx, deps, input, refID, count, executionID, startTime.UnixMilli())
		})
		if err != nil {
			return nil, err
		}
		return t, nil
	})
}

// --- 以下为辅助函数 ---

func buildFilterParams(input *AssetFilterInput) repo.FilterAssetsParams {
	params := repo.FilterAssetsParams{Limit: 30, Offset: 0}
	if input.DateFrom != "" {
		if t, err := time.Parse("2006-01-02", input.DateFrom); err == nil {
			params.DateFrom.Time = t
			params.DateFrom.Valid = true
		}
	}
	if input.DateTo != "" {
		if t, err := time.Parse("2006-01-02", input.DateTo); err == nil {
			params.DateTo.Time = t
			params.DateTo.Valid = true
		}
	}
	if input.Type != "" {
		params.AssetType = &input.Type
	}
	if input.Filename != "" {
		mode := "contains"
		params.FilenameMode = &mode
	}
	if input.Raw != nil {
		params.IsRaw = input.Raw
	}
	if input.Rating != nil {
		rating := int32(*input.Rating)
		params.Rating = &rating
	}
	if input.Liked != nil {
		params.Liked = input.Liked
	}
	return params
}

func storeAssets(ctx context.Context, deps *core.ToolDependencies, assets []repo.Asset, input *AssetFilterInput) string {
	if deps.ReferenceManager == nil {
		return ""
	}
	description := fmt.Sprintf("%d filtered assets", len(assets))
	return deps.ReferenceManager.StoreWithID(ctx, assets, description)
}

func handleFilterResult(ctx context.Context, deps *core.ToolDependencies, input *AssetFilterInput, refID string, count int, executionID string, startTimeMs int64) (*AssetFilterOutput, error) {
	duration := time.Now().UnixMilli() - startTimeMs

	if deps.SideChannel != nil {
		data, err := deps.ReferenceManager.Get(ctx, refID)
		if err == nil {
			assets, ok := data.([]repo.Asset)
			if ok {
				dtos := make([]dto.AssetDTO, 0, len(assets))
				for _, a := range assets {
					dtos = append(dtos, dto.ToAssetDTO(a))
				}
				sendSuccessEvent(ctx, deps, executionID, startTimeMs, count, refID, input, dtos)
			}
		}
	}

	var message strings.Builder
	message.WriteString(fmt.Sprintf("Found %d %s", count, pluralize("asset", count)))
	if refID != "" {
		message.WriteString(fmt.Sprintf(" (ref_id: %s)", refID))
	}
	message.WriteString(". The view has been updated for the user.")
	if refID != "" {
		message.WriteString(" You can use this ref_id in other tools to reference these assets. Do not tell user the ref_id")
	}

	return &AssetFilterOutput{
		Message:  message.String(),
		RefID:    refID,
		Count:    count,
		Duration: duration,
	}, nil
}

func handleNoResults(ctx context.Context, deps *core.ToolDependencies, executionID string, startTimeMs int64) (*AssetFilterOutput, error) {
	duration := time.Now().UnixMilli() - startTimeMs
	if deps.SideChannel != nil {
		deps.SideChannel <- &core.SideChannelEvent{
			Type: "tool_execution", Timestamp: time.Now().UnixMilli(),
			Tool:      core.ToolIdentity{Name: "filter_assets", ExecutionID: executionID},
			Execution: core.ExecutionInfo{Status: core.ExecutionStatusSuccess, Message: "No assets found matching criteria", Duration: duration},
		}
	}
	return &AssetFilterOutput{
		Message:  "No assets found matching the criteria.",
		Count:    0,
		Duration: duration,
	}, nil
}

func handleFilterError(ctx context.Context, deps *core.ToolDependencies, executionID string, startTime time.Time, err error) (*AssetFilterOutput, error) {
	if deps.SideChannel != nil {
		deps.SideChannel <- &core.SideChannelEvent{
			Type: "tool_execution", Timestamp: time.Now().UnixMilli(),
			Tool: core.ToolIdentity{Name: "filter_assets", ExecutionID: executionID},
			Execution: core.ExecutionInfo{
				Status:   core.ExecutionStatusError,
				Message:  "Failed to query database",
				Error:    &core.ErrorInfo{Code: "DB_QUERY_FAILED", Message: err.Error()},
				Duration: time.Since(startTime).Milliseconds(),
			},
		}
	}
	return &AssetFilterOutput{Message: fmt.Sprintf("Error querying database: %v", err)}, nil
}

func sendPendingEvent(ctx context.Context, deps *core.ToolDependencies, execID string, startTime time.Time, input *AssetFilterInput) {
	if deps.SideChannel == nil {
		return
	}
	deps.SideChannel <- &core.SideChannelEvent{
		Type:      "tool_execution",
		Timestamp: startTime.UnixMilli(),
		Tool:      core.ToolIdentity{Name: "filter_assets", ExecutionID: execID},
		Execution: core.ExecutionInfo{Status: core.ExecutionStatusPending, Message: "Preparing to filter assets...", Parameters: input},
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

func sendSuccessEvent(ctx context.Context, deps *core.ToolDependencies, execID string, startTimeMs int64, count int, refID string, input *AssetFilterInput, dtos []dto.AssetDTO) {
	if deps.SideChannel == nil {
		return
	}
	deps.SideChannel <- &core.SideChannelEvent{
		Type:      "tool_execution",
		Timestamp: time.Now().UnixMilli(),
		Tool:      core.ToolIdentity{Name: "filter_assets", ExecutionID: execID},
		Execution: core.ExecutionInfo{
			Status:     core.ExecutionStatusSuccess,
			Message:    fmt.Sprintf("Found %d %s", count, pluralize("asset", count)),
			Duration:   time.Now().UnixMilli() - startTimeMs,
			Parameters: input,
		},
		Data: &core.DataPayload{
			RefID:       refID,
			PayloadType: "AssetDTO[]",
			Payload:     dtos,
			Rendering: &core.RenderingConfig{
				Component: core.ComponentJustifiedGallery,
				Config:    &core.JustifiedGalleryConfig{GroupBy: "date"},
			},
		},
	}
}

func pluralize(s string, count int) string {
	if count == 1 {
		return s
	}
	return s + "s"
}
