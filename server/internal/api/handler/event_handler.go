package handler

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/db/catalogtx"
	"server/internal/event"
	"server/internal/pipeline"
	"server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type EventHandler struct {
	service   *event.Service
	db        *sql.DB
	writer    *catalogtx.Writer
	reader    *sql.DB
	shares    service.ResolvedSnapshotShareService
	relations *event.RelationService
}

func NewEventHandler(eventService *event.Service, db *sql.DB, shares service.ResolvedSnapshotShareService) *EventHandler {
	return NewEventHandlerWithReader(eventService, db, catalogtx.NewWriter(db, nil), db, shares)
}

func NewEventHandlerWithReader(eventService *event.Service, db *sql.DB, writer *catalogtx.Writer, reader *sql.DB, shares service.ResolvedSnapshotShareService) *EventHandler {
	handler := &EventHandler{
		service: eventService, db: db, writer: writer, reader: reader, shares: shares,
		relations: event.NewRelationService(reader),
	}
	return handler
}

type eventCursor struct {
	Version       int    `json:"v"`
	StartAt       int64  `json:"start_at"`
	EventID       string `json:"event_id"`
	RepositoryID  string `json:"repository_id,omitempty"`
	IncludeHidden bool   `json:"include_hidden,omitempty"`
}

// ListEvents returns the authenticated owner's active visible Events, with an
// optional repository projection over their member media.
// @Summary List Events
// @Description List owner-scoped Events, optionally projecting membership counts and covers to one repository.
// @Tags events
// @Produce json
// @Param repository_id query string false "Optional repository UUID filter"
// @Param include_hidden query bool false "Include Events hidden from the default grid" default(false)
// @Param limit query int false "Page size"
// @Param cursor query string false "Opaque cursor"
// @Success 200 {object} dto.EventListPageDTO
// @Router /api/v1/events [get]
func (h *EventHandler) ListEvents(c *gin.Context) {
	ownerID, ok := eventOwner(c)
	if !ok {
		return
	}
	repositoryID, ok := parseOptionalRepositoryUUID(c)
	if !ok {
		return
	}
	repositoryFilter := ""
	if repositoryID.Valid {
		repositoryFilter = repositoryID.UUID.String()
	}
	includeHidden := parseBoolQuery(c, "include_hidden")
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			api.WriteProblem(c, api.BadRequest(errors.New("invalid limit")))
			return
		}
		limit = value
	}
	var cursor eventCursor
	if raw := c.Query("cursor"); raw != "" {
		body, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil || json.Unmarshal(body, &cursor) != nil || cursor.Version != 1 ||
			uuid.Validate(cursor.EventID) != nil || cursor.RepositoryID != repositoryFilter ||
			cursor.IncludeHidden != includeHidden {
			api.WriteProblem(c, api.BadRequest(errors.New("invalid cursor")))
			return
		}
	}
	rows, err := h.reader.QueryContext(c, `
SELECT e.event_id FROM events e
WHERE e.owner_id=? AND e.status='active' AND (? OR e.is_hidden=0)
AND (?='' OR EXISTS (
  SELECT 1
  FROM event_media_items emi
  JOIN media_items mi
    ON mi.media_item_id=emi.media_item_id AND mi.owner_id=emi.owner_id
  WHERE emi.event_id=e.event_id AND emi.owner_id=e.owner_id
    AND EXISTS (
      SELECT 1
      FROM media_item_assets scoped_membership
      JOIN active_asset_occurrences occurrence
        ON occurrence.asset_id=scoped_membership.asset_id
      WHERE scoped_membership.media_item_id=mi.media_item_id
        AND occurrence.repository_id=?
    )
))
AND (?='' OR e.start_at<? OR (e.start_at=? AND e.event_id<?))
ORDER BY e.start_at DESC,e.event_id DESC LIMIT ?`,
		ownerID, includeHidden, repositoryFilter, repositoryFilter,
		cursor.EventID, cursor.StartAt, cursor.StartAt, cursor.EventID, limit+1)
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			api.WriteProblem(c, api.Internal(err))
			return
		}
		ids = append(ids, id)
	}
	response := dto.EventListPageDTO{Events: []dto.EventSummaryDTO{}}
	hasMore := len(ids) > limit
	if hasMore {
		ids = ids[:limit]
	}
	for _, id := range ids {
		summary, err := h.service.Resolver().Resolve(c, ownerID, id)
		if err != nil {
			api.WriteProblem(c, api.Internal(err))
			return
		}
		if repositoryFilter != "" {
			summary, err = h.service.Resolver().ProjectToRepository(c, ownerID, summary, repositoryFilter)
			if err != nil {
				api.WriteProblem(c, api.Internal(err))
				return
			}
		}
		response.Events = append(response.Events, eventSummaryDTO(summary))
	}
	if hasMore && len(response.Events) > 0 {
		last := response.Events[len(response.Events)-1]
		body, _ := json.Marshal(eventCursor{
			Version: 1, StartAt: last.StartAt, EventID: last.EventID,
			RepositoryID: repositoryFilter, IncludeHidden: includeHidden,
		})
		response.NextCursor = base64.RawURLEncoding.EncodeToString(body)
	}
	api.JSONOK(c, response)
}

