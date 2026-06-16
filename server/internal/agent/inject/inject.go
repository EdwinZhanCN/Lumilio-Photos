// Package inject materializes ask-time context and mention bindings into the
// session ref ledger and instruction extras. Asset data never crosses the LLM
// boundary — only receipts and sanitized labels (INV-1, INV-7).
package inject

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"server/internal/agent/pins"
	"server/internal/agent/ref"
	"server/internal/db/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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

// PrepareResult holds instruction extras and metadata for the chat response.
type PrepareResult struct {
	InstructionExtras string
	DroppedMentions   []DroppedMention
}

// Dependencies for ask-time injection.
type Dependencies struct {
	Queries  *repo.Queries
	RefStore ref.Store
	Pins     *pins.Service
	UserID   int32
	ThreadID string
}

// Prepare materializes context refs and mention bindings before the agent run.
func Prepare(ctx context.Context, deps Dependencies, contextItems []ContextItem, mentionItems []MentionItem) (PrepareResult, error) {
	scope := ref.Scope{UserID: deps.UserID, ThreadID: deps.ThreadID}

	var contextLines []string
	for _, item := range contextItems {
		line, err := materializeContext(ctx, deps, scope, item)
		if err != nil {
			return PrepareResult{}, err
		}
		if line != "" {
			contextLines = append(contextLines, line)
		}
	}

	var boundLines []string
	var dropped []DroppedMention
	for _, m := range mentionItems {
		switch strings.ToLower(strings.TrimSpace(m.Type)) {
		case "person":
			line, drop := materializePersonMention(ctx, deps, m)
			if line != "" {
				boundLines = append(boundLines, line)
			}
			if drop != nil {
				dropped = append(dropped, *drop)
			}
		case "album":
			line, drop := materializeAlbumMention(ctx, deps, m)
			if line != "" {
				boundLines = append(boundLines, line)
			}
			if drop != nil {
				dropped = append(dropped, *drop)
			}
		case "pin":
			if err := materializePinMention(ctx, deps, scope, m); err != nil {
				dropped = append(dropped, DroppedMention{
					Type: m.Type, ID: m.ID, Label: m.Label, Reason: "not_found",
				})
			}
		case "camera":
			line := materializeStringMention("camera", m)
			if line != "" {
				boundLines = append(boundLines, line)
			}
		case "lens":
			line := materializeStringMention("lens", m)
			if line != "" {
				boundLines = append(boundLines, line)
			}
		default:
			dropped = append(dropped, DroppedMention{
				Type: m.Type, ID: m.ID, Label: m.Label, Reason: "unsupported_type",
			})
		}
	}

	return PrepareResult{
		InstructionExtras: FormatInstructionExtras(contextLines, boundLines),
		DroppedMentions:   dropped,
	}, nil
}

func materializeContext(ctx context.Context, deps Dependencies, scope ref.Scope, item ContextItem) (string, error) {
	if len(item.AssetIDs) == 0 {
		return "", nil
	}

	ids, err := parseAndValidateAssetIDs(ctx, deps.Queries, item.AssetIDs)
	if err != nil {
		return "", err
	}
	if len(ids) == 0 {
		return "", nil
	}

	op, hint := contextOpAndHint(item.Type)
	label := ref.SanitizeUserText(item.Label, ref.MaxFacetValueLen)
	if label == "" {
		label = hint
	}

	summary := fmt.Sprintf("context(%s) → %d assets", hint, len(ids))
	r := deps.RefStore.Create(
		scope,
		ref.Plan{Op: op, Params: map[string]string{"label": label}},
		hint,
		summary,
		ids,
		false,
	)
	return fmt.Sprintf("%s — %s", r.ID, label), nil
}

func contextOpAndHint(contextType string) (op, hint string) {
	switch strings.ToLower(strings.TrimSpace(contextType)) {
	case "viewing":
		return "context.viewing", "viewing"
	default:
		return "context.selection", "selected"
	}
}

