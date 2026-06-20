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

// RegisterShow registers the photo-grid terminal (INV-1 — the model never
// enumerates photos in text).
func RegisterShow() {
	info := &schema.ToolInfo{
		Name: "show",
		Desc: "Display the assets in a ref to the user as a photo grid. " +
			"Use widget tools rather than describing or listing photos in text.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *ShowInput) (*ShowOutput, error) {
			return showWidget(deps, info.Name, input.RefID, input.Title, core.WidgetAssetGrid)
		})
	})
}

// RegisterShowFacets renders aggregate facets for a ref.
func RegisterShowFacets() {
	info := &schema.ToolInfo{
		Name: "show_facets",
		Desc: "Display a ref as an aggregate dashboard: count, date range, histogram, type distribution, top places, people and cameras. " +
			"Use this when the user asks for an overview or wants to understand a collection before opening individual photos.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *ShowInput) (*ShowOutput, error) {
			return showWidget(deps, info.Name, input.RefID, input.Title, core.WidgetFacetDashboard)
		})
	})
}

// RegisterShowTimeline renders the ref's capture-time distribution.
func RegisterShowTimeline() {
	info := &schema.ToolInfo{
		Name: "show_timeline",
		Desc: "Display a ref as a chronological timeline using the collection's time histogram. " +
			"Use this for trips, seasons, yearly reviews, or any request where time flow matters.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *ShowInput) (*ShowOutput, error) {
			return showWidget(deps, info.Name, input.RefID, input.Title, core.WidgetTimeline)
		})
	})
}

// RegisterShowStoryline renders representative assets as a visual sequence.
func RegisterShowStoryline() {
	info := &schema.ToolInfo{
		Name: "show_storyline",
		Desc: "Display the leading assets in a ref as a compact visual storyline. " +
			"For better stories, first rank/sample/top the ref into the intended sequence, then call this tool.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *ShowInput) (*ShowOutput, error) {
			return showWidget(deps, info.Name, input.RefID, input.Title, core.WidgetStoryline)
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
