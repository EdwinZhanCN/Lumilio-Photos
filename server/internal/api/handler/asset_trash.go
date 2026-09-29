package handler

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"server/internal/api"
	"server/internal/api/problem"
	"server/internal/storage/trash"
)

// trashRequestFrom names the signed-in user as the actor of a Delete or
// Restore; an Idempotency-Key header becomes its journal request ID.
func trashRequestFrom(c *gin.Context, assetIDs ...uuid.UUID) trash.Request {
	request := trash.Request{AssetIDs: assetIDs, Actor: "web:user", RequestID: strings.TrimSpace(c.GetHeader("Idempotency-Key"))}
	if user, ok := currentUserFromContext(c); ok {
		id := int32(user.UserID)
		request.ActorUserID = &id
		request.Actor = fmt.Sprintf("web:user:%d", id)
	}
	return request
}

// writeTrashProblem reports a Delete or Restore failure. A rejected request
// moved nothing and is a repository conflict whose conflict_type names the
// reason: repository_offline, file_changed, asset_missing, not_trashed,
// trash_file_missing, or move_failed.
func writeTrashProblem(c *gin.Context, err error) {
	var rejection *trash.Rejection
	if errors.As(err, &rejection) {
		repositoryID := ""
		if rejection.RepositoryID != uuid.Nil {
			repositoryID = rejection.RepositoryID.String()
		}
		actions := []string{}
		switch rejection.Reason {
		case trash.ReasonRepositoryOffline:
			actions = []string{"reconnect"}
		case trash.ReasonFileChanged:
			actions = []string{"rescan"}
		}
		api.WriteProblem(c, problem.NewRepositoryConflict(err, rejection.Reason, repositoryID, actions))
		return
	}
	log.Printf("asset trash operation failed: %v", err)
	api.WriteProblem(c, api.Internal(err))
}