// GetEvent resolves active and redirected Event URLs.
// @Summary Get Event
// @Tags events
// @Produce json
// @Param id path string true "Event UUID"
// @Param repository_id query string false "Optional repository Browse Scope"
// @Success 200 {object} dto.EventDetailDTO
// @Router /api/v1/events/{id} [get]
func (h *EventHandler) GetEvent(c *gin.Context) {
	ownerID, ok := eventOwner(c)
	if !ok {
		return
	}
	summary, err := h.service.Resolver().Resolve(c, ownerID, c.Param("id"))
	if err != nil {
		h.respondError(c, err)
		return
	}
	if repositoryID, valid := parseOptionalRepositoryUUID(c); !valid {
		return
	} else if repositoryID.Valid {
		summary, err = h.service.Resolver().ProjectToRepository(c, ownerID, summary, repositoryID.UUID.String())
		if err != nil {
			h.respondError(c, err)
			return
		}
	}
	pending := h.pending(c, ownerID)
	api.JSONOK(c, dto.EventDetailDTO{
		EventSummaryDTO:  eventSummaryDTO(summary),
		AlgorithmVersion: event.AlgorithmVersion,
		PendingRebuild:   pending,
	})
}

// GetEventAssets returns ordered displayable Event assets.
// @Summary Get Event assets
// @Tags events
// @Produce json
// @Param id path string true "Event UUID"
// @Param repository_id query string false "Optional repository Browse Scope"
// @Param limit query int false "Page size"
// @Success 200 {object} dto.EventAssetsPageDTO
// @Router /api/v1/events/{id}/assets [get]
func (h *EventHandler) GetEventAssets(c *gin.Context) {
	ownerID, ok := eventOwner(c)
	if !ok {
		return
	}
	limit := 100
	if raw := c.Query("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 500 {
			api.WriteProblem(c, api.BadRequest(errors.New("invalid limit")))
			return
		}
		limit = value
	}
	repositoryID, valid := parseOptionalRepositoryUUID(c)
	if !valid {
		return
	}
	var assets []event.ResolvedAsset
	var omitted int
	var err error
	if repositoryID.Valid {
		assets, omitted, err = h.service.Resolver().OrderedAssetsForRepository(c, ownerID, c.Param("id"), repositoryID.UUID.String(), limit)
	} else {
		assets, omitted, err = h.service.Resolver().OrderedAssets(c, ownerID, c.Param("id"), limit)
	}
	if err != nil {
		h.respondError(c, err)
		return
	}
	response := dto.EventAssetsPageDTO{Assets: make([]dto.EventAssetDTO, len(assets)), OmittedMembers: omitted}
	for i, asset := range assets {
		response.Assets[i] = dto.EventAssetDTO(asset)
	}
	api.JSONOK(c, response)
}

