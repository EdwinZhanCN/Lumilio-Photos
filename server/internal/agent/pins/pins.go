// Package pins promotes session refs to durable widgets. A pin stores the
// frozen snapshot and the plan that produced it; frozen pins always serve
// the stored snapshot, live pins replay the plan on hydration when the plan
// is a self-contained producer expression (filter_assets / search_*).
// Transformed or combined refs pin as frozen — their plans reference session
// refs that do not outlive the conversation.
package pins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/ref"
	"server/internal/db/repo"
	"server/internal/search"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	ModeFrozen = "frozen"
	ModeLive   = "live"
)

var ErrNotFound = errors.New("pin not found")

// Service owns pin persistence and hydration.
type Service struct {
	queries  *repo.Queries
	refStore ref.Store
	search   core.RetrieverSearch
}

func NewService(queries *repo.Queries, refStore ref.Store, search core.RetrieverSearch) *Service {
	return &Service{queries: queries, refStore: refStore, search: search}
}

// CreateParams describes a pin request from the frontend.
type CreateParams struct {
	UserID   int32
	ThreadID string
	RefID    string
	Title    string
	Widget   string
	Mode     string
	Layout   Layout
}

// Layout is the react-grid-layout cell of a pin.
type Layout struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// CreateFromRef copies a session ref into a durable pin. Live mode silently
// downgrades to frozen when the plan is not replayable.
func (s *Service) CreateFromRef(ctx context.Context, params CreateParams) (repo.AgentPin, error) {
	scope := ref.Scope{UserID: params.UserID, ThreadID: params.ThreadID}
	r, refErr := s.refStore.Get(scope, params.RefID)
	if refErr != nil {
		return repo.AgentPin{}, ErrNotFound
	}

	mode := params.Mode
	if mode != ModeLive || !isReplayable(r.Plan) {
		mode = ModeFrozen
	}
	widget := params.Widget
	if widget == "" {
		widget = core.WidgetCoverCard
	}

	planJSON, err := json.Marshal(r.Plan)
	if err != nil {
		return repo.AgentPin{}, fmt.Errorf("marshal plan: %w", err)
	}

	layout := params.Layout
	if layout.W <= 0 {
		layout.W = 4
	}
	if layout.H <= 0 {
		layout.H = 4
	}

	assetIDs := make([]pgtype.UUID, len(r.AssetIDs))
	for i, id := range r.AssetIDs {
		assetIDs[i] = pgtype.UUID{Bytes: id, Valid: true}
	}

	return s.queries.CreateAgentPin(ctx, repo.CreateAgentPinParams{
		UserID:    params.UserID,
		Title:     params.Title,
		Widget:    widget,
		Mode:      mode,
		Plan:      planJSON,
		Summary:   r.Summary,
		AssetIds:  assetIDs,
		Truncated: r.Truncated,
		LayoutX:   int32(layout.X),
		LayoutY:   int32(layout.Y),
		LayoutW:   int32(layout.W),
		LayoutH:   int32(layout.H),
	})
}

// List returns the user's pins in creation order.
func (s *Service) List(ctx context.Context, userID int32) ([]repo.AgentPin, error) {
	return s.queries.ListAgentPins(ctx, userID)
}

// Delete removes a pin; missing/cross-user pins report ErrNotFound.
func (s *Service) Delete(ctx context.Context, userID int32, pinID uuid.UUID) error {
	return s.queries.DeleteAgentPin(ctx, repo.DeleteAgentPinParams{
		PinID:  pgtype.UUID{Bytes: pinID, Valid: true},
		UserID: userID,
	})
}

// UpdateLayout persists one pin's grid cell.
func (s *Service) UpdateLayout(ctx context.Context, userID int32, pinID uuid.UUID, layout Layout) error {
	return s.queries.UpdateAgentPinLayout(ctx, repo.UpdateAgentPinLayoutParams{
		PinID:   pgtype.UUID{Bytes: pinID, Valid: true},
		UserID:  userID,
		LayoutX: int32(layout.X),
		LayoutY: int32(layout.Y),
		LayoutW: int32(layout.W),
		LayoutH: int32(layout.H),
	})
}

// ErrUnknownWidget rejects switching a pin to an unregistered view.
var ErrUnknownWidget = errors.New("unknown widget view")

// UpdateWidget switches which view a pin renders through. The widget is just
// the selected view over the same pinned ref, so this only validates the view
// is registered and rewrites the column — no snapshot or plan changes.
func (s *Service) UpdateWidget(ctx context.Context, userID int32, pinID uuid.UUID, widget string) error {
	if !core.IsKnownWidget(widget) {
		return ErrUnknownWidget
	}
	return s.queries.UpdateAgentPinWidget(ctx, repo.UpdateAgentPinWidgetParams{
		PinID:  pgtype.UUID{Bytes: pinID, Valid: true},
		UserID: userID,
		Widget: widget,
	})
}

// UpdateTitle renames one pin.
func (s *Service) UpdateTitle(ctx context.Context, userID int32, pinID uuid.UUID, title string) error {
	return s.queries.UpdateAgentPinTitle(ctx, repo.UpdateAgentPinTitleParams{
		PinID:  pgtype.UUID{Bytes: pinID, Valid: true},
		UserID: userID,
		Title:  title,
	})
}

