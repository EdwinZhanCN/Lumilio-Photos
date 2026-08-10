package handler

import (
	"net/http/httptest"
	"testing"

	"server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestApplyAssetOwnershipScope_AdminEventQueryUsesCurrentOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("current_user", &service.UserResponse{UserID: 42, Role: "admin"})
	eventID := "event-1"

	params := applyAssetOwnershipScope(ctx, service.QueryAssetsParams{EventID: &eventID})

	require.NotNil(t, params.OwnerID)
	require.Equal(t, int32(42), *params.OwnerID)
}

func TestApplyAssetOwnershipScope_AdminLibraryQueryRemainsGlobal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("current_user", &service.UserResponse{UserID: 42, Role: "admin"})

	params := applyAssetOwnershipScope(ctx, service.QueryAssetsParams{})

	require.Nil(t, params.OwnerID)
}
