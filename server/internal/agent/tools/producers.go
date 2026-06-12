package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/ref"
	"server/internal/db/repo"
	"server/internal/search"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// SearchSemanticInput finds assets by visual/semantic similarity to a text
// query (CLIP embeddings + pgvector). Membership is decided by a per-query
// calibrated relevance cutoff — the ref is the full relevant set, not a
// fixed TopK. Use rank/top to trim it.
type SearchSemanticInput struct {
	Query      string `json:"query" jsonschema:"description=What the photos should depict, e.g. 'sunset over the sea'"`
	Strictness string `json:"strictness,omitempty" jsonschema:"enum=loose,enum=normal,enum=strict,description=Relevance bar. Default normal. Only use strict when a previous receipt reported the snapshot cap was hit, or the user explicitly insists on the most exhaustive precise matching — it runs an expensive exact scan"`
}

// SearchTextInput finds assets whose OCR'd text matches a query. tsquery
// matching is the membership test, so the result is naturally a set.
type SearchTextInput struct {
	Query string `json:"query" jsonschema:"description=Text to find inside photos (signs, documents, screenshots)"`
}

// SearchPeopleInput finds assets containing at least one of the given people.
type SearchPeopleInput struct {
	PersonIDs []int `json:"person_ids" jsonschema:"description=Person ids from lookup_people. Union semantics: assets with ANY of these people; intersect refs for 'both people'"`
}

// RegisterSearchSemantic registers the semantic search producer: a true set
// tool. The per-query calibrated cutoff decides membership; the receipt
// reports whether the set is complete so the agent knows when (and only
// when) a strict retry is justified.
func RegisterSearchSemantic() {
	info := &schema.ToolInfo{
		Name: "search_semantic",
		Desc: "Find all photos semantically matching a text description. Returns a ref holding the " +
			"complete relevant set (relevance-cutoff filtered), ordered by relevance — use rank/top to trim. " +
			"strictness defaults to normal; only set strict when a receipt reported the snapshot cap was hit " +
			"or the user explicitly demands the most exhaustive matching.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *SearchSemanticInput) (*RefToolOutput, error) {
			start := time.Now()
			execID := newExecutionID()
			sendRunning(deps, info.Name, execID, "Searching...", input)

			query := strings.TrimSpace(input.Query)
			if query == "" {
				refErr := ref.InvalidArgument("query must not be empty")
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}
			if deps.Search == nil {
				refErr := ref.FeatureUnavailable("search backend is not configured")
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}

			strictness := search.ParseStrictness(input.Strictness)
			ids, meta, err := deps.Search.SearchAssetIDsSemantic(ctx, query, strictness, ref.MaxSnapshotSize)
			if err != nil {
				refErr := ref.FeatureUnavailable("semantic search is currently unavailable")
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}

			summary := semanticSetSummary(query, strictness, ids, meta)
			r := deps.RefStore.Create(
				deps.Scope(),
				ref.Plan{Op: info.Name, Params: map[string]string{"query": query, "strictness": string(strictness)}},
				query,
				summary,
				ids,
				!meta.Complete,
			)
			sendSuccess(deps, info.Name, execID, start, summary, &core.DataPayload{RefID: r.ID, Count: r.Count()})
			return receiptOutput(r, summary), nil
		})
	})
}

// semanticSetSummary spells out set completeness so the agent can reason
// about strict retries instead of guessing.
func semanticSetSummary(query string, strictness search.SetStrictness, ids []uuid.UUID, meta search.SetMeta) string {
	var b strings.Builder
	fmt.Fprintf(&b, "search_semantic(%q, %s) → %d assets within relevance cutoff", query, strictness, len(ids))
	switch {
	case len(ids) == 0:
		b.WriteString(" (empty set — nothing in the library matches)")
	case !meta.Calibrated:
		b.WriteString(" (library too small to calibrate; returned all candidates)")
	case meta.Exact:
		if meta.Complete {
			b.WriteString(" (exact scan, complete)")
		} else {
			fmt.Fprintf(&b, " (exact scan, truncated at %d)", ref.MaxSnapshotSize)
		}
	case meta.Complete:
		fmt.Fprintf(&b, " (complete; scanned %d candidates)", meta.Scanned)
	default:
		fmt.Fprintf(&b, " (hit snapshot cap %d — set may be incomplete; retry with strictness=strict if completeness matters)", ref.MaxSnapshotSize)
	}
	return b.String()
}

