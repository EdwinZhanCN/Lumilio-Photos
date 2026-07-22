package app

import (
	"context"
	"errors"
	"testing"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"

	"github.com/jackc/pgx/v5/pgtype"
)

type hostOwnerRepositoryManagerStub struct {
	storage.RepositoryManager
	hostOwnerID      *int32
	attachedOwnerID  *int32
	registeredCopyID *int32
}

func (s *hostOwnerRepositoryManagerStub) HostOwnerID(context.Context) (*int32, error) {
	return s.hostOwnerID, nil
}

func (s *hostOwnerRepositoryManagerStub) AddRepository(_ string, ownerID *int32, _ dbtypes.RepoRole, _ ...pgtype.UUID) (*repo.Repository, error) {
	s.attachedOwnerID = ownerID
	return &repo.Repository{Name: "Archive", Status: dbtypes.RepoStatusActive}, nil
}

func (s *hostOwnerRepositoryManagerStub) RegisterRepositoryCopy(_ context.Context, _ string, ownerID *int32, _ dbtypes.RepoRole) (*repo.Repository, error) {
	s.registeredCopyID = ownerID
	return &repo.Repository{Name: "Archive copy", Status: dbtypes.RepoStatusActive}, nil
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