// PatchEvent updates user-owned Event presentation state.
// @Summary Update Event
// @Tags events
// @Accept json
// @Produce json
// @Param id path string true "Event UUID"
// @Param request body dto.EventPatchRequestDTO true "Event changes"
// @Success 200 {object} dto.EventMutationResponseDTO
// @Router /api/v1/events/{id} [patch]
func (h *EventHandler) PatchEvent(c *gin.Context) {
	ownerID, ok := eventOwner(c)
	if !ok {
		return
	}
	var request dto.EventPatchRequestDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	if (request.TitleOverride != nil && request.ClearTitleOverride) ||
		(request.CoverMediaItemID != nil && request.ClearCoverOverride) {
		api.WriteProblem(c, api.BadRequest(errors.New("contradictory set and clear")))
		return
	}
	summary, err := h.service.Resolver().Resolve(c, ownerID, c.Param("id"))
	if err != nil {
		h.respondError(c, err)
		return
	}
	title := summary.TitleOverride
	if request.ClearTitleOverride {
		title = nil
	} else if request.TitleOverride != nil {
		value := strings.TrimSpace(*request.TitleOverride)
		if value == "" {
			api.WriteProblem(c, api.BadRequest(errors.New("empty title")))
			return
		}
		title = &value
	}
	// A generated cover is presentation output, not a user override.  A
	// rename/hide-only patch must preserve a nil override so the next rebuild
	// can select a new generated cover when membership changes.
	cover := summary.CoverOverrideMediaItem
	if request.ClearCoverOverride {
		cover = nil
	} else if request.CoverMediaItemID != nil {
		if uuid.Validate(*request.CoverMediaItemID) != nil {
			api.WriteProblem(c, api.BadRequest(errors.New("invalid cover")))
			return
		}
		cover = request.CoverMediaItemID
	}
	hidden := summary.Hidden
	if request.IsHidden != nil {
		hidden = *request.IsHidden
	}
	result, err := h.writer.ExecContext(c, catalogtx.OperationEventPatch, `
UPDATE events SET title_override=?,cover_override_media_item_id=?,is_hidden=?,updated_at=?
WHERE event_id=? AND owner_id=? AND status='active'`,
		title, cover, hidden, apiNowMicros(), summary.EventID, ownerID)
	if err != nil {
		eventConflict(c, err, "Event update conflicts with membership")
		return
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		h.respondError(c, event.ErrNotFound)
		return
	}
	updated, err := h.service.Resolver().Resolve(c, ownerID, summary.EventID)
	if err != nil {
		h.respondError(c, err)
		return
	}
	api.JSONOK(c, dto.EventMutationResponseDTO{Event: eventSummaryDTO(updated), PendingRebuild: h.pending(c, ownerID)})
}

// RebuildEvents enqueues one owner-wide deterministic Event rebuild.
// @Summary Rebuild Events
// @Tags events
// @Accept json
// @Produce json
// @Param request body dto.EventRebuildRequestDTO true "Rebuild options"
// @Success 202 {object} dto.EventRebuildAcceptedDTO
// @Router /api/v1/events/rebuild [post]
func (h *EventHandler) RebuildEvents(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	var request dto.EventRebuildRequestDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	ownerID := int32(user.UserID)
	if request.OwnerID != nil && *request.OwnerID != ownerID {
		if !currentUserIsAdmin(c) {
			api.WriteProblem(c, api.Forbidden(errors.New("admin required")))
			return
		}
		ownerID = *request.OwnerID
	}
	now := apiNowMicros()
	tx, err := h.writer.BeginTx(c, catalogtx.OperationEventRebuildRequest, nil)
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(c, `
INSERT INTO event_owner_state(owner_id,active_algorithm_version,initialized_at,revision,source_revision,published_revision,updated_at)
VALUES(?,?,?,0,1,0,?) ON CONFLICT(owner_id) DO NOTHING`, ownerID, event.AlgorithmVersion, now, now); err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	var requestedRevision int64
	if err := tx.QueryRowContext(c, `SELECT source_revision FROM event_owner_state WHERE owner_id=?`, ownerID).Scan(&requestedRevision); err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	runID := ""
	lookupErr := tx.QueryRowContext(c, `
SELECT run_id FROM event_rebuild_runs
WHERE owner_id=? AND state IN ('queued','running')
	ORDER BY requested_at,run_id LIMIT 1`, ownerID).Scan(&runID)
	if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		api.WriteProblem(c, api.Internal(lookupErr))
		return
	}
	if runID == "" {
		runID = uuid.NewString()
		if _, err := tx.ExecContext(c, `
INSERT INTO event_rebuild_runs(run_id,owner_id,state,requested_revision,requested_at)
VALUES(?,?, 'queued',?,?)`, runID, ownerID, requestedRevision, now); err != nil {
			api.WriteProblem(c, api.Internal(err))
			return
		}
	}
	if err := pipeline.RequestEventProjectionTx(c, tx.Raw(), ownerID, uint64(requestedRevision), true, uuid.New()); err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	if err := tx.Commit(); err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	c.JSON(http.StatusAccepted, dto.EventRebuildAcceptedDTO{RunID: runID, OwnerID: ownerID, RequestedRevision: requestedRevision})
}

