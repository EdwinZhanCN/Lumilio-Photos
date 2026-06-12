package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/ref"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

const maxPeekRows = 10

// PeekInput inspects the first members of a ref.
type PeekInput struct {
	RefID string `json:"ref_id" jsonschema:"description=Ref to peek into"`
	N     int    `json:"n,omitempty" jsonschema:"description=How many rows (default 5, max 10)"`
}

// PeekOutput holds one sanitized line per asset: date · filename · type ·
// rating/liked markers. Filenames are user content (INV-7) — data only.
type PeekOutput struct {
	RefID string     `json:"ref_id,omitempty"`
	Lines []string   `json:"lines,omitempty"`
	Error *ref.Error `json:"error,omitempty"`
}

// RegisterPeek registers the peek observer.
func RegisterPeek() {
	info := &schema.ToolInfo{
		Name: "peek",
		Desc: "Look at the first few assets of a ref (one line each: date, filename, type, rating). " +
			"Budgeted at 10 rows — use describe for aggregate views of large sets.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *PeekInput) (*PeekOutput, error) {
			start := time.Now()
			execID := newExecutionID()

			r, refErr := deps.RefStore.Get(deps.Scope(), input.RefID)
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return &PeekOutput{Error: refErr}, nil
			}

			n := input.N
			if n <= 0 {
				n = 5
			}
			if n > maxPeekRows {
				n = maxPeekRows
			}
			page := r.Slice(0, n)
			if len(page) == 0 {
				refErr := ref.EmptySet(r.ID)
				sendError(deps, info.Name, execID, start, refErr)
				return &PeekOutput{Error: refErr}, nil
			}

			rows, err := deps.Queries.AgentPeekAssets(ctx, toPgUUIDs(page))
			if err != nil {
				refErr := ref.Internal("peek query")
				sendError(deps, info.Name, execID, start, refErr)
				return &PeekOutput{Error: refErr}, nil
			}

			type peekRow struct {
				line string
			}
			byID := make(map[uuid.UUID]peekRow, len(rows))
			for _, row := range rows {
				var parts []string
				if row.CapturedAt.Valid {
					parts = append(parts, row.CapturedAt.Time.Format("2006-01-02"))
				}
				if row.OriginalFilename != "" {
					parts = append(parts, ref.SanitizeUserText(row.OriginalFilename, ref.MaxPeekFieldLen))
				}
				parts = append(parts, strings.ToLower(row.Type))
				if row.Rating != nil && *row.Rating > 0 {
					parts = append(parts, fmt.Sprintf("%d★", *row.Rating))
				}
				if row.Liked != nil && *row.Liked {
					parts = append(parts, "liked")
				}
				byID[uuid.UUID(row.AssetID.Bytes)] = peekRow{line: strings.Join(parts, " · ")}
			}

			lines := make([]string, 0, len(page))
			for _, id := range page {
				if row, ok := byID[id]; ok {
					lines = append(lines, row.line)
				}
			}

			sendSuccess(deps, info.Name, execID, start,
				fmt.Sprintf("Peeked %d of %d assets", len(lines), r.Count()),
				&core.DataPayload{RefID: r.ID, Count: r.Count()})
			return &PeekOutput{RefID: r.ID, Lines: lines}, nil
		})
	})
}
