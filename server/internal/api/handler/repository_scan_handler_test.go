package handler

import (
	"context"
	"encoding/json"
	"errors"
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

func TestStorageSupportBundlePathRedactionIsStableAndIrreversibleInOutput(t *testing.T) {
	path := "/Volumes/Private Family Disk/Photos"
	redacted := redactSupportPath(path)
	if redacted == path || strings.Contains(redacted, "Private") || !strings.HasPrefix(redacted, "<redacted:") {
		t.Fatalf("redacted path = %q", redacted)
	}
	if again := redactSupportPath(path); again != redacted {
		t.Fatalf("redaction changed: %q != %q", again, redacted)
	}
}

func TestStorageDiagnosticDTOCarriesRepositoryParentLocation(t *testing.T) {
	item := storageDiagnosticDTO(
		"repository",
		"repository-id",
		"storage-location-id",
		"Family Archive",
		t.TempDir(),
		"active",
		"repository-id",
		storage.StoragePathInfo{},
		false,
	)

	if item.ParentTargetID != "storage-location-id" {
		t.Fatalf("parent target id = %q, want storage-location-id", item.ParentTargetID)
	}
}

func TestStorageSupportBundleRedactsPathsNestedInAuditDetails(t *testing.T) {
	details := json.RawMessage(`{"error":"open /Volumes/Private Family Disk/Photos/image.jpg: permission denied","nested":{"path":"C:\\Users\\Alice\\Pictures"},"safe":"offline"}`)
	redacted := redactSupportDetails(details, true)
	text := string(redacted)
	for _, secret := range []string{"Private Family Disk", "Alice", "/Volumes/", `C:\\Users`} {
		if strings.Contains(text, secret) {
			t.Fatalf("support details leaked %q: %s", secret, text)
		}
	}
	if !strings.Contains(text, `"safe":"offline"`) || !strings.Contains(text, "redacted:") {
		t.Fatalf("support details lost safe context or redaction marker: %s", text)
	}
}

func TestStorageSupportBundleRedactsAdversarialEmbeddedPathForms(t *testing.T) {
	cases := []struct {
		name   string
		value  string
		secret string
	}{
		{name: "parenthesized POSIX", value: "open(/Volumes/Family Photos/private.jpg): denied", secret: "Family Photos"},
		{name: "file URI", value: "source=file:///Users/Alice/Pictures/private.jpg, retrying", secret: "Alice"},
		{name: "network URI", value: "source=smb://nas01/family-share/private.jpg", secret: "family-share"},
		{name: "UNC", value: `source=(\\\\nas01\\family-share\\private.jpg); denied`, secret: "family-share"},
		{name: "Windows drive", value: `path=C:\\Users\\Alice\\Pictures\\private.jpg; denied`, secret: "Alice"},
		{name: "punctuation", value: `failed:[/srv/private-family/image.jpg], denied`, secret: "private-family"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			redacted, ok := redactSupportValue(test.value).(string)
			if !ok {
				t.Fatalf("redacted type = %T", redactSupportValue(test.value))
			}
			if strings.Contains(redacted, test.secret) || !strings.HasPrefix(redacted, "<redacted:") {
				t.Fatalf("redacted value = %q", redacted)
			}
		})
	}
}

type createRepositoryManagerStub struct {
	storage.RepositoryManager
	hostOwnerID  *int32
	createdSpec  storage.CreateRepositorySpec
	createCalled bool
	createErr    error
}

func (s *createRepositoryManagerStub) HostOwnerID(context.Context) (*int32, error) {
	return s.hostOwnerID, nil
}

func (s *createRepositoryManagerStub) CreateRepository(_ context.Context, spec storage.CreateRepositorySpec) (*storage.CreateRepositoryResult, error) {
	s.createCalled = true
	s.createdSpec = spec
	if s.createErr != nil {
		return nil, s.createErr
	}
	return &storage.CreateRepositoryResult{
		Repository: &repo.Repository{
			RepoID:         uuid.MustParse("7e32cc57-bfe0-42b2-943b-d43e0510e0bd"),
			Name:           spec.Name,
			Role:           dbtypes.RepoRoleRegular,
			Reachability:   dbtypes.RepositoryReachabilityActive,
			Activity:       dbtypes.RepositoryActivityIdle,
			DefaultOwnerID: spec.OwnerID,
		},
	}, nil
}

