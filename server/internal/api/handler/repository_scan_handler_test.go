package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"server/internal/api/problem"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage"
	roecontroller "server/internal/storage/roe/controller"

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
		storage.StoragePathInfo{CapacityKnown: true, TotalBytes: 100 << 30, AvailableBytes: 20 << 30},
		false,
	)

	if item.ParentTargetID != "storage-location-id" {
		t.Fatalf("parent target id = %q, want storage-location-id", item.ParentTargetID)
	}
	if item.SafetyMarginBytes != 5<<30 || item.WritableBudgetBytes != 15<<30 {
		t.Fatalf("capacity explanation = margin %d budget %d", item.SafetyMarginBytes, item.WritableBudgetBytes)
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

type storageDiagnosticsManagerStub struct {
	storage.RepositoryManager
	roots        []repo.RepositoryRoot
	repositories []*repo.Repository
}

type repositoryScanServiceStub struct {
	cancelled    repo.RepositoryScanRun
	cancelErr    error
	repositoryID string
	operationID  string
}

func (*repositoryScanServiceStub) EnqueueManualScan(context.Context, string, string, bool) (roecontroller.Receipt, error) {
	return roecontroller.Receipt{}, nil
}

func (*repositoryScanServiceStub) GetScanRun(context.Context, string, string) (repo.RepositoryScanRun, error) {
	return repo.RepositoryScanRun{}, nil
}

func (*repositoryScanServiceStub) GetLatestScanRun(context.Context, string) (repo.RepositoryScanRun, error) {
	return repo.RepositoryScanRun{}, nil
}

func (*repositoryScanServiceStub) ListScanRuns(context.Context, string, int32, int32) ([]repo.RepositoryScanRun, error) {
	return nil, nil
}

func (stub *repositoryScanServiceStub) CancelScanRun(_ context.Context, repositoryID, operationID string) (repo.RepositoryScanRun, error) {
	stub.repositoryID = repositoryID
	stub.operationID = operationID
	return stub.cancelled, stub.cancelErr
}

func TestCancelRepositoryScanReturnsDurableCancellationState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repositoryID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	operationID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	stub := &repositoryScanServiceStub{cancelled: repo.RepositoryScanRun{
		RunID: operationID, RepositoryID: repositoryID, RequestedEpoch: 2,
		Mode: "manual", Status: roecontroller.StatusCrawling,
		CancellationRequested: 1, CreatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	}}
	handler := NewRepositoryScanHandler(stub, nil)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/repositories/"+repositoryID.String()+"/scans/"+operationID.String()+"/cancel", nil)
	ctx.Params = gin.Params{
		{Key: "id", Value: repositoryID.String()},
		{Key: "operation_id", Value: operationID.String()},
	}

	handler.CancelRepositoryScan(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if stub.repositoryID != repositoryID.String() || stub.operationID != operationID.String() {
		t.Fatalf("cancel scope = %q/%q", stub.repositoryID, stub.operationID)
	}
	var response struct {
		OperationID           string `json:"operation_id"`
		CancellationRequested bool   `json:"cancellation_requested"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.OperationID != operationID.String() || !response.CancellationRequested {
		t.Fatalf("cancel response = %+v", response)
	}
}

func (s *storageDiagnosticsManagerStub) ListRepositoryRoots(context.Context) ([]repo.RepositoryRoot, error) {
	return s.roots, nil
}

func (s *storageDiagnosticsManagerStub) ListRepositories() ([]*repo.Repository, error) {
	return s.repositories, nil
}

func TestStorageDiagnosticsCarriesStorageEntitySemantics(t *testing.T) {
	rootID := uuid.MustParse("8df0b4a5-5c67-44d9-80d0-ea4119ae26f9")
	repositoryID := uuid.MustParse("6fd24928-9c5c-4b03-a8cc-84971654144c")
	manager := &storageDiagnosticsManagerStub{
		roots: []repo.RepositoryRoot{{
			RootID: rootID,
			Name:   "legacy default name",
			Path:   t.TempDir(),
			Kind:   dbtypes.RepositoryRootKindDefault,
			Status: dbtypes.RepositoryRootStatusActive,
		}},
		repositories: []*repo.Repository{{
			RepoID:       repositoryID,
			RootID:       rootID,
			Name:         "legacy primary name",
			Path:         t.TempDir(),
			Role:         dbtypes.RepoRolePrimary,
			Reachability: dbtypes.RepositoryReachabilityActive,
		}},
	}

	items, err := NewRepositoryScanHandler(nil, manager).storageDiagnostics(context.Background(), false)
	if err != nil {
		t.Fatalf("storage diagnostics: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("diagnostic count = %d, want 2", len(items))
	}
	if items[0].Kind != string(dbtypes.RepositoryRootKindDefault) || items[0].Role != "" {
		t.Fatalf("Storage Location semantics = kind %q role %q", items[0].Kind, items[0].Role)
	}
	if items[1].Kind != "" || items[1].Role != string(dbtypes.RepoRolePrimary) {
		t.Fatalf("Repository semantics = kind %q role %q", items[1].Kind, items[1].Role)
	}
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
	var response problem.RepositoryConflictDetails
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ConflictType != "existing_repository_found" || len(response.Actions) != 1 || response.Actions[0] != "open" {
		t.Fatalf("structured recovery response = %+v", response)
	}
	if strings.Contains(recorder.Body.String(), manager.createErr.Error()) || strings.Contains(recorder.Body.String(), "/storage/") {
		t.Fatalf("public Problem exposed a host path or raw cause: %s", recorder.Body.String())
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
	var response problem.RepositoryConflictDetails
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
	var response problem.RepositoryConflictDetails
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.ConflictType != "repository_identity" || len(response.Actions) != 2 {
		t.Fatalf("registered identity conflict = %+v", response)
	}
}
