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
	riskConfirmed bool
	cancelCalled  bool
}

func (s *ownedHostActionManagerStub) GetHostAction(context.Context, string) (storage.HostAction, error) {
	return s.action, nil
}

func (s *ownedHostActionManagerStub) ResolveHostAction(_ context.Context, _ string, resolution string, riskConfirmation ...bool) (storage.HostAction, error) {
	s.resolveCalled = true
	s.resolution = resolution
	s.riskConfirmed = len(riskConfirmation) > 0 && riskConfirmation[0]
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
			RiskWarnings: []string{"network_filesystem", "mount_fingerprint_changed"},
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

	handler := NewHostActionHandler(manager, true)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "action-risk"}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/host-actions/action-risk/resolve",
		strings.NewReader(`{"resolution":"confirm_risk","risk_confirmation":true}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("current_user", &service.UserResponse{UserID: int(ownerID), Role: "admin"})
	handler.ResolveHostAction(ctx)
	if recorder.Code != http.StatusOK || !manager.resolveCalled || manager.resolution != "confirm_risk" || !manager.riskConfirmed {
		t.Fatalf("risk resolve status=%d body=%s resolution=%q confirmed=%v", recorder.Code, recorder.Body.String(), manager.resolution, manager.riskConfirmed)
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
		{method: http.MethodDelete, path: "/api/v1/host-actions/action-1", call: handler.CancelHostAction},
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
