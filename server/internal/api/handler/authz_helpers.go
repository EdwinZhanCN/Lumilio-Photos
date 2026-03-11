package handler

import (
	"errors"
	"fmt"

	"server/internal/api"
	"server/internal/service"

	"github.com/gin-gonic/gin"
)

func currentUserFromContext(c *gin.Context) (*service.UserResponse, bool) {
	value, exists := c.Get("current_user")
	if !exists {
		return nil, false
	}

	switch user := value.(type) {
	case *service.UserResponse:
		return user, user != nil
	case service.UserResponse:
		return &user, true
	default:
		return nil, false
	}
}

func currentUserIDFromContext(c *gin.Context) (*int32, error) {
	user, ok := currentUserFromContext(c)
	if ok {
		converted := int32(user.UserID)
		return &converted, nil
	}

	value, exists := c.Get("user_id")
	if !exists {
		return nil, errors.New("user id not found in context")
	}

	switch userID := value.(type) {
	case int:
		converted := int32(userID)
		return &converted, nil
	case int32:
		converted := userID
		return &converted, nil
	case int64:
		converted := int32(userID)
		return &converted, nil
	default:
		return nil, fmt.Errorf("unexpected user id type %T", value)
	}
}

func currentUserIsAdmin(c *gin.Context) bool {
	user, ok := currentUserFromContext(c)
	return ok && service.IsAdminRole(user.Role)
}

func requireCurrentUser(c *gin.Context) (*service.UserResponse, bool) {
	user, ok := currentUserFromContext(c)
	if !ok {
		api.GinUnauthorized(c, errors.New("user not found in context"), "Unauthorized")
		return nil, false
	}
	return user, true
}

func requireAdminUser(c *gin.Context) (*service.UserResponse, bool) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return nil, false
	}
	if !service.IsAdminRole(user.Role) {
		api.GinForbidden(c, errors.New("admin access required"), "Admin access required")
		return nil, false
	}
	return user, true
}

func ensureOwnerAccess(c *gin.Context, ownerID *int32, unauthorizedMessage, forbiddenMessage string) bool {
	if ownerID == nil {
		return true
	}

	user, ok := currentUserFromContext(c)
	if !ok {
		api.GinUnauthorized(c, errors.New("authentication required"), unauthorizedMessage)
		return false
	}

	if service.IsAdminRole(user.Role) || int32(user.UserID) == *ownerID {
		return true
	}

	api.GinForbidden(c, errors.New("access denied"), forbiddenMessage)
	return false
}
