package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"server/internal/cloud"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type createRepositoryManagerStub struct {
	storage.RepositoryManager
	hostOwnerID *int32
	createdSpec storage.CreateRepositorySpec
}

func (s *createRepositoryManagerStub) HostOwnerID(context.Context) (*int32, error) {
	return s.hostOwnerID, nil
}

func (s *createRepositoryManagerStub) CreateRepository(_ context.Context, spec storage.CreateRepositorySpec) (*storage.CreateRepositoryResult, error) {
	s.createdSpec = spec
	return &storage.CreateRepositoryResult{
		Repository: &repo.Repository{
			RepoID:         pgtype.UUID{Bytes: uuid.MustParse("7e32cc57-bfe0-42b2-943b-d43e0510e0bd"), Valid: true},
			Name:           spec.Name,
			Role:           dbtypes.RepoRoleRegular,
			Status:         dbtypes.RepoStatusActive,
			DefaultOwnerID: spec.OwnerID,
		},
	}, nil
}

type cloudSyncServiceStub struct {
	cloud.CloudSyncService
	bindInput cloud.BindRepositoryCredentialInput
}

func (s *cloudSyncServiceStub) BindRepositoryCredentialAndStartImport(_ context.Context, input cloud.BindRepositoryCredentialInput) (uuid.UUID, error) {
	s.bindInput = input
	return uuid.MustParse("aa8df1d6-92d5-4921-a253-0d66b60f945b"), nil
}

func TestCreateRepositoryUsesHostOwnerNotActingAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hostOwnerID := int32(1)
	manager := &createRepositoryManagerStub{hostOwnerID: &hostOwnerID}
	handler := NewRepositoryScanHandler(nil, manager, nil)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/repositories", strings.NewReader(`{"name":"Archive"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("current_user", &service.UserResponse{UserID: 99, Role: "admin"})

	handler.CreateRepository(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if manager.createdSpec.OwnerID == nil || *manager.createdSpec.OwnerID != hostOwnerID {
		t.Fatalf("repository owner = %v, want Host Owner %d", manager.createdSpec.OwnerID, hostOwnerID)
	}
}

func TestCreateCloudRepositoryUsesActingAdminCredentialAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hostOwnerID := int32(1)
	actorUserID := 99
	actorOwnerID := int32(actorUserID)
	manager := &createRepositoryManagerStub{hostOwnerID: &hostOwnerID}
	cloudService := &cloudSyncServiceStub{}
	handler := NewRepositoryScanHandler(nil, manager, cloudService)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/repositories", strings.NewReader(`{"name":"Cloud Archive","cloud_credential_id":"9e71fa01-7881-462c-970b-d750af832314"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("current_user", &service.UserResponse{UserID: actorUserID, Role: "admin"})

	handler.CreateRepository(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if manager.createdSpec.OwnerID == nil || *manager.createdSpec.OwnerID != hostOwnerID {
		t.Fatalf("repository owner = %v, want Host Owner %d", manager.createdSpec.OwnerID, hostOwnerID)
	}
	if cloudService.bindInput.Access.UserID != actorOwnerID || !cloudService.bindInput.Access.IsAdmin {
		t.Fatalf("cloud credential access = %+v, want admin %d", cloudService.bindInput.Access, actorOwnerID)
	}
}