// GetRebuildStatus reports owner-wide Event source/published revisions and
// the latest observable rebuild runs.
// @Summary Get Event rebuild status
// @Tags events
// @Produce json
// @Success 200 {object} dto.EventRebuildStatusDTO
// @Router /api/v1/events/rebuild/status [get]
func (h *EventHandler) GetRebuildStatus(c *gin.Context) {
	ownerID, ok := eventOwner(c)
	if !ok {
		return
	}
	var response dto.EventRebuildStatusDTO
	var paused int
	err := h.reader.QueryRowContext(c, `
SELECT active_algorithm_version,automatic_rebuild_paused,revision,source_revision,published_revision,
 (SELECT count(*) FROM event_dirty_ranges WHERE owner_id=?)
FROM event_owner_state WHERE owner_id=?`, ownerID, ownerID).
		Scan(&response.AlgorithmVersion, &paused, &response.Revision, &response.SourceRevision,
			&response.PublishedRevision, &response.PendingRanges)
	if errors.Is(err, sql.ErrNoRows) {
		response.AlgorithmVersion = event.AlgorithmVersion
		api.JSONOK(c, response)
		return
	}
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	response.Initialized, response.Paused = true, paused != 0
	response.Pending = response.SourceRevision > response.PublishedRevision
	rows, rowsErr := h.reader.QueryContext(c, `
SELECT run_id,state,COALESCE(error_code,'') FROM event_rebuild_runs
WHERE owner_id=? ORDER BY requested_at DESC,run_id DESC LIMIT 20`, ownerID)
	if rowsErr != nil {
		api.WriteProblem(c, api.Internal(rowsErr))
		return
	}
	defer rows.Close()
	for rows.Next() {
		var runID, state, errorCode string
		if err := rows.Scan(&runID, &state, &errorCode); err != nil {
			api.WriteProblem(c, api.Internal(err))
			return
		}
		switch state {
		case "queued":
			if response.QueuedRunID == "" {
				response.QueuedRunID = runID
			}
		case "running":
			if response.RunningRunID == "" {
				response.RunningRunID = runID
			}
		case "succeeded":
			if response.LastSuccessRunID == "" {
				response.LastSuccessRunID = runID
			}
		case "failed":
			if response.LastFailureRunID == "" {
				response.LastFailureRunID, response.LastErrorCode = runID, errorCode
			}
		}
	}
	if err := rows.Err(); err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	api.JSONOK(c, response)
}

