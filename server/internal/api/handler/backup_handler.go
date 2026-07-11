package handler

import (
	"net/http"
	"strings"

	"server/internal/api"
	"server/internal/api/dto"

	"github.com/gin-gonic/gin"
)

// Database-backup admin endpoints, part of the SettingsHandler surface
// (Settings → Server tab). Every filename from the client goes through
// BackupService.ResolvePath, which accepts only names matching the backup
// filename grammar — path traversal is rejected by construction.

// ListBackups lists the dumps in the backups directory, newest first.
// @Summary List database backups
// @Description List database dumps (routine backups and restore points), newest first.
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

// CreateBackup enqueues an immediate database dump.
// @Summary Create a database backup now
// @Description Enqueue an immediate database dump; it appears in the list when the job finishes.
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

// DownloadBackup streams a dump file to the client.
// @Summary Download a database backup
// @Description Download one dump file as gzip.
// @Tags settings
// @Produce application/gzip
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

// DeleteBackup removes one dump file.
// @Summary Delete a database backup
// @Description Delete one dump file from the backups directory.
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

// RestoreBackup synchronously restores a dump with restore-point + rollback.
// @Summary Restore a database backup
// @Description Restore the named dump. A restore point of the current database is taken first; on failure the database is rolled back automatically. Synchronous — the response arrives when the restore has finished.
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Param name path string true "Backup file name"
// @Success 200 {object} api.SuccessResponse "Backup restored"
// @Failure 400 {object} api.ErrorResponse "Invalid backup name"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 409 {object} api.ErrorResponse "Another restore is already in progress"
// @Failure 500 {object} api.ErrorResponse "Restore failed (database rolled back)"
// @Router /api/v1/settings/backups/{name}/restore [post]
func (h *SettingsHandler) RestoreBackup(c *gin.Context) {
	name := c.Param("name")
	if err := h.backupService.Restore(c.Request.Context(), name); err != nil {
		switch {
		case strings.Contains(err.Error(), "invalid backup name"):
			api.GinBadRequest(c, err, "Invalid backup name")
		case strings.Contains(err.Error(), "already in progress"):
			api.GinError(c, http.StatusConflict, err, http.StatusConflict, "Another restore is already in progress")
		default:
			api.GinInternalError(c, err, "Restore failed")
		}
		return
	}
	api.JSONOK(c, api.SuccessResponse{Message: "backup restored"})
}
