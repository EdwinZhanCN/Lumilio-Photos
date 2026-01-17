package tools

import (
	"context"
	pkgagent "server/internal/agent"
	"server/internal/api/dto"
	"server/internal/db/repo"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5/pgtype"
)

// AssetFilterInput 输入参数 - 只包含 Agent 需要提供的字段
type AssetFilterInput struct {
	// 资产类型: "photo", "video", "raw", 等
	Type *string `json:"type,omitempty"`

	// 评分筛选 (0-5)
	Rating *int `json:"rating,omitempty"`

	// 是否只显示喜欢的资产
	Liked *bool `json:"liked,omitempty"`

	// 文件名搜索
	Filename *dto.FilenameFilterDTO `json:"filename,omitempty"`

	// 日期范围筛选
	Date *dto.DateRangeDTO `json:"date,omitempty"`

	// 相机品牌/型号
	CameraMake *string `json:"camera_make,omitempty"`

	// 镜头型号
	Lens *string `json:"lens,omitempty"`
}

type AssetFilterOutput []dto.AssetDTO

func RegisterFilterAsset() {
	info := &schema.ToolInfo{
		Name: "asset_filter",
		Desc: "Filter and search photo assets based on criteria like type, rating, filename, date range, camera, lens, etc. Returns a list of matching assets.",
	}
	pkgagent.GetRegistry().Register(info, func(ctx context.Context, deps *pkgagent.ToolDependencies) (tool.BaseTool, error) {
		return utils.NewTool(info, func(ctx context.Context, input *AssetFilterInput) (output AssetFilterOutput, err error) {
			// Convert input to FilterAssetsParams
			params := repo.FilterAssetsParams{
				Limit:  50, // 程序控制: 限制返回数量,避免数据过多
				Offset: 0,  // 程序控制: 暂不支持分页,从第0条开始
			}

			// 程序处理: RepositoryID 可以从上下文或配置中获取
			// 这里先留空,查询所有仓库
			// TODO: 如果需要多仓库支持,可以在这里添加逻辑

			// 程序处理: OwnerID 应该从认证上下文中获取
			// 这里先留空,允许查询所有用户的数据
			// TODO: 集成认证后,从 context 获取当前用户ID

			// Agent 提供的参数处理
			if input.Type != nil {
				params.AssetType = input.Type
			}

			// 程序处理: RAW 状态
			// 如果用户指定了 type,自动判断是否需要 RAW
			if input.Type != nil {
				isRaw := *input.Type == "raw"
				params.IsRaw = &isRaw
			}

			// Agent 提供的参数处理
			if input.Rating != nil {
				rating := int32(*input.Rating)
				params.Rating = &rating
			}

			// Agent 提供的参数处理
			if input.Liked != nil {
				params.Liked = input.Liked
			}

			// Agent 提供的参数处理
			if input.Filename != nil {
				params.FilenameVal = &input.Filename.Value
				params.FilenameMode = &input.Filename.Mode
			}

			// Agent 提供的参数处理
			if input.Date != nil {
				if input.Date.From != nil {
					params.DateFrom = pgtype.Timestamptz{Time: *input.Date.From, Valid: true}
				}
				if input.Date.To != nil {
					params.DateTo = pgtype.Timestamptz{Time: *input.Date.To, Valid: true}
				}
			}

			// Agent 提供的参数处理
			if input.CameraMake != nil {
				params.CameraModel = input.CameraMake
			}

			// Agent 提供的参数处理
			if input.Lens != nil {
				params.LensModel = input.Lens
			}

			// Execute the query
			assets, err := deps.Queries.FilterAssets(ctx, params)
			if err != nil {
				return nil, err
			}

			// Convert repo.Asset to dto.AssetDTO
			var assetDTOs []dto.AssetDTO
			for _, asset := range assets {
				assetDTO := dto.ToAssetDTO(asset)
				assetDTOs = append(assetDTOs, assetDTO)
			}

			return assetDTOs, nil
		}), nil
	})
}
