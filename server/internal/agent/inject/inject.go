// Package inject materializes ask-time context and mention bindings into the
// session ref ledger and a fixed-schema, explicitly untrusted data message.
// Asset data never crosses the LLM boundary (INV-1, INV-7).
package inject

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"server/internal/agent/core"
	"server/internal/agent/pins"
	"server/internal/agent/ref"

	"github.com/google/uuid"
)

// ContextItem is one user-attached context chip from the frontend.
type ContextItem struct {
	Type     string   `json:"type"`
	AssetIDs []string `json:"asset_ids"`
	Label    string   `json:"label"`
}

// MentionItem is a structured mention binding; the server does not parse
// mention markers from the query text.
type MentionItem struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

// DroppedMention records a mention the server rejected (missing or unauthorized).
type DroppedMention struct {
	Type   string `json:"type"`
	ID     string `json:"id"`
	Label  string `json:"label"`
	Reason string `json:"reason"`
}

// PrepareResult holds the low-trust synthetic data and response metadata.
type PrepareResult struct {
	SyntheticData   string
	DroppedMentions []DroppedMention
}

type syntheticRefBinding struct {
	RefID       string `json:"ref_id"`
	ContextType string `json:"context_type"`
	Label       string `json:"label"`
	Count       int    `json:"count"`
}

type syntheticEntityBinding struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Dependencies for ask-time injection.
type Dependencies struct {
	Library  *core.AuthorizedLibrary
	RefStore ref.Store
	Pins     *pins.Service
	UserID   int32
	ThreadID string
}

// Prepare materializes context refs and mention bindings before the agent run.
func Prepare(ctx context.Context, deps Dependencies, contextItems []ContextItem, mentionItems []MentionItem) (PrepareResult, error) {
	scope := ref.Scope{UserID: deps.UserID, ThreadID: deps.ThreadID}

	var refBindings []syntheticRefBinding
	for _, item := range contextItems {
		binding, err := materializeContext(ctx, deps, scope, item)
		if err != nil {
			return PrepareResult{}, err
		}
		if binding.RefID != "" {
			refBindings = append(refBindings, binding)
		}
	}

	var entityBindings []syntheticEntityBinding
	var dropped []DroppedMention
	for _, m := range mentionItems {
		switch strings.ToLower(strings.TrimSpace(m.Type)) {
		case "person":
			binding, drop := materializePersonMention(ctx, deps, m)
			if binding != nil {
				entityBindings = append(entityBindings, *binding)
			}
			if drop != nil {
				dropped = append(dropped, *drop)
			}
		case "album":
			binding, drop := materializeAlbumMention(ctx, deps, m)
			if binding != nil {
				entityBindings = append(entityBindings, *binding)
			}
			if drop != nil {
				dropped = append(dropped, *drop)
			}
		case "pin":
			binding, err := materializePinMention(ctx, deps, scope, m)
			if err != nil {
				dropped = append(dropped, DroppedMention{
					Type: m.Type, ID: m.ID, Label: m.Label, Reason: "not_found",
				})
			} else if binding != nil {
				refBindings = append(refBindings, *binding)
			}
		case "camera":
			if binding := materializeStringMention("camera", m); binding != nil {
				entityBindings = append(entityBindings, *binding)
			}
		case "lens":
			if binding := materializeStringMention("lens", m); binding != nil {
				entityBindings = append(entityBindings, *binding)
			}
		default:
			dropped = append(dropped, DroppedMention{
				Type: m.Type, ID: m.ID, Label: m.Label, Reason: "unsupported_type",
			})
		}
	}

	synthetic, err := json.Marshal(map[string]any{
		"schema":         "agent-context/v1",
		"trust":          "untrusted_data",
		"attached_refs":  refBindings,
		"bound_entities": entityBindings,
	})
	if err != nil {
		return PrepareResult{}, err
	}
	return PrepareResult{
		SyntheticData: string(synthetic), DroppedMentions: dropped,
	}, nil
}

func materializeContext(ctx context.Context, deps Dependencies, scope ref.Scope, item ContextItem) (syntheticRefBinding, error) {
	if len(item.AssetIDs) == 0 {
		return syntheticRefBinding{}, nil
	}

	ids, err := parseAndValidateAssetIDs(ctx, deps.Library, item.AssetIDs)
	if err != nil {
		return syntheticRefBinding{}, err
	}
	if len(ids) == 0 {
		return syntheticRefBinding{}, nil
	}

	op, hint := contextOpAndHint(item.Type)
	label := ref.SanitizeUserText(item.Label, ref.MaxFacetValueLen)
	if label == "" {
		label = hint
	}

	summary := fmt.Sprintf("context(%s) → %d assets", hint, len(ids))
	r, refErr := deps.RefStore.Create(
		ctx, scope,
		ref.Plan{Op: op, Payload: ref.TypedPayload(map[string]any{"label": label})},
		hint,
		summary,
		ids,
		false,
	)
	if refErr != nil {
		return syntheticRefBinding{}, refErr
	}
	return syntheticRefBinding{
		RefID: r.ID, ContextType: hint, Label: label, Count: r.Count(),
	}, nil
}

