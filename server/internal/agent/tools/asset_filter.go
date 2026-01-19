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
)

// AssetFilterInput 定义了 LLM 如何调用此工具
// 基于系统的完整过滤选项设计
type AssetFilterInput struct {
	// 基本过滤选项
	DateFrom string `json:"date_from,omitempty" jsonschema:"description=Start date in YYYY-MM-DD format"`
	DateTo   string `json:"date_to,omitempty" jsonschema:"description=End date in YYYY-MM-DD format"`
	Type     string `json:"type,omitempty" jsonschema:"description=Asset type (PHOTO, VIDEO, AUDIO)"`

	// 文件名过滤
	Filename string `json:"filename,omitempty" jsonschema:"description=Filename pattern to search for"`

	// 高级过滤选项
	Raw    *bool `json:"raw,omitempty" jsonschema:"description=Filter for RAW photos only"`
	Rating *int  `json:"rating,omitempty" jsonschema:"description=Filter by rating (0-5)"`
	Liked  *bool `json:"liked,omitempty" jsonschema:"description=Filter for liked/favorited assets"`
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
		Desc: "Search and filter assets. Use this to show photos to the user based on criteria like date, type, or content description.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.NewTool(info, func(ctx context.Context, input *AssetFilterInput) (*AssetFilterOutput, error) {
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
						Name:        "filter_assets",
						ExecutionID: executionID,
					},
					Execution: core.ExecutionInfo{
						Status:     core.ExecutionStatusPending,
						Message:    "Preparing to filter assets...",
						Parameters: input,
					},
				}
			}

			// =====================================================
			// 阶段 2: 发送 running 事件
			// =====================================================
			if deps.SideChannel != nil {
				deps.SideChannel <- &core.SideChannelEvent{
					Type:      "tool_execution",
					Timestamp: time.Now().UnixMilli(),
					Tool: core.ToolIdentity{
						Name:        "filter_assets",
						ExecutionID: executionID,
					},
					Execution: core.ExecutionInfo{
						Status:  core.ExecutionStatusRunning,
						Message: "Querying database for matching assets...",
					},
				}
			}

			// =====================================================
			// 阶段 3: 构造查询参数
			// =====================================================
			params := repo.FilterAssetsParams{
				Limit:  30, // 默认给 UI 的数量可以大一点
				Offset: 0,
			}

			// 处理日期
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

			// 处理类型
			if input.Type != "" {
				params.AssetType = &input.Type
			}

			// 处理文件名过滤
			if input.Filename != "" {
				mode := "contains"
				params.FilenameMode = &mode
			}

			// 处理高级过滤选项
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

			// =====================================================
			// 阶段 4: 执行数据库查询
			// =====================================================
			assets, err := deps.Queries.FilterAssets(ctx, params)
			if err != nil {
				// 发送错误事件
				if deps.SideChannel != nil {
					deps.SideChannel <- &core.SideChannelEvent{
						Type:      "tool_execution",
						Timestamp: time.Now().UnixMilli(),
						Tool: core.ToolIdentity{
							Name:        "filter_assets",
							ExecutionID: executionID,
						},
						Execution: core.ExecutionInfo{
							Status:  core.ExecutionStatusError,
							Message: "Failed to query database",
							Error: &core.ErrorInfo{
								Code:    "DB_QUERY_FAILED",
								Message: err.Error(),
							},
							Duration: time.Since(startTime).Milliseconds(),
						},
					}
				}

				return &AssetFilterOutput{
					Message: fmt.Sprintf("Error querying database: %v", err),
				}, nil
			}

			count := len(assets)
			duration := time.Since(startTime).Milliseconds()

			// =====================================================
			// 阶段 5: 处理结果
			// =====================================================
			if count == 0 {
				// 无结果情况
				if deps.SideChannel != nil {
					deps.SideChannel <- &core.SideChannelEvent{
						Type:      "tool_execution",
						Timestamp: time.Now().UnixMilli(),
						Tool: core.ToolIdentity{
							Name:        "filter_assets",
							ExecutionID: executionID,
						},
						Execution: core.ExecutionInfo{
							Status:   core.ExecutionStatusSuccess,
							Message:  "No assets found matching criteria",
							Duration: duration,
						},
					}
				}

				return &AssetFilterOutput{
					Message:  "No assets found matching the criteria.",
					Count:    0,
					Duration: duration,
				}, nil
			}

			// =====================================================
			// 阶段 6: 存储到 ReferenceManager
			// =====================================================
			refID := ""
			description := fmt.Sprintf("%d filtered assets", count)
			if input.DateFrom != "" || input.DateTo != "" {
				description += " from "
				if input.DateFrom != "" {
					description += input.DateFrom
				} else {
					description += "start"
				}
				description += " to "
				if input.DateTo != "" {
					description += input.DateTo
				} else {
					description += "end"
				}
			}

			if deps.ReferenceManager != nil {
				refID = deps.ReferenceManager.StoreWithID(assets, description)
			}

			// =====================================================
			// 阶段 7: 转换数据并发送 SideChannelEvent
			// =====================================================
			if deps.SideChannel != nil {
				// 将 DB Model 转为 API DTO
				dtos := make([]dto.AssetDTO, 0, len(assets))
				for _, a := range assets {
					dtos = append(dtos, dto.ToAssetDTO(a))
				}

				// 构建描述信息
				var criteriaDesc strings.Builder
				if input.Type != "" {
					criteriaDesc.WriteString(fmt.Sprintf("%s ", input.Type))
				}
				if input.DateFrom != "" || input.DateTo != "" {
					criteriaDesc.WriteString(fmt.Sprintf("from %s to %s ",
						func() string {
							if input.DateFrom != "" {
								return input.DateFrom
							}
							return "start"
						}(),
						func() string {
							if input.DateTo != "" {
								return input.DateTo
							}
							return "end"
						}()))
				}

				// 发送成功事件 + 数据
				deps.SideChannel <- &core.SideChannelEvent{
					Type:      "tool_execution",
					Timestamp: time.Now().UnixMilli(),
					Tool: core.ToolIdentity{
						Name:        "filter_assets",
						ExecutionID: executionID,
					},
					Execution: core.ExecutionInfo{
						Status: core.ExecutionStatusSuccess,
						Message: fmt.Sprintf("Found %d %s", count, func() string {
							if count == 1 {
								return "asset"
							} else {
								return "assets"
							}
						}()),
						Duration:   duration,
						Parameters: input,
					},
					Data: &core.DataPayload{
						RefID:       refID,
						PayloadType: "AssetDTO[]",
						Payload:     dtos,
						Rendering: &core.RenderingConfig{
							Component: core.ComponentJustifiedGallery,
							Config: &core.JustifiedGalleryConfig{
								GroupBy: "date",
							},
						},
					},
					Extra: &core.ExtraInfo{
						ExtraType: "FilterAssetsRequestDTO",
						Data:      input,
					},
					Metadata: map[string]interface{}{
						"count":         count,
						"description":   description,
						"criteria_text": criteriaDesc.String(),
					},
				}
			}

			// =====================================================
			// 阶段 8: 返回给 LLM 的简报
			// =====================================================
			var message strings.Builder
			message.WriteString(fmt.Sprintf("Found %d %s", count, func() string {
				if count == 1 {
					return "asset"
				} else {
					return "assets"
				}
			}()))

			if refID != "" {
				message.WriteString(fmt.Sprintf(" (ref_id: %s)", refID))
			}

			message.WriteString(". The view has been updated for the user.")

			if refID != "" {
				message.WriteString(" You can use this ref_id in other tools to reference these assets.")
			}

			return &AssetFilterOutput{
				Message:  message.String(),
				RefID:    refID,
				Count:    count,
				Duration: duration,
			}, nil
		}), nil
	})
}
