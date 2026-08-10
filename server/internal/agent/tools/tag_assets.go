package tools

import (
	"context"
	"encoding/gob"
	"fmt"
	"strings"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/ref"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

type TagAssetsInput struct {
	RefID string   `json:"ref_id" jsonschema:"description=Ref of the asset set to tag/untag,required"`
	Tags  []string `json:"tags" jsonschema:"description=Tag names to add or remove,minItems=1"`
	Mode  string   `json:"mode" jsonschema:"enum=add,enum=remove,default=add"`
}

type TagAssetsOutput struct {
	RefID            string     `json:"ref_id,omitempty"`
	Count            int        `json:"count,omitempty"`
	Summary          string     `json:"summary,omitempty"`
	AlreadyCommitted bool       `json:"already_committed,omitempty"`
	Error            *ref.Error `json:"error,omitempty"`
}

type TagConfirmationInfo struct {
	EffectID string   `json:"effect_id"`
	Action   string   `json:"action"`
	RefID    string   `json:"ref_id"`
	Count    int      `json:"count"`
	Mode     string   `json:"mode"`
	Tags     []string `json:"tags"`
}

type tagInterruptState struct {
	EffectID string
	RefID    string
	Count    int
}

func init() {
	gob.Register(&TagConfirmationInfo{})
	gob.Register(&tagInterruptState{})
}

func RegisterTagAssets() {
	info := &schema.ToolInfo{
		Name: "tag_assets",
		Desc: "Add or remove tags on every asset in a ref. The user must confirm before commit; sets larger than 1000 are rejected.",
	}
	policy := core.EffectPolicy{
		Class: "asset_tags", Reversible: true, Confirmation: true,
		MaxCardinality: maxBulkMutationSize, Idempotency: "effect_id",
		Authorization: "owner_snapshot_recheck", PolicyVersion: core.CurrentAgentPolicyVersion,
	}
	core.GetRegistry().RegisterEffect(info, policy, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *TagAssetsInput) (*TagAssetsOutput, error) {
			start := time.Now()
			execID := newExecutionID()
			if interrupted, hasState, state := compose.GetInterruptState[*tagInterruptState](ctx); interrupted && hasState {
				effectID, err := uuid.Parse(state.EffectID)
				if err != nil {
					return &TagAssetsOutput{Error: ref.Internal("effect identity")}, nil
				}
				if !resumeApproved(ctx) {
					message := "Tag change was not applied: the user declined."
					if err := deps.Effects.Reject(ctx, deps.UserID, deps.ThreadID, effectID); err != nil {
						refErr := ref.Internal("tag rejection")
						sendError(deps, info.Name, execID, start, refErr)
						return &TagAssetsOutput{Error: refErr}, nil
					}
					sendEffectReceipt(deps, info.Name, execID, rejectedEffectReceipt(state.EffectID, info.Name, state.Count, message))
					sendSuccess(deps, info.Name, execID, start, message, nil)
					return &TagAssetsOutput{Summary: message}, nil
				}
				sendRunning(deps, info.Name, execID, "Applying tag change...", nil)
				receipt, err := deps.Effects.Commit(ctx, deps.UserID, deps.ThreadID, deps.RunID, effectID)
				if err != nil {
					refErr := ref.Internal("tag commit")
					sendError(deps, info.Name, execID, start, refErr)
					return &TagAssetsOutput{Error: refErr}, nil
				}
				message := committedReceiptMessage(receipt)
				sendEffectReceipt(deps, info.Name, execID, receipt)
				sendSuccess(deps, info.Name, execID, start, message, &core.DataPayload{RefID: state.RefID, Count: receipt.Count})
				return &TagAssetsOutput{RefID: state.RefID, Count: receipt.Count, Summary: message, AlreadyCommitted: receipt.AlreadyCommitted}, nil
			}

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
			r, refErr := deps.ResolveRef(ctx, input.RefID)
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return &TagAssetsOutput{Error: refErr}, nil
			}
			if r.Count() == 0 {
				refErr = ref.EmptySet(r.ID)
			} else if r.Count() > policy.MaxCardinality {
				refErr = ref.LimitExceeded(r.Count(), policy.MaxCardinality)
			}
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return &TagAssetsOutput{Error: refErr}, nil
			}
			effectID, err := deps.Effects.Prepare(ctx, deps.UserID, deps.ThreadID, deps.RunID, info.Name, r.AssetIDs,
				map[string]any{"mode": mode, "tags": tagNames}, map[string]any{})
			if err != nil {
				refErr := ref.Internal("pending effect persistence")
				sendError(deps, info.Name, execID, start, refErr)
				return &TagAssetsOutput{Error: refErr}, nil
			}
			return nil, compose.StatefulInterrupt(ctx,
				&TagConfirmationInfo{EffectID: effectID.String(), Action: info.Name, RefID: r.ID, Count: r.Count(), Mode: mode, Tags: tagNames},
				&tagInterruptState{EffectID: effectID.String(), RefID: r.ID, Count: r.Count()},
			)
		})
	})
}

func normalizeTagNames(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, value := range tags {
		name := strings.TrimSpace(value)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
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
	return fmt.Sprintf("%s tags on %d assets: %s", mode, count, strings.Join(names, ", "))
}
