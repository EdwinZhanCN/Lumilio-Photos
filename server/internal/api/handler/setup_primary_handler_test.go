package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"server/internal/api/dto"
	"server/internal/api/problem"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage"
)

type adoptPrimaryManagerStub struct {
	storage.RepositoryManager
	hostOwnerID *int32
	spec        storage.AdoptPrimarySpec
	result      *storage.AdoptPrimaryResult
	err         error
	review      *storage.SetupPrimaryAssessment
	defaults    storage.DefaultStorageAssessment
}

func (s *adoptPrimaryManagerStub) HostOwnerID(context.Context) (*int32, error) {
	return s.hostOwnerID, nil
}

func (s *adoptPrimaryManagerStub) AdoptPrimaryRepository(_ context.Context, spec storage.AdoptPrimarySpec) (*storage.AdoptPrimaryResult, error) {
	s.spec = spec
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func (s *adoptPrimaryManagerStub) AssessSetupPrimaryTarget(context.Context) (*storage.SetupPrimaryAssessment, error) {
	return s.review, nil
}

func (s *adoptPrimaryManagerStub) AssessDefaultStorageLocation(_ context.Context, path string) (storage.DefaultStorageAssessment, error) {
	result := s.defaults
	result.ConfiguredPath = path
	return result, nil
}

func (s *adoptPrimaryManagerStub) GetRepositoryDefaults(context.Context) (storage.RepoDefaults, error) {
	return storage.RepoDefaults{Strategy: "date", DuplicateHandling: "rename"}, nil
}

func (s *adoptPrimaryManagerStub) StorageRuntimeStatus(context.Context) (storage.StorageRuntimeStatus, error) {
	return storage.StorageRuntimeStatus{State: storage.StorageRuntimeStateActive}, nil
}

type setupBootstrapStub struct{ ready bool }

func (s setupBootstrapStub) Phase(context.Context) (string, error) {
	if s.ready {
		return service.BootstrapPhaseReady, nil
	}
	return service.BootstrapPhaseAdminCreated, nil
}
func (s setupBootstrapStub) Reconcile(ctx context.Context) (string, error) { return s.Phase(ctx) }
func (s setupBootstrapStub) IsReady(context.Context) (bool, error)         { return s.ready, nil }

func setupGetContext(path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, path, nil)
	return ctx, recorder
}

