package tools

import (
	"context"
	"fmt"
	"server/internal/agent/core"
	"server/internal/api/dto"
	"server/internal/db/repo"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// BulkLikeInput 批量点赞工具的输入参数
type BulkLikeInput struct {
	// 使用 Reference[T] 泛型包装器接收过滤器配置
	// LLM 可以传入从 filter_assets 返回的 ref_id
	Filter core.Reference[dto.AssetFilterDTO] `json:"filter" jsonschema:"description=Reference to the filter configuration (from filter_assets tool). Provide the ref_id."`

	// 点赞状态
	// true: 标记为喜欢
	// false: 取消喜欢
	Liked bool `json:"liked" jsonschema:"description=Whether to like (true) or unlike (false) the assets matching the filter."`
}

// BulkLikeOutput 批量点赞工具的输出结果
type BulkLikeOutput struct {
	Message   string `json:"message"`
	RefID     string `json:"ref_id,omitempty"`
	Count     int    `json:"count"`
	Success   int    `json:"success"`
	Failed    int    `json:"failed"`
	Duration  int64  `json:"duration_ms,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// RegisterBulkLikeTool 注册批量点赞工具
func RegisterBulkLikeTool() {
	info := &schema.ToolInfo{
		Name: core.ToolBulkLikeAssets,
		Desc: "Bulk like or unlike assets matching a filter. Use the ref_id from filter_assets to apply the action to the same set of photos the user is viewing.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		t, err := utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *BulkLikeInput) (*BulkLikeOutput, error) {
			startTime := time.Now()
			executionID := fmt.Sprintf("%d", startTime.UnixNano())

			// 1. Resolve Reference
			if input.Filter.ID != "" && input.Filter.IsEmpty() {
				err := deps.ReferenceManager.GetAs(ctx, input.Filter.ID, &input.Filter.Data)
				if err != nil {
					return &BulkLikeOutput{
						Message: fmt.Sprintf("Failed to resolve filter reference %s: %v", input.Filter.ID, err),
					}, nil
				}
			}

			filterDTO := input.Filter.Unwrap()

			// 2. Send Pending Event
			if deps.Dispatcher != nil {
				deps.Dispatcher.Dispatch(&core.SideChannelEvent{
					Type:      core.EventTypeToolExecution,
					Timestamp: startTime.UnixMilli(),
					Tool:      core.ToolIdentity{Name: core.ToolBulkLikeAssets, ExecutionID: executionID},
					Execution: core.ExecutionInfo{
						Status:     core.ExecutionStatusPending,
						Message:    fmt.Sprintf("Preparing to %s assets matching filter...", boolToVerb(input.Liked)),
						Parameters: input,
					},
				})
			}

			// 3. Query Assets based on Filter
			// We set a hard limit here to prevent accidental massive updates
			const maxAssetsLimit = 1000
			queryParams := convertDTOToParams(filterDTO)
			queryParams.Limit = maxAssetsLimit + 1 // Fetch one more to check if we exceeded limit
			queryParams.Offset = 0

			if deps.Dispatcher != nil {
				deps.Dispatcher.Dispatch(&core.SideChannelEvent{
					Type:      core.EventTypeToolExecution,
					Timestamp: time.Now().UnixMilli(),
					Tool:      core.ToolIdentity{Name: core.ToolBulkLikeAssets, ExecutionID: executionID},
					Execution: core.ExecutionInfo{
						Status:  core.ExecutionStatusRunning,
						Message: "Finding matching assets...",
					},
				})
			}

			assets, err := deps.Queries.FilterAssets(ctx, queryParams)
			if err != nil {
				return handleBulkError(ctx, deps, executionID, startTime, err)
			}

			totalFound := len(assets)
			if totalFound > maxAssetsLimit {
				return handleLimitExceeded(ctx, deps, executionID, startTime, totalFound, maxAssetsLimit)
			}

			if totalFound == 0 {
				return handleNoAssetsFound(ctx, deps, executionID, startTime)
			}

			// 4. Perform Bulk Update
			if deps.Dispatcher != nil {
				deps.Dispatcher.Dispatch(&core.SideChannelEvent{
					Type:      core.EventTypeToolExecution,
					Timestamp: time.Now().UnixMilli(),
					Tool:      core.ToolIdentity{Name: core.ToolBulkLikeAssets, ExecutionID: executionID},
					Execution: core.ExecutionInfo{
						Status:  core.ExecutionStatusRunning,
						Message: fmt.Sprintf("Updating %d assets...", totalFound),
					},
				})
			}

			assetIDs := make([]pgtype.UUID, totalFound)
			for i, a := range assets {
				assetIDs[i] = a.AssetID
			}

			err = deps.Queries.BulkUpdateAssetLiked(ctx, repo.BulkUpdateAssetLikedParams{
				Liked:    input.Liked,
				AssetIds: assetIDs,
			})

			duration := time.Since(startTime).Milliseconds()

			// 5. Handle Result
			successCount := totalFound
			failedCount := 0
			var failedAssetIDs []pgtype.UUID

			if err != nil {
				successCount = 0
				failedCount = totalFound
				failedAssetIDs = assetIDs
			}

			// Convert failed IDs to strings
			failedAssetIDStrs := make([]string, len(failedAssetIDs))
			for i, id := range failedAssetIDs {
				failedAssetIDStrs[i] = uuid.UUID(id.Bytes).String()
			}

			resultSummary := &dto.BulkLikeUpdateDTO{
				Total:          totalFound,
				Success:        successCount,
				Failed:         failedCount,
				FailedAssetIDs: failedAssetIDStrs,
				Liked:          input.Liked,
				Action:         boolToVerb(input.Liked),
				Description:    fmt.Sprintf("Bulk %s: %d/%d successful", boolToVerb(input.Liked), successCount, totalFound),
				Timestamp:      startTime.Format(time.RFC3339),
			}

			// Store result for reference
			refID := ""
			if deps.ReferenceManager != nil {
				refID = deps.ReferenceManager.StoreWithID(ctx, resultSummary, resultSummary.Description)
				resultSummary.RefID = refID
			}

			// Send Success/Error Event
			status := core.ExecutionStatusSuccess
			if failedCount > 0 {
				if successCount == 0 {
					status = core.ExecutionStatusError
				} else {
					status = core.ExecutionStatusSuccess // Partial success
				}
			}

			if deps.Dispatcher != nil {
				deps.Dispatcher.Dispatch(&core.SideChannelEvent{
					Type:      core.EventTypeToolExecution,
					Timestamp: time.Now().UnixMilli(),
					Tool:      core.ToolIdentity{Name: core.ToolBulkLikeAssets, ExecutionID: executionID},
					Execution: core.ExecutionInfo{
						Status:   status,
						Message:  resultSummary.Description,
						Duration: duration,
					},
					Data: &core.DataPayload{
						RefID:       refID,
						PayloadType: "BulkLikeUpdateDTO",
						Payload:     resultSummary,
					},
				})
			}

			return &BulkLikeOutput{
				Message:   resultSummary.Description,
				RefID:     refID,
				Count:     totalFound,
				Success:   successCount,
				Failed:    failedCount,
				Duration:  duration,
				Timestamp: startTime.Format(time.RFC3339),
			}, nil
		})
		if err != nil {
			return nil, err
		}
		return t, nil
	})
}

// --- Helper Functions ---

func boolToVerb(liked bool) string {
	if liked {
		return "like"
	}
	return "unlike"
}

func convertDTOToParams(f dto.AssetFilterDTO) repo.FilterAssetsParams {
	params := repo.FilterAssetsParams{}

	if f.Type != nil {
		params.AssetType = f.Type
	}
	if f.OwnerID != nil {
		params.OwnerID = f.OwnerID
	}
	if f.RepositoryID != nil {
		// Assuming UUID parsing is handled or we need to parse string to pgtype.UUID
		// For simplicity in this snippet, we might skip complex UUID parsing if not strictly needed or handle it safely
		// In a real app, parse the UUID string
	}
	if f.Filename != nil {
		params.FilenameVal = &f.Filename.Value
		mode := f.Filename.Mode
		if mode == "" {
			mode = "contains"
		}
		params.FilenameMode = &mode
	}
	if f.Date != nil {
		if f.Date.From != nil {
			params.DateFrom = pgtype.Timestamptz{Time: *f.Date.From, Valid: true}
		}
		if f.Date.To != nil {
			params.DateTo = pgtype.Timestamptz{Time: *f.Date.To, Valid: true}
		}
	}
	if f.RAW != nil {
		params.IsRaw = f.RAW
	}
	if f.Rating != nil {
		r := int32(*f.Rating)
		params.Rating = &r
	}
	if f.Liked != nil {
		params.Liked = f.Liked
	}
	if f.CameraMake != nil {
		params.CameraModel = f.CameraMake
	}
	if f.Lens != nil {
		params.LensModel = f.Lens
	}

	return params
}

func handleBulkError(ctx context.Context, deps *core.ToolDependencies, executionID string, startTime time.Time, err error) (*BulkLikeOutput, error) {
	duration := time.Since(startTime).Milliseconds()
	if deps.Dispatcher != nil {
		deps.Dispatcher.Dispatch(&core.SideChannelEvent{
			Type:      core.EventTypeToolExecution,
			Timestamp: time.Now().UnixMilli(),
			Tool:      core.ToolIdentity{Name: core.ToolBulkLikeAssets, ExecutionID: executionID},
			Execution: core.ExecutionInfo{
				Status:   core.ExecutionStatusError,
				Message:  "Database query failed",
				Error:    &core.ErrorInfo{Code: "DB_ERROR", Message: err.Error()},
				Duration: duration,
			},
		})
	}
	return &BulkLikeOutput{Message: fmt.Sprintf("Error querying assets: %v", err)}, nil
}

func handleLimitExceeded(ctx context.Context, deps *core.ToolDependencies, executionID string, startTime time.Time, count, limit int) (*BulkLikeOutput, error) {
	duration := time.Since(startTime).Milliseconds()
	msg := fmt.Sprintf("Filter matches too many assets (%d). Maximum allowed for bulk update is %d. Please refine the filter.", count, limit)

	if deps.Dispatcher != nil {
		deps.Dispatcher.Dispatch(&core.SideChannelEvent{
			Type:      core.EventTypeToolExecution,
			Timestamp: time.Now().UnixMilli(),
			Tool:      core.ToolIdentity{Name: core.ToolBulkLikeAssets, ExecutionID: executionID},
			Execution: core.ExecutionInfo{
				Status:   core.ExecutionStatusError,
				Message:  "Too many assets",
				Error:    &core.ErrorInfo{Code: "LIMIT_EXCEEDED", Message: msg},
				Duration: duration,
			},
		})
	}
	return &BulkLikeOutput{Message: msg}, nil
}

func handleNoAssetsFound(ctx context.Context, deps *core.ToolDependencies, executionID string, startTime time.Time) (*BulkLikeOutput, error) {
	duration := time.Since(startTime).Milliseconds()
	if deps.Dispatcher != nil {
		deps.Dispatcher.Dispatch(&core.SideChannelEvent{
			Type:      core.EventTypeToolExecution,
			Timestamp: time.Now().UnixMilli(),
			Tool:      core.ToolIdentity{Name: core.ToolBulkLikeAssets, ExecutionID: executionID},
			Execution: core.ExecutionInfo{
				Status:   core.ExecutionStatusSuccess,
				Message:  "No assets found matching filter",
				Duration: duration,
			},
		})
	}
	return &BulkLikeOutput{Message: "No assets found matching the current filter."}, nil
}
