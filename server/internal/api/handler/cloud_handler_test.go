package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"server/internal/cloud"
	"server/internal/db/repo"
	"server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type cloudAccessServiceStub struct {
	cloud.CloudSyncService
	listAccess       cloud.CredentialAccess
	disconnectAccess cloud.CredentialAccess
	disconnectErr    error
}

func (s *cloudAccessServiceStub) ListCredentials(_ context.Context, access cloud.CredentialAccess) ([]repo.CloudCredential, error) {
	s.listAccess = access
	return []repo.CloudCredential{{OwnerID: access.UserID, Provider: "icloud", DisplayName: "Mine"}}, nil
}

func (s *cloudAccessServiceStub) DisconnectCredential(_ context.Context, _ uuid.UUID, access cloud.CredentialAccess) error {
	s.disconnectAccess = access
	return s.disconnectErr
}

func (s *cloudAccessServiceStub) ProviderTitle(cloud.ProviderKind) string {
	return "iCloud"
}

func TestListCloudCredentialsUsesOwnerScopeForRegularUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	serviceStub := &cloudAccessServiceStub{}
	handler := NewCloudHandler(serviceStub)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/cloud/credentials", nil)
	ctx.Set("current_user", &service.UserResponse{UserID: 7, Role: "user"})

	handler.ListCredentials(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if serviceStub.listAccess.UserID != 7 || serviceStub.listAccess.IsAdmin {
		t.Fatalf("access = %+v, want regular user 7", serviceStub.listAccess)
	}
}

func TestListCloudCredentialsAllowsAdministratorScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	serviceStub := &cloudAccessServiceStub{}
	handler := NewCloudHandler(serviceStub)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/cloud/credentials", nil)
	ctx.Set("current_user", &service.UserResponse{UserID: 1, Role: "admin"})

	handler.ListCredentials(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if serviceStub.listAccess.UserID != 1 || !serviceStub.listAccess.IsAdmin {
		t.Fatalf("access = %+v, want administrator 1", serviceStub.listAccess)
	}
}

func TestCloudCredentialIDOperationRejectsDifferentOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	serviceStub := &cloudAccessServiceStub{disconnectErr: cloud.ErrCredentialAccessDenied}
	handler := NewCloudHandler(serviceStub)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/cloud/credentials/550e8400-e29b-41d4-a716-446655440000/disconnect", nil)
	ctx.Params = gin.Params{{Key: "id", Value: "550e8400-e29b-41d4-a716-446655440000"}}
	ctx.Set("current_user", &service.UserResponse{UserID: 8, Role: "user"})

	handler.DisconnectCredential(ctx)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if serviceStub.disconnectAccess.UserID != 8 || serviceStub.disconnectAccess.IsAdmin {
		t.Fatalf("access = %+v, want regular user 8", serviceStub.disconnectAccess)
	}
}
