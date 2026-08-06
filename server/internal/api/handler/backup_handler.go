package handler

import (
	"log"
	"net/http"
	"os"
	"strings"

	"server/internal/api"
	"server/internal/api/dto"

	"github.com/gin-gonic/gin"
)

// Database-backup admin endpoints, part of the SettingsHandler surface
// (Settings → Server tab). Every filename from the client goes through
// BackupService.ResolvePath, which accepts only names matching the backup
// filename grammar — path traversal is rejected by construction.

// ListBackups lists the snapshots in the backups directory, newest first.
// @Summary List database backups
// @Description List SQLite snapshots (routine backups and restore points), newest first.
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.BackupListDTO "Backups listed successfully"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/settings/backups [get]
func (h *SettingsHandler) ListBackups(c *gin.Context) {
	entries, err := h.backupService.List(c.Request.Context())
	if err != nil {
		api.GinInternalError(c, err, "Failed to list backups")
		return
	}
	api.JSONOK(c, dto.ToBackupListDTO(entries))
}

// CreateBackup enqueues an immediate SQLite snapshot.
// @Summary Create a database backup now
// @Description Enqueue an immediate SQLite snapshot; it appears in the list when the job finishes.
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Success 202 {object} api.SuccessResponse "Backup enqueued"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/settings/backups [post]
func (h *SettingsHandler) CreateBackup(c *gin.Context) {
	if err := h.backupService.TriggerNow(c.Request.Context()); err != nil {
		api.GinInternalError(c, err, "Failed to enqueue backup")
		return
	}
	c.JSON(http.StatusAccepted, api.SuccessResponse{Message: "backup enqueued"})
}

// DownloadBackup streams a SQLite snapshot file to the client.
// @Summary Download a database backup
// @Description Download one standalone SQLite snapshot.
// @Tags settings
// @Produce application/octet-stream
// @Security BearerAuth
// @Param name path string true "Backup file name"
// @Success 200 {file} file "Backup file"
// @Failure 400 {object} api.ErrorResponse "Invalid backup name"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 404 {object} api.ErrorResponse "Backup not found"
// @Router /api/v1/settings/backups/{name}/download [get]
func (h *SettingsHandler) DownloadBackup(c *gin.Context) {
	name := c.Param("name")
	path, err := h.backupService.ResolvePath(name)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid backup name")
		return
	}
	c.FileAttachment(path, name)
}

// DeleteBackup removes one SQLite snapshot and its manifest.
// @Summary Delete a database backup
// @Description Delete one SQLite snapshot and its manifest from the backups directory.
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Param name path string true "Backup file name"
// @Success 200 {object} api.SuccessResponse "Backup deleted"
// @Failure 400 {object} api.ErrorResponse "Invalid backup name"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/settings/backups/{name} [delete]
func (h *SettingsHandler) DeleteBackup(c *gin.Context) {
	name := c.Param("name")
	if err := h.backupService.Delete(c.Request.Context(), name); err != nil {
		if strings.Contains(err.Error(), "invalid backup name") {
			api.GinBadRequest(c, err, "Invalid backup name")
			return
		}
		api.GinInternalError(c, err, "Failed to delete backup")
		return
	}
	api.JSONOK(c, api.SuccessResponse{Message: "backup deleted"})
}

// RestoreBackup accepts a restore operation and flushes its durable receipt
// before the current runtime generation begins draining.
// @Summary Restore a database backup
// @Description Validate and stage the named SQLite snapshot. The accepted operation continues across a runtime restart; poll the returned operation ID for completion or rollback.
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Param name path string true "Backup file name"
// @Success 202 {object} dto.RestoreOperationDTO "Restore accepted"
// @Failure 400 {object} api.ErrorResponse "Invalid backup name"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 409 {object} api.ErrorResponse "Another restore is already in progress"
// @Failure 500 {object} api.ErrorResponse "Restore could not be staged"
// @Router /api/v1/settings/backups/{name}/restore [post]
func (h *SettingsHandler) RestoreBackup(c *gin.Context) {
	name := c.Param("name")
	operation, err := h.backupService.Restore(c.Request.Context(), name)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "invalid backup name"):
			api.GinBadRequest(c, err, "Invalid backup name")
		case strings.Contains(err.Error(), "already in progress"):
			api.GinError(c, http.StatusConflict, err, http.StatusConflict, "Another restore is already in progress")
		default:
			api.GinInternalError(c, err, "Restore could not be staged")
		}
		return
	}

	c.JSON(http.StatusAccepted, dto.ToRestoreOperationDTO(operation))
	c.Writer.Flush()
	if err := h.backupService.RestartRestore(operation.ID); err != nil {
		// The operation receipt is already on the wire. Preserve the real error in
		// server logs; the durable operation endpoint remains the user-facing source.
		log.Printf("failed to restart into restore operation %s: %v", operation.ID, err)
	}
}

// GetRestoreOperation returns the durable status of one accepted restore.
// @Summary Get database restore operation
// @Description Return the latest durable phase for an accepted restore operation, including completion or successful rollback after a runtime restart.
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Param id path string true "Restore operation ID"
// @Success 200 {object} dto.RestoreOperationDTO "Restore operation"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 404 {object} api.ErrorResponse "Restore operation not found"
// @Failure 500 {object} api.ErrorResponse "Restore operation could not be read"
// @Router /api/v1/settings/backup-restores/{id} [get]
func (h *SettingsHandler) GetRestoreOperation(c *gin.Context) {
	operation, err := h.backupService.GetRestoreOperation(c.Request.Context(), c.Param("id"))
	if err != nil {
		if os.IsNotExist(err) {
			api.GinNotFound(c, err, "Restore operation not found")
			return
		}
		api.GinInternalError(c, err, "Failed to read restore operation")
		return
	}
	api.JSONOK(c, dto.ToRestoreOperationDTO(operation))
}

// GetLatestRestoreOperation lets a reloaded browser resume observing the most
// recently accepted restore without relying on in-memory UI state.
// @Summary Get latest database restore operation
// @Description Return the latest durable restore receipt, if one exists.
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.RestoreOperationDTO "Latest restore operation"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 404 {object} api.ErrorResponse "No restore operation exists"
// @Failure 500 {object} api.ErrorResponse "Restore operation could not be read"
// @Router /api/v1/settings/backup-restores/latest [get]
func (h *SettingsHandler) GetLatestRestoreOperation(c *gin.Context) {
	operation, err := h.backupService.LatestRestoreOperation(c.Request.Context())
	if err != nil {
		if os.IsNotExist(err) {
			api.GinNotFound(c, err, "Restore operation not found")
			return
		}
		api.GinInternalError(c, err, "Failed to read restore operation")
		return
	}
	api.JSONOK(c, dto.ToRestoreOperationDTO(operation))
}
