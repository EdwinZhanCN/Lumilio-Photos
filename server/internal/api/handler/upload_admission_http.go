package handler

import (
	"errors"
	"net/http"

	"server/internal/api"
	"server/internal/db/repo"
	"server/internal/storage"

	"github.com/gin-gonic/gin"
)

func (h *AssetHandler) guardUploadWriteCapacity(c *gin.Context, repository repo.Repository, size uint64) bool {
	decision, err := h.repoManager.CheckRepositoryWriteCapacity(c.Request.Context(), repository.RepoID.String(), size)
	if admErr := storage.CheckUploadAdmission(repository, storage.WriteFactsFromCapacity(decision)); admErr != nil {
		h.respondRepositoryError(c, admErr)
		return false
	}
	if err != nil {
		h.respondCapacityError(c, err)
		return false
	}
	return true
}

// writeUploadAdmissionError maps shared upload admission failures to 409
// responses. It returns true when err was handled.
func writeUploadAdmissionError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, storage.ErrRepositoryOffline),
		errors.Is(err, storage.ErrRepositoryBusy),
		errors.Is(err, storage.ErrRepositoryIdentityError),
		errors.Is(err, storage.ErrRepositoryRecoveryRequired),
		errors.Is(err, storage.ErrRepositoryUploadPaused),
		errors.Is(err, storage.ErrRepositoryUploadLowSpace),
		errors.Is(err, storage.ErrRepositoryReadOnly):
		api.WriteProblem(c, api.StatusProblem(http.StatusConflict, err))
		return true
	default:
		return false
	}
}
