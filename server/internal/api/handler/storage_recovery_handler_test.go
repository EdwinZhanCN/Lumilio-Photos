package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"server/internal/api/dto"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage"
	"server/internal/storage/recovery"
	"server/internal/storage/trash"
)

type recoveryServiceStub struct {
	inventory recovery.Inventory
	err       error
	deleted   string
	content   string
}

func (s *recoveryServiceStub) Inventory(context.Context, uuid.UUID) (recovery.Inventory, error) {
	return s.inventory, s.err
}

func (s *recoveryServiceStub) OpenItem(_ context.Context, _ uuid.UUID, itemID string) (io.ReadCloser, recovery.Item, error) {
	if s.err != nil {
		return nil, recovery.Item{}, s.err
	}
	for _, item := range s.inventory.Items {
		if item.ItemID == itemID {
			return io.NopCloser(strings.NewReader(s.content)), item, nil
		}
	}
	return nil, recovery.Item{}, recovery.ErrItemNotFound
}

func (s *recoveryServiceStub) DeleteItem(_ context.Context, _ uuid.UUID, itemID string, _ recovery.Request) (recovery.Item, error) {
	if s.err != nil {
		return recovery.Item{}, s.err
	}
	for _, item := range s.inventory.Items {
		if item.ItemID == itemID {
			if item.Disposition == storage.DispositionReassociated {
				return item, recovery.ErrItemNotDeletable
			}
			s.deleted = itemID
			item.Disposition = storage.DispositionDeleted
			item.Present = false
			item.Actions = nil
			return item, nil
		}
	}
	return recovery.Item{}, recovery.ErrItemNotFound
}

func recoveryTestContext(method, target string, params gin.Params) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = params
	ctx.Request = httptest.NewRequest(method, target, nil)
	ctx.Set("current_user", &service.UserResponse{UserID: 3, Role: "admin"})
	return ctx, recorder
}

// The inventory projection carries receipts, items with their actions, and
// the held Trash; the export streams bytes as an attachment; refusals map to
// the shared Problems (unavailable Repository, missing item, non-deletable).
func TestRepositoryRecoveryHandlersProjectInventoryAndMapRefusals(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repositoryID := uuid.New()
	now := time.Now().UTC()
	stagingID, studioID := uuid.NewString(), uuid.NewString()
	stub := &recoveryServiceStub{content: "staged bytes", inventory: recovery.Inventory{
		RepositoryID: repositoryID,
		Receipts: []recovery.ReceiptSummary{{ReceiptID: uuid.NewString(), Event: storage.RecoveryEventOpen, RecordedAt: now, Actor: "web:user:1",
			Trash: storage.RecoverySummary{Files: 2, Bytes: 10}, Logs: storage.RecoverySummary{Files: 2}}},
		Items: []recovery.Item{
			{RecoveryItem: storage.RecoveryItem{ItemID: stagingID, Class: storage.PrivateClassStaging, Path: ".lumilio/staging/incoming/a.part", Size: 11, Disposition: storage.DispositionMoved, MovedTo: ".lumilio/recovery/x/staging/incoming/a.part"}, Present: true, Actions: []string{"export", "delete"}},
			{RecoveryItem: storage.RecoveryItem{ItemID: studioID, Class: storage.PrivateClassStudio, Path: ".lumilio/sidecars/old.lumilio-sidecar", Disposition: storage.DispositionReassociated, ResolvedAssetID: uuid.NewString()}, Present: true, Actions: []string{}},
		},
		TrashHeld:          recovery.TrashHeld{Count: 1, Bytes: 5},
		UnreadableReceipts: []string{},
	}}
	handler := NewStorageHandler(nil, nil, nil)
	handler.SetRecovery(stub)
	params := gin.Params{{Key: "id", Value: repositoryID.String()}}

	ctx, recorder := recoveryTestContext(http.MethodGet, "/api/v1/storage/repositories/x/recovery", params)
	handler.GetRepositoryRecovery(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("inventory status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var projected dto.RepositoryRecoveryDTO
	if err := json.Unmarshal(recorder.Body.Bytes(), &projected); err != nil {
		t.Fatal(err)
	}
	if projected.RepositoryID != repositoryID.String() || len(projected.Receipts) != 1 || projected.Receipts[0].TrashFiles != 2 ||
		len(projected.Items) != 2 || projected.Items[0].Actions[0] != "export" || projected.Items[1].Disposition != "reassociated" ||
		projected.Trash.HeldCount != 1 || projected.UnreadableReceipts == nil {
		t.Fatalf("projection = %+v", projected)
	}
	if strings.Contains(recorder.Body.String(), "/tmp/") || strings.Contains(recorder.Body.String(), `"moved_to"`) {
		t.Fatalf("projection leaks host or internal paths: %s", recorder.Body.String())
	}

	itemParams := append(gin.Params{}, params...)
	itemParams = append(itemParams, gin.Param{Key: "item", Value: stagingID})
	ctx, recorder = recoveryTestContext(http.MethodGet, "/api/v1/storage/repositories/x/recovery/items/y/content", itemParams)
	handler.ExportRecoveryItem(ctx)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "staged bytes" ||
		!strings.Contains(recorder.Header().Get("Content-Disposition"), "filename=a.part") || recorder.Header().Get("Content-Length") != "11" {
		t.Fatalf("export = %d %q headers=%v", recorder.Code, recorder.Body.String(), recorder.Header())
	}

	ctx, recorder = recoveryTestContext(http.MethodDelete, "/api/v1/storage/repositories/x/recovery/items/y", itemParams)
	handler.DeleteRecoveryItem(ctx)
	if recorder.Code != http.StatusOK || stub.deleted != stagingID || !strings.Contains(recorder.Body.String(), `"disposition":"deleted"`) {
		t.Fatalf("delete = %d %s (deleted %q)", recorder.Code, recorder.Body.String(), stub.deleted)
	}

	studioParams := append(gin.Params{}, params...)
	studioParams = append(studioParams, gin.Param{Key: "item", Value: studioID})
	ctx, recorder = recoveryTestContext(http.MethodDelete, "/api/v1/storage/repositories/x/recovery/items/y", studioParams)
	handler.DeleteRecoveryItem(ctx)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("deleting a reassociated record = %d %s", recorder.Code, recorder.Body.String())
	}

	missingParams := append(gin.Params{}, params...)
	missingParams = append(missingParams, gin.Param{Key: "item", Value: uuid.NewString()})
	ctx, recorder = recoveryTestContext(http.MethodGet, "/api/v1/storage/repositories/x/recovery/items/y/content", missingParams)
	handler.ExportRecoveryItem(ctx)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("export of an unknown item = %d", recorder.Code)
	}

	stub.err = recovery.ErrRepositoryOffline
	ctx, recorder = recoveryTestContext(http.MethodGet, "/api/v1/storage/repositories/x/recovery", params)
	handler.GetRepositoryRecovery(ctx)
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "repository/unavailable") {
		t.Fatalf("offline inventory = %d %s", recorder.Code, recorder.Body.String())
	}

	ctx, recorder = recoveryTestContext(http.MethodGet, "/api/v1/storage/repositories/x/recovery", gin.Params{{Key: "id", Value: "not-a-uuid"}})
	handler.GetRepositoryRecovery(ctx)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid repository id = %d", recorder.Code)
	}
}

