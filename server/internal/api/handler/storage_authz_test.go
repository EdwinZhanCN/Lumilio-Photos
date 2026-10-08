package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"server/internal/api/dto"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type storageAuthzRepositoryManager struct {
	storage.RepositoryManager
	repositories []*repo.Repository
}

func (stub storageAuthzRepositoryManager) ListRepositories() ([]*repo.Repository, error) {
	return stub.repositories, nil
}

func TestStorageTargetsJSONOmitsSensitiveFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repositoryID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	storageLocationID := uuid.MustParse("660e8400-e29b-41d4-a716-446655440001")
	handler := NewStorageHandler(storageAuthzRepositoryManager{
		repositories: []*repo.Repository{{
			RepoID:            repositoryID,
			Name:              "Family Photos",
			Path:              "/secret/path/primary",
			Role:              dbtypes.RepoRolePrimary,
			StorageLocationID: storageLocationID,
			Reachability:      dbtypes.RepositoryReachabilityActive,
			Activity:          dbtypes.RepositoryActivityIdle,
		}},
	}, nil, nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/storage/targets", nil)
	ctx.Set("current_user", &service.UserResponse{UserID: 2, Role: "user"})

	handler.GetStorageTargets(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	body := recorder.Body.String()
	for _, forbidden := range []string{
		`"path"`, `"location_id"`, `"storage_location_id"`, `"capacity"`, `"verification"`, `"is_primary"`, "/secret/",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response leaked %q: %s", forbidden, body)
		}
	}

	var response dto.StorageTargetsResponseDTO
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Targets) != 1 || response.Targets[0].Role != "primary" || !response.Targets[0].Read.Allowed {
		t.Fatalf("unexpected target payload: %+v", response.Targets)
	}
	if response.Targets[0].Read.Reasons == nil || response.Targets[0].Upload.Reasons == nil {
		t.Fatalf("unrestricted reasons must be an empty array, got read=%#v upload=%#v",
			response.Targets[0].Read.Reasons, response.Targets[0].Upload.Reasons)
	}
}

func TestNonAdminForbiddenOnStorageViewAndDetach(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repositoryID := uuid.NewString()

	authStub := &authMiddlewareStub{
		user: &service.UserResponse{UserID: 2, Role: "user"},
	}
	router := gin.New()
	router.Use(authStub.AuthMiddleware(), authStub.RequireAdmin())
	router.GET("/api/v1/storage/view", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.POST("/api/v1/storage/repositories/:id/detach", func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, route := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/storage/view"},
		{http.MethodPost, "/api/v1/storage/repositories/" + repositoryID + "/detach"},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(route.method, route.path, nil)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("%s %s status = %d, want 403", route.method, route.path, recorder.Code)
		}
	}
}

type authMiddlewareStub struct {
	user *service.UserResponse
}

func (stub *authMiddlewareStub) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if stub.user != nil {
			c.Set("current_user", stub.user)
		}
		c.Next()
	}
}

func (stub *authMiddlewareStub) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := c.Get("current_user")
		if !ok {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		current, ok := user.(*service.UserResponse)
		if !ok || current.Role != "admin" {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}
