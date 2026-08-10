package app

import (
	"context"
	"errors"
	"testing"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
)

type hostOwnerRepositoryManagerStub struct {
	storage.RepositoryManager
	hostOwnerID      *int32
	attachedOwnerID  *int32
	registeredCopyID *int32
	attachedRequest  storage.LifecycleRequest
	copyRequest      storage.LifecycleRequest
}

func (s *hostOwnerRepositoryManagerStub) HostOwnerID(context.Context) (*int32, error) {
	return s.hostOwnerID, nil
}

func (s *hostOwnerRepositoryManagerStub) OpenRepository(_ context.Context, _ string, ownerID *int32, _ dbtypes.RepoRole, request storage.LifecycleRequest) (*repo.Repository, error) {
	s.attachedOwnerID = ownerID
	s.attachedRequest = request
	return &repo.Repository{Name: "Archive", Reachability: dbtypes.RepositoryReachabilityActive}, nil
}

func (s *hostOwnerRepositoryManagerStub) RegisterRepositoryCopy(_ context.Context, _ string, ownerID *int32, _ dbtypes.RepoRole, requests ...storage.LifecycleRequest) (*repo.Repository, error) {
	s.registeredCopyID = ownerID
	if len(requests) > 0 {
		s.copyRequest = requests[0]
	}
	return &repo.Repository{Name: "Archive copy", Reachability: dbtypes.RepositoryReachabilityActive}, nil
}

func (s *hostOwnerRepositoryManagerStub) ScheduleInitialRepositoryScan(context.Context, string) error {
	return nil
}

func TestRepositoryControlAttachUsesHostOwner(t *testing.T) {
	hostOwnerID := int32(7)
	manager := &hostOwnerRepositoryManagerStub{hostOwnerID: &hostOwnerID}
	control := newRepositoryControl(manager)

	if _, err := control.AttachRepository(context.Background(), "/external/archive"); err != nil {
		t.Fatal(err)
	}
	if manager.attachedOwnerID == nil || *manager.attachedOwnerID != hostOwnerID {
		t.Fatalf("attached owner = %v, want Host Owner %d", manager.attachedOwnerID, hostOwnerID)
	}
	if manager.attachedRequest.RequestID == "" || manager.attachedRequest.HostInstanceID == "" ||
		manager.attachedRequest.Actor != "desktop_host:"+manager.attachedRequest.HostInstanceID ||
		manager.attachedRequest.ConfirmationType != "native_directory_selection" {
		t.Fatalf("attach lifecycle request = %#v", manager.attachedRequest)
	}
}

func TestRepositoryControlCopyUsesHostOwner(t *testing.T) {
	hostOwnerID := int32(7)
	manager := &hostOwnerRepositoryManagerStub{hostOwnerID: &hostOwnerID}
	control := newRepositoryControl(manager)

	if _, err := control.ResolveRepositoryConflict(context.Background(), "copy", "old-id", "/external/archive-copy"); err != nil {
		t.Fatal(err)
	}
	if manager.registeredCopyID == nil || *manager.registeredCopyID != hostOwnerID {
		t.Fatalf("copy owner = %v, want Host Owner %d", manager.registeredCopyID, hostOwnerID)
	}
	if manager.copyRequest.RequestID == "" || manager.copyRequest.HostInstanceID == "" ||
		manager.copyRequest.Actor != "desktop_host:"+manager.copyRequest.HostInstanceID ||
		manager.copyRequest.ConfirmationType != "independent_identity" || !manager.copyRequest.RiskConfirmation {
		t.Fatalf("copy lifecycle request = %#v", manager.copyRequest)
	}
}

func TestRepositoryControlRejectsAttachBeforeHostOwnerExists(t *testing.T) {
	manager := &hostOwnerRepositoryManagerStub{}
	control := newRepositoryControl(manager)

	if _, err := control.AttachRepository(context.Background(), "/external/archive"); !errors.Is(err, ErrHostOwnerUnavailable) {
		t.Fatalf("attach error = %v, want ErrHostOwnerUnavailable", err)
	}
	if manager.attachedOwnerID != nil {
		t.Fatalf("repository was attached without a Host Owner: %v", manager.attachedOwnerID)
	}
}
