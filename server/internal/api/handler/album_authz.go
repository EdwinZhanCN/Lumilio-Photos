package handler

import (
	"database/sql"
	"errors"

	"server/internal/api"
	"server/internal/db/repo"

	"github.com/gin-gonic/gin"
)

func (h *AlbumHandler) getAuthorizedAlbum(c *gin.Context, albumID int32, unauthorizedMessage, forbiddenMessage string) (*repo.Album, bool) {
	album, err := h.queries.GetAlbumByID(c.Request.Context(), albumID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			api.GinNotFound(c, err, "Album not found")
			return nil, false
		}
		api.GinInternalError(c, err, "Failed to access album")
		return nil, false
	}

	if !ensureOwnerAccess(c, &album.UserID, unauthorizedMessage, forbiddenMessage) {
		return nil, false
	}

	return &album, true
}
