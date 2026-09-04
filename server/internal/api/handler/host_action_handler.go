package handler

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/api/problem"
	"server/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type HostActionHandler struct {
	manager             storage.RepositoryManager
	nativeHostAvailable bool
}

func NewHostActionHandler(manager storage.RepositoryManager, nativeHostAvailable bool) *HostActionHandler {
	return &HostActionHandler{manager: manager, nativeHostAvailable: nativeHostAvailable}
}

// GetNativeHostCapability reports whether a Desktop host can approve native filesystem actions.
// @Summary Get native host capability
// @Tags host-actions
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.NativeHostCapabilityDTO
// @Router /api/v1/host-actions/native-capability [get]
func (h *HostActionHandler) GetNativeHostCapability(c *gin.Context) {
	api.JSONOK(c, dto.NativeHostCapabilityDTO{Available: h != nil && h.nativeHostAvailable})
}

// CreateHostAction creates a short-lived request for the local Desktop host.
// @Summary Request a native host storage action
// @Description Creates a persistent, expiring task. Filesystem paths and approval nonces never enter this HTTP request or response.
// @Tags host-actions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Idempotency-Key header string false "Stable request identifier"
// @Param request body dto.CreateHostActionRequestDTO true "Native host action"
// @Success 200 {object} dto.HostActionDTO
// @Failure 400 {object} api.ProblemResponse
// @Failure 409 {object} api.ProblemResponse
// @Router /api/v1/host-actions [post]
func (h *HostActionHandler) CreateHostAction(c *gin.Context) {
	if h == nil || h.manager == nil {
		api.WriteProblem(c, api.Internal(errors.New("repository manager unavailable")))
		return
	}
	if !h.nativeHostAvailable {
		api.WriteProblem(c, api.StatusProblem(http.StatusConflict, errors.New("native_host_unavailable")))
		return
	}
	var req dto.CreateHostActionRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	requestID := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if requestID == "" {
		requestID = uuid.NewString()
	}
	c.Header("Idempotency-Key", requestID)
	actorUserID := adminIDFromContext(c)
	actor := "web:admin"
	if actorUserID != nil {
		actor = fmt.Sprintf("web:user:%d", *actorUserID)
	}
	ttl := time.Duration(req.ExpiresInSeconds) * time.Second
	action, err := h.manager.CreateHostAction(c.Request.Context(), storage.CreateHostActionInput{
		RequestID:   requestID,
		Kind:        storage.HostActionKind(req.Kind),
		Actor:       actor,
		ActorUserID: actorUserID,
		SessionID:   strings.TrimSpace(req.SessionID),
		Summary: storage.HostActionSummary{
			Name: strings.TrimSpace(req.Name), Purpose: strings.TrimSpace(req.Purpose),
			RootID: strings.TrimSpace(req.RootID), RepositoryID: strings.TrimSpace(req.RepositoryID),
		},
		ExpectedVersion: req.ExpectedVersion,
		TTL:             ttl,
	})
	if err != nil {
		if errors.Is(err, storage.ErrHostActionConflict) {
			api.WriteProblem(c, api.StatusProblem(http.StatusConflict, err))
		} else {
			api.WriteProblem(c, api.BadRequest(err))
		}
		return
	}
	api.JSONOK(c, toHostActionDTO(action))
}

// GetHostAction returns the durable state of one native host action.
// @Summary Get native host action
// @Tags host-actions
// @Produce json
// @Security BearerAuth
// @Param id path string true "Host action ID"
// @Success 200 {object} dto.HostActionDTO
// @Failure 404 {object} api.ProblemResponse
// @Router /api/v1/host-actions/{id} [get]
func (h *HostActionHandler) GetHostAction(c *gin.Context) {
	action, err := h.ownedHostAction(c)
	if err != nil {
		writeHostActionOwnershipError(c, err)
		return
	}
	api.JSONOK(c, toHostActionDTO(action))
}

// ListHostActions returns unfinished native-host tasks owned by the current administrator.
// @Summary List unfinished native host actions
// @Tags host-actions
// @Produce json
// @Security BearerAuth
// @Success 200 {array} dto.HostActionDTO
// @Router /api/v1/host-actions [get]
func (h *HostActionHandler) ListHostActions(c *gin.Context) {
	actorUserID := adminIDFromContext(c)
	if actorUserID == nil {
		api.WriteProblem(c, api.StatusProblem(http.StatusForbidden, errors.New("administrator identity unavailable")))
		return
	}
	actions, err := h.manager.ListHostActionsForActor(c.Request.Context(), *actorUserID)
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	items := make([]dto.HostActionDTO, 0, len(actions))
	for _, action := range actions {
		items = append(items, toHostActionDTO(action))
	}
	c.JSON(http.StatusOK, items)
}

