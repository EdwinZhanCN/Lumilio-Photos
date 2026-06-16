package tools

import (
	"context"
	"encoding/gob"
	"fmt"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/ref"
	"server/internal/db/repo"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// AddToAlbumInput adds assets from a ref into an existing album.
type AddToAlbumInput struct {
	RefID   string `json:"ref_id" jsonschema:"description=Ref of the assets to add"`
	AlbumID int    `json:"album_id" jsonschema:"description=Target album id"`
}

// AddToAlbumOutput reports the mutation outcome.
type AddToAlbumOutput struct {
	Message string     `json:"message,omitempty"`
	AlbumID int        `json:"album_id,omitempty"`
	Count   int        `json:"count,omitempty"`
	Error   *ref.Error `json:"error,omitempty"`
}

// AddToAlbumConfirmationInfo is the user-facing interrupt payload.
type AddToAlbumConfirmationInfo struct {
	Action  string `json:"action"`
	Message string `json:"message,omitempty"`
	AlbumID int    `json:"album_id"`
	RefID   string `json:"ref_id"`
	Count   int    `json:"count"`
}

type addToAlbumInterruptState struct {
	RefID   string
	AlbumID int32
	Count   int
}

func init() {
	gob.Register(&AddToAlbumConfirmationInfo{})
	gob.Register(&addToAlbumInterruptState{})
}

// RegisterAddToAlbum registers the add_to_album terminal with confirmation.
func RegisterAddToAlbum() {
	info := &schema.ToolInfo{
		Name: "add_to_album",
		Desc: "Add every asset in a ref to an existing album. The user is shown a preview and must confirm " +
			"before assets are added. Sets larger than 5000 are rejected.",
	}

	const maxAddToAlbumAssets = 5000

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *AddToAlbumInput) (*AddToAlbumOutput, error) {
			start := time.Now()
			execID := newExecutionID()

			if wasInterrupted, hasState, state := compose.GetInterruptState[*addToAlbumInterruptState](ctx); wasInterrupted && hasState {
				return resumeAddToAlbum(ctx, deps, info.Name, execID, start, state)
			}

			if input.AlbumID <= 0 {
				refErr := ref.InvalidArgument("album_id must be positive")
				sendError(deps, info.Name, execID, start, refErr)
				return &AddToAlbumOutput{Error: refErr}, nil
			}

			album, err := deps.Queries.GetAlbumByID(ctx, int32(input.AlbumID))
			if err != nil || album.UserID != deps.UserID {
				refErr := ref.InvalidArgument(fmt.Sprintf("album %d not found", input.AlbumID))
				sendError(deps, info.Name, execID, start, refErr)
				return &AddToAlbumOutput{Error: refErr}, nil
			}

			r, refErr := deps.RefStore.Get(deps.Scope(), input.RefID)
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return &AddToAlbumOutput{Error: refErr}, nil
			}
			if r.Count() == 0 {
				refErr := ref.EmptySet(r.ID)
				sendError(deps, info.Name, execID, start, refErr)
				return &AddToAlbumOutput{Error: refErr}, nil
			}
			if r.Count() > maxAddToAlbumAssets {
				refErr := ref.LimitExceeded(r.Count(), maxAddToAlbumAssets)
				sendError(deps, info.Name, execID, start, refErr)
				return &AddToAlbumOutput{Error: refErr}, nil
			}

			return nil, compose.StatefulInterrupt(ctx,
				&AddToAlbumConfirmationInfo{
					Action:  "add_to_album",
					AlbumID: input.AlbumID,
					RefID:   r.ID,
					Count:   r.Count(),
				},
				&addToAlbumInterruptState{RefID: r.ID, AlbumID: int32(input.AlbumID), Count: r.Count()},
			)
		})
	})
}

func resumeAddToAlbum(ctx context.Context, deps *core.ToolDependencies, toolName, execID string, start time.Time, state *addToAlbumInterruptState) (*AddToAlbumOutput, error) {
	approved := false
	if _, hasData, data := compose.GetResumeContext[map[string]any](ctx); hasData {
		if v, ok := data["approved"].(bool); ok {
			approved = v
		}
	}

	if !approved {
		message := fmt.Sprintf("Album update was not applied: the user declined.")
		sendSuccess(deps, toolName, execID, start, message, nil)
		return &AddToAlbumOutput{Message: message}, nil
	}

	r, refErr := deps.RefStore.Get(deps.Scope(), state.RefID)
	if refErr != nil {
		sendError(deps, toolName, execID, start, refErr)
		return &AddToAlbumOutput{Error: refErr}, nil
	}

	sendRunning(deps, toolName, execID, fmt.Sprintf("Adding %d assets to album...", r.Count()), nil)

	for i, assetID := range toPgUUIDs(r.AssetIDs) {
		position := int32(i)
		if err := deps.Queries.AddAssetToAlbum(ctx, repo.AddAssetToAlbumParams{
			AssetID:  assetID,
			AlbumID:  state.AlbumID,
			Position: &position,
		}); err != nil {
			refErr := ref.Internal("adding assets to album")
			sendError(deps, toolName, execID, start, refErr)
			return &AddToAlbumOutput{Error: refErr, AlbumID: int(state.AlbumID)}, nil
		}
	}

	message := fmt.Sprintf("Added %d photos to album", r.Count())
	sendSuccess(deps, toolName, execID, start, message, &core.DataPayload{RefID: r.ID, Count: r.Count()})
	return &AddToAlbumOutput{Message: message, AlbumID: int(state.AlbumID), Count: r.Count()}, nil
}