// SetRebuildState pauses or resumes automatic Event rebuilds.
// @Summary Set Event rebuild state
// @Tags events
// @Accept json
// @Produce json
// @Param request body dto.EventRebuildStateRequestDTO true "Rebuild state"
// @Success 200 {object} dto.EventRebuildStatusDTO
// @Router /api/v1/events/rebuild/state [patch]
func (h *EventHandler) SetRebuildState(c *gin.Context) {
	ownerID, ok := eventOwner(c)
	if !ok {
		return
	}
	var request dto.EventRebuildStateRequestDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	if request.OwnerID != nil && *request.OwnerID != ownerID {
		if !currentUserIsAdmin(c) {
			api.WriteProblem(c, api.Forbidden(errors.New("admin required")))
			return
		}
		ownerID = *request.OwnerID
	}
	now := apiNowMicros()
	tx, err := h.writer.BeginTx(c, catalogtx.OperationEventRebuildState, nil)
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(c, `
INSERT INTO event_owner_state(owner_id,active_algorithm_version,initialized_at,automatic_rebuild_paused,revision,source_revision,published_revision,updated_at)
VALUES(?,?,?, ?,0,0,0,?)
ON CONFLICT(owner_id) DO UPDATE SET automatic_rebuild_paused=excluded.automatic_rebuild_paused,
updated_at=excluded.updated_at`, ownerID, event.AlgorithmVersion, now, request.Paused, now); err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	if !request.Paused {
		var sourceRevision, publishedRevision int64
		if err := tx.QueryRowContext(c, `SELECT source_revision,published_revision FROM event_owner_state WHERE owner_id=?`, ownerID).
			Scan(&sourceRevision, &publishedRevision); err != nil {
			api.WriteProblem(c, api.Internal(err))
			return
		}
		if sourceRevision > publishedRevision {
			var queued int
			_ = tx.QueryRowContext(c, `SELECT count(*) FROM event_rebuild_runs WHERE owner_id=? AND state IN ('queued','running')`, ownerID).Scan(&queued)
			if queued == 0 {
				_, err = tx.ExecContext(c, `INSERT INTO event_rebuild_runs(run_id,owner_id,state,requested_revision,requested_at) VALUES(?,?, 'queued',?,?)`, uuid.NewString(), ownerID, sourceRevision, now)
				if err != nil {
					api.WriteProblem(c, api.Internal(err))
					return
				}
			}
			if err := pipeline.RequestEventProjectionTx(c, tx.Raw(), ownerID, uint64(sourceRevision), false, uuid.New()); err != nil {
				api.WriteProblem(c, api.Internal(err))
				return
			}
		}
	}
	if err := tx.Commit(); err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	h.GetRebuildStatus(c)
}

// ShareEvent creates an immutable displayable-asset snapshot.
// @Summary Share Event
// @Tags events
// @Accept json
// @Produce json
// @Param id path string true "Event UUID"
// @Param request body dto.EventShareRequestDTO true "Share options"
// @Success 200 {object} dto.CreateShareLinkResponseDTO
// @Router /api/v1/events/{id}/share [post]
func (h *EventHandler) ShareEvent(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	var request dto.EventShareRequestDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	ownerID := int32(user.UserID)
	tx, err := h.writer.BeginTx(c, catalogtx.OperationEventShare, &sql.TxOptions{ReadOnly: false})
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	defer tx.Rollback()
	summary, err := event.ResolveTx(c, tx.Raw(), ownerID, c.Param("id"))
	if err != nil {
		h.respondError(c, err)
		return
	}
	assets, _, err := event.OrderedAssetsTx(c, tx.Raw(), ownerID, summary.EventID, service.ShareLinkMaxAssets+1)
	if err != nil {
		h.respondError(c, err)
		return
	}
	if len(assets) > service.ShareLinkMaxAssets {
		eventConflict(c, service.ErrShareLinkTooLarge, "event_share_too_large")
		return
	}
	ids := make([]uuid.UUID, 0, len(assets))
	for _, asset := range assets {
		id, err := uuid.Parse(asset.AssetID)
		if err != nil {
			api.WriteProblem(c, api.Internal(err))
			return
		}
		ids = append(ids, id)
	}
	link, token, err := h.shares.CreateResolvedSnapshotTx(c, tx.Raw(), service.ShareLinkCreateParams{
		OwnerID: ownerID, OwnerScope: &ownerID, Title: request.Title,
		Description: request.Description, ExpiresInDays: request.ExpiresInDays,
		AllowDownload: request.AllowDownload, IncludeOriginals: request.IncludeOriginals,
	}, ids, "event:"+summary.EventID)
	if err != nil {
		if errors.Is(err, service.ErrShareLinkTooLarge) {
			eventConflict(c, err, "event_share_too_large")
		} else {
			api.WriteProblem(c, api.Internal(err))
		}
		return
	}
	if err := tx.Commit(); err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	api.JSONOK(c, dto.CreateShareLinkResponseDTO{ShareLinkDTO: dto.ToShareLinkDTO(link), Token: token})
}

