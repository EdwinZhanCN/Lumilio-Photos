package tools

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/ref"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// RankInput reorders a ref. relevance restores the producer's own ranking
// and is only meaningful for search-produced refs.
type RankInput struct {
	RefID string `json:"ref_id" jsonschema:"description=Ref to reorder"`
	By    string `json:"by" jsonschema:"enum=time,enum=quality,enum=relevance,description=Ranking dimension"`
	Desc  *bool  `json:"desc,omitempty" jsonschema:"description=Descending order (default true: newest/best first)"`
}

// TopInput keeps the first n members of a ref (snapshot order is semantic).
type TopInput struct {
	RefID string `json:"ref_id" jsonschema:"description=Ref to take from"`
	N     int    `json:"n" jsonschema:"description=How many members to keep from the front"`
}

// SampleInput picks n members of a ref.
type SampleInput struct {
	RefID    string `json:"ref_id" jsonschema:"description=Ref to sample from"`
	N        int    `json:"n" jsonschema:"description=Sample size"`
	Strategy string `json:"strategy,omitempty" jsonschema:"enum=random,enum=spread_over_time,description=random (default) or spread_over_time for an even chronological spread"`
}

// RegisterRank registers the rank transformer.
func RegisterRank() {
	info := &schema.ToolInfo{
		Name: "rank",
		Desc: "Reorder a ref by time (capture date), quality (aesthetic score from SigLIP MLP head, " +
			"falling back to rating/liked/resolution for unscored assets) or " +
			"relevance (restore search ranking; only valid for search-produced refs). Returns a new ref.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *RankInput) (*RefToolOutput, error) {
			start := time.Now()
			execID := newExecutionID()
			sendRunning(deps, info.Name, execID, fmt.Sprintf("Ranking by %s...", input.By), input)

			r, refErr := deps.RefStore.Get(deps.Scope(), input.RefID)
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}
			if r.Count() == 0 {
				refErr := ref.EmptySet(r.ID)
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}

			desc := true
			if input.Desc != nil {
				desc = *input.Desc
			}

			var ranked []uuid.UUID
			switch input.By {
			case "time":
				rows, err := deps.Queries.RankAssetIDsByTime(ctx, toPgUUIDs(r.AssetIDs))
				if err != nil {
					refErr := ref.Internal("rank query")
					sendError(deps, info.Name, execID, start, refErr)
					return errorOutput(refErr), nil
				}
				ranked = fromPgUUIDs(rows)
			case "quality":
				rows, err := deps.Queries.RankAssetIDsByQuality(ctx, toPgUUIDs(r.AssetIDs))
				if err != nil {
					refErr := ref.Internal("rank query")
					sendError(deps, info.Name, execID, start, refErr)
					return errorOutput(refErr), nil
				}
				ranked = fromPgUUIDs(rows)
			case "relevance":
				if !isSearchPlan(r.Plan.Op) {
					refErr := ref.InvalidArgument(
						"relevance ranking only applies to refs produced by search_semantic, search_text or search_people; " +
							"use by=time or by=quality for this ref")
					sendError(deps, info.Name, execID, start, refErr)
					return errorOutput(refErr), nil
				}
				// The snapshot order of a search ref is its relevance order.
				ranked = append([]uuid.UUID(nil), r.AssetIDs...)
				desc = true // already best-first; desc flag is a no-op here
			default:
				refErr := ref.InvalidArgument(fmt.Sprintf("by %q is not one of time, quality, relevance", input.By))
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}

			// SQL ranks ascending; default presentation is best/newest first.
			if desc && input.By != "relevance" {
				reverse(ranked)
			}

			summary := fmt.Sprintf("rank(%s, by=%s, desc=%t) → %d assets", r.ID, input.By, desc, len(ranked))
			out := deps.RefStore.Create(
				deps.Scope(),
				ref.Plan{Op: info.Name, Params: map[string]string{"by": input.By, "desc": strconv.FormatBool(desc)}, Parents: []string{r.ID}},
				input.By,
				summary,
				ranked,
				r.Truncated,
			)
			sendSuccess(deps, info.Name, execID, start, summary, &core.DataPayload{RefID: out.ID, Count: out.Count()})
			return receiptOutput(out, summary), nil
		})
	})
}

