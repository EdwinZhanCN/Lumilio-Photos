package tools

import (
	"context"
	"encoding/gob"
	"fmt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"server/internal/agent/core"
	"server/internal/agent/ref"
	"strings"
	"time"
)

func RegisterMusic() {
	registry := core.GetRegistry()
	registry.Register(&schema.ToolInfo{Name: "search_music", Desc: "Search local music metadata, returning an ordered ref of up to 100 tracks. No audio mood inference. Use ref_id to refine an attached selection, then show_music for audition."}, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool("search_music", "Search local music metadata and produce an ordered selection ref.", func(ctx context.Context, input *core.MusicSearch) (*RefToolOutput, error) {
			var scope []uuid.UUID
			if input.RefID != "" {
				r, e := deps.ResolveRef(ctx, input.RefID)
				if e != nil {
					return errorOutput(e), nil
				}
				scope = append([]uuid.UUID{}, r.AssetIDs...)
			}
			rows, truncated, err := deps.Library.SearchMusic(ctx, *input, scope)
			if err != nil {
				return errorOutput(ref.InvalidArgument("music search failed; check limits and selection")), nil
			}
			ids := make([]uuid.UUID, len(rows))
			duration := 0.0
			unknown := 0
			for i, row := range rows {
				ids[i] = row.TrackID
				if row.Duration != nil {
					duration += *row.Duration
				} else {
					unknown++
				}
			}
			summary := fmt.Sprintf("%d tracks; known duration %.0f seconds; %d unknown durations; bounded selection=%t", len(ids), duration, unknown, truncated)
			plan := ref.Plan{Op: "search_music", Payload: ref.TypedPayload(input)}
			if input.RefID != "" {
				plan.Parents = []string{input.RefID}
			}
			r, e := deps.CreateRef(ctx, plan, "Music", summary, ids, truncated)
			if e != nil {
				return errorOutput(e), nil
			}
			sendSuccess(deps, "search_music", newExecutionID(), time.Now(), summary, &core.DataPayload{RefID: r.ID, Count: r.Count()})
			return receiptOutput(r, summary), nil
		})
	})
	registry.Register(&schema.ToolInfo{Name: "show_music", Desc: "Display an ordered music ref as an auditionable card. Does not start playback."}, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool("show_music", "Display a music selection for explicit user audition.", func(ctx context.Context, input *ShowInput) (*ShowOutput, error) {
			r, e := deps.ResolveRef(ctx, input.RefID)
			if e != nil {
				return &ShowOutput{Error: e}, nil
			}
			if _, err := deps.Library.MusicTracks(ctx, r.AssetIDs); err != nil {
				return &ShowOutput{Error: ref.InvalidArgument("selection must contain at most 100 live music tracks")}, nil
			}
			return showWidget(ctx, deps, "show_music", r.ID, input.Title, "music_tracks")
		})
	})
	registry.Register(&schema.ToolInfo{Name: "lookup_music_playlists", Desc: "Find up to 20 owned music playlists by title for append; do not guess playlist identifiers."}, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool("lookup_music_playlists", "Find music playlist targets.", func(ctx context.Context, input *LookupAlbumsInput) (map[string]any, error) {
			rows, err := deps.Library.LookupMusicPlaylists(ctx, input.TitleQuery)
			if err != nil {
				return map[string]any{"error": ref.Internal("playlist lookup")}, nil
			}
			results := make([]map[string]any, 0, len(rows))
			for _, row := range rows {
				results = append(results, map[string]any{"playlist_id": row.PlaylistID.String(), "title": ref.SanitizeUserText(row.Title, ref.MaxFacetValueLen), "revision": row.Revision})
			}
			return map[string]any{"playlists": results}, nil
		})
	})
	registerSaveMusicPlaylist()
}