// GetEventRelations returns direct, typed Event relations.
// @Summary Get Event relations
// @Tags events
// @Produce json
// @Param id path string true "Event UUID"
// @Success 200 {object} dto.EventRelationsResponseDTO
// @Router /api/v1/events/{id}/relations [get]
func (h *EventHandler) GetEventRelations(c *gin.Context) {
	ownerID, ok := eventOwner(c)
	if !ok {
		return
	}
	result, err := h.relations.ForEvent(c, ownerID, c.Param("id"))
	if err != nil {
		h.respondError(c, err)
		return
	}
	api.JSONOK(c, dto.EventRelationsResponseDTO{
		Relations: result.Relations, Complete: result.Complete,
		SourceVersion: result.SourceVersion,
	})
}

// GetPersonEvents returns typed Person-to-Event relations for the authenticated owner.
// @Summary Get a Person's Events
// @Tags people
// @Produce json
// @Param id path int true "Person ID"
// @Success 200 {object} dto.EventRelationsResponseDTO
// @Router /api/v1/people/{id}/events [get]
func (h *EventHandler) GetPersonEvents(c *gin.Context) {
	h.getPersonRelations(c, "appears_in")
}

// GetPersonRelations returns supported direct Person relations.
// @Summary Get Person relations
// @Tags people
// @Produce json
// @Param id path int true "Person ID"
// @Param relation query string true "Relation type" Enums(co_occurs_with)
// @Success 200 {object} dto.EventRelationsResponseDTO
// @Router /api/v1/people/{id}/relations [get]
func (h *EventHandler) GetPersonRelations(c *gin.Context) {
	if c.Query("relation") != "co_occurs_with" {
		api.WriteProblem(c, api.BadRequest(errors.New("unsupported relation")))
		return
	}
	h.getPersonRelations(c, "co_occurs_with")
}

