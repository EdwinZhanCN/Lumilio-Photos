package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/api/problem"
	"server/internal/storage/recovery"
)

// StorageRecoveryService is the private-recovery inventory the admin
// Storage page reads and acts on.
type StorageRecoveryService interface {
	Inventory(ctx context.Context, repositoryID uuid.UUID) (recovery.Inventory, error)
	OpenItem(ctx context.Context, repositoryID uuid.UUID, itemID string) (io.ReadCloser, recovery.Item, error)
	DeleteItem(ctx context.Context, repositoryID uuid.UUID, itemID string, request recovery.Request) (recovery.Item, error)
}

// SetRecovery installs the private-recovery service once it exists.
func (h *StorageHandler) SetRecovery(service StorageRecoveryService) { h.recovery = service }

// GetRepositoryRecovery returns the private-recovery inventory of one Repository.
// @Summary Get repository recovery inventory
// @Description Return the recovery receipts of a Repository, every preserved private file that awaits or had a decision, and the inherited Trash still held out of automatic expiry. Paths are relative to the Repository's private workspace.
// @Tags storage
// @Produce json
// @Security BearerAuth
// @Param id path string true "Repository UUID"
// @Success 200 {object} dto.RepositoryRecoveryDTO
// @Failure 400 {object} api.ProblemResponse
// @Failure 401 {object} api.ProblemResponse
// @Failure 403 {object} api.ProblemResponse
// @Failure 404 {object} api.ProblemResponse
// @Failure 503 {object} api.ProblemResponse "Repository unavailable"
// @Router /api/v1/storage/repositories/{id}/recovery [get]
func (h *StorageHandler) GetRepositoryRecovery(c *gin.Context) {
	repositoryID, ok := h.recoveryRepository(c)
	if !ok {
		return
	}
	inventory, err := h.recovery.Inventory(c.Request.Context(), repositoryID)
	if err != nil {
		writeRecoveryFailure(c, err)
		return
	}
	api.JSONOK(c, toRepositoryRecoveryDTO(inventory))
}

// ExportRecoveryItem streams the preserved bytes of one recovery item.
// @Summary Export a recovery item
// @Description Download the bytes of one preserved private file (uncommitted staging, an unknown file, or an unproven Studio record) as an attachment. The item is unchanged.
// @Tags storage
// @Produce application/octet-stream
// @Security BearerAuth
// @Param id path string true "Repository UUID"
// @Param item path string true "Recovery item UUID"
// @Success 200 {file} binary
// @Failure 400 {object} api.ProblemResponse
// @Failure 401 {object} api.ProblemResponse
// @Failure 403 {object} api.ProblemResponse
// @Failure 404 {object} api.ProblemResponse
// @Failure 503 {object} api.ProblemResponse "Repository unavailable"
// @Router /api/v1/storage/repositories/{id}/recovery/items/{item}/content [get]
func (h *StorageHandler) ExportRecoveryItem(c *gin.Context) {
	repositoryID, ok := h.recoveryRepository(c)
	if !ok {
		return
	}
	itemID, ok := recoveryItemID(c)
	if !ok {
		return
	}
	reader, item, err := h.recovery.OpenItem(c.Request.Context(), repositoryID, itemID)
	if err != nil {
		writeRecoveryFailure(c, err)
		return
	}
	defer reader.Close()
	filename := path.Base(item.Path)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "no-store")
	if item.Size > 0 {
		c.Header("Content-Length", strconv.FormatInt(item.Size, 10))
	}
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, reader)
}

// DeleteRecoveryItem removes one preserved file explicitly.
// @Summary Delete a recovery item
// @Description Remove one preserved private file from disk and record the decision in the Repository's recovery receipt. Reassociated Studio records are live state and cannot be deleted here.
// @Tags storage
// @Produce json
// @Security BearerAuth
// @Param id path string true "Repository UUID"
// @Param item path string true "Recovery item UUID"
// @Success 200 {object} dto.RepositoryRecoveryItemDTO
// @Failure 400 {object} api.ProblemResponse
// @Failure 401 {object} api.ProblemResponse
// @Failure 403 {object} api.ProblemResponse
// @Failure 404 {object} api.ProblemResponse
// @Failure 409 {object} api.ProblemResponse "The item is not deletable from the inventory"
// @Failure 503 {object} api.ProblemResponse "Repository unavailable"
// @Router /api/v1/storage/repositories/{id}/recovery/items/{item} [delete]
func (h *StorageHandler) DeleteRecoveryItem(c *gin.Context) {
	repositoryID, ok := h.recoveryRepository(c)
	if !ok {
		return
	}
	itemID, ok := recoveryItemID(c)
	if !ok {
		return
	}
	request := recovery.Request{Actor: "web:admin", RequestID: strings.TrimSpace(c.GetHeader("Idempotency-Key"))}
	if actorID := adminIDFromContext(c); actorID != nil {
		request.ActorUserID = actorID
		request.Actor = fmt.Sprintf("web:user:%d", *actorID)
	}
	item, err := h.recovery.DeleteItem(c.Request.Context(), repositoryID, itemID, request)
	if err != nil {
		writeRecoveryFailure(c, err)
		return
	}
	api.JSONOK(c, toRepositoryRecoveryItemDTO(item))
}

