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
	"math"
	"sort"
	"strings"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/ref"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/search"

	"github.com/google/uuid"
)

const (
	ModeFrozen = "frozen"
	ModeLive   = "live"
)

var ErrNotFound = errors.New("pin not found")

// Service owns pin persistence and hydration.
type Service struct {
	queries   *repo.Queries
	refStore  ref.Store
	libraries *core.AuthorizedLibraryFactory
}

func NewService(queries *repo.Queries, refStore ref.Store, libraries *core.AuthorizedLibraryFactory) *Service {
	return &Service{queries: queries, refStore: refStore, libraries: libraries}
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
	r, refErr := s.refStore.Get(ctx, scope, params.RefID)
	if refErr != nil {
		return repo.AgentPin{}, ErrNotFound
	}
	if _, err := s.libraries.ForUser(params.UserID).AuthorizeAssetIDs(ctx, params.UserID, r.AssetIDs); err != nil {
		return repo.AgentPin{}, ErrNotFound
	}

	mode := params.Mode
	if mode != ModeLive || !isReplayable(r.Plan, params.UserID) {
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

	return s.queries.CreateAgentPin(ctx, repo.CreateAgentPinParams{
		UserID:    params.UserID,
		Title:     params.Title,
		Widget:    widget,
		Mode:      mode,
		Plan:      dbtypes.JSON(planJSON),
		Summary:   r.Summary,
		AssetIds:  dbtypes.UUIDs(r.AssetIDs),
		Truncated: r.Truncated,
		LayoutX:   int64(layout.X),
		LayoutY:   int64(layout.Y),
		LayoutW:   int64(layout.W),
		LayoutH:   int64(layout.H),
		PinID:     uuid.New(),
	})
}

// List returns the user's pins in creation order.
func (s *Service) List(ctx context.Context, userID int32) ([]repo.AgentPin, error) {
	return s.queries.ListAgentPins(ctx, userID)
}

// Delete removes a pin; missing/cross-user pins report ErrNotFound.
func (s *Service) Delete(ctx context.Context, userID int32, pinID uuid.UUID) error {
	return s.queries.DeleteAgentPin(ctx, repo.DeleteAgentPinParams{
		PinID:  pinID,
		UserID: userID,
	})
}

// UpdateLayout persists one pin's grid cell.
func (s *Service) UpdateLayout(ctx context.Context, userID int32, pinID uuid.UUID, layout Layout) error {
	return s.queries.UpdateAgentPinLayout(ctx, repo.UpdateAgentPinLayoutParams{
		PinID:   pinID,
		UserID:  userID,
		LayoutX: int64(layout.X),
		LayoutY: int64(layout.Y),
		LayoutW: int64(layout.W),
		LayoutH: int64(layout.H),
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
		PinID:  pinID,
		UserID: userID,
		Widget: widget,
	})
}

// UpdateTitle renames one pin.
func (s *Service) UpdateTitle(ctx context.Context, userID int32, pinID uuid.UUID, title string) error {
	return s.queries.UpdateAgentPinTitle(ctx, repo.UpdateAgentPinTitleParams{
		PinID:  pinID,
		UserID: userID,
		Title:  title,
	})
}

// AssetIDs resolves a pin's current membership: the stored snapshot for
// frozen pins, a plan replay for live pins (falling back to the snapshot
// when the replay fails, so widgets degrade instead of breaking).
func (s *Service) AssetIDs(ctx context.Context, userID int32, pinID uuid.UUID) (repo.AgentPin, []uuid.UUID, error) {
	pin, ids, _, err := s.AssetIDsWithMeta(ctx, userID, pinID)
	return pin, ids, err
}

type HydrationMeta struct {
	Source           string
	FallbackReason   string
	LastSuccessfulAt *time.Time
}

func (s *Service) AssetIDsWithMeta(ctx context.Context, userID int32, pinID uuid.UUID) (repo.AgentPin, []uuid.UUID, HydrationMeta, error) {
	pin, err := s.queries.GetAgentPin(ctx, repo.GetAgentPinParams{
		PinID:  pinID,
		UserID: userID,
	})
	if err != nil {
		return repo.AgentPin{}, nil, HydrationMeta{}, ErrNotFound
	}

	if pin.Mode == ModeLive {
		var plan ref.Plan
		fallbackReason := "invalid_plan"
		if err := json.Unmarshal(pin.Plan, &plan); err == nil {
			fallbackReason = "unsupported_plan_version"
			if isReplayable(plan, userID) {
				fallbackReason = "replay_failed"
			}
		}
		if isReplayable(plan, userID) {
			if ids, err := s.replay(ctx, userID, plan); err == nil {
				now := time.Now().UTC()
				_ = s.queries.TouchAgentPinLiveRefresh(ctx, repo.TouchAgentPinLiveRefreshParams{
					PinID: pin.PinID, UserID: userID,
					LastSuccessfulRefreshAt: dbtypes.NewTimestamp(now),
				})
				return pin, ids, HydrationMeta{Source: "live_replay", LastSuccessfulAt: &now}, nil
			}
		}
		ids, err := s.authorizedFrozenIDs(ctx, userID, pin)
		if err != nil {
			return repo.AgentPin{}, nil, HydrationMeta{}, err
		}
		return pin, ids, HydrationMeta{
			Source: "frozen_fallback", FallbackReason: fallbackReason,
			LastSuccessfulAt: dbTimePointer(pin.LastSuccessfulRefreshAt),
		}, nil
	}

	ids, err := s.authorizedFrozenIDs(ctx, userID, pin)
	if err != nil {
		return repo.AgentPin{}, nil, HydrationMeta{}, err
	}
	return pin, ids, HydrationMeta{
		Source: "frozen_snapshot", LastSuccessfulAt: dbTimePointer(pin.LastSuccessfulRefreshAt),
	}, nil
}

func (s *Service) authorizedFrozenIDs(ctx context.Context, userID int32, pin repo.AgentPin) ([]uuid.UUID, error) {
	ids := append([]uuid.UUID(nil), pin.AssetIds...)
	if _, err := s.libraries.ForUser(userID).AuthorizeAssetIDs(ctx, userID, ids); err != nil {
		return nil, ErrNotFound
	}
	return ids, nil
}

func dbTimePointer(value dbtypes.Timestamp) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}