// RegisterSearchText registers the OCR full-text search producer.
func RegisterSearchText() {
	info := &schema.ToolInfo{
		Name: "search_text",
		Desc: "Find all photos containing specific text (signs, menus, documents, screenshots) via OCR " +
			"full-text search. Text matching defines membership, so the ref is the complete matching set, " +
			"ordered by relevance.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *SearchTextInput) (*RefToolOutput, error) {
			start := time.Now()
			execID := newExecutionID()
			sendRunning(deps, info.Name, execID, "Searching...", input)

			query := strings.TrimSpace(input.Query)
			if query == "" {
				refErr := ref.InvalidArgument("query must not be empty")
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}
			if deps.Search == nil {
				refErr := ref.FeatureUnavailable("search backend is not configured")
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}

			ids, err := deps.Search.SearchAssetIDsOCR(ctx, query, ref.MaxSnapshotSize+1)
			if err != nil {
				refErr := ref.FeatureUnavailable("text search is currently unavailable")
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}
			truncated := len(ids) > ref.MaxSnapshotSize
			if truncated {
				ids = ids[:ref.MaxSnapshotSize]
			}

			summary := fmt.Sprintf("search_text(%q) → %d assets, relevance order", query, len(ids))
			if len(ids) == 0 {
				summary += " (empty set)"
			}
			if truncated {
				summary += fmt.Sprintf(" (truncated at %d)", ref.MaxSnapshotSize)
			}
			r := deps.RefStore.Create(
				deps.Scope(),
				ref.Plan{Op: info.Name, Params: map[string]string{"query": query}},
				query,
				summary,
				ids,
				truncated,
			)
			sendSuccess(deps, info.Name, execID, start, summary, &core.DataPayload{RefID: r.ID, Count: r.Count()})
			return receiptOutput(r, summary), nil
		})
	})
}

// RegisterSearchPeople registers the people producer.
func RegisterSearchPeople() {
	info := &schema.ToolInfo{
		Name: "search_people",
		Desc: "Find photos containing specific people. Use lookup_people first to resolve names to person ids. " +
			"Multiple ids mean 'any of these people'; intersect two refs via combine for 'both people'.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *SearchPeopleInput) (*RefToolOutput, error) {
			start := time.Now()
			execID := newExecutionID()
			sendRunning(deps, info.Name, execID, "Finding photos of these people...", input)

			if len(input.PersonIDs) == 0 {
				refErr := ref.InvalidArgument("person_ids must not be empty; resolve names with lookup_people first")
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}

			personIDs := make([]int32, len(input.PersonIDs))
			idStrs := make([]string, len(input.PersonIDs))
			for i, id := range input.PersonIDs {
				personIDs[i] = int32(id)
				idStrs[i] = strconv.Itoa(id)
			}

			rows, err := deps.Queries.GetAssetIDsByPersonIDs(ctx, repo.GetAssetIDsByPersonIDsParams{
				PersonIds: personIDs,
				Limit:     ref.MaxSnapshotSize + 1,
			})
			if err != nil {
				refErr := ref.Internal("people search query")
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}

			truncated := len(rows) > ref.MaxSnapshotSize
			if truncated {
				rows = rows[:ref.MaxSnapshotSize]
			}
			snapshot := fromPgUUIDs(rows)

			summary := fmt.Sprintf("search_people(%s) → %d assets", strings.Join(idStrs, ", "), len(snapshot))
			if len(snapshot) == 0 {
				summary += " (empty set)"
			}
			if truncated {
				summary += fmt.Sprintf(" (truncated at %d)", ref.MaxSnapshotSize)
			}

			r := deps.RefStore.Create(
				deps.Scope(),
				ref.Plan{Op: info.Name, Params: map[string]string{"person_ids": strings.Join(idStrs, ",")}},
				"people",
				summary,
				snapshot,
				truncated,
			)
			sendSuccess(deps, info.Name, execID, start, summary, &core.DataPayload{RefID: r.ID, Count: r.Count()})
			return receiptOutput(r, summary), nil
		})
	})
}
