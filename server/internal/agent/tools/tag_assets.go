package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/ref"
	"server/internal/db/repo"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// tagSourceUser mirrors the asset_tags.source 'user' value enforced by the
// asset_tags_source_check constraint (see service.AssetTagSourceUser). The tool
// layer talks to repo.Queries directly, so the constant is local to avoid
// coupling the agent tools package to the service layer.
const tagSourceUser = "user"

// TagAssetsInput tags or untags every asset in a ref.
type TagAssetsInput struct {
	RefID string   `json:"ref_id" jsonschema:"description=Ref of the asset set to tag/untag (from filter_assets or combine),required"`
	Tags  []string `json:"tags" jsonschema:"description=Tag names to add or remove,minItems=1"`
	Mode  string   `json:"mode" jsonschema:"description=Whether to add or remove the tags,enum=add,enum=remove,default=add"`
}

// TagAssetsOutput reports the mutation outcome: ref handle, count and a
// one-line summary only — never asset ids (INV-1).
type TagAssetsOutput struct {
	RefID   string     `json:"ref_id,omitempty"`
	Count   int        `json:"count,omitempty"`
	Summary string     `json:"summary,omitempty"`
	Error   *ref.Error `json:"error,omitempty"`
}

// RegisterTagAssets registers the tag_assets terminal. Add uses get-or-create +
// attach semantics (source='user', confidence 1.0, matching
// AssetService.AddManualTagToAsset); remove resolves each tag by name and
// detaches it, skipping names that don't exist.
func RegisterTagAssets() {
	info := &schema.ToolInfo{
		Name: "tag_assets",
		Desc: "Add or remove tags on every asset in a ref. Check the ref's count first; " +
			"sets larger than 1000 are rejected — narrow them before mutating.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *TagAssetsInput) (*TagAssetsOutput, error) {
			start := time.Now()
			execID := newExecutionID()

			mode := strings.ToLower(strings.TrimSpace(input.Mode))
			if mode == "" {
				mode = "add"
			}
			if mode != "add" && mode != "remove" {
				refErr := ref.InvalidArgument(fmt.Sprintf("mode %q must be add or remove", input.Mode))
				sendError(deps, info.Name, execID, start, refErr)
				return &TagAssetsOutput{Error: refErr}, nil
			}
			tagNames := normalizeTagNames(input.Tags)
			if len(tagNames) == 0 {
				refErr := ref.InvalidArgument("tags must contain at least one non-empty name")
				sendError(deps, info.Name, execID, start, refErr)
				return &TagAssetsOutput{Error: refErr}, nil
			}

			sendRunning(deps, info.Name, execID, fmt.Sprintf("%s tags on assets...", tagVerb(mode)), input)

			r, refErr := deps.RefStore.Get(deps.Scope(), input.RefID)
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return &TagAssetsOutput{Error: refErr}, nil
			}
			if r.Count() == 0 {
				refErr := ref.EmptySet(r.ID)
				sendError(deps, info.Name, execID, start, refErr)
				return &TagAssetsOutput{Error: refErr}, nil
			}
			if r.Count() > maxBulkMutationSize {
				refErr := ref.LimitExceeded(r.Count(), maxBulkMutationSize)
				sendError(deps, info.Name, execID, start, refErr)
				return &TagAssetsOutput{Error: refErr}, nil
			}

			if mode == "add" {
				if refErr := applyTagsAdd(ctx, deps, r.AssetIDs, tagNames); refErr != nil {
					sendError(deps, info.Name, execID, start, refErr)
					return &TagAssetsOutput{Error: refErr}, nil
				}
			} else {
				if refErr := applyTagsRemove(ctx, deps, r.AssetIDs, tagNames); refErr != nil {
					sendError(deps, info.Name, execID, start, refErr)
					return &TagAssetsOutput{Error: refErr}, nil
				}
			}

			summary := tagSummary(mode, r.Count(), tagNames)
			sendSuccess(deps, info.Name, execID, start, summary, &core.DataPayload{RefID: r.ID, Count: r.Count()})
			return &TagAssetsOutput{RefID: r.ID, Count: r.Count(), Summary: summary}, nil
		})
	})
}

