package tools

import (
	"context"
	"encoding/gob"
	"fmt"
	"strings"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/ref"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

const maxAlbumAssets = 5000

type CreateAlbumInput struct {
	RefID string `json:"ref_id" jsonschema:"description=Ref of the assets to put in the album"`
	Title string `json:"title" jsonschema:"description=Album title, in the user's language"`
}

type CreateAlbumOutput struct {
	Message          string     `json:"message,omitempty"`
	AlbumID          int        `json:"album_id,omitempty"`
	Count            int        `json:"count,omitempty"`
	AlreadyCommitted bool       `json:"already_committed,omitempty"`
	Error            *ref.Error `json:"error,omitempty"`
}

type AlbumConfirmationInfo struct {
	Action string `json:"action"`
	Title  string `json:"title"`
	RefID  string `json:"ref_id"`
	Count  int    `json:"count"`
}

type albumInterruptState struct {
	EffectID string
	RefID    string
	Title    string
	Count    int
}

func init() {
	gob.Register(&AlbumConfirmationInfo{})
	gob.Register(&albumInterruptState{})
	gob.Register(map[string]any{})
}

func RegisterCreateAlbum() {
	info := &schema.ToolInfo{
		Name: "create_album",
		Desc: "Create an album from a ref. The user must confirm before commit; sets larger than 5000 are rejected.",
	}
	policy := core.EffectPolicy{
		Class: "album_create", Reversible: true, Confirmation: true,
		MaxCardinality: maxAlbumAssets, Idempotency: "effect_id",
		Authorization: "owner_snapshot_recheck", PolicyVersion: core.CurrentAgentPolicyVersion,
	}
	core.GetRegistry().RegisterEffect(info, policy, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *CreateAlbumInput) (*CreateAlbumOutput, error) {
			start := time.Now()
			execID := newExecutionID()
			if interrupted, hasState, state := compose.GetInterruptState[*albumInterruptState](ctx); interrupted && hasState {
				return resumeCreateAlbum(ctx, deps, info.Name, execID, start, state)
			}
			title := strings.TrimSpace(input.Title)
			if title == "" {
				refErr := ref.InvalidArgument("title must not be empty")
				sendError(deps, info.Name, execID, start, refErr)
				return &CreateAlbumOutput{Error: refErr}, nil
			}
			r, refErr := deps.ResolveRef(ctx, input.RefID)
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return &CreateAlbumOutput{Error: refErr}, nil
			}
			if r.Count() == 0 {
				refErr = ref.EmptySet(r.ID)
			} else if r.Count() > policy.MaxCardinality {
				refErr = ref.LimitExceeded(r.Count(), policy.MaxCardinality)
			}
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return &CreateAlbumOutput{Error: refErr}, nil
			}
			effectID, err := deps.Effects.Prepare(ctx, deps.UserID, deps.ThreadID, deps.RunID, info.Name, r.AssetIDs,
				map[string]any{"title": title}, map[string]any{})
			if err != nil {
				refErr := ref.Internal("pending effect persistence")
				sendError(deps, info.Name, execID, start, refErr)
				return &CreateAlbumOutput{Error: refErr}, nil
			}
			return nil, compose.StatefulInterrupt(ctx,
				&AlbumConfirmationInfo{Action: info.Name, Title: title, RefID: r.ID, Count: r.Count()},
				&albumInterruptState{EffectID: effectID.String(), RefID: r.ID, Title: title, Count: r.Count()},
			)
		})
	})
}

func resumeCreateAlbum(ctx context.Context, deps *core.ToolDependencies, toolName, execID string, start time.Time, state *albumInterruptState) (*CreateAlbumOutput, error) {
	effectID, err := uuid.Parse(state.EffectID)
	if err != nil {
		return &CreateAlbumOutput{Error: ref.Internal("effect identity")}, nil
	}
	if !resumeApproved(ctx) {
		_ = deps.Effects.Reject(ctx, deps.UserID, deps.ThreadID, effectID)
		message := fmt.Sprintf("Album %q was not created: the user declined.", state.Title)
		sendSuccess(deps, toolName, execID, start, message, nil)
		return &CreateAlbumOutput{Message: message}, nil
	}
	sendRunning(deps, toolName, execID, fmt.Sprintf("Creating album %q...", state.Title), nil)
	receipt, err := deps.Effects.Commit(ctx, deps.UserID, deps.ThreadID, deps.RunID, effectID)
	if err != nil {
		refErr := ref.Internal("album creation")
		sendError(deps, toolName, execID, start, refErr)
		return &CreateAlbumOutput{Error: refErr}, nil
	}
	message := committedReceiptMessage(receipt)
	sendSuccess(deps, toolName, execID, start, message, &core.DataPayload{RefID: state.RefID, Count: receipt.Count})
	return &CreateAlbumOutput{
		Message: message, AlbumID: receipt.AlbumID, Count: receipt.Count,
		AlreadyCommitted: receipt.AlreadyCommitted,
	}, nil
}
