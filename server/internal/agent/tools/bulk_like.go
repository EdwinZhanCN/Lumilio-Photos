package tools

import (
	"context"
	"encoding/gob"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/ref"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

const maxBulkMutationSize = 1000

type BulkLikeInput struct {
	RefID string `json:"ref_id" jsonschema:"description=Ref of the asset set to like/unlike"`
	Liked bool   `json:"liked" jsonschema:"description=true to like, false to unlike"`
}

type BulkLikeOutput struct {
	Message          string     `json:"message,omitempty"`
	Count            int        `json:"count,omitempty"`
	AlreadyCommitted bool       `json:"already_committed,omitempty"`
	Error            *ref.Error `json:"error,omitempty"`
}

type BulkLikeConfirmationInfo struct {
	Action string `json:"action"`
	RefID  string `json:"ref_id"`
	Count  int    `json:"count"`
	Liked  bool   `json:"liked"`
}

type bulkLikeInterruptState struct {
	EffectID string
	RefID    string
	Count    int
}

func init() {
	gob.Register(&BulkLikeConfirmationInfo{})
	gob.Register(&bulkLikeInterruptState{})
}

func RegisterBulkLike() {
	info := &schema.ToolInfo{
		Name: "bulk_like_assets",
		Desc: "Like or unlike every asset in a ref. The user must confirm before commit; sets larger than 1000 are rejected.",
	}
	policy := core.EffectPolicy{
		Class: "asset_metadata", Reversible: true, Confirmation: true,
		MaxCardinality: maxBulkMutationSize, Idempotency: "effect_id",
		Authorization: "owner_snapshot_recheck", PolicyVersion: core.CurrentAgentPolicyVersion,
	}
	core.GetRegistry().RegisterEffect(info, policy, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *BulkLikeInput) (*BulkLikeOutput, error) {
			start := time.Now()
			execID := newExecutionID()
			if interrupted, hasState, state := compose.GetInterruptState[*bulkLikeInterruptState](ctx); interrupted && hasState {
				approved := resumeApproved(ctx)
				effectID, err := uuid.Parse(state.EffectID)
				if err != nil {
					return &BulkLikeOutput{Error: ref.Internal("effect identity")}, nil
				}
				if !approved {
					_ = deps.Effects.Reject(ctx, deps.UserID, deps.ThreadID, effectID)
					message := "Like change was not applied: the user declined."
					sendSuccess(deps, info.Name, execID, start, message, nil)
					return &BulkLikeOutput{Message: message}, nil
				}
				sendRunning(deps, info.Name, execID, "Applying like change...", nil)
				receipt, err := deps.Effects.Commit(ctx, deps.UserID, deps.ThreadID, deps.RunID, effectID)
				if err != nil {
					refErr := ref.Internal("bulk like commit")
					sendError(deps, info.Name, execID, start, refErr)
					return &BulkLikeOutput{Error: refErr}, nil
				}
				message := committedReceiptMessage(receipt)
				sendSuccess(deps, info.Name, execID, start, message, &core.DataPayload{RefID: state.RefID, Count: receipt.Count})
				return &BulkLikeOutput{Message: message, Count: receipt.Count, AlreadyCommitted: receipt.AlreadyCommitted}, nil
			}

			r, refErr := deps.ResolveRef(ctx, input.RefID)
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return &BulkLikeOutput{Error: refErr}, nil
			}
			if r.Count() == 0 {
				refErr = ref.EmptySet(r.ID)
			} else if r.Count() > policy.MaxCardinality {
				refErr = ref.LimitExceeded(r.Count(), policy.MaxCardinality)
			}
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return &BulkLikeOutput{Error: refErr}, nil
			}
			effectID, err := deps.Effects.Prepare(ctx, deps.UserID, deps.ThreadID, deps.RunID, info.Name, r.AssetIDs,
				map[string]any{"liked": input.Liked}, map[string]any{})
			if err != nil {
				refErr := ref.Internal("pending effect persistence")
				sendError(deps, info.Name, execID, start, refErr)
				return &BulkLikeOutput{Error: refErr}, nil
			}
			return nil, compose.StatefulInterrupt(ctx,
				&BulkLikeConfirmationInfo{Action: info.Name, RefID: r.ID, Count: r.Count(), Liked: input.Liked},
				&bulkLikeInterruptState{EffectID: effectID.String(), RefID: r.ID, Count: r.Count()},
			)
		})
	})
}

func resumeApproved(ctx context.Context) bool {
	if _, hasData, data := compose.GetResumeContext[map[string]any](ctx); hasData {
		approved, _ := data["approved"].(bool)
		return approved
	}
	return false
}

func likeVerb(liked bool) string {
	if liked {
		return "like"
	}
	return "unlike"
}