func TestCreateRepositoryUsesHostOwnerNotActingAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hostOwnerID := int32(1)
	manager := &createRepositoryManagerStub{hostOwnerID: &hostOwnerID}
	handler := NewRepositoryScanHandler(nil, manager)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/repositories", strings.NewReader(`{"name":"Family Media","directory_name":"family-media"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("current_user", &service.UserResponse{UserID: 99, Role: "admin"})

	handler.CreateRepository(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if manager.createdSpec.OwnerID == nil || *manager.createdSpec.OwnerID != hostOwnerID {
		t.Fatalf("repository owner = %v, want Host Owner %d", manager.createdSpec.OwnerID, hostOwnerID)
	}
	if manager.createdSpec.Name != "Family Media" {
		t.Fatalf("repository name = %q, want exact input", manager.createdSpec.Name)
	}
	if manager.createdSpec.DirectoryName != "family-media" {
		t.Fatalf("repository storage folder = %q, want family-media", manager.createdSpec.DirectoryName)
	}
	if manager.createdSpec.ActorUserID == nil || *manager.createdSpec.ActorUserID != 99 || manager.createdSpec.HostInstanceID == "" {
		t.Fatalf("repository audit context = actor %v host %q", manager.createdSpec.ActorUserID, manager.createdSpec.HostInstanceID)
	}
}

func TestCreateRepositoryRejectsInvalidNameBeforeManagerCall(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hostOwnerID := int32(1)
	manager := &createRepositoryManagerStub{hostOwnerID: &hostOwnerID}
	handler := NewRepositoryScanHandler(nil, manager)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/repositories", strings.NewReader(`{"name":" Family","directory_name":"family"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("current_user", &service.UserResponse{UserID: 99, Role: "admin"})

	handler.CreateRepository(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if manager.createCalled {
		t.Fatal("repository manager was called for an invalid name")
	}
}

func TestCreateRepositoryRejectsMissingRegularStorageFolder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hostOwnerID := int32(1)
	manager := &createRepositoryManagerStub{hostOwnerID: &hostOwnerID}
	handler := NewRepositoryScanHandler(nil, manager)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/repositories", strings.NewReader(`{"name":"Family Media"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("current_user", &service.UserResponse{UserID: 99, Role: "admin"})

	handler.CreateRepository(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if manager.createCalled {
		t.Fatal("repository manager was called without a storage folder")
	}
}

func TestCreateRepositoryReturnsExistingMarkerAsStructuredRecoveryFact(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hostOwnerID := int32(1)
	manager := &createRepositoryManagerStub{
		hostOwnerID: &hostOwnerID,
		createErr: &storage.ExistingRepositoryFoundError{
			RepositoryID:  "7e32cc57-bfe0-42b2-943b-d43e0510e0bd",
			RequestedPath: "/storage/family-media",
		},
	}
	handler := NewRepositoryScanHandler(nil, manager)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/repositories", strings.NewReader(`{"name":"Family Media","directory_name":"family-media"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("current_user", &service.UserResponse{UserID: 99, Role: "admin"})

	handler.CreateRepository(ctx)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response dto.RepositoryConflictDTO
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ConflictType != "existing_repository_found" || len(response.Actions) != 1 || response.Actions[0] != "open" {
		t.Fatalf("structured recovery response = %+v", response)
	}
}

func TestCreateRepositoryReturnsInvalidMarkerAsStructuredRecoveryFact(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hostOwnerID := int32(1)
	manager := &createRepositoryManagerStub{
		hostOwnerID: &hostOwnerID,
		createErr: &storage.RepositoryMarkerInvalidError{
			RequestedPath: "/storage/family-media",
			Cause:         errors.New("invalid marker"),
		},
	}
	handler := NewRepositoryScanHandler(nil, manager)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/repositories", strings.NewReader(`{"name":"Family Media","directory_name":"family-media"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("current_user", &service.UserResponse{UserID: 99, Role: "admin"})

	handler.CreateRepository(ctx)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response dto.RepositoryConflictDTO
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ConflictType != "repository_marker_invalid" || len(response.Actions) != 1 || response.Actions[0] != "diagnose" {
		t.Fatalf("structured recovery response = %+v", response)
	}
}

func TestCreateRepositoryReturnsRegisteredIdentityAsMoveOrCopyConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hostOwnerID := int32(1)
	manager := &createRepositoryManagerStub{
		hostOwnerID: &hostOwnerID,
		createErr: &storage.RepositoryConflictError{
			RepositoryID:   "7e32cc57-bfe0-42b2-943b-d43e0510e0bd",
			RegisteredPath: "/storage/registered", RequestedPath: "/storage/family-media",
			Actions: []string{"relocate", "copy"},
		},
	}
	handler := NewRepositoryScanHandler(nil, manager)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/repositories", strings.NewReader(`{"name":"Family Media","directory_name":"family-media"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("current_user", &service.UserResponse{UserID: 99, Role: "admin"})

	handler.CreateRepository(ctx)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response dto.RepositoryConflictDTO
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.ConflictType != "repository_identity" || len(response.Actions) != 2 {
		t.Fatalf("registered identity conflict = %+v", response)
	}
}
