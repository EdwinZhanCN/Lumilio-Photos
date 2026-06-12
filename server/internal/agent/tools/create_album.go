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

// CreateAlbumInput creates an album from a ref after user confirmation.
type CreateAlbumInput struct {
	RefID string `json:"ref_id" jsonschema:"description=Ref of the assets to put in the album"`
	Title string `json:"title" jsonschema:"description=Album title, in the user's language"`
}

// CreateAlbumOutput reports the result; only ids/counts, never asset data.
type CreateAlbumOutput struct {
	Message string     `json:"message,omitempty"`
	AlbumID int        `json:"album_id,omitempty"`
	Count   int        `json:"count,omitempty"`
	Error   *ref.Error `json:"error,omitempty"`
}

// AlbumConfirmationInfo is the user-facing interrupt payload: the preview
// the frontend renders before the user approves the album creation.
type AlbumConfirmationInfo struct {
	Action  string `json:"action"`
	Message string `json:"message,omitempty"`
	Title   string `json:"title"`
	RefID   string `json:"ref_id"`
	Count   int    `json:"count"`
}

// albumInterruptState is the tool's persisted state across interrupt/resume.
type albumInterruptState struct {
	RefID string
	Title string
	Count int
}

// albumResumeDecision mirrors the frontend's resume target payload.
type albumResumeDecision struct {
	Approved bool `json:"approved"`
}

func init() {
	// Interrupt state and info ride the checkpoint (gob-encoded interface
	// values) — they must be registered or resume after restart fails.
	gob.Register(&AlbumConfirmationInfo{})
	gob.Register(&albumInterruptState{})
	gob.Register(map[string]any{})
}

// RegisterCreateAlbum registers the create_album terminal. It follows the
// preview-then-confirm contract for consequential actions: the first call
// interrupts with a preview (count + title); the actual write happens only
// after the user resumes with approval.
func RegisterCreateAlbum() {
	info := &schema.ToolInfo{
		Name: "create_album",
		Desc: "Create an album from the assets in a ref. The user is shown a preview and must confirm " +
			"before the album is created. Sets larger than 5000 are rejected.",
	}

	const maxAlbumAssets = 5000

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *CreateAlbumInput) (*CreateAlbumOutput, error) {
			start := time.Now()
			execID := newExecutionID()

			// Resume path: the tool reruns after the user answered the preview.
			if wasInterrupted, hasState, state := compose.GetInterruptState[*albumInterruptState](ctx); wasInterrupted && hasState {
				return resumeCreateAlbum(ctx, deps, info.Name, execID, start, state)
			}

			// First run: validate, then interrupt with a preview.
			if input.Title == "" {
				refErr := ref.InvalidArgument("title must not be empty")
				sendError(deps, info.Name, execID, start, refErr)
				return &CreateAlbumOutput{Error: refErr}, nil
			}
			r, refErr := deps.RefStore.Get(deps.Scope(), input.RefID)
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return &CreateAlbumOutput{Error: refErr}, nil
			}
			if r.Count() == 0 {
				refErr := ref.EmptySet(r.ID)
				sendError(deps, info.Name, execID, start, refErr)
				return &CreateAlbumOutput{Error: refErr}, nil
			}
			if r.Count() > maxAlbumAssets {
				refErr := ref.LimitExceeded(r.Count(), maxAlbumAssets)
				sendError(deps, info.Name, execID, start, refErr)
				return &CreateAlbumOutput{Error: refErr}, nil
			}

			return nil, compose.StatefulInterrupt(ctx,
				&AlbumConfirmationInfo{
					Action: "create_album",
					Title:  input.Title,
					RefID:  r.ID,
					Count:  r.Count(),
				},
				&albumInterruptState{RefID: r.ID, Title: input.Title, Count: r.Count()},
			)
		})
	})
}

func resumeCreateAlbum(ctx context.Context, deps *core.ToolDependencies, toolName, execID string, start time.Time, state *albumInterruptState) (*CreateAlbumOutput, error) {
	approved := false
	if _, hasData, data := compose.GetResumeContext[map[string]any](ctx); hasData {
		if v, ok := data["approved"].(bool); ok {
			approved = v
		}
	}

	if !approved {
		message := fmt.Sprintf("Album %q was not created: the user declined.", state.Title)
		sendSuccess(deps, toolName, execID, start, message, nil)
		return &CreateAlbumOutput{Message: message}, nil
	}

	// The ref may have expired while waiting for confirmation.
	r, refErr := deps.RefStore.Get(deps.Scope(), state.RefID)
	if refErr != nil {
		sendError(deps, toolName, execID, start, refErr)
		return &CreateAlbumOutput{Error: refErr}, nil
	}

	sendRunning(deps, toolName, execID, fmt.Sprintf("Creating album %q...", state.Title), nil)

	album, err := deps.Queries.CreateAlbum(ctx, repo.CreateAlbumParams{
		UserID:    deps.UserID,
		AlbumName: state.Title,
		AlbumType: repo.AlbumTypeDefault,
	})
	if err != nil {
		refErr := ref.Internal("album creation")
		sendError(deps, toolName, execID, start, refErr)
		return &CreateAlbumOutput{Error: refErr}, nil
	}

	for i, assetID := range toPgUUIDs(r.AssetIDs) {
		position := int32(i)
		if err := deps.Queries.AddAssetToAlbum(ctx, repo.AddAssetToAlbumParams{
			AssetID:  assetID,
			AlbumID:  album.AlbumID,
			Position: &position,
		}); err != nil {
			refErr := ref.Internal("adding assets to album")
			sendError(deps, toolName, execID, start, refErr)
			return &CreateAlbumOutput{Error: refErr, AlbumID: int(album.AlbumID)}, nil
		}
	}

	message := fmt.Sprintf("Created album %q with %d photos", state.Title, r.Count())
	sendSuccess(deps, toolName, execID, start, message, &core.DataPayload{RefID: r.ID, Count: r.Count()})
	return &CreateAlbumOutput{Message: message, AlbumID: int(album.AlbumID), Count: r.Count()}, nil
}
