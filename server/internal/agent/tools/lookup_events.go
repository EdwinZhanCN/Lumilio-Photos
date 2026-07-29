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

type LookupEventsInput struct {
	TitleQuery string `json:"title_query,omitempty" jsonschema:"description=Partial Event title; empty lists recent Events"`
}

type LookupEvent struct {
	EventID    string `json:"event_id"`
	Title      string `json:"title,omitempty"`
	StartAt    int64  `json:"start_at"`
	EndAt      int64  `json:"end_at"`
	MediaCount int    `json:"media_count"`
}

type LookupEventsOutput struct {
	Events []LookupEvent `json:"events,omitempty"`
	Error  *ref.Error    `json:"error,omitempty"`
}

func RegisterLookupEvents() {
	info := &schema.ToolInfo{
		Name: "lookup_events",
		Desc: "Resolve owner-scoped Events by title or recency. Returns stable Event IDs for filter_assets(event_id).",
	}
	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *LookupEventsInput) (*LookupEventsOutput, error) {
			start := time.Now()
			execID := newExecutionID()
			sendRunning(deps, info.Name, execID, "Looking up Events...", input)
			var query *string
			if input.TitleQuery != "" {
				query = &input.TitleQuery
			}
			rows, err := deps.Library.LookupEvents(ctx, query, maxLookupResults)
			if err != nil {
				refErr := ref.Internal("Event lookup")
				sendError(deps, info.Name, execID, start, refErr)
				return &LookupEventsOutput{Error: refErr}, nil
			}
			events := make([]LookupEvent, 0, len(rows))
			for _, row := range rows {
				title := ""
				if row.TitleOverride != nil {
					title = ref.SanitizeUserText(*row.TitleOverride, ref.MaxFacetValueLen)
				}
				events = append(events, LookupEvent{
					EventID: row.EventID, Title: title,
					StartAt: row.StartAt, EndAt: row.EndAt, MediaCount: int(row.MediaCount),
				})
			}
			sendSuccess(deps, info.Name, execID, start, fmt.Sprintf("Found %d Events", len(events)), nil)
			return &LookupEventsOutput{Events: events}, nil
		})
	})
}
