package tools

import (
	"context"
	"fmt"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/ref"
	"server/internal/utils/phash"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// DedupeInput collapses near-duplicates within a ref.
type DedupeInput struct {
	RefID string `json:"ref_id" jsonschema:"description=Ref to collapse near-duplicates in"`
}

// RegisterDedupe registers the dedupe transformer: a ref-scoped near-duplicate
// collapse over perceptual hashes. It keeps one representative per cluster and
// preserves order, so running it after rank keeps the best frame of each burst.
func RegisterDedupe() {
	info := &schema.ToolInfo{
		Name: "dedupe",
		Desc: "Collapse near-duplicate and burst photos in a ref (perceptual-hash similarity), keeping one " +
			"representative per cluster while preserving order. Run it after rank so the kept frame is the " +
			"best of each burst (typical curate chain: filter → rank → dedupe → top). Returns a new ref.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *DedupeInput) (*RefToolOutput, error) {
			start := time.Now()
			execID := newExecutionID()
			sendRunning(deps, info.Name, execID, "Removing near-duplicates...", input)

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

			rows, err := deps.Queries.GetPHashEmbeddingsByAssetIDs(ctx, toPgUUIDs(r.AssetIDs))
			if err != nil {
				refErr := ref.Internal("dedupe query")
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}

			hashOf := make(map[uuid.UUID]uint64, len(rows))
			for _, row := range rows {
				if row.Vector == nil {
					continue
				}
				if h, ok := phash.FromVector(row.Vector.Slice()); ok {
					hashOf[uuid.UUID(row.AssetID.Bytes)] = h
				}
			}

			kept, clusters, removed := dedupeByPHash(r.AssetIDs, hashOf)

			summary := fmt.Sprintf("dedupe(%s) → %d assets (collapsed %d near-duplicate cluster(s), removed %d of %d)",
				r.ID, len(kept), clusters, removed, r.Count())
			out := deps.RefStore.Create(
				deps.Scope(),
				ref.Plan{Op: info.Name, Parents: []string{r.ID}},
				"dedupe",
				summary,
				kept,
				r.Truncated,
			)
			sendSuccess(deps, info.Name, execID, start, summary, &core.DataPayload{RefID: out.ID, Count: out.Count()})
			return receiptOutput(out, summary), nil
		})
	})
}

// dedupeByPHash keeps the first member (in ref order) of each near-duplicate
// cluster plus every asset without a usable pHash, preserving input order.
// Returns the kept ids, the number of multi-member clusters collapsed, and how
// many assets were removed. Picking the earliest member as representative means
// a preceding rank decides which frame survives each burst.
func dedupeByPHash(ids []uuid.UUID, hashOf map[uuid.UUID]uint64) (kept []uuid.UUID, clusters, removed int) {
	// Cluster only the hashable subset; hashIDs stays in ref order, so the
	// smallest index in a group is its earliest (representative) member.
	hashIDs := make([]uuid.UUID, 0, len(ids))
	hashes := make([]uint64, 0, len(ids))
	for _, id := range ids {
		if h, ok := hashOf[id]; ok {
			hashIDs = append(hashIDs, id)
			hashes = append(hashes, h)
		}
	}

	dropped := make(map[uuid.UUID]bool)
	for _, group := range phash.Cluster(hashes, phash.DefaultDuplicateThreshold) {
		if len(group) < 2 {
			continue
		}
		clusters++
		minIdx := group[0]
		for _, idx := range group {
			if idx < minIdx {
				minIdx = idx
			}
		}
		for _, idx := range group {
			if idx != minIdx {
				dropped[hashIDs[idx]] = true
				removed++
			}
		}
	}

	kept = make([]uuid.UUID, 0, len(ids)-removed)
	for _, id := range ids {
		if !dropped[id] {
			kept = append(kept, id)
		}
	}
	return kept, clusters, removed
}
