package tools

import (
	"context"
	"fmt"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/ref"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

// ShowInput renders a ref to the user. The event only carries the handle;
// the frontend hydrates assets through GET /api/v1/agent/refs/{id}/assets,
// so the displayed set is exactly the snapshot the agent operated on.
type ShowInput struct {
	RefID string `json:"ref_id" jsonschema:"description=Ref of the asset set to display"`
	Title string `json:"title,omitempty" jsonschema:"description=Optional short caption shown above the widget, in the user's language"`
}

// ShowOutput confirms the render request to the LLM.
type ShowOutput struct {
	Message string     `json:"message,omitempty"`
	Error   *ref.Error `json:"error,omitempty"`
}

// RegisterShow registers the show terminal: the single display path for
// asset sets (INV-1 — the model never enumerates photos in text).
func RegisterShow() {
	info := &schema.ToolInfo{
		Name: "show",
		Desc: "Display the assets in a ref to the user as a photo grid. " +
			"This is the only way to show photos — never describe or list them in text.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *ShowInput) (*ShowOutput, error) {
			start := time.Now()
			execID := newExecutionID()

			r, refErr := deps.RefStore.Get(deps.Scope(), input.RefID)
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return &ShowOutput{Error: refErr}, nil
			}
			if r.Count() == 0 {
				refErr := ref.EmptySet(r.ID)
				sendError(deps, info.Name, execID, start, refErr)
				return &ShowOutput{Error: refErr}, nil
			}

			params := map[string]any{}
			if input.Title != "" {
				params["title"] = input.Title
			}
			deps.Send(&core.SideChannelEvent{
				Type:      core.EventTypeWidgetShow,
				Timestamp: time.Now().UnixMilli(),
				Tool:      core.ToolIdentity{Name: info.Name, ExecutionID: execID},
				Execution: core.ExecutionInfo{
					Status:   core.ExecutionStatusSuccess,
					Message:  fmt.Sprintf("Displaying %d assets", r.Count()),
					Duration: time.Since(start).Milliseconds(),
				},
				Data: &core.DataPayload{
					RefID:  r.ID,
					Count:  r.Count(),
					Widget: core.WidgetAssetGrid,
					Params: params,
				},
			})

			return &ShowOutput{Message: fmt.Sprintf("Displayed %d assets to the user.", r.Count())}, nil
		})
	})
}