func (h *StorageHandler) recoveryRepository(c *gin.Context) (uuid.UUID, bool) {
	if h == nil || h.recovery == nil {
		api.WriteProblem(c, api.Internal(errors.New("private recovery is unavailable")))
		return uuid.Nil, false
	}
	repositoryID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		api.WriteProblem(c, api.BadRequest(fmt.Errorf("invalid repository id: %w", err)))
		return uuid.Nil, false
	}
	return repositoryID, true
}

func recoveryItemID(c *gin.Context) (string, bool) {
	itemID := strings.TrimSpace(c.Param("item"))
	if _, err := uuid.Parse(itemID); err != nil {
		api.WriteProblem(c, api.BadRequest(fmt.Errorf("invalid recovery item id: %w", err)))
		return "", false
	}
	return itemID, true
}

func writeRecoveryFailure(c *gin.Context, err error) {
	switch {
	case errors.Is(err, recovery.ErrRepositoryOffline):
		api.WriteProblem(c, api.KnownProblem(problem.RepositoryUnavailable, err))
	case errors.Is(err, recovery.ErrItemNotFound):
		api.WriteProblem(c, api.NotFound(err))
	case errors.Is(err, recovery.ErrItemNotDeletable):
		api.WriteProblem(c, api.StatusProblem(http.StatusConflict, err))
	default:
		api.WriteProblem(c, api.Internal(err))
	}
}

func toRepositoryRecoveryDTO(inventory recovery.Inventory) dto.RepositoryRecoveryDTO {
	out := dto.RepositoryRecoveryDTO{
		RepositoryID:       inventory.RepositoryID.String(),
		Receipts:           make([]dto.RepositoryRecoveryReceiptDTO, 0, len(inventory.Receipts)),
		Items:              make([]dto.RepositoryRecoveryItemDTO, 0, len(inventory.Items)),
		Trash:              dto.RepositoryRecoveryTrashDTO{HeldCount: inventory.TrashHeld.Count, HeldBytes: inventory.TrashHeld.Bytes},
		UnreadableReceipts: append([]string{}, inventory.UnreadableReceipts...),
	}
	for _, receipt := range inventory.Receipts {
		out.Receipts = append(out.Receipts, dto.RepositoryRecoveryReceiptDTO{
			ID: receipt.ReceiptID, Event: string(receipt.Event), RecordedAt: receipt.RecordedAt, Actor: receipt.Actor,
			SourceRepositoryID: receipt.SourceRepositoryID,
			TrashFiles:         receipt.Trash.Files, TrashBytes: receipt.Trash.Bytes, LogFiles: receipt.Logs.Files, DerivedFiles: receipt.Derived.Files,
			ItemsTruncated: receipt.ItemsTruncated,
		})
	}
	for _, item := range inventory.Items {
		out.Items = append(out.Items, toRepositoryRecoveryItemDTO(item))
	}
	return out
}

func toRepositoryRecoveryItemDTO(item recovery.Item) dto.RepositoryRecoveryItemDTO {
	actions := item.Actions
	if actions == nil {
		actions = []string{}
	}
	return dto.RepositoryRecoveryItemDTO{
		ID: item.ItemID, ReceiptID: item.ReceiptID, Class: string(item.Class), Path: item.Path, Size: item.Size,
		Disposition: string(item.Disposition), Present: item.Present, Actions: actions,
		AssetID: item.AssetID, ResolvedAssetID: item.ResolvedAssetID, ResolvedAt: item.ResolvedAt, Detail: item.Detail,
	}
}