// applyTagsAdd resolves each tag name (creating it if absent) and attaches it
// to every asset with source='user' and confidence 1.0 — the same semantics as
// AssetService.AddManualTagToAsset. Tag ids are resolved once up front so the
// inner loop is a single upsert per (asset, tag) pair.
func applyTagsAdd(ctx context.Context, deps *core.ToolDependencies, assetIDs []uuid.UUID, tagNames []string) *ref.Error {
	tagIDs, refErr := resolveOrCreateTagIDs(ctx, deps, tagNames)
	if refErr != nil {
		return refErr
	}

	confidence := pgtype.Numeric{}
	if err := confidence.Scan("1.000"); err != nil {
		return ref.Internal("tag confidence conversion")
	}

	for _, pgAssetID := range toPgUUIDs(assetIDs) {
		for _, tagID := range tagIDs {
			if err := deps.Queries.AddTagToAsset(ctx, repo.AddTagToAssetParams{
				AssetID:    pgAssetID,
				TagID:      tagID,
				Confidence: confidence,
				Source:     tagSourceUser,
			}); err != nil {
				return ref.Internal("adding tags to assets")
			}
		}
	}
	return nil
}

// applyTagsRemove resolves each tag name to an existing tag id (names that
// don't exist are skipped silently — you can't remove what's not there) and
// detaches it from every asset.
func applyTagsRemove(ctx context.Context, deps *core.ToolDependencies, assetIDs []uuid.UUID, tagNames []string) *ref.Error {
	tagIDs, refErr := resolveExistingTagIDs(ctx, deps, tagNames)
	if refErr != nil {
		return refErr
	}
	if len(tagIDs) == 0 {
		return nil // nothing to remove; treated as a no-op success
	}

	for _, pgAssetID := range toPgUUIDs(assetIDs) {
		for _, tagID := range tagIDs {
			if err := deps.Queries.RemoveTagFromAsset(ctx, repo.RemoveTagFromAssetParams{
				AssetID: pgAssetID,
				TagID:   tagID,
			}); err != nil {
				return ref.Internal("removing tags from assets")
			}
		}
	}
	return nil
}

// resolveOrCreateTagIDs maps each name to a tag id, creating the tag when it
// doesn't yet exist (mirrors AssetService.GetOrCreateTagByName).
func resolveOrCreateTagIDs(ctx context.Context, deps *core.ToolDependencies, names []string) ([]int32, *ref.Error) {
	ids := make([]int32, 0, len(names))
	for _, name := range names {
		tag, err := deps.Queries.GetTagByName(ctx, name)
		if err == nil {
			ids = append(ids, tag.TagID)
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, ref.Internal("tag lookup")
		}
		created, err := deps.Queries.CreateTag(ctx, repo.CreateTagParams{TagName: name})
		if err != nil {
			return nil, ref.Internal("tag create")
		}
		ids = append(ids, created.TagID)
	}
	return ids, nil
}

// resolveExistingTagIDs maps each name to an existing tag id. Names with no
// matching tag are skipped silently.
func resolveExistingTagIDs(ctx context.Context, deps *core.ToolDependencies, names []string) ([]int32, *ref.Error) {
	ids := make([]int32, 0, len(names))
	for _, name := range names {
		tag, err := deps.Queries.GetTagByName(ctx, name)
		if err == nil {
			ids = append(ids, tag.TagID)
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, ref.Internal("tag lookup")
		}
	}
	return ids, nil
}

// normalizeTagNames trims, lower-cases and de-duplicates tag names, dropping
// empties. Tag names are stored verbatim by the repo, but normalized input
// keeps the agent's idempotency predictable across calls.
func normalizeTagNames(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		name := strings.TrimSpace(t)
		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func tagVerb(mode string) string {
	if mode == "remove" {
		return "removing"
	}
	return "adding"
}

func tagSummary(mode string, count int, names []string) string {
	verb := "tagged"
	if mode == "remove" {
		verb = "removed tags from"
	}
	list := strings.Join(names, ", ")
	if mode == "remove" {
		return fmt.Sprintf("%s %d assets (tags: %s)", verb, count, list)
	}
	return fmt.Sprintf("%s %d assets with tags: %s", verb, count, list)
}
