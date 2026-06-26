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
	RefID  string `json:"ref_id" jsonschema:"description=Ref of the asset set to display,required"`
	Title  string `json:"title,omitempty" jsonschema:"description=Optional title for the widget"`
	Widget string `json:"widget,omitempty" jsonschema:"description=Initial view to render (the user can switch it on the board),enum=cover_card,enum=number_card,enum=spark_card,enum=mosaic_card,default=cover_card"`
}

// ShowOutput confirms the render request to the LLM.
type ShowOutput struct {
	Message string     `json:"message,omitempty"`
	Error   *ref.Error `json:"error,omitempty"`
}

// RegisterShow registers the single terminal "show" tool (INV-1 — the model
// never enumerates photos in text). The optional widget param selects the
// render style; it defaults to cover_card when omitted.
func RegisterShow() {
	info := &schema.ToolInfo{
		Name: "show",
		Desc: "Display the assets in a ref to the user as a widget. " +
			"Use this tool rather than describing or listing photos in text. " +
			"The widget param is only the initial view (the user can switch it on the board): " +
			"set it to \"number_card\" for a compact count/stat view, otherwise omit for a cover card.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *ShowInput) (*ShowOutput, error) {
			widget := input.Widget
			if widget == "" {
				widget = core.WidgetCoverCard
			}
			return showWidget(deps, info.Name, input.RefID, input.Title, widget)
		})
	})
}

func showWidget(deps *core.ToolDependencies, toolName, refID, title, widget string) (*ShowOutput, error) {
	start := time.Now()
	execID := newExecutionID()

	r, refErr := deps.RefStore.Get(deps.Scope(), refID)
	if refErr != nil {
		sendError(deps, toolName, execID, start, refErr)
		return &ShowOutput{Error: refErr}, nil
	}
	if r.Count() == 0 {
		refErr := ref.EmptySet(r.ID)
		sendError(deps, toolName, execID, start, refErr)
		return &ShowOutput{Error: refErr}, nil
	}

	params := map[string]any{}
	if title != "" {
		params["title"] = title
	}
	deps.Send(&core.SideChannelEvent{
		Type:      core.EventTypeWidgetShow,
		Timestamp: time.Now().UnixMilli(),
		Tool:      core.ToolIdentity{Name: toolName, ExecutionID: execID},
		Execution: core.ExecutionInfo{
			Status:   core.ExecutionStatusSuccess,
			Message:  fmt.Sprintf("Displaying %d assets", r.Count()),
			Duration: time.Since(start).Milliseconds(),
		},
		Data: &core.DataPayload{
			RefID:  r.ID,
			Count:  r.Count(),
			Widget: widget,
			Params: params,
		},
	})

	return &ShowOutput{Message: fmt.Sprintf("Displayed %d assets to the user.", r.Count())}, nil
}
