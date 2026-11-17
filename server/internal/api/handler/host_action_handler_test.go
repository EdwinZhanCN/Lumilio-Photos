package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"server/internal/service"
	"server/internal/storage"

	"github.com/gin-gonic/gin"
)

type ownedHostActionManagerStub struct {
	storage.RepositoryManager
	action        storage.HostAction
	resolveCalled bool
	resolution    string
	acknowledged  []string
	cancelCalled  bool
}

func (s *ownedHostActionManagerStub) GetHostAction(context.Context, string) (storage.HostAction, error) {
	return s.action, nil
}

func (s *ownedHostActionManagerStub) ResolveHostAction(_ context.Context, _ string, resolution string, acknowledgedRisks []string) (storage.HostAction, error) {
	s.resolveCalled = true
	s.resolution = resolution
	s.acknowledged = acknowledgedRisks
	return s.action, nil
}

func TestHostActionRiskDecisionProjectionAndConfirmationRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int32(7)
	now := time.Now().UTC()
	manager := &ownedHostActionManagerStub{action: storage.HostAction{
		ActionID: "action-risk", ActorUserID: &ownerID, Status: storage.HostActionNeedsDecision,
		Result: &storage.HostActionResult{Conflict: &storage.HostActionConflict{
			Type: "storage_risk", Actions: []string{"confirm_risk"},
			RiskWarnings: []string{"removable_storage", "mount_fingerprint_changed"},
			Fingerprint:  "0123456789abcdef",
		}},
		ExpiresAt: now.Add(time.Minute), CreatedAt: now, UpdatedAt: now,
	}}
	projected := toHostActionDTO(manager.action)
	if projected.Result == nil || projected.Result.Conflict == nil ||
		len(projected.Result.Conflict.RiskWarnings) != 2 ||
		len(projected.Result.Conflict.AllowedResolutions) != 1 ||
		projected.Result.Conflict.AllowedResolutions[0] != "confirm_risk" {
		t.Fatalf("risk projection = %#v", projected)
	}
	if encoded, _ := json.Marshal(projected); strings.Contains(string(encoded), "0123456789abcdef") {
		t.Fatalf("HTTP projection leaked the review fingerprint: %s", encoded)
	}

	handler := NewHostActionHandler(manager, true)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "action-risk"}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/host-actions/action-risk/resolve",
		strings.NewReader(`{"resolution":"confirm_risk","acknowledged_risks":["removable_storage","mount_fingerprint_changed"]}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("current_user", &service.UserResponse{UserID: int(ownerID), Role: "admin"})
	handler.ResolveHostAction(ctx)
	if recorder.Code != http.StatusOK || !manager.resolveCalled || manager.resolution != "confirm_risk" ||
		len(manager.acknowledged) != 2 || manager.acknowledged[0] != "removable_storage" {
		t.Fatalf("risk resolve status=%d body=%s resolution=%q acknowledged=%v", recorder.Code, recorder.Body.String(), manager.resolution, manager.acknowledged)
	}
}

type refusingHostActionManagerStub struct {
	ownedHostActionManagerStub
	err error
}

func (s *refusingHostActionManagerStub) ResolveHostAction(context.Context, string, string, []string) (storage.HostAction, error) {
	return storage.HostAction{}, s.err
}

// A refused decision answers with the same Problems as the HTTP storage
// commands: the consent gate names the risk codes still to acknowledge, and a
// review whose facts moved is a stale assessment.
func TestHostActionResolveRefusalsUseTheSharedStorageProblems(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int32(7)
	for _, test := range []struct {
		name     string
		err      error
		wantType string
		wantBody string
	}{
		{
			name:     "consent names the outstanding risks",
			err:      &storage.StorageRiskConfirmationError{Risks: []storage.StorageRisk{storage.RiskRemovable, storage.RiskNonDefault}},
			wantType: "https://lumilio.org/problems/storage/confirmation-required",
			wantBody: `"risks":["removable_storage","non_default_location"]`,
		},
		{
			name:     "moved facts are a stale assessment",
			err:      storage.ErrStorageAssessmentStale,
			wantType: "https://lumilio.org/problems/storage/assessment-stale",
		},
		{
			name:     "an original that is online is typed",
			err:      storage.ErrRepositoryOriginalOnline,
			wantType: "https://lumilio.org/problems/storage/original-online",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager := &refusingHostActionManagerStub{err: test.err}
			manager.action = storage.HostAction{ActionID: "action-risk", ActorUserID: &ownerID, Status: storage.HostActionNeedsDecision}
			handler := NewHostActionHandler(manager, true)
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Params = gin.Params{{Key: "id", Value: "action-risk"}}
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/storage/native-tasks/action-risk/resolve",
				strings.NewReader(`{"resolution":"confirm_risk","acknowledged_risks":["removable_storage"]}`))
			ctx.Request.Header.Set("Content-Type", "application/json")
			ctx.Set("current_user", &service.UserResponse{UserID: int(ownerID), Role: "admin"})
			handler.ResolveHostAction(ctx)
			body := recorder.Body.String()
			if recorder.Code != http.StatusConflict || !strings.Contains(body, test.wantType) || !strings.Contains(body, test.wantBody) {
				t.Fatalf("status=%d body=%s", recorder.Code, body)
			}
		})
	}
}

// A failed task whose cause is a typed storage refusal is projected as that
// Problem with the policy verdict, so the Web renders the HTTP vocabulary;
// an untyped failure stays the generic host-action failure, and an expired
// task is its own normal outcome.
func TestHostActionFailureProjectionKeepsTheTypedReason(t *testing.T) {
	now := time.Now().UTC()
	refused := toHostActionDTO(storage.HostAction{
		ActionID: "action-refused", Kind: storage.HostActionLocateRepository, Status: storage.HostActionFailed,
		ErrorCode: "storage/action-unavailable", ErrorMessage: "withheld",
		Result: &storage.HostActionResult{Refusal: &storage.HostActionRefusal{
			Action: "reconnect", Blockers: []string{"primary_protected", "original_available"}, Alternatives: []string{"default_configuration"},
		}},
		ExpiresAt: now.Add(time.Minute), CreatedAt: now, UpdatedAt: now,
	})
	if refused.Problem == nil || refused.Problem.Type != "https://lumilio.org/problems/storage/action-unavailable" ||
		refused.Refusal == nil || refused.Refusal.Action != "reconnect" || len(refused.Refusal.Blockers) != 2 || refused.Refusal.Alternatives[0] != "default_configuration" {
		t.Fatalf("refused projection = %#v refusal=%#v", refused.Problem, refused.Refusal)
	}
	network := toHostActionDTO(storage.HostAction{
		ActionID: "action-network", Kind: storage.HostActionAuthorizeStorageLocation, Status: storage.HostActionFailed,
		ErrorCode: "storage/unsupported-filesystem", ExpiresAt: now.Add(time.Minute), CreatedAt: now, UpdatedAt: now,
	})
	if network.Problem == nil || network.Problem.Type != "https://lumilio.org/problems/storage/unsupported-filesystem" || network.Refusal != nil {
		t.Fatalf("network projection = %#v", network.Problem)
	}
	generic := toHostActionDTO(storage.HostAction{
		ActionID: "action-generic", Kind: storage.HostActionOpenRepository, Status: storage.HostActionFailed,
		ErrorCode: "open_repository_failed", ExpiresAt: now.Add(time.Minute), CreatedAt: now, UpdatedAt: now,
	})
	if generic.Problem == nil || generic.Problem.Type != "https://lumilio.org/problems/storage/host-action-failed" {
		t.Fatalf("generic projection = %#v", generic.Problem)
	}
	expired := toHostActionDTO(storage.HostAction{
		ActionID: "action-expired", Kind: storage.HostActionOpenRepository, Status: storage.HostActionExpired,
		ErrorCode: "expired", ExpiresAt: now.Add(-time.Minute), CreatedAt: now, UpdatedAt: now,
	})
	if expired.Problem == nil || expired.Problem.Type != "https://lumilio.org/problems/storage/host-action-expired" {
		t.Fatalf("expired projection = %#v", expired.Problem)
	}
}

func (s *ownedHostActionManagerStub) CancelHostAction(context.Context, string) (storage.HostAction, error) {
	s.cancelCalled = true
	return s.action, nil
}

func TestHostActionHTTPProjectionUsesUserFacingResolutions(t *testing.T) {
	now := time.Now().UTC()
	action := storage.HostAction{
		ActionID: "action-1", RequestID: "request-1", Kind: storage.HostActionOpenRepository,
		Actor: "web:user:1", Status: storage.HostActionNeedsDecision,
		Summary: storage.HostActionSummary{RepositoryID: "repository-1", Purpose: "Open an existing repository"},
		Result: &storage.HostActionResult{Conflict: &storage.HostActionConflict{
			Type: "repository_identity", RepositoryID: "repository-1",
			RegisteredPath: "/old", RequestedPath: "/new", Actions: []string{"relocate", "copy", "unknown"},
		}},
		ExpiresAt: now.Add(time.Minute), CreatedAt: now, UpdatedAt: now,
	}

	projected := toHostActionDTO(action)
	if projected.Result == nil || projected.Result.Conflict == nil {
		t.Fatalf("projected action = %#v", projected)
	}
	got := projected.Result.Conflict.AllowedResolutions
	if len(got) != 2 || got[0] != "update_location" || got[1] != "add_separate" {
		t.Fatalf("allowed resolutions = %#v", got)
	}
	if projected.Actor != "web:user:1" || projected.RepositoryID != "repository-1" || projected.Purpose == "" {
		t.Fatalf("durable workflow projection = %#v", projected)
	}
	encoded, err := json.Marshal(projected)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "/old") || strings.Contains(string(encoded), "/new") {
		t.Fatalf("HTTP projection leaked native paths: %s", encoded)
	}
}

func TestHostActionEndpointsRejectAnotherActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int32(7)
	manager := &ownedHostActionManagerStub{action: storage.HostAction{
		ActionID: "action-1", ActorUserID: &ownerID, Status: storage.HostActionPending,
	}}
	handler := NewHostActionHandler(manager, true)
	for _, test := range []struct {
		method string
		path   string
		body   string
		call   func(*gin.Context)
	}{
		{method: http.MethodGet, path: "/api/v1/host-actions/action-1", call: handler.GetHostAction},
		{method: http.MethodPost, path: "/api/v1/host-actions/action-1/resolve", body: `{"resolution":"add_separate"}`, call: handler.ResolveHostAction},
		{method: http.MethodPost, path: "/api/v1/storage/native-tasks/action-1/cancel", call: handler.CancelHostAction},
	} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Params = gin.Params{{Key: "id", Value: "action-1"}}
		ctx.Request = httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Set("current_user", &service.UserResponse{UserID: 8, Role: "admin"})
		test.call(ctx)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("%s %s status = %d, body=%s", test.method, test.path, recorder.Code, recorder.Body.String())
		}
	}
	if manager.resolveCalled || manager.cancelCalled {
		t.Fatal("cross-actor request reached a host-action mutation")
	}
}