// The public status is phase-only; the administrator's review carries the
// defaults, the default Storage Location state, and the primary target with
// its actions, identity, and recovery preview, but no host path for the
// Repository itself.
func TestSetupStatusIsPhaseOnlyAndThePrimaryTargetCarriesTheReview(t *testing.T) {
	repositoryID := uuid.NewString()
	manager := &adoptPrimaryManagerStub{
		defaults: storage.DefaultStorageAssessment{Registered: true, State: storage.DefaultStorageReady, RegisteredPath: "/Volumes/Photos"},
		review: &storage.SetupPrimaryAssessment{
			CreateTargetAssessment: &storage.CreateTargetAssessment{Intent: storage.IntentSetupPrimary, DirectoryName: "primary", Path: "/Volumes/Photos/primary", Token: "v1.setup", Risks: []storage.StorageRisk{storage.RiskRemovable}},
			State:                  storage.SetupPrimaryLayoutRepairable, Actions: []string{storage.SetupActionAdopt, storage.SetupActionFreshElsewhere},
			LayoutMissing: []string{".lumilio"},
			Existing:      &storage.SetupPrimaryExisting{RepositoryID: repositoryID, Name: "Old Primary", MarkerVersion: "1.0"},
			Recovery:      &storage.SetupPrimaryRecoveryPreview{TrashFiles: 2, StudioRecords: 1, StagingFiles: 2, UnknownFiles: 1},
		},
	}
	handler := NewSetupHandler(service.NewSetupService(setupBootstrapStub{}, manager, "/Volumes/Photos"), true)

	ctx, recorder := setupGetContext("/api/v1/setup/status")
	handler.GetSetupStatus(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if strings.Contains(body, "/Volumes") || strings.Contains(body, "v1.setup") || strings.Contains(body, repositoryID) || strings.Contains(body, "repository_defaults") {
		t.Fatalf("public setup status leaks host facts: %s", body)
	}
	var status dto.SetupStatusDTO
	if err := json.Unmarshal(recorder.Body.Bytes(), &status); err != nil || !status.AdminInitialized || status.PrimaryRepositoryInitialized {
		t.Fatalf("status = %+v err=%v", status, err)
	}

	ctx, recorder = setupGetContext("/api/v1/setup/primary-target")
	handler.GetSetupPrimaryTarget(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("primary target = %d %s", recorder.Code, recorder.Body.String())
	}
	var target dto.SetupPrimaryTargetDTO
	if err := json.Unmarshal(recorder.Body.Bytes(), &target); err != nil {
		t.Fatal(err)
	}
	if target.SetupComplete || !target.NativeHost || target.RepositoryDefaults.StorageLocation != "/Volumes/Photos" || target.RepositoryDefaults.Strategy != "date" ||
		target.DefaultStorage.State != "ready" || target.DefaultStorage.Path != "/Volumes/Photos" || !target.DefaultStorage.Registered {
		t.Fatalf("primary target = %+v", target)
	}
	review := target.Target
	if review == nil || review.State != "layout_repairable" || len(review.Actions) != 2 || review.Actions[0] != "adopt" ||
		review.Assessment.Token != "v1.setup" || len(review.Assessment.Risks) != 1 || review.LayoutMissing[0] != ".lumilio" ||
		review.Existing == nil || review.Existing.RepositoryID != repositoryID || review.Existing.Name != "Old Primary" ||
		review.Recovery == nil || review.Recovery.TrashFiles != 2 || review.Recovery.StudioRecords != 1 {
		t.Fatalf("review = %+v existing=%+v recovery=%+v", review, review.Existing, review.Recovery)
	}
	if strings.Contains(recorder.Body.String(), "/Volumes/Photos/primary") {
		t.Fatalf("review leaks the Repository path: %s", recorder.Body.String())
	}

	// Once setup is complete the review is not computed at all.
	done := NewSetupHandler(service.NewSetupService(setupBootstrapStub{ready: true}, manager, "/Volumes/Photos"), false)
	ctx, recorder = setupGetContext("/api/v1/setup/primary-target")
	done.GetSetupPrimaryTarget(ctx)
	var completed dto.SetupPrimaryTargetDTO
	if err := json.Unmarshal(recorder.Body.Bytes(), &completed); err != nil || !completed.SetupComplete || completed.Target != nil || completed.NativeHost {
		t.Fatalf("completed primary target = %+v err=%v", completed, err)
	}
}

// Adoption binds the administrator's review to the command, assigns the
// Host Owner, and maps every refusal to its registered Problem.
func TestAdoptPrimaryRepositoryBindsTheReviewAndMapsRefusals(t *testing.T) {
	hostOwner := int32(1)
	repositoryID := uuid.New()
	adopted := &repo.Repository{RepoID: repositoryID, Name: "Old Primary", Path: "/Volumes/Photos/primary", Role: dbtypes.RepoRolePrimary, DefaultOwnerID: &hostOwner}
	receipt := &storage.RecoveryReceipt{
		ReceiptID: uuid.NewString(), Event: storage.RecoveryEventAdopt,
		Trash: storage.RecoverySummary{Files: 2, Bytes: 10},
		Items: []storage.RecoveryItem{{Class: storage.PrivateClassStudio}, {Class: storage.PrivateClassStaging}, {Class: storage.PrivateClassUnknown}},
	}
	manager := &adoptPrimaryManagerStub{hostOwnerID: &hostOwner, result: &storage.AdoptPrimaryResult{
		Repository: adopted, AcknowledgedRisks: []storage.StorageRisk{storage.RiskRemovable}, ReceiptID: receipt.ReceiptID, Receipt: receipt, LayoutRepaired: []string{".lumilio"},
	}}
	handler := NewRepositoryScanHandler(nil, manager)
	handler.SetBootstrapService(setupBootstrapStub{})
	body := `{"expected_repository_id":"` + repositoryID.String() + `","assessment_token":"v1.setup","acknowledged_risks":["removable_storage"]}`

	recorder := postStorageJSON(t, handler.AdoptPrimaryRepository, "/api/v1/setup/primary-repository/adopt", body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("adopt = %d %s", recorder.Code, recorder.Body.String())
	}
	if manager.spec.ExpectedRepositoryID != repositoryID.String() || manager.spec.AssessmentToken != "v1.setup" || len(manager.spec.AcknowledgedRisks) != 1 ||
		manager.spec.OwnerID == nil || *manager.spec.OwnerID != hostOwner || manager.spec.ActorUserID == nil || *manager.spec.ActorUserID != 99 ||
		manager.spec.Actor != "web:user:99" || manager.spec.RequestID == "" {
		t.Fatalf("adopt spec = %+v", manager.spec)
	}
	var response dto.AdoptPrimaryRepositoryResponseDTO
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Repository.ID != repositoryID.String() || !response.Repository.IsPrimary || response.RecoveryReceiptID != receipt.ReceiptID ||
		response.Recovery == nil || response.Recovery.TrashFiles != 2 || response.Recovery.StudioRecords != 1 || response.Recovery.StagingFiles != 1 || response.Recovery.UnknownFiles != 1 ||
		len(response.LayoutRepaired) != 1 || len(response.AcknowledgedRisks) != 1 {
		t.Fatalf("adopt response = %+v recovery=%+v", response, response.Recovery)
	}

	// The owner falls back to the acting administrator when no host owner exists yet.
	manager.hostOwnerID = nil
	postStorageJSON(t, handler.AdoptPrimaryRepository, "/api/v1/setup/primary-repository/adopt", body)
	if manager.spec.OwnerID == nil || *manager.spec.OwnerID != 99 {
		t.Fatalf("owner fallback = %v", manager.spec.OwnerID)
	}

	if recorder := postStorageJSON(t, handler.AdoptPrimaryRepository, "/api/v1/setup/primary-repository/adopt", `{"assessment_token":"v1.setup"}`); recorder.Code != http.StatusBadRequest {
		t.Fatalf("adopt without the expected identity = %d", recorder.Code)
	}

	for _, tc := range []struct {
		err  error
		want problem.Descriptor
	}{
		{&storage.StorageRiskConfirmationError{Risks: []storage.StorageRisk{storage.RiskRemovable}, Token: "v1.setup"}, problem.StorageConfirmationRequired},
		{storage.ErrStorageAssessmentStale, problem.StorageAssessmentStale},
		{storage.ErrSetupPrimaryNotAdoptable, problem.StorageAssessmentStale},
		{storage.ErrRepositoryLayoutObstructed, problem.StorageLayoutObstructed},
		{storage.ErrRepositoryOwnershipUnavailable, problem.StorageOwnershipUnavailable},
		{storage.ErrLifecycleRecoveryRequired, problem.StorageRecoveryRequired},
		{storage.ErrStorageUnsupportedFilesystem, problem.StorageUnsupportedFilesystem},
		{&storage.RepositoryConflictError{RepositoryID: repositoryID.String(), RegisteredPath: "/elsewhere", RequestedPath: "/Volumes/Photos/primary", Actions: []string{"copy"}}, problem.RepositoryConflict},
	} {
		manager.err = tc.err
		recorder := postStorageJSON(t, handler.AdoptPrimaryRepository, "/api/v1/setup/primary-repository/adopt", body)
		var details problem.Details
		if err := json.Unmarshal(recorder.Body.Bytes(), &details); err != nil {
			t.Fatalf("%v: decode: %v", tc.err, err)
		}
		if recorder.Code != tc.want.Status || details.Type != tc.want.Type {
			t.Fatalf("%v: status %d type %s, want %d %s", tc.err, recorder.Code, details.Type, tc.want.Status, tc.want.Type)
		}
		if strings.Contains(recorder.Body.String(), "/Volumes") || strings.Contains(recorder.Body.String(), "/elsewhere") {
			t.Fatalf("%v: Problem leaks a host path: %s", tc.err, recorder.Body.String())
		}
	}

	// Once the primary exists the command refuses as the create does; a retry
	// of a completed adoption is answered by the journal, not refused here.
	manager.err = storage.ErrPrimaryRepositoryExists
	if recorder := postStorageJSON(t, handler.AdoptPrimaryRepository, "/api/v1/setup/primary-repository/adopt", body); recorder.Code != http.StatusConflict {
		t.Fatalf("adopt after setup = %d", recorder.Code)
	}
	manager.err = nil
	// A non-administrator session is refused.
	gin.SetMode(gin.TestMode)
	recorder = httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/setup/primary-repository/adopt", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("current_user", &service.UserResponse{UserID: 5, Role: "user"})
	handler.AdoptPrimaryRepository(ctx)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("adopt by a user = %d", recorder.Code)
	}
	if errors.Is(nil, storage.ErrSetupPrimaryNotAdoptable) {
		t.Fatal("unreachable")
	}
}
