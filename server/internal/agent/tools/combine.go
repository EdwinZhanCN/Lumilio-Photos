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

// CombineInput is the set-algebra transformer input: refs in, ref out
// (INV-2). diff is the "exclude this person / this period" operator and the
// first ref is always the base set.
type CombineInput struct {
	Op   string   `json:"op" jsonschema:"enum=union,enum=intersect,enum=diff,description=Set operation. diff = first ref minus all others"`
	Refs []string `json:"refs" jsonschema:"description=Two or more ref ids. Order matters: the first ref's ordering is kept"`
}

// RegisterCombine registers the combine transformer.
func RegisterCombine() {
	info := &schema.ToolInfo{
		Name: "combine",
		Desc: "Combine two or more refs with set algebra: union (either), intersect (both), " +
			"diff (in the first but none of the others). Returns a new ref; the first ref's order is preserved.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *CombineInput) (*RefToolOutput, error) {
			start := time.Now()
			execID := newExecutionID()
			sendRunning(deps, info.Name, execID, fmt.Sprintf("Combining refs (%s)...", input.Op), input)

			if input.Op != "union" && input.Op != "intersect" && input.Op != "diff" {
				refErr := ref.InvalidArgument(fmt.Sprintf("op %q is not one of union, intersect, diff", input.Op))
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}
			if len(input.Refs) < 2 {
				refErr := ref.InvalidArgument("combine needs at least two refs")
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}

			operands := make([]*ref.Ref, 0, len(input.Refs))
			for _, id := range input.Refs {
				r, refErr := deps.RefStore.Get(deps.Scope(), id)
				if refErr != nil {
					sendError(deps, info.Name, execID, start, refErr)
					return errorOutput(refErr), nil
				}
				operands = append(operands, r)
			}

			combined := combineSnapshots(input.Op, operands)
			truncated := len(combined) > ref.MaxSnapshotSize
			if truncated {
				combined = combined[:ref.MaxSnapshotSize]
			}

			summary := combineSummary(input.Op, input.Refs, operands, len(combined))
			r := deps.RefStore.Create(
				deps.Scope(),
				ref.Plan{Op: info.Name, Params: map[string]string{"op": input.Op}, Parents: input.Refs},
				input.Op,
				summary,
				combined,
				truncated,
			)
			sendSuccess(deps, info.Name, execID, start, summary, &core.DataPayload{RefID: r.ID, Count: r.Count()})
			return receiptOutput(r, summary), nil
		})
	})
}

// combineSummary spells out each operand's size and how the result relates to
// the base set, so the model can tell which operand constrained an intersect or
// diff instead of guessing from a bare result count.
func combineSummary(op string, refIDs []string, operands []*ref.Ref, resultCount int) string {
	labels := make([]string, len(operands))
	for i, o := range operands {
		labels[i] = fmt.Sprintf("%s[%d]", refIDs[i], o.Count())
	}
	summary := fmt.Sprintf("%s(%s) → %d assets", op, strings.Join(labels, ", "), resultCount)

	base := operands[0].Count()
	switch op {
	case "intersect":
		summary += fmt.Sprintf(" (in all; %d of base %s dropped)", base-resultCount, refIDs[0])
	case "diff":
		summary += fmt.Sprintf(" (%d of base %s excluded)", base-resultCount, refIDs[0])
	}
	if resultCount == 0 {
		summary += " (empty set)"
	}
	return summary
}

// combineSnapshots applies the set operation with explicit order semantics:
// intersect/diff keep the base (first) ref's order; union keeps the base
// order and appends unseen members of later refs in their own order.
func combineSnapshots(op string, operands []*ref.Ref) []uuid.UUID {
	base := operands[0]
	rest := operands[1:]

	switch op {
	case "union":
		seen := make(map[uuid.UUID]struct{}, base.Count())
		out := make([]uuid.UUID, 0, base.Count())
		for _, operand := range operands {
			for _, id := range operand.AssetIDs {
				if _, ok := seen[id]; !ok {
					seen[id] = struct{}{}
					out = append(out, id)
				}
			}
		}
		return out

	case "intersect":
		out := make([]uuid.UUID, 0, base.Count())
		memberships := make([]map[uuid.UUID]struct{}, len(rest))
		for i, operand := range rest {
			memberships[i] = idSet(operand.AssetIDs)
		}
	nextIntersect:
		for _, id := range base.AssetIDs {
			for _, members := range memberships {
				if _, ok := members[id]; !ok {
					continue nextIntersect
				}
			}
			out = append(out, id)
		}
		return out

	default: // diff
		excluded := make(map[uuid.UUID]struct{})
		for _, operand := range rest {
			for _, id := range operand.AssetIDs {
				excluded[id] = struct{}{}
			}
		}
		out := make([]uuid.UUID, 0, base.Count())
		for _, id := range base.AssetIDs {
			if _, ok := excluded[id]; !ok {
				out = append(out, id)
			}
		}
		return out
	}
}

func idSet(ids []uuid.UUID) map[uuid.UUID]struct{} {
	set := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}