// ResolveHostAction applies an explicit recovery decision after a native selection exposed an identity conflict.
// @Summary Resolve native host action conflict
// @Tags host-actions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Host action ID"
// @Param request body dto.ResolveHostActionRequestDTO true "Recovery decision"
// @Success 200 {object} dto.HostActionDTO
// @Failure 400 {object} api.ProblemResponse
// @Failure 409 {object} api.ProblemResponse
// @Router /api/v1/host-actions/{id}/resolve [post]
func (h *HostActionHandler) ResolveHostAction(c *gin.Context) {
	var req dto.ResolveHostActionRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	if _, err := h.ownedHostAction(c); err != nil {
		writeHostActionOwnershipError(c, err)
		return
	}
	action, err := h.manager.ResolveHostAction(c.Request.Context(), c.Param("id"), req.Resolution, req.RiskConfirmation)
	if err != nil {
		if errors.Is(err, storage.ErrHostActionDecisionNeeded) || errors.Is(err, storage.ErrRepositoryOriginalOnline) {
			api.WriteProblem(c, api.StatusProblem(http.StatusConflict, err))
		} else {
			api.WriteProblem(c, api.BadRequest(err))
		}
		return
	}
	api.JSONOK(c, toHostActionDTO(action))
}

// CancelHostAction cancels a pending native host request.
// @Summary Cancel native host action
// @Tags host-actions
// @Produce json
// @Security BearerAuth
// @Param id path string true "Host action ID"
// @Success 200 {object} dto.HostActionDTO
// @Failure 409 {object} api.ProblemResponse
// @Router /api/v1/host-actions/{id} [delete]
func (h *HostActionHandler) CancelHostAction(c *gin.Context) {
	if _, err := h.ownedHostAction(c); err != nil {
		writeHostActionOwnershipError(c, err)
		return
	}
	action, err := h.manager.CancelHostAction(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, storage.ErrHostActionNotPending) {
			api.WriteProblem(c, api.StatusProblem(http.StatusConflict, err))
		} else if errors.Is(err, sql.ErrNoRows) {
			api.WriteProblem(c, api.NotFound(err))
		} else {
			api.WriteProblem(c, api.BadRequest(err))
		}
		return
	}
	api.JSONOK(c, toHostActionDTO(action))
}

var errHostActionNotOwned = errors.New("native host action is owned by another administrator")

func (h *HostActionHandler) ownedHostAction(c *gin.Context) (storage.HostAction, error) {
	action, err := h.manager.GetHostAction(c.Request.Context(), c.Param("id"))
	if err != nil {
		return storage.HostAction{}, err
	}
	actorUserID := adminIDFromContext(c)
	if actorUserID == nil || action.ActorUserID == nil || *action.ActorUserID != *actorUserID {
		return storage.HostAction{}, errHostActionNotOwned
	}
	return action, nil
}

func writeHostActionOwnershipError(c *gin.Context, err error) {
	if errors.Is(err, errHostActionNotOwned) {
		api.WriteProblem(c, api.StatusProblem(http.StatusForbidden, err))
	} else if errors.Is(err, sql.ErrNoRows) {
		api.WriteProblem(c, api.NotFound(err))
	} else {
		api.WriteProblem(c, api.BadRequest(err))
	}
}

func toHostActionDTO(action storage.HostAction) dto.HostActionDTO {
	result := (*dto.HostActionResultDTO)(nil)
	if action.Result != nil {
		result = &dto.HostActionResultDTO{
			RepositoryID: action.Result.RepositoryID,
			RootID:       action.Result.RootID,
			Name:         action.Result.Name,
		}
		if action.Result.Conflict != nil {
			conflict := action.Result.Conflict
			result.Conflict = &dto.HostActionConflictDTO{
				Type:               conflict.Type,
				RepositoryID:       conflict.RepositoryID,
				RootID:             conflict.RootID,
				AllowedResolutions: hostActionResolutions(conflict.Actions),
				RiskWarnings:       conflict.RiskWarnings,
			}
		}
	}
	var operationProblem *problem.Reference
	switch action.Status {
	case storage.HostActionExpired:
		value := problem.ReferenceFor(problem.HostActionExpired, action.ActionID, true)
		operationProblem = &value
	case storage.HostActionFailed:
		value := problem.ReferenceFor(problem.HostActionFailed, action.ActionID, true)
		operationProblem = &value
	}
	return dto.HostActionDTO{
		ID: action.ActionID, RequestID: action.RequestID, Kind: string(action.Kind), Actor: action.Actor,
		Purpose: action.Summary.Purpose, Name: action.Summary.Name, ExpectedVersion: action.ExpectedVersion,
		RootID: action.Summary.RootID, RepositoryID: action.Summary.RepositoryID,
		Status: string(action.Status), Result: result, Problem: operationProblem,
		ExpiresAt: action.ExpiresAt, CreatedAt: action.CreatedAt, UpdatedAt: action.UpdatedAt, CompletedAt: action.CompletedAt,
	}
}

func hostActionResolutions(actions []string) []string {
	resolutions := make([]string, 0, len(actions))
	for _, action := range actions {
		switch action {
		case "relocate":
			resolutions = append(resolutions, "update_location")
		case "copy":
			resolutions = append(resolutions, "add_separate")
		case "confirm_risk":
			resolutions = append(resolutions, "confirm_risk")
		}
	}
	return resolutions
}
