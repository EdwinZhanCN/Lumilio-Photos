package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"server/internal/api/dto"
	"server/internal/api/problem"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage"
	"server/internal/storage/scan"

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
	hostOwnerID     *int32
	createdSpec     storage.CreateRepositorySpec
	createCalled    bool
	createErr       error
	assessment      *storage.CreateTargetAssessment
	assessErr       error
	assessedRequest storage.AssessCreateTargetRequest
}

type storageDiagnosticsManagerStub struct {
	storage.RepositoryManager
	storageLocations []repo.StorageLocation
	repositories     []*repo.Repository
}

type repositoryScanServiceStub struct {
	cancelled    repo.RepositoryScan
	cancelErr    error
	repositoryID string
	operationID  string
}

func (*repositoryScanServiceStub) RequestScan(context.Context, string, string, string) (scan.Receipt, error) {
	return scan.Receipt{}, nil
}

func (*repositoryScanServiceStub) GetScan(context.Context, string, string) (repo.RepositoryScan, error) {
	return repo.RepositoryScan{}, nil
}

func (*repositoryScanServiceStub) GetLatestScan(context.Context, string) (repo.RepositoryScan, error) {
	return repo.RepositoryScan{}, nil
}

func (*repositoryScanServiceStub) ListScans(context.Context, string, int32, int32) ([]repo.RepositoryScan, error) {
	return nil, nil
}

func (stub *repositoryScanServiceStub) CancelScan(_ context.Context, repositoryID, operationID string) (repo.RepositoryScan, error) {
	stub.repositoryID = repositoryID
	stub.operationID = operationID
	return stub.cancelled, stub.cancelErr
}

func TestCancelRepositoryScanReturnsDurableCancellationState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repositoryID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	operationID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	stub := &repositoryScanServiceStub{cancelled: repo.RepositoryScan{
		ScanID: operationID, RepositoryID: repositoryID,
		Trigger: scan.TriggerManual, Status: scan.StatusWalking,
		CancellationRequested: 1, CreatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	}}
	handler := NewRepositoryScanHandler(stub, nil)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/storage/repositories/"+repositoryID.String()+"/verifications/"+operationID.String()+"/cancel", nil)
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

func (s *storageDiagnosticsManagerStub) ListStorageLocations(context.Context) ([]repo.StorageLocation, error) {
	return s.storageLocations, nil
}

func (s *storageDiagnosticsManagerStub) ListRepositories() ([]*repo.Repository, error) {
	return s.repositories, nil
}