func contextOpAndHint(contextType string) (op, hint string) {
	switch strings.ToLower(strings.TrimSpace(contextType)) {
	case "viewing":
		return "context.viewing", "viewing"
	default:
		return "context.selection", "selected"
	}
}

func parseAndValidateAssetIDs(ctx context.Context, library *core.AuthorizedLibrary, raw []string) ([]uuid.UUID, error) {
	seen := make(map[uuid.UUID]struct{}, len(raw))
	ordered := make([]uuid.UUID, 0, len(raw))

	for _, s := range raw {
		id, err := uuid.Parse(strings.TrimSpace(s))
		if err != nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ordered = append(ordered, id)
	}
	if len(ordered) == 0 {
		return nil, nil
	}
	return library.AuthorizeAssetIDs(ctx, library.UserID(), ordered)
}

func materializePersonMention(ctx context.Context, deps Dependencies, m MentionItem) (*syntheticEntityBinding, *DroppedMention) {
	personID, err := strconv.Atoi(strings.TrimSpace(m.ID))
	if err != nil || personID <= 0 {
		return nil, &DroppedMention{Type: m.Type, ID: m.ID, Label: m.Label, Reason: "invalid_id"}
	}

	row, err := deps.Library.Person(ctx, int32(personID))
	if err != nil {
		return nil, &DroppedMention{Type: m.Type, ID: m.ID, Label: m.Label, Reason: "not_found"}
	}

	name := ""
	if row.ClusterName != nil {
		name = ref.SanitizeUserText(*row.ClusterName, ref.MaxFacetValueLen)
	}
	if name == "" {
		name = ref.SanitizeUserText(m.Label, ref.MaxFacetValueLen)
	}
	return &syntheticEntityBinding{Type: "person", ID: strconv.Itoa(personID), Label: name}, nil
}

func materializeAlbumMention(ctx context.Context, deps Dependencies, m MentionItem) (*syntheticEntityBinding, *DroppedMention) {
	albumID, err := strconv.Atoi(strings.TrimSpace(m.ID))
	if err != nil || albumID <= 0 {
		return nil, &DroppedMention{Type: m.Type, ID: m.ID, Label: m.Label, Reason: "invalid_id"}
	}

	album, err := deps.Library.Album(ctx, int32(albumID))
	if err != nil {
		return nil, &DroppedMention{Type: m.Type, ID: m.ID, Label: m.Label, Reason: "not_found"}
	}
	title := ref.SanitizeUserText(album.AlbumName, ref.MaxFacetValueLen)
	if title == "" {
		title = ref.SanitizeUserText(m.Label, ref.MaxFacetValueLen)
	}
	return &syntheticEntityBinding{Type: "album", ID: strconv.Itoa(albumID), Label: title}, nil
}

func materializeStringMention(kind string, m MentionItem) *syntheticEntityBinding {
	label := ref.SanitizeUserText(m.Label, ref.MaxFacetValueLen)
	if label == "" {
		label = ref.SanitizeUserText(m.ID, ref.MaxFacetValueLen)
	}
	if label == "" {
		return nil
	}
	return &syntheticEntityBinding{Type: kind, ID: label, Label: label}
}

func materializePinMention(ctx context.Context, deps Dependencies, scope ref.Scope, m MentionItem) (*syntheticRefBinding, error) {
	pinID, err := uuid.Parse(strings.TrimSpace(m.ID))
	if err != nil {
		return nil, err
	}

	pin, assetIDs, err := deps.Pins.AssetIDs(ctx, deps.UserID, pinID)
	if err != nil {
		return nil, err
	}

	var plan ref.Plan
	if len(pin.Plan) > 0 {
		_ = plan // plan stored on pin; frozen snapshot is authoritative for injection
	}

	hint := ref.SanitizeUserText(pin.Title, maxHintLen)
	if hint == "" {
		hint = "pin"
	}
	label := ref.SanitizeUserText(m.Label, ref.MaxFacetValueLen)
	if label == "" {
		label = hint
	}

	summary := fmt.Sprintf("pin(%s) → %d assets", label, len(assetIDs))
	r, refErr := deps.RefStore.Create(
		ctx, scope,
		ref.Plan{Op: "context.pin", Payload: ref.TypedPayload(map[string]any{"pin_id": pinID, "label": label})},
		hint,
		summary,
		assetIDs,
		pin.Truncated,
	)
	if refErr != nil {
		return nil, refErr
	}
	return &syntheticRefBinding{
		RefID: r.ID, ContextType: "pin", Label: label, Count: r.Count(),
	}, nil
}

const maxHintLen = 12
