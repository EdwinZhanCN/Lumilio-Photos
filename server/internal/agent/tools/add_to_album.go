package tools

import (
	"context"
	"encoding/gob"
	"fmt"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/ref"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

type AddToAlbumInput struct {
	RefID   string `json:"ref_id" jsonschema:"description=Ref of the assets to add"`
	AlbumID int    `json:"album_id" jsonschema:"description=Target album id"`
}

type AddToAlbumOutput struct {
	Message          string     `json:"message,omitempty"`
	AlbumID          int        `json:"album_id,omitempty"`
	Count            int        `json:"count,omitempty"`
	AlreadyCommitted bool       `json:"already_committed,omitempty"`
	Error            *ref.Error `json:"error,omitempty"`
}

type AddToAlbumConfirmationInfo struct {
	EffectID string `json:"effect_id"`
	Action   string `json:"action"`
	AlbumID  int    `json:"album_id"`
	RefID    string `json:"ref_id"`
	Count    int    `json:"count"`
}

type addToAlbumInterruptState struct {
	EffectID string
	RefID    string
	AlbumID  int32
	Count    int
}

func init() {
	gob.Register(&AddToAlbumConfirmationInfo{})
	gob.Register(&addToAlbumInterruptState{})
}

func RegisterAddToAlbum() {
	info := &schema.ToolInfo{
		Name: "add_to_album",
		Desc: "Add every asset in a ref to an owned album. The user must confirm before commit; sets larger than 5000 are rejected.",
	}
	policy := core.EffectPolicy{
		Class: "album_membership", Reversible: true, Confirmation: true,
		MaxCardinality: maxAlbumAssets, Idempotency: "effect_id",
		Authorization: "owner_snapshot_and_target_recheck", PolicyVersion: core.CurrentAgentPolicyVersion,
	}
	core.GetRegistry().RegisterEffect(info, policy, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *AddToAlbumInput) (*AddToAlbumOutput, error) {
			start := time.Now()
			execID := newExecutionID()
			if interrupted, hasState, state := compose.GetInterruptState[*addToAlbumInterruptState](ctx); interrupted && hasState {
				return resumeAddToAlbum(ctx, deps, info.Name, execID, start, state)
			}
			if input.AlbumID <= 0 {
				refErr := ref.InvalidArgument("album_id must be positive")
				sendError(deps, info.Name, execID, start, refErr)
				return &AddToAlbumOutput{Error: refErr}, nil
			}
			if _, err := deps.Library.Album(ctx, int32(input.AlbumID)); err != nil {
				refErr := ref.InvalidArgument(fmt.Sprintf("album %d not found", input.AlbumID))
				sendError(deps, info.Name, execID, start, refErr)
				return &AddToAlbumOutput{Error: refErr}, nil
			}
			r, refErr := deps.ResolveRef(ctx, input.RefID)
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return &AddToAlbumOutput{Error: refErr}, nil
			}
			if r.Count() == 0 {
				refErr = ref.EmptySet(r.ID)
			} else if r.Count() > policy.MaxCardinality {
				refErr = ref.LimitExceeded(r.Count(), policy.MaxCardinality)
			}
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return &AddToAlbumOutput{Error: refErr}, nil
			}
			effectID, err := deps.Effects.Prepare(ctx, deps.UserID, deps.ThreadID, deps.RunID, info.Name, r.AssetIDs,
				map[string]any{}, map[string]any{"album_id": int32(input.AlbumID)})
			if err != nil {
				refErr := ref.Internal("pending effect persistence")
				sendError(deps, info.Name, execID, start, refErr)
				return &AddToAlbumOutput{Error: refErr}, nil
			}
			return nil, compose.StatefulInterrupt(ctx,
				&AddToAlbumConfirmationInfo{EffectID: effectID.String(), Action: info.Name, AlbumID: input.AlbumID, RefID: r.ID, Count: r.Count()},
				&addToAlbumInterruptState{EffectID: effectID.String(), RefID: r.ID, AlbumID: int32(input.AlbumID), Count: r.Count()},
			)
		})
	})
}

func resumeAddToAlbum(ctx context.Context, deps *core.ToolDependencies, toolName, execID string, start time.Time, state *addToAlbumInterruptState) (*AddToAlbumOutput, error) {
	effectID, err := uuid.Parse(state.EffectID)
	if err != nil {
		return &AddToAlbumOutput{Error: ref.Internal("effect identity")}, nil
	}
	if !resumeApproved(ctx) {
		message := "Album update was not applied: the user declined."
		if err := deps.Effects.Reject(ctx, deps.UserID, deps.ThreadID, effectID); err != nil {
			refErr := ref.Internal("album update rejection")
			sendError(deps, toolName, execID, start, refErr)
			return &AddToAlbumOutput{Error: refErr}, nil
		}
		sendEffectReceipt(deps, toolName, execID, rejectedEffectReceipt(state.EffectID, toolName, state.Count, message))
		sendSuccess(deps, toolName, execID, start, message, nil)
		return &AddToAlbumOutput{Message: message}, nil
	}
	sendRunning(deps, toolName, execID, fmt.Sprintf("Adding %d assets to album...", state.Count), nil)
	receipt, err := deps.Effects.Commit(ctx, deps.UserID, deps.ThreadID, deps.RunID, effectID)
	if err != nil {
		refErr := ref.Internal("adding assets to album")
		sendError(deps, toolName, execID, start, refErr)
		return &AddToAlbumOutput{Error: refErr}, nil
	}
	message := committedReceiptMessage(receipt)
	sendEffectReceipt(deps, toolName, execID, receipt)
	sendSuccess(deps, toolName, execID, start, message, &core.DataPayload{RefID: state.RefID, Count: receipt.Count})
	return &AddToAlbumOutput{
		Message: message, AlbumID: receipt.AlbumID, Count: receipt.Count,
		AlreadyCommitted: receipt.AlreadyCommitted,
	}, nil
}