// AssetIDs resolves a pin's current membership: the stored snapshot for
// frozen pins, a plan replay for live pins (falling back to the snapshot
// when the replay fails, so widgets degrade instead of breaking).
func (s *Service) AssetIDs(ctx context.Context, userID int32, pinID uuid.UUID) (repo.AgentPin, []uuid.UUID, error) {
	pin, err := s.queries.GetAgentPin(ctx, repo.GetAgentPinParams{
		PinID:  pgtype.UUID{Bytes: pinID, Valid: true},
		UserID: userID,
	})
	if err != nil {
		return repo.AgentPin{}, nil, ErrNotFound
	}

	if pin.Mode == ModeLive {
		var plan ref.Plan
		if err := json.Unmarshal(pin.Plan, &plan); err == nil {
			if ids, err := s.replay(ctx, plan); err == nil {
				return pin, ids, nil
			}
		}
	}

	ids := make([]uuid.UUID, 0, len(pin.AssetIds))
	for _, id := range pin.AssetIds {
		if id.Valid {
			ids = append(ids, uuid.UUID(id.Bytes))
		}
	}
	return pin, ids, nil
}

// isReplayable reports whether a plan is a self-contained producer
// expression that can be re-executed without session refs.
func isReplayable(plan ref.Plan) bool {
	switch plan.Op {
	case "filter_assets", "search_semantic", "search_text", "search_people":
		return len(plan.Parents) == 0
	default:
		return false
	}
}

// replay re-executes a producer plan and returns fresh ids.
func (s *Service) replay(ctx context.Context, plan ref.Plan) ([]uuid.UUID, error) {
	switch plan.Op {
	case "filter_assets":
		return s.replayFilter(ctx, plan.Params)
	case "search_semantic":
		if s.search == nil {
			return nil, errors.New("search unavailable")
		}
		ids, _, err := s.search.SearchAssetIDsSemantic(ctx, plan.Params["query"],
			search.ParseStrictness(plan.Params["strictness"]), ref.MaxSnapshotSize)
		return ids, err
	case "search_text":
		if s.search == nil {
			return nil, errors.New("search unavailable")
		}
		return s.search.SearchAssetIDsOCR(ctx, plan.Params["query"], ref.MaxSnapshotSize)
	case "search_people":
		var personIDs []int32
		for _, part := range strings.Split(plan.Params["person_ids"], ",") {
			if id, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
				personIDs = append(personIDs, int32(id))
			}
		}
		if len(personIDs) == 0 {
			return nil, errors.New("no person ids in plan")
		}
		rows, err := s.queries.GetAssetIDsByPersonIDs(ctx, repo.GetAssetIDsByPersonIDsParams{
			PersonIds: personIDs,
			Limit:     ref.MaxSnapshotSize,
		})
		if err != nil {
			return nil, err
		}
		return fromPg(rows), nil
	default:
		return nil, fmt.Errorf("plan op %q is not replayable", plan.Op)
	}
}

func (s *Service) replayFilter(ctx context.Context, params map[string]string) ([]uuid.UUID, error) {
	q := repo.GetAssetIDsUnifiedParams{Limit: ref.MaxSnapshotSize}
	if v := params["date_from"]; v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			q.DateFrom = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}
	if v := params["date_to"]; v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			q.DateTo = pgtype.Timestamptz{Time: t.Add(24*time.Hour - time.Nanosecond), Valid: true}
		}
	}
	if v := params["type"]; v != "" {
		assetType := strings.ToUpper(v)
		q.AssetType = &assetType
	}
	if v := params["filename"]; v != "" {
		operator := "contains"
		q.FilenameVal = &v
		q.FilenameOperator = &operator
	}
	if v := params["raw"]; v != "" {
		raw := v == "true"
		q.IsRaw = &raw
	}
	if v := params["rating"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			rating := int32(n)
			q.Rating = &rating
		}
	}
	if v := params["liked"]; v != "" {
		liked := v == "true"
		q.Liked = &liked
	}
	if v := params["place"]; v != "" {
		q.Place = &v
	}
	if v := params["camera"]; v != "" {
		q.CameraModel = &v
	}
	if v := params["lens"]; v != "" {
		q.LensModel = &v
	}
	if v := params["album_id"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			albumID := int32(n)
			q.AlbumID = &albumID
		}
	}
	if v := params["tag_names"]; v != "" {
		tagNames := make([]string, 0, strings.Count(v, ",")+1)
		for _, part := range strings.Split(v, ",") {
			if name := strings.TrimSpace(part); name != "" {
				tagNames = append(tagNames, name)
			}
		}
		if len(tagNames) > 0 {
			q.TagNames = tagNames
		}
	}
	rows, err := s.queries.GetAssetIDsUnified(ctx, q)
	if err != nil {
		return nil, err
	}
	return fromPg(rows), nil
}

func fromPg(ids []pgtype.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id.Valid {
			out = append(out, uuid.UUID(id.Bytes))
		}
	}
	return out
}
