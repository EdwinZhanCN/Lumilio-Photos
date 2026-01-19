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
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// BulkLikeInput 批量点赞工具的输入参数
type BulkLikeInput struct {
	// 使用 Reference[T] 泛型包装器接收资产集合
	// LLM 可以传入从 filter_assets 等工具返回的 ref_id
	Assets core.Reference[[]repo.Asset] `json:"assets" jsonschema:"description=Reference to assets collection (e.g., from filter_assets tool). Provide the ref_id as a string."`

	// 点赞状态
	// true: 标记为喜欢
	// false: 取消喜欢
	Liked bool `json:"liked" jsonschema:"description=Whether to like (true) or unlike (false) the assets."`
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
		Name: "bulk_like_assets",
		Desc: "Bulk like or unlike a collection of assets by reference ID. Use this after filter_assets to mark multiple photos as favorites or unfavorite them. The operation updates the database and returns a summary.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.NewTool(info, func(ctx context.Context, input *BulkLikeInput) (*BulkLikeOutput, error) {
			startTime := time.Now()

			// =====================================================
			// 阶段 1: 发送 pending 事件
			// =====================================================
			executionID := fmt.Sprintf("%d", time.Now().UnixNano())

			if deps.SideChannel != nil {
				deps.SideChannel <- &core.SideChannelEvent{
					Type:      "tool_execution",
					Timestamp: startTime.UnixMilli(),
					Tool: core.ToolIdentity{
						Name:        "bulk_like_assets",
						ExecutionID: executionID,
					},
					Execution: core.ExecutionInfo{
						Status: core.ExecutionStatusPending,
						Message: fmt.Sprintf("Preparing to %s assets...", func() string {
							if input.Liked {
								return "like"
							} else {
								return "unlike"
							}
						}()),
						Parameters: input,
					},
				}
			}

			// =====================================================
			// 阶段 2: 验证输入
			// =====================================================
			if input.Assets.IsEmpty() {
				duration := time.Since(startTime).Milliseconds()

				// 发送错误事件
				if deps.SideChannel != nil {
					deps.SideChannel <- &core.SideChannelEvent{
						Type:      "tool_execution",
						Timestamp: time.Now().UnixMilli(),
						Tool: core.ToolIdentity{
							Name:        "bulk_like_assets",
							ExecutionID: executionID,
						},
						Execution: core.ExecutionInfo{
							Status:   core.ExecutionStatusError,
							Message:  "No assets provided",
							Duration: duration,
							Error: &core.ErrorInfo{
								Code:    "EMPTY_ASSETS",
								Message: "Reference[T].Assets is empty or missing ref_id",
							},
						},
					}
				}

				return &BulkLikeOutput{
					Message:  "No assets provided. Please provide assets reference from filter_assets or similar tool.",
					Count:    0,
					Success:  0,
					Failed:   0,
					Duration: duration,
				}, nil
			}

			// =====================================================
			// 阶段 2.5: 验证 ref_id 格式
			// =====================================================
			inputRefID := input.Assets.ID
			if inputRefID != "" {
				// 验证 ref_id 格式：ref_ 前缀 或 UUID 格式
				isValid := false
				if strings.HasPrefix(inputRefID, "ref_") && len(inputRefID) > 4 {
					isValid = true
				} else if len(inputRefID) == 36 && inputRefID[8] == '-' && inputRefID[13] == '-' && inputRefID[18] == '-' && inputRefID[23] == '-' {
					isValid = true
				}

				if !isValid {
					duration := time.Since(startTime).Milliseconds()

					if deps.SideChannel != nil {
						deps.SideChannel <- &core.SideChannelEvent{
							Type:      "tool_execution",
							Timestamp: time.Now().UnixMilli(),
							Tool: core.ToolIdentity{
								Name:        "bulk_like_assets",
								ExecutionID: executionID,
							},
							Execution: core.ExecutionInfo{
								Status:   core.ExecutionStatusError,
								Message:  "Invalid reference ID format",
								Duration: duration,
								Error: &core.ErrorInfo{
									Code:    "INVALID_REF_ID",
									Message: fmt.Sprintf("Invalid ref_id format: '%s'. Expected 'ref_xxx' or UUID.", inputRefID),
								},
							},
						}
					}

					return &BulkLikeOutput{
						Message:  fmt.Sprintf("Invalid reference ID format: '%s'. Expected 'ref_xxx' or UUID format.", inputRefID),
						Count:    0,
						Success:  0,
						Failed:   0,
						Duration: duration,
					}, nil
				}
			}

			// =====================================================
			// 阶段 3: 发送 running 事件
			// =====================================================
			if deps.SideChannel != nil {
				deps.SideChannel <- &core.SideChannelEvent{
					Type:      "tool_execution",
					Timestamp: time.Now().UnixMilli(),
					Tool: core.ToolIdentity{
						Name:        "bulk_like_assets",
						ExecutionID: executionID,
					},
					Execution: core.ExecutionInfo{
						Status:  core.ExecutionStatusRunning,
						Message: fmt.Sprintf("Processing %d assets...", len(input.Assets.Unwrap())),
					},
				}
			}

			// =====================================================
			// 阶段 4: 获取资产数据
			// =====================================================
			assets := input.Assets.Unwrap()
			totalAssets := len(assets)

			// 验证资产数量上限（防止内存问题）
			const maxAssetsPerBatch = 1000
			if totalAssets > maxAssetsPerBatch {
				duration := time.Since(startTime).Milliseconds()

				if deps.SideChannel != nil {
					deps.SideChannel <- &core.SideChannelEvent{
						Type:      "tool_execution",
						Timestamp: time.Now().UnixMilli(),
						Tool: core.ToolIdentity{
							Name:        "bulk_like_assets",
							ExecutionID: executionID,
						},
						Execution: core.ExecutionInfo{
							Status:   core.ExecutionStatusError,
							Message:  "Too many assets in batch",
							Duration: duration,
							Error: &core.ErrorInfo{
								Code:    "BATCH_TOO_LARGE",
								Message: fmt.Sprintf("Batch size %d exceeds maximum %d", totalAssets, maxAssetsPerBatch),
							},
						},
					}
				}

				return &BulkLikeOutput{
					Message:  fmt.Sprintf("Too many assets: %d (max %d). Please split into smaller batches.", totalAssets, maxAssetsPerBatch),
					Count:    totalAssets,
					Success:  0,
					Failed:   0,
					Duration: duration,
				}, nil
			}

			if totalAssets == 0 {
				duration := time.Since(startTime).Milliseconds()

				if deps.SideChannel != nil {
					deps.SideChannel <- &core.SideChannelEvent{
						Type:      "tool_execution",
						Timestamp: time.Now().UnixMilli(),
						Tool: core.ToolIdentity{
							Name:        "bulk_like_assets",
							ExecutionID: executionID,
						},
						Execution: core.ExecutionInfo{
							Status:   core.ExecutionStatusError,
							Message:  "Reference contains no assets",
							Duration: duration,
							Error: &core.ErrorInfo{
								Code:    "NO_ASSETS",
								Message: "The referenced collection is empty",
							},
						},
					}
				}

				return &BulkLikeOutput{
					Message:  "The referenced asset collection is empty.",
					Count:    0,
					Success:  0,
					Failed:   0,
					Duration: duration,
				}, nil
			}

			// =====================================================
			// 阶段 5: 批量更新数据库
			// =====================================================
			// 提取所有 asset_id
			assetIDs := make([]pgtype.UUID, len(assets))
			for i, asset := range assets {
				assetIDs[i] = asset.AssetID
			}

			// 使用批量更新接口（一条 SQL 更新所有记录）
			err := deps.Queries.BulkUpdateAssetLiked(ctx, repo.BulkUpdateAssetLikedParams{
				Liked:    input.Liked,
				AssetIds: assetIDs,
			})

			duration := time.Since(startTime).Milliseconds()

			// 处理结果
			successCount := totalAssets
			failedCount := 0
			var failedAssetIDs []pgtype.UUID

			if err != nil {
				// 批量更新失败，所有记录都视为失败
				successCount = 0
				failedCount = totalAssets
				failedAssetIDs = assetIDs
			}

			// =====================================================
			// 阶段 6: 处理结果并发送 SideChannelEvent
			// =====================================================

			// 转换失败资产 ID 为字符串
			failedAssetIDStrs := make([]string, len(failedAssetIDs))
			for i, id := range failedAssetIDs {
				failedAssetIDStrs[i] = uuid.UUID(id.Bytes).String()
			}

			// 创建规范的 DTO 对象
			action := "unlike"
			if input.Liked {
				action = "like"
			}

			resultSummary := &dto.BulkLikeUpdateDTO{
				Total:          totalAssets,
				Success:        successCount,
				Failed:         failedCount,
				FailedAssetIDs: failedAssetIDStrs,
				Liked:          input.Liked,
				Action:         action,
				Description:    fmt.Sprintf("Bulk %s: %d/%d successful", action, successCount, totalAssets),
				Timestamp:      startTime.Format(time.RFC3339),
			}

			// 存储操作结果到 ReferenceManager（供后续工具引用）
			refID := ""
			if deps.ReferenceManager != nil {
				refID = deps.ReferenceManager.StoreWithID(
					resultSummary,
					resultSummary.Description,
				)
				resultSummary.RefID = refID
			}

			// 确定最终状态
			var status core.ExecutionStatus
			var statusMessage string
			var errorInfo *core.ErrorInfo

			if failedCount == 0 {
				// 全部成功
				status = core.ExecutionStatusSuccess
				statusMessage = fmt.Sprintf("Successfully %sd %d assets", resultSummary.Action, successCount)
			} else if successCount == 0 {
				// 全部失败
				status = core.ExecutionStatusError
				statusMessage = fmt.Sprintf("Failed to %s any assets", resultSummary.Action)
				errorInfo = &core.ErrorInfo{
					Code:    "BULK_OPERATION_FAILED",
					Message: "All asset updates failed",
					Details: map[string]interface{}{
						"failed_asset_ids": resultSummary.FailedAssetIDs,
					},
				}
			} else {
				// 部分成功
				status = core.ExecutionStatusSuccess // 仍然视为成功，但有警告
				statusMessage = fmt.Sprintf("Partially successful: %sd %d, failed %d", resultSummary.Action, successCount, failedCount)
				errorInfo = &core.ErrorInfo{
					Code:    "PARTIAL_FAILURE",
					Message: "Some assets could not be updated",
					Details: map[string]interface{}{
						"failed_asset_ids": resultSummary.FailedAssetIDs,
					},
				}
			}

			// 发送最终事件
			if deps.SideChannel != nil {
				deps.SideChannel <- &core.SideChannelEvent{
					Type:      "tool_execution",
					Timestamp: time.Now().UnixMilli(),
					Tool: core.ToolIdentity{
						Name:        "bulk_like_assets",
						ExecutionID: executionID,
					},
					Execution: core.ExecutionInfo{
						Status:     status,
						Message:    statusMessage,
						Error:      errorInfo,
						Duration:   duration,
						Parameters: input,
					},
					Data: &core.DataPayload{
						RefID:       refID,
						PayloadType: "BulkLikeUpdateDTO",
						Payload:     resultSummary,
						Rendering:   &core.RenderingConfig{},
					},
					Extra: &core.ExtraInfo{
						ExtraType: "BulkLikeRequestDTO",
						Data:      input,
					},
					Metadata: map[string]interface{}{
						"ref_id":           input.Assets.ID,
						"operation_type":   "bulk_like",
						"action":           resultSummary.Action,
						"failed_asset_ids": resultSummary.FailedAssetIDs,
					},
				}
			}

			// =====================================================
			// 阶段 7: 返回给 LLM 的简报
			// =====================================================
			var message strings.Builder
			actionVerb := func() string {
				if input.Liked {
					return "Liked"
				} else {
					return "Unliked"
				}
			}()

			if failedCount == 0 {
				message.WriteString(fmt.Sprintf("SUCCESS: %s %d assets successfully.", actionVerb, successCount))
			} else if successCount == 0 {
				message.WriteString(fmt.Sprintf("FAILED: Could not %s any of the %d assets.", resultSummary.Action, totalAssets))
			} else {
				message.WriteString(fmt.Sprintf("PARTIAL: %s %d out of %d assets successfully. %d failed.",
					actionVerb, successCount, totalAssets, failedCount))
			}

			if refID != "" {
				message.WriteString(fmt.Sprintf(" (ref_id: %s)", refID))
			}

			return &BulkLikeOutput{
				Message:   message.String(),
				RefID:     refID,
				Count:     totalAssets,
				Success:   successCount,
				Failed:    failedCount,
				Duration:  duration,
				Timestamp: startTime.Format(time.RFC3339),
			}, nil
		}), nil
	})
}