func (h *EventHandler) getPersonRelations(c *gin.Context, relation string) {
	ownerID, ok := eventOwner(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil || id <= 0 {
		api.WriteProblem(c, api.BadRequest(errors.New("invalid Person ID")))
		return
	}
	result, err := h.relations.ForPerson(c, ownerID, int32(id))
	if err != nil {
		h.respondError(c, err)
		return
	}
	filtered := result.Relations[:0]
	for _, item := range result.Relations {
		if item.Relation == relation {
			filtered = append(filtered, item)
		}
	}
	api.JSONOK(c, dto.EventRelationsResponseDTO{
		Relations: filtered, Complete: result.Complete, SourceVersion: result.SourceVersion,
	})
}

// MergeEvents applies an explicit survivor and redirects the losing Events.
// @Summary Merge Events
// @Tags events
// @Accept json
// @Produce json
// @Param request body dto.EventMergeRequestDTO true "Merge request"
// @Success 200 {object} dto.EventMutationResponseDTO
// @Router /api/v1/events/merge [post]
func (h *EventHandler) MergeEvents(c *gin.Context) {
	ownerID, ok := eventOwner(c)
	if !ok {
		return
	}
	var request dto.EventMergeRequestDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	summary, err := h.service.Merge(c, ownerID, request.EventIDs, request.SurvivorEventID)
	if err != nil {
		h.respondError(c, err)
		return
	}
	api.JSONOK(c, dto.EventMutationResponseDTO{Event: eventSummaryDTO(summary), PendingRebuild: true})
}

// SplitEvent creates a user-confirmed boundary before one logical media item.
// @Summary Split Event
// @Tags events
// @Accept json
// @Produce json
// @Param id path string true "Event UUID"
// @Param request body dto.EventSplitRequestDTO true "Split request"
// @Success 200 {object} dto.EventMutationResponseDTO
// @Router /api/v1/events/{id}/split [post]
func (h *EventHandler) SplitEvent(c *gin.Context) {
	ownerID, ok := eventOwner(c)
	if !ok {
		return
	}
	var request dto.EventSplitRequestDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	summary, err := h.service.Split(c, ownerID, c.Param("id"), request.BeforeMediaItemID)
	if err != nil {
		h.respondError(c, err)
		return
	}
	api.JSONOK(c, dto.EventMutationResponseDTO{Event: eventSummaryDTO(summary), PendingRebuild: true})
}

// RemoveEventMember removes one logical media item and records an exclude pin.
// @Summary Remove Event member
// @Tags events
// @Produce json
// @Param id path string true "Event UUID"
// @Param mediaItemId path string true "Media item UUID"
// @Success 200 {object} dto.EventMutationResponseDTO
// @Router /api/v1/events/{id}/members/{mediaItemId} [delete]
func (h *EventHandler) RemoveEventMember(c *gin.Context) {
	ownerID, ok := eventOwner(c)
	if !ok {
		return
	}
	summary, err := h.service.RemoveMember(c, ownerID, c.Param("id"), c.Param("mediaItemId"))
	if err != nil {
		h.respondError(c, err)
		return
	}
	api.JSONOK(c, dto.EventMutationResponseDTO{Event: eventSummaryDTO(summary), PendingRebuild: true})
}

// AddEventMembers resolves physical asset IDs to logical media and pins them.
// @Summary Add Event members
// @Tags events
// @Accept json
// @Produce json
// @Param id path string true "Event UUID"
// @Param request body dto.EventAddMembersRequestDTO true "Assets to add"
// @Success 200 {object} dto.EventMutationResponseDTO
// @Router /api/v1/events/{id}/members [post]
func (h *EventHandler) AddEventMembers(c *gin.Context) {
	ownerID, ok := eventOwner(c)
	if !ok {
		return
	}
	var request dto.EventAddMembersRequestDTO
	if err := c.ShouldBindJSON(&request); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	summary, err := h.service.AddAssets(c, ownerID, c.Param("id"), request.AssetIDs)
	if err != nil {
		h.respondError(c, err)
		return
	}
	api.JSONOK(c, dto.EventMutationResponseDTO{Event: eventSummaryDTO(summary), PendingRebuild: true})
}

func (h *EventHandler) pending(c *gin.Context, ownerID int32) bool {
	var pending bool
	_ = h.reader.QueryRowContext(c, `
SELECT source_revision>published_revision FROM event_owner_state WHERE owner_id=?`, ownerID).Scan(&pending)
	return pending
}

func (h *EventHandler) respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, event.ErrNotFound):
		api.WriteProblem(c, api.NotFound(err))
	case errors.Is(err, event.ErrConstraintConflict), errors.Is(err, event.ErrWouldBeEmpty),
		errors.Is(err, event.ErrPaused), errors.Is(err, event.ErrStaleRevision):
		eventConflict(c, err, "Event conflict")
	default:
		api.WriteProblem(c, api.Internal(err))
	}
}

func eventOwner(c *gin.Context) (int32, bool) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return 0, false
	}
	return int32(user.UserID), true
}

func eventSummaryDTO(summary event.Summary) dto.EventSummaryDTO {
	return dto.EventSummaryDTO{
		EventID: summary.EventID, RedirectedFrom: summary.RedirectedFrom,
		StartAt: summary.StartAt, EndAt: summary.EndAt, Timezone: summary.Timezone,
		TitleOverride: summary.TitleOverride, CoverMediaItemID: summary.CoverMediaItem,
		CoverAssetID: summary.CoverAssetID,
		IsHidden:     summary.Hidden, MediaCount: summary.MediaCount,
		DisplayableCount: summary.DisplayableCount,
		CanonicalCount:   summary.CanonicalMediaCount, ProjectedCount: summary.ProjectedMediaCount,
	}
}

func apiNowMicros() int64 { return timeNow().UnixMicro() }

var timeNow = func() time.Time { return time.Now().UTC() }

func eventConflict(c *gin.Context, err error, message string) {
	api.WriteProblem(c, api.StatusProblem(http.StatusConflict, err))
}