// isReplayable reports whether a plan is a self-contained producer
// expression that can be re-executed without session refs.
func isReplayable(plan ref.Plan, userID int32) bool {
	if plan.SchemaVersion != ref.CurrentPlanSchemaVersion ||
		plan.ToolVersion != ref.CurrentToolVersion ||
		plan.AuthorizationScope.UserID != userID ||
		plan.CreationPolicyVersion != ref.CurrentPolicyVersion {
		return false
	}
	switch plan.Op {
	case "filter_assets", "search_semantic", "search_text", "search_people":
		return len(plan.Parents) == 0
	default:
		return false
	}
}

type semanticReplayPayload struct {
	Query           string               `json:"query"`
	Strictness      search.SetStrictness `json:"strictness"`
	ProfileVersion  string               `json:"profile_version"`
	ModelVersion    string               `json:"model_version"`
	IndexVersion    string               `json:"index_version"`
	LanguageVersion string               `json:"language_version"`
}

type textReplayPayload struct {
	Query string `json:"query"`
}

type peopleReplayPayload struct {
	PersonIDs []int32 `json:"person_ids"`
}

type filterReplayPayload struct {
	DateFrom             string   `json:"date_from,omitempty"`
	DateTo               string   `json:"date_to,omitempty"`
	Type                 string   `json:"type,omitempty"`
	Filename             string   `json:"filename,omitempty"`
	Composition          string   `json:"composition,omitempty"`
	StackMembership      string   `json:"stack_membership,omitempty"`
	StackKinds           []string `json:"stack_kinds,omitempty"`
	Rating               *int     `json:"rating,omitempty"`
	Liked                *bool    `json:"liked,omitempty"`
	Place                string   `json:"place,omitempty"`
	Camera               string   `json:"camera,omitempty"`
	Lens                 string   `json:"lens,omitempty"`
	AlbumID              *int     `json:"album_id,omitempty"`
	TagNames             []string `json:"tag_names,omitempty"`
	MinQualityPercentile *float64 `json:"min_quality_percentile,omitempty"`
}

// replay re-executes a producer plan and returns fresh ids.
func (s *Service) replay(ctx context.Context, userID int32, plan ref.Plan) ([]uuid.UUID, error) {
	library := s.libraries.ForUser(userID)
	switch plan.Op {
	case "filter_assets":
		return s.replayFilter(ctx, library, plan.Payload)
	case "search_semantic":
		var payload semanticReplayPayload
		if err := json.Unmarshal(plan.Payload, &payload); err != nil || payload.Query == "" {
			return nil, errors.New("invalid semantic replay payload")
		}
		if payload.IndexVersion == "" || plan.EmbeddingIndexVersion != payload.IndexVersion {
			return nil, errors.New("semantic replay index version mismatch")
		}
		ids, meta, err := library.SearchSemantic(ctx, payload.Query, payload.Strictness, ref.MaxSnapshotSize)
		if err != nil {
			return nil, err
		}
		if meta.ProfileVersion != payload.ProfileVersion ||
			meta.ModelVersion != payload.ModelVersion ||
			meta.IndexVersion != payload.IndexVersion ||
			meta.LanguageVersion != payload.LanguageVersion {
			return nil, errors.New("semantic replay profile is no longer available")
		}
		return ids, nil
	case "search_text":
		var payload textReplayPayload
		if err := json.Unmarshal(plan.Payload, &payload); err != nil || payload.Query == "" {
			return nil, errors.New("invalid OCR replay payload")
		}
		return library.SearchOCR(ctx, payload.Query, ref.MaxSnapshotSize)
	case "search_people":
		var payload peopleReplayPayload
		if err := json.Unmarshal(plan.Payload, &payload); err != nil || len(payload.PersonIDs) == 0 {
			return nil, errors.New("invalid people replay payload")
		}
		rows, err := library.SearchPeople(ctx, payload.PersonIDs, ref.MaxSnapshotSize)
		if err != nil {
			return nil, err
		}
		return append([]uuid.UUID(nil), rows...), nil
	default:
		return nil, fmt.Errorf("plan op %q is not replayable", plan.Op)
	}
}