func TestStorageDiagnosticsCarriesStorageEntitySemantics(t *testing.T) {
	storageLocationID := uuid.MustParse("8df0b4a5-5c67-44d9-80d0-ea4119ae26f9")
	repositoryID := uuid.MustParse("6fd24928-9c5c-4b03-a8cc-84971654144c")
	manager := &storageDiagnosticsManagerStub{
		storageLocations: []repo.StorageLocation{{
			StorageLocationID: storageLocationID,
			Name:              "legacy default name",
			Path:              t.TempDir(),
			Kind:              dbtypes.StorageLocationKindDefault,
			Status:            dbtypes.StorageLocationStatusActive,
		}},
		repositories: []*repo.Repository{{
			RepoID:            repositoryID,
			StorageLocationID: storageLocationID,
			Name:              "legacy primary name",
			Path:              t.TempDir(),
			Role:              dbtypes.RepoRolePrimary,
			Reachability:      dbtypes.RepositoryReachabilityActive,
		}},
	}

	items, err := NewRepositoryScanHandler(nil, manager).storageDiagnostics(context.Background(), false)
	if err != nil {
		t.Fatalf("storage diagnostics: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("diagnostic count = %d, want 2", len(items))
	}
	if items[0].Kind != string(dbtypes.StorageLocationKindDefault) || items[0].Role != "" {
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
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/storage/repositories", strings.NewReader(`{"name":"Family Media","directory_name":"family-media"}`))
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
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/storage/repositories", strings.NewReader(`{"name":" Family","directory_name":"family"}`))
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
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/storage/repositories", strings.NewReader(`{"name":"Family Media"}`))
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
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/storage/repositories", strings.NewReader(`{"name":"Family Media","directory_name":"family-media"}`))
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
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/storage/repositories", strings.NewReader(`{"name":"Family Media","directory_name":"family-media"}`))
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
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/storage/repositories", strings.NewReader(`{"name":"Family Media","directory_name":"family-media"}`))
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

func (s *createRepositoryManagerStub) AssessCreateTarget(_ context.Context, request storage.AssessCreateTargetRequest) (*storage.CreateTargetAssessment, error) {
	s.assessedRequest = request
	if s.assessErr != nil {
		return nil, s.assessErr
	}
	if s.assessment != nil {
		return s.assessment, nil
	}
	return &storage.CreateTargetAssessment{
		Intent: request.Intent, DirectoryName: request.DirectoryName, Path: "/Volumes/Backup/" + request.DirectoryName,
		StorageLocationID: uuid.MustParse("6b767057-d6f1-465c-b816-d1229f622a20"), StorageLocationKind: dbtypes.StorageLocationKindExternal,
		Risks: []storage.StorageRisk{storage.RiskRemovable, storage.RiskNonDefault}, Token: "v1.review",
	}, nil
}

func postStorageJSON(t *testing.T, handle func(*gin.Context), path, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("current_user", &service.UserResponse{UserID: 99, Role: "admin"})
	handle(ctx)
	return recorder
}

func TestCreateRepositoryReturnsTheConsentProblemWithRisksAndToken(t *testing.T) {
	hostOwnerID := int32(1)
	manager := &createRepositoryManagerStub{
		hostOwnerID: &hostOwnerID,
		createErr:   &storage.StorageRiskConfirmationError{Risks: []storage.StorageRisk{storage.RiskRemovable, storage.RiskNonDefault}, Token: "v1.review"},
	}
	handler := NewRepositoryScanHandler(nil, manager)

	recorder := postStorageJSON(t, handler.CreateRepository, "/api/v1/storage/repositories",
		`{"name":"Archive","directory_name":"archive","storage_location_id":"6b767057-d6f1-465c-b816-d1229f622a20","assessment_token":"v1.old","acknowledged_risks":["removable_storage"]}`)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response problem.StorageConfirmationRequiredDetails
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Type != problem.StorageConfirmationRequired.Type || response.AssessmentToken != "v1.review" ||
		len(response.Risks) != 2 || response.Risks[0] != "removable_storage" || response.Risks[1] != "non_default_location" {
		t.Fatalf("consent Problem = %+v", response)
	}
	if manager.createdSpec.AssessmentToken != "v1.old" || len(manager.createdSpec.AcknowledgedRisks) != 1 {
		t.Fatalf("spec did not carry the acknowledgement: %+v", manager.createdSpec)
	}
	if strings.Contains(recorder.Body.String(), "/Volumes") {
		t.Fatalf("consent Problem exposed a host path: %s", recorder.Body.String())
	}
}

func TestCreateCommandsMapTypedStorageRefusals(t *testing.T) {
	hostOwnerID := int32(1)
	for _, tc := range []struct {
		err  error
		want problem.Descriptor
	}{
		{storage.ErrStorageAssessmentStale, problem.StorageAssessmentStale},
		{storage.ErrStorageUnsupportedFilesystem, problem.StorageUnsupportedFilesystem},
		{storage.ErrRepositoryExistingTargetNotMountPoint, problem.StorageNotMountedVolume},
		{storage.ErrStoragePermissionDenied, problem.StoragePermissionDenied},
		{storage.ErrStorageMaterializationRequired, problem.StorageMaterializationRequired},
		{storage.ErrStorageObservationUnknown, problem.StorageObservationUnknown},
	} {
		for name, handle := range map[string]func(*createRepositoryManagerStub) (func(*gin.Context), string, string){
			"create": func(m *createRepositoryManagerStub) (func(*gin.Context), string, string) {
				return NewRepositoryScanHandler(nil, m).CreateRepository, "/api/v1/storage/repositories", `{"name":"Archive","directory_name":"archive"}`
			},
			"setup": func(m *createRepositoryManagerStub) (func(*gin.Context), string, string) {
				return NewRepositoryScanHandler(nil, m).CreatePrimaryRepository, "/api/v1/setup/primary-repository", `{"name":"Primary"}`
			},
		} {
			manager := &createRepositoryManagerStub{hostOwnerID: &hostOwnerID, createErr: tc.err}
			handleFunc, path, body := handle(manager)
			recorder := postStorageJSON(t, handleFunc, path, body)
			var response problem.Details
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("%s %v: decode: %v", name, tc.err, err)
			}
			if recorder.Code != tc.want.Status || response.Type != tc.want.Type {
				t.Fatalf("%s %v: status %d type %s, want %d %s", name, tc.err, recorder.Code, response.Type, tc.want.Status, tc.want.Type)
			}
		}
	}
}

func TestAssessStorageTargetReturnsTheReviewWithoutAHostPath(t *testing.T) {
	hostOwnerID := int32(1)
	manager := &createRepositoryManagerStub{hostOwnerID: &hostOwnerID}
	handler := NewRepositoryScanHandler(nil, manager)

	recorder := postStorageJSON(t, handler.AssessStorageTarget, "/api/v1/storage/assessments",
		`{"intent":"create_repository","storage_location_id":"6b767057-d6f1-465c-b816-d1229f622a20","directory_name":"archive"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["token"] != "v1.review" || response["intent"] != "create_repository" || response["directory_name"] != "archive" ||
		response["storage_location_kind"] != "external" || response["support"] != "" {
		t.Fatalf("assessment body = %s", recorder.Body.String())
	}
	if risks, _ := response["risks"].([]any); len(risks) != 2 {
		t.Fatalf("assessment risks = %v", response["risks"])
	}
	if _, exposed := response["path"]; exposed || strings.Contains(recorder.Body.String(), "/Volumes") {
		t.Fatalf("assessment exposed the resolved path: %s", recorder.Body.String())
	}
	if manager.assessedRequest.Intent != storage.IntentCreateRepository || manager.assessedRequest.DirectoryName != "archive" {
		t.Fatalf("assessment request = %+v", manager.assessedRequest)
	}

	bad := postStorageJSON(t, handler.AssessStorageTarget, "/api/v1/storage/assessments", `{"intent":"relocate"}`)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("unknown intent status = %d", bad.Code)
	}
	manager.assessErr = storage.ErrRepositoryDirectoryConflict
	conflict := postStorageJSON(t, handler.AssessStorageTarget, "/api/v1/storage/assessments", `{"intent":"create_repository","directory_name":"Archive"}`)
	var conflictBody problem.RepositoryConflictDetails
	if err := json.Unmarshal(conflict.Body.Bytes(), &conflictBody); err != nil || conflict.Code != http.StatusConflict || conflictBody.ConflictType != "repository_directory_conflict" {
		t.Fatalf("directory conflict review = %d %s", conflict.Code, conflict.Body.String())
	}
}

// repositoryCommandManagerStub answers the rename and detach commands with
// the typed refusals the storage policy produces.
type repositoryCommandManagerStub struct {
	storage.RepositoryManager
	repository *repo.Repository
	renameErr  error
	removeErr  error
	receipt    storage.LifecycleReceipt
	removed    []string
}

func (s *repositoryCommandManagerStub) GetRepository(string) (*repo.Repository, error) {
	if s.repository == nil {
		return nil, sql.ErrNoRows
	}
	return s.repository, nil
}

func (s *repositoryCommandManagerStub) RenameRepository(_ context.Context, _, name string, _ ...storage.LifecycleRequest) (*storage.RepositoryRenameResult, error) {
	if s.renameErr != nil {
		return nil, s.renameErr
	}
	renamed := *s.repository
	renamed.Name = name
	return &storage.RepositoryRenameResult{Repository: renamed, Receipt: s.receipt}, nil
}

func (s *repositoryCommandManagerStub) RemoveRepository(_ context.Context, id string, _ ...storage.LifecycleRequest) (storage.LifecycleReceipt, error) {
	s.removed = append(s.removed, id)
	if s.removeErr != nil {
		return storage.LifecycleReceipt{}, s.removeErr
	}
	return s.receipt, nil
}

func TestRenameRepositoryReturnsTheActionUnavailableProblemWithBlockers(t *testing.T) {
	manager := &repositoryCommandManagerStub{
		repository: &repo.Repository{RepoID: uuid.New(), Name: "Primary", Role: dbtypes.RepoRolePrimary},
		renameErr: &storage.RepositoryActionUnavailableError{
			Action: storage.ActionRename, Blockers: []storage.RepositoryActionBlocker{storage.BlockerPrimaryProtected},
			Alternatives: []storage.RepositoryActionAlternative{storage.AlternativeDefaultConfiguration},
		},
	}
	handler := NewRepositoryScanHandler(nil, manager)
	recorder := postStorageJSON(t, handler.RenameRepository, "/api/v1/storage/repositories/x/rename", `{"name":"Renamed"}`)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d body %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Type         string   `json:"type"`
		Action       string   `json:"action"`
		Blockers     []string `json:"blockers"`
		Alternatives []string `json:"alternatives"`
		Detail       string   `json:"detail"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Type != problem.StorageActionUnavailable.Type || body.Action != "rename" ||
		strings.Join(body.Blockers, ",") != "primary_protected" || strings.Join(body.Alternatives, ",") != "default_configuration" {
		t.Fatalf("problem body = %s", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "/") && strings.Contains(recorder.Body.String(), "path") {
		t.Fatalf("problem body leaks a host path: %s", recorder.Body.String())
	}
}

func TestRepositoryCommandsReturnTypedBarrierIdentityAndOnlineProblems(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want problem.Descriptor
	}{
		{"busy", storage.ErrRepositoryBusy, problem.StorageBusy},
		{"running operation", storage.ErrLifecycleOperationRunning, problem.StorageBusy},
		{"ownership", storage.ErrRepositoryOwnershipUnavailable, problem.StorageOwnershipUnavailable},
		{"identity", storage.ErrRepositoryIDMismatch, problem.StorageIdentityMismatch},
		{"original online", storage.ErrRepositoryOriginalOnline, problem.StorageOriginalOnline},
		{"offline", storage.ErrRepositoryOffline, problem.RepositoryUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manager := &repositoryCommandManagerStub{
				repository: &repo.Repository{RepoID: uuid.New(), Name: "Archive", Role: dbtypes.RepoRoleRegular},
				renameErr:  fmt.Errorf("wrapped: %w", tc.err), removeErr: fmt.Errorf("wrapped: %w", tc.err),
			}
			handler := NewRepositoryScanHandler(nil, manager)
			for _, run := range []struct {
				name string
				rec  *httptest.ResponseRecorder
			}{
				{"rename", postStorageJSON(t, handler.RenameRepository, "/api/v1/storage/repositories/x/rename", `{"name":"Renamed"}`)},
				{"detach", postStorageJSON(t, handler.DetachRepository, "/api/v1/storage/repositories/x/detach", `{"confirmation_name":"Archive"}`)},
			} {
				var body struct {
					Type string `json:"type"`
				}
				if err := json.Unmarshal(run.rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if run.rec.Code != tc.want.Status || body.Type != tc.want.Type {
					t.Fatalf("%s: status %d type %s, want %d %s", run.name, run.rec.Code, body.Type, tc.want.Status, tc.want.Type)
				}
			}
		})
	}
}

func TestDetachRepositoryReturnsItsReceiptAndReplaysAfterTheRowIsGone(t *testing.T) {
	receipt := storage.LifecycleReceipt{OperationID: "op-1", RequestID: "req-1", Kind: "detach_repository", Status: "completed"}
	manager := &repositoryCommandManagerStub{
		repository: &repo.Repository{RepoID: uuid.New(), Name: "Archive", Role: dbtypes.RepoRoleRegular},
		receipt:    receipt,
	}
	handler := NewRepositoryScanHandler(nil, manager)
	recorder := postStorageJSON(t, handler.DetachRepository, "/api/v1/storage/repositories/x/detach", `{"confirmation_name":"Archive"}`)
	var response dto.DetachRepositoryResponseDTO
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || recorder.Code != http.StatusOK {
		t.Fatalf("detach = %d %s (%v)", recorder.Code, recorder.Body.String(), err)
	}
	if response.Operation.OperationID != "op-1" || response.Operation.Kind != "detach_repository" || !response.FilesPreserved || response.Operation.Replayed {
		t.Fatalf("detach response = %+v", response)
	}

	// The row is gone and the journal answers the retry with the same receipt.
	manager.repository = nil
	manager.receipt.Replayed = true
	recorder = postStorageJSON(t, handler.DetachRepository, "/api/v1/storage/repositories/x/detach", `{"confirmation_name":"Archive"}`)
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || recorder.Code != http.StatusOK {
		t.Fatalf("replayed detach = %d %s (%v)", recorder.Code, recorder.Body.String(), err)
	}
	if !response.Operation.Replayed || response.Operation.OperationID != "op-1" {
		t.Fatalf("replayed detach response = %+v", response)
	}

	// A wrong confirmation never reaches the command.
	manager.repository = &repo.Repository{RepoID: uuid.New(), Name: "Archive", Role: dbtypes.RepoRoleRegular}
	before := len(manager.removed)
	recorder = postStorageJSON(t, handler.DetachRepository, "/api/v1/storage/repositories/x/detach", `{"confirmation_name":"Other"}`)
	if recorder.Code != http.StatusBadRequest || len(manager.removed) != before {
		t.Fatalf("wrong confirmation = %d, removed %d -> %d", recorder.Code, before, len(manager.removed))
	}
}