type holdAssetService struct {
	stubAssetService
	held         []uuid.UUID
	acceptedFor  []uuid.UUID
	repositoryID uuid.NullUUID
	owner        *int32
}

func (s *holdAssetService) HeldTrashAssetIDs(_ context.Context, repositoryID uuid.NullUUID, owner *int32) ([]uuid.UUID, error) {
	s.repositoryID, s.owner = repositoryID, owner
	return s.held, nil
}

func (s *holdAssetService) AcceptTrashRetention(_ context.Context, request trash.Request, repositoryID uuid.NullUUID, owner *int32) (trash.AcceptRetentionResult, error) {
	s.acceptedFor, s.repositoryID, s.owner = request.AssetIDs, repositoryID, owner
	return trash.AcceptRetentionResult{Entries: int64(len(request.AssetIDs))}, nil
}

// Held listing and retention acceptance follow the Trash's scope rules: a
// user sees and decides only their own Assets; a Repository-wide acceptance
// is an administrator action; the explicit confirmation is required.
func TestTrashHoldEndpointsRespectOwnerScopeAndConfirmation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	own, other := uuid.New(), uuid.New()
	stub := &holdAssetService{held: []uuid.UUID{own}, stubAssetService: stubAssetService{getAssetFn: func(_ context.Context, id uuid.UUID) (*repo.Asset, error) {
		owner := int32(1)
		if id == other {
			owner = 2
		}
		return &repo.Asset{AssetID: id, OwnerID: &owner}, nil
	}}}
	handler := &AssetHandler{assetService: stub}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("current_user", &service.UserResponse{UserID: 1, Role: "user"})
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/assets/trash-holds", nil)
	handler.GetTrashHolds(ctx)
	if recorder.Code != http.StatusOK || stub.owner == nil || *stub.owner != 1 || !strings.Contains(recorder.Body.String(), own.String()) {
		t.Fatalf("user holds = %d %s owner=%v", recorder.Code, recorder.Body.String(), stub.owner)
	}

	ctx, recorder = lifecycleTestContext(`{"asset_ids":["` + own.String() + `","` + other.String() + `"],"confirm":true}`)
	handler.AcceptTrashRetention(ctx)
	if recorder.Code != http.StatusForbidden || len(stub.acceptedFor) != 0 {
		t.Fatalf("acceptance over another user's Asset = %d (accepted %v)", recorder.Code, stub.acceptedFor)
	}

	ctx, recorder = lifecycleTestContext(`{"asset_ids":["` + own.String() + `"]}`)
	handler.AcceptTrashRetention(ctx)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("acceptance without confirmation = %d", recorder.Code)
	}

	ctx, recorder = lifecycleTestContext(`{"repository_id":"` + uuid.NewString() + `","confirm":true}`)
	handler.AcceptTrashRetention(ctx)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("Repository-wide acceptance by a user = %d", recorder.Code)
	}

	ctx, recorder = lifecycleTestContext(`{"asset_ids":["` + own.String() + `"],"confirm":true}`)
	handler.AcceptTrashRetention(ctx)
	if recorder.Code != http.StatusOK || len(stub.acceptedFor) != 1 || stub.acceptedFor[0] != own || !strings.Contains(recorder.Body.String(), `"entries":1`) {
		t.Fatalf("acceptance = %d %s accepted=%v", recorder.Code, recorder.Body.String(), stub.acceptedFor)
	}
	if errors.Is(nil, recovery.ErrItemNotFound) {
		t.Fatal("unreachable")
	}
}