func (s *Service) replayFilter(ctx context.Context, library *core.AuthorizedLibrary, raw json.RawMessage) ([]uuid.UUID, error) {
	var payload filterReplayPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, errors.New("invalid filter replay payload")
	}
	q := repo.GetMediaItemRefsUnifiedParams{Limit: ref.MaxSnapshotSize}
	if v := payload.DateFrom; v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			q.DateFrom = dbtypes.NewTimestamp(t)
		}
	}
	if v := payload.DateTo; v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			q.DateTo = dbtypes.NewTimestamp(t.Add(24*time.Hour - time.Nanosecond))
		}
	}
	if v := payload.Type; v != "" {
		assetType := strings.ToUpper(v)
		q.AssetType = &assetType
	}
	if v := payload.Filename; v != "" {
		operator := "contains"
		q.FilenameVal = &v
		q.FilenameOperator = &operator
	}
	if v := strings.ToLower(strings.TrimSpace(payload.Composition)); v != "" {
		q.Composition = v
	}
	if v := strings.ToLower(strings.TrimSpace(payload.StackMembership)); v != "" {
		q.StackMembership = v
	}
	if len(payload.StackKinds) > 0 {
		kinds := make([]string, 0, len(payload.StackKinds))
		for _, raw := range payload.StackKinds {
			if kind := strings.ToLower(strings.TrimSpace(raw)); kind != "" {
				kinds = append(kinds, kind)
			}
		}
		if len(kinds) > 0 {
			q.StackKinds = dbtypes.StringsJSONParam(kinds)
		}
	}
	if payload.Rating != nil {
		rating := int32(*payload.Rating)
		q.Rating = &rating
	}
	if payload.Liked != nil {
		q.Liked = payload.Liked
	}
	if v := payload.Place; v != "" {
		q.Place = &v
	}
	if v := payload.Camera; v != "" {
		q.CameraModel = &v
	}
	if v := payload.Lens; v != "" {
		q.LensModel = &v
	}
	if payload.AlbumID != nil {
		albumID := int32(*payload.AlbumID)
		q.AlbumID = &albumID
	}
	if len(payload.TagNames) > 0 {
		q.TagNames = dbtypes.StringsJSONParam(payload.TagNames)
	}
	rows, err := library.FilterAssetIDs(ctx, q)
	if err != nil {
		return nil, err
	}
	ids := append([]uuid.UUID(nil), rows...)
	if payload.MinQualityPercentile == nil {
		return ids, nil
	}
	if *payload.MinQualityPercentile < 1 || *payload.MinQualityPercentile > 99 {
		return nil, errors.New("invalid quality percentile")
	}
	scores, err := library.AestheticScores(ctx, ids)
	if err != nil {
		return nil, err
	}
	scoreOf := make(map[uuid.UUID]float64, len(scores))
	values := make([]float64, 0, len(scores))
	for _, row := range scores {
		id := row.AssetID
		scoreOf[id] = float64(row.Score)
		values = append(values, float64(row.Score))
	}
	if len(values) == 0 {
		return nil, nil
	}
	sort.Float64s(values)
	cut := interpolatedPercentile(values, *payload.MinQualityPercentile/100)
	kept := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if score, ok := scoreOf[id]; ok && score >= cut {
			kept = append(kept, id)
		}
	}
	return kept, nil
}

func interpolatedPercentile(sorted []float64, fraction float64) float64 {
	if len(sorted) == 1 || fraction <= 0 {
		return sorted[0]
	}
	if fraction >= 1 {
		return sorted[len(sorted)-1]
	}
	position := fraction * float64(len(sorted)-1)
	low, high := int(math.Floor(position)), int(math.Ceil(position))
	if low == high {
		return sorted[low]
	}
	weight := position - float64(low)
	return sorted[low]*(1-weight) + sorted[high]*weight
}
