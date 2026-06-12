package tools

import (
	"context"
	"fmt"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/ref"
	"server/internal/db/repo"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

// maxBulkMutationSize bounds how many assets one mutation may touch.
const maxBulkMutationSize = 1000

// BulkLikeInput takes a ref handle; the snapshot it stands for is mutated
// directly, so the affected set is exactly what the receipt promised (INV-5)
// — never a re-run of the original query.
type BulkLikeInput struct {
	RefID string `json:"ref_id" jsonschema:"description=Ref of the asset set to like/unlike (from filter_assets or combine)"`
	Liked bool   `json:"liked" jsonschema:"description=true to like, false to unlike"`
}

// BulkLikeOutput reports the mutation outcome: count and message only,
// never asset ids (INV-1).
type BulkLikeOutput struct {
	Message string     `json:"message,omitempty"`
	Count   int        `json:"count,omitempty"`
	Error   *ref.Error `json:"error,omitempty"`
}

// RegisterBulkLike registers the bulk_like_assets terminal.
func RegisterBulkLike() {
	info := &schema.ToolInfo{
		Name: "bulk_like_assets",
		Desc: "Like or unlike every asset in a ref. Check the ref's count first; " +
			"sets larger than 1000 are rejected — narrow them before mutating.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *BulkLikeInput) (*BulkLikeOutput, error) {
			start := time.Now()
			execID := newExecutionID()
			sendRunning(deps, info.Name, execID, fmt.Sprintf("Applying %s to assets...", likeVerb(input.Liked)), input)

			r, refErr := deps.RefStore.Get(deps.Scope(), input.RefID)
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return &BulkLikeOutput{Error: refErr}, nil
			}
			if r.Count() == 0 {
				refErr := ref.EmptySet(r.ID)
				sendError(deps, info.Name, execID, start, refErr)
				return &BulkLikeOutput{Error: refErr}, nil
			}
			if r.Count() > maxBulkMutationSize {
				refErr := ref.LimitExceeded(r.Count(), maxBulkMutationSize)
				sendError(deps, info.Name, execID, start, refErr)
				return &BulkLikeOutput{Error: refErr}, nil
			}

			err := deps.Queries.BulkUpdateAssetLiked(ctx, repo.BulkUpdateAssetLikedParams{
				Liked:    input.Liked,
				AssetIds: toPgUUIDs(r.AssetIDs),
			})
			if err != nil {
				refErr := ref.Internal("bulk like update")
				sendError(deps, info.Name, execID, start, refErr)
				return &BulkLikeOutput{Error: refErr}, nil
			}

			message := fmt.Sprintf("%sd %d assets", likeVerb(input.Liked), r.Count())
			sendSuccess(deps, info.Name, execID, start, message, &core.DataPayload{RefID: r.ID, Count: r.Count()})
			return &BulkLikeOutput{Message: message, Count: r.Count()}, nil
		})
	})
}

func likeVerb(liked bool) string {
	if liked {
		return "like"
	}
	return "unlike"
}