// RegisterTop registers the top transformer.
func RegisterTop() {
	info := &schema.ToolInfo{
		Name: "top",
		Desc: "Keep the first n members of a ref, in its current order. Typically used after rank.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *TopInput) (*RefToolOutput, error) {
			start := time.Now()
			execID := newExecutionID()
			sendRunning(deps, info.Name, execID, fmt.Sprintf("Taking top %d...", input.N), input)

			r, refErr := deps.RefStore.Get(deps.Scope(), input.RefID)
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}
			if input.N <= 0 {
				refErr := ref.InvalidArgument("n must be positive")
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}

			n := input.N
			if n > r.Count() {
				n = r.Count()
			}
			kept := append([]uuid.UUID(nil), r.AssetIDs[:n]...)

			summary := fmt.Sprintf("top(%s, %d) → %d assets", r.ID, input.N, len(kept))
			if len(kept) == 0 {
				summary += " (empty set)"
			}
			out := deps.RefStore.Create(
				deps.Scope(),
				ref.Plan{Op: info.Name, Params: map[string]string{"n": strconv.Itoa(input.N)}, Parents: []string{r.ID}},
				fmt.Sprintf("top%d", input.N),
				summary,
				kept,
				false,
			)
			sendSuccess(deps, info.Name, execID, start, summary, &core.DataPayload{RefID: out.ID, Count: out.Count()})
			return receiptOutput(out, summary), nil
		})
	})
}

// RegisterSample registers the sample transformer.
func RegisterSample() {
	info := &schema.ToolInfo{
		Name: "sample",
		Desc: "Pick n members of a ref: random, or spread_over_time for an even chronological spread " +
			"(good for 'a look across the year').",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *SampleInput) (*RefToolOutput, error) {
			start := time.Now()
			execID := newExecutionID()
			sendRunning(deps, info.Name, execID, "Sampling...", input)

			r, refErr := deps.RefStore.Get(deps.Scope(), input.RefID)
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}
			if input.N <= 0 {
				refErr := ref.InvalidArgument("n must be positive")
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}
			if r.Count() == 0 {
				refErr := ref.EmptySet(r.ID)
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}

			strategy := input.Strategy
			if strategy == "" {
				strategy = "random"
			}

			var sampled []uuid.UUID
			switch strategy {
			case "random":
				sampled = sampleRandom(r.AssetIDs, input.N)
			case "spread_over_time":
				rows, err := deps.Queries.RankAssetIDsByTime(ctx, toPgUUIDs(r.AssetIDs))
				if err != nil {
					refErr := ref.Internal("sample query")
					sendError(deps, info.Name, execID, start, refErr)
					return errorOutput(refErr), nil
				}
				sampled = sampleSpread(fromPgUUIDs(rows), input.N)
			default:
				refErr := ref.InvalidArgument(fmt.Sprintf("strategy %q is not one of random, spread_over_time", input.Strategy))
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}

			summary := fmt.Sprintf("sample(%s, %d, %s) → %d assets", r.ID, input.N, strategy, len(sampled))
			out := deps.RefStore.Create(
				deps.Scope(),
				ref.Plan{Op: info.Name, Params: map[string]string{"n": strconv.Itoa(input.N), "strategy": strategy}, Parents: []string{r.ID}},
				"sample",
				summary,
				sampled,
				false,
			)
			sendSuccess(deps, info.Name, execID, start, summary, &core.DataPayload{RefID: out.ID, Count: out.Count()})
			return receiptOutput(out, summary), nil
		})
	})
}

func isSearchPlan(op string) bool {
	return strings.HasPrefix(op, "search_")
}

func reverse(ids []uuid.UUID) {
	for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
		ids[i], ids[j] = ids[j], ids[i]
	}
}

// sampleRandom picks n members uniformly, preserving their original
// relative order so the result still reads chronologically.
func sampleRandom(ids []uuid.UUID, n int) []uuid.UUID {
	if n >= len(ids) {
		return append([]uuid.UUID(nil), ids...)
	}
	picked := rand.Perm(len(ids))[:n]
	keep := make(map[int]struct{}, n)
	for _, idx := range picked {
		keep[idx] = struct{}{}
	}
	out := make([]uuid.UUID, 0, n)
	for i, id := range ids {
		if _, ok := keep[i]; ok {
			out = append(out, id)
		}
	}
	return out
}

// sampleSpread picks n members evenly spaced across a chronologically
// ordered slice.
func sampleSpread(ids []uuid.UUID, n int) []uuid.UUID {
	if n >= len(ids) {
		return append([]uuid.UUID(nil), ids...)
	}
	out := make([]uuid.UUID, 0, n)
	step := float64(len(ids)) / float64(n)
	for i := 0; i < n; i++ {
		out = append(out, ids[int(float64(i)*step)])
	}
	return out
}