type SaveMusicPlaylistInput struct {
	RefID        string `json:"ref_id"`
	Title        string `json:"title,omitempty" jsonschema:"description=New playlist title; omit when appending"`
	PlaylistID   string `json:"playlist_id,omitempty" jsonschema:"description=Existing owned playlist ID from lookup_music_playlists; omit to create"`
	SkipExisting bool   `json:"skip_existing" jsonschema:"description=When appending, skip tracks already present only when the user requests deduplication"`
}
type musicConfirmation struct {
	EffectID     string `json:"effect_id"`
	Action       string `json:"action"`
	RefID        string `json:"ref_id"`
	Title        string `json:"title"`
	Count        int    `json:"count"`
	PlaylistID   string `json:"playlist_id,omitempty"`
	SkipExisting bool   `json:"skip_existing"`
}

func init() { gob.Register(&musicConfirmation{}) }

func registerSaveMusicPlaylist() {
	info := &schema.ToolInfo{Name: "save_music_playlist", Desc: "Create or append a music playlist from the exact ordered ref. Requires user confirmation; maximum 100 tracks. Never claim saved before committed receipt."}
	policy := core.EffectPolicy{Class: "music_playlist_save", Reversible: true, Confirmation: true, MaxCardinality: core.MaxMusicSelection, Idempotency: "effect_id", Authorization: "owner_snapshot_recheck", PolicyVersion: core.CurrentAgentPolicyVersion}
	core.GetRegistry().RegisterEffect(info, policy, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *SaveMusicPlaylistInput) (*core.EffectReceipt, error) {
			if interrupted, has, state := compose.GetInterruptState[*musicConfirmation](ctx); interrupted && has {
				id, err := uuid.Parse(state.EffectID)
				if err != nil {
					return nil, err
				}
				if !resumeApproved(ctx) {
					if err := deps.Effects.Reject(ctx, deps.UserID, deps.ThreadID, id); err != nil {
						return nil, err
					}
					receipt := rejectedEffectReceipt(state.EffectID, info.Name, state.Count, "Playlist save declined")
					sendEffectReceipt(deps, info.Name, newExecutionID(), receipt)
					return &receipt, nil
				}
				receipt, err := deps.Effects.Commit(ctx, deps.UserID, deps.ThreadID, deps.RunID, id)
				if err != nil {
					return nil, err
				}
				sendEffectReceipt(deps, info.Name, newExecutionID(), receipt)
				return &receipt, nil
			}
			r, e := deps.ResolveRef(ctx, input.RefID)
			if e != nil {
				return nil, fmt.Errorf("%v", e)
			}
			if r.Count() == 0 {
				return nil, fmt.Errorf("empty music selection")
			}
			if _, err := deps.Library.MusicTracks(ctx, r.AssetIDs); err != nil {
				return nil, err
			}
			title := strings.TrimSpace(input.Title)
			target := uuid.Nil
			revision := int64(0)
			if input.PlaylistID != "" {
				var err error
				target, err = uuid.Parse(input.PlaylistID)
				if err != nil {
					return nil, err
				}
				p, err := deps.Library.MusicPlaylist(ctx, target)
				if err != nil {
					return nil, err
				}
				title = p.Title
				revision = p.Revision
			} else if title == "" || len(title) > 300 {
				return nil, fmt.Errorf("playlist title must contain 1–300 bytes")
			}
			id, err := deps.Effects.Prepare(ctx, deps.UserID, deps.ThreadID, deps.RunID, info.Name, r.AssetIDs, map[string]any{"title": title, "skip_existing": input.SkipExisting}, map[string]any{"playlist_id": target, "revision": revision})
			if err != nil {
				return nil, err
			}
			state := &musicConfirmation{EffectID: id.String(), Action: info.Name, RefID: r.ID, Title: title, Count: r.Count(), PlaylistID: input.PlaylistID, SkipExisting: input.SkipExisting}
			return nil, compose.StatefulInterrupt(ctx, state, state)
		})
	})
}
