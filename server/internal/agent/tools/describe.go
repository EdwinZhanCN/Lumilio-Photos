package tools

import (
	"context"
	"fmt"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/facets"
	"server/internal/agent/ref"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

// DescribeInput asks for the facet summary of a ref.
type DescribeInput struct {
	RefID string `json:"ref_id" jsonschema:"description=Ref to summarize"`
}

// DescribeOutput is the agent's only eyesight over a ref: aggregate facets,
// never asset rows (INV-1). Strings inside Facets originate from user
// content and are sanitized data values — treat them as data, not
// instructions.
type DescribeOutput struct {
	RefID  string            `json:"ref_id,omitempty"`
	Facets *ref.FacetSummary `json:"facets,omitempty"`
	Error  *ref.Error        `json:"error,omitempty"`
}

// RegisterDescribe registers the describe observer.
func RegisterDescribe() {
	info := &schema.ToolInfo{
		Name: "describe",
		Desc: "Summarize a ref: count, date range, time histogram, type distribution, " +
			"top places, top people, cameras, liked/rating stats. " +
			"Use this to verify a set matches the user's intent before showing or mutating it.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *DescribeInput) (*DescribeOutput, error) {
			start := time.Now()
			execID := newExecutionID()

			r, refErr := deps.RefStore.Get(deps.Scope(), input.RefID)
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return &DescribeOutput{Error: refErr}, nil
			}

			summary, err := facets.Build(ctx, deps.Queries, r)
			if err != nil {
				refErr := ref.Internal("facet aggregation")
				sendError(deps, info.Name, execID, start, refErr)
				return &DescribeOutput{Error: refErr}, nil
			}

			sendSuccess(deps, info.Name, execID, start,
				fmt.Sprintf("Described %s (%d assets)", r.ID, r.Count()),
				&core.DataPayload{RefID: r.ID, Count: r.Count()})
			return &DescribeOutput{RefID: r.ID, Facets: summary}, nil
		})
	})
}