func parseAndValidateAssetIDs(ctx context.Context, queries *repo.Queries, raw []string) ([]uuid.UUID, error) {
	seen := make(map[uuid.UUID]struct{}, len(raw))
	ordered := make([]uuid.UUID, 0, len(raw))
	pgIDs := make([]pgtype.UUID, 0, len(raw))

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
		pgIDs = append(pgIDs, pgtype.UUID{Bytes: id, Valid: true})
	}
	if len(pgIDs) == 0 {
		return nil, nil
	}

	rows, err := queries.GetAssetsByIDs(ctx, pgIDs)
	if err != nil {
		return nil, fmt.Errorf("validate context assets: %w", err)
	}

	existing := make(map[uuid.UUID]struct{}, len(rows))
	for _, row := range rows {
		if row.IsDeleted != nil && *row.IsDeleted {
			continue
		}
		existing[uuid.UUID(row.AssetID.Bytes)] = struct{}{}
	}

	validated := make([]uuid.UUID, 0, len(ordered))
	for _, id := range ordered {
		if _, ok := existing[id]; ok {
			validated = append(validated, id)
		}
	}
	return validated, nil
}

func materializePersonMention(ctx context.Context, deps Dependencies, m MentionItem) (string, *DroppedMention) {
	personID, err := strconv.Atoi(strings.TrimSpace(m.ID))
	if err != nil || personID <= 0 {
		return "", &DroppedMention{Type: m.Type, ID: m.ID, Label: m.Label, Reason: "invalid_id"}
	}

	row, err := deps.Queries.GetPersonByIDScoped(ctx, repo.GetPersonByIDScopedParams{
		ClusterID: int32(personID),
	})
	if err != nil {
		return "", &DroppedMention{Type: m.Type, ID: m.ID, Label: m.Label, Reason: "not_found"}
	}

	name := ""
	if row.ClusterName != nil {
		name = ref.SanitizeUserText(*row.ClusterName, ref.MaxFacetValueLen)
	}
	if name == "" {
		name = ref.SanitizeUserText(m.Label, ref.MaxFacetValueLen)
	}
	return fmt.Sprintf("person: %s (person_id=%d)", name, personID), nil
}

func materializeAlbumMention(ctx context.Context, deps Dependencies, m MentionItem) (string, *DroppedMention) {
	albumID, err := strconv.Atoi(strings.TrimSpace(m.ID))
	if err != nil || albumID <= 0 {
		return "", &DroppedMention{Type: m.Type, ID: m.ID, Label: m.Label, Reason: "invalid_id"}
	}

	album, err := deps.Queries.GetAlbumByID(ctx, int32(albumID))
	if err != nil {
		return "", &DroppedMention{Type: m.Type, ID: m.ID, Label: m.Label, Reason: "not_found"}
	}
	if album.UserID != deps.UserID {
		return "", &DroppedMention{Type: m.Type, ID: m.ID, Label: m.Label, Reason: "not_found"}
	}

	title := ref.SanitizeUserText(album.AlbumName, ref.MaxFacetValueLen)
	if title == "" {
		title = ref.SanitizeUserText(m.Label, ref.MaxFacetValueLen)
	}
	return fmt.Sprintf("album: %s (album_id=%d)", title, albumID), nil
}

func materializeStringMention(kind string, m MentionItem) string {
	label := ref.SanitizeUserText(m.Label, ref.MaxFacetValueLen)
	if label == "" {
		label = ref.SanitizeUserText(m.ID, ref.MaxFacetValueLen)
	}
	if label == "" {
		return ""
	}
	return fmt.Sprintf("%s: %s (%s=%q)", kind, label, kind, label)
}

func materializePinMention(ctx context.Context, deps Dependencies, scope ref.Scope, m MentionItem) error {
	pinID, err := uuid.Parse(strings.TrimSpace(m.ID))
	if err != nil {
		return err
	}

	pin, assetIDs, err := deps.Pins.AssetIDs(ctx, deps.UserID, pinID)
	if err != nil {
		return err
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
	deps.RefStore.Create(
		scope,
		ref.Plan{Op: "context.pin", Params: map[string]string{"pin_id": pinID.String(), "label": label}},
		hint,
		summary,
		assetIDs,
		pin.Truncated,
	)
	return nil
}

const maxHintLen = 12

// FormatInstructionExtras builds the Attached context / Bound entities blocks.
func FormatInstructionExtras(contextLines, boundLines []string) string {
	var b strings.Builder
	if len(contextLines) > 0 {
		b.WriteString("\n\nAttached context:\n")
		for _, line := range contextLines {
			fmt.Fprintf(&b, "- %s\n", line)
		}
	}
	if len(boundLines) > 0 {
		b.WriteString("\n\nBound entities:\n")
		for _, line := range boundLines {
			fmt.Fprintf(&b, "- %s\n", line)
		}
	}
	return b.String()
}
