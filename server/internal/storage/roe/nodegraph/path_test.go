package nodegraph

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"

	"server/internal/db/repo"
)

type nodeReader map[uuid.UUID]repo.RepositoryNode

func (reader nodeReader) GetRepositoryNode(_ context.Context, params repo.GetRepositoryNodeParams) (repo.RepositoryNode, error) {
	node, ok := reader[params.NodeID]
	if !ok || node.RepositoryID != params.RepositoryID {
		return repo.RepositoryNode{}, sql.ErrNoRows
	}
	return node, nil
}

func TestProjectPathRejectsTombstonedAncestor(t *testing.T) {
	repositoryID := uuid.New()
	rootID := uuid.New()
	directoryID := uuid.New()
	fileID := uuid.New()
	reader := nodeReader{
		rootID: {
			NodeID: rootID, RepositoryID: repositoryID, Name: "", Kind: "directory", Lifecycle: "active",
		},
		directoryID: {
			NodeID: directoryID, RepositoryID: repositoryID,
			ParentNodeID: uuid.NullUUID{UUID: rootID, Valid: true},
			Name:         "gone", Kind: "directory", Lifecycle: "tombstoned",
		},
		fileID: {
			NodeID: fileID, RepositoryID: repositoryID,
			ParentNodeID: uuid.NullUUID{UUID: directoryID, Valid: true},
			Name:         "photo.jpg", Kind: "file", Lifecycle: "active",
		},
	}
	if _, err := ProjectPath(context.Background(), reader, repositoryID, fileID); !errors.Is(err, ErrNodeNotActive) {
		t.Fatalf("ProjectPath error = %v, want ErrNodeNotActive", err)
	}
}

func TestProjectPathProjectsOnlyActiveChain(t *testing.T) {
	repositoryID := uuid.New()
	rootID := uuid.New()
	directoryID := uuid.New()
	fileID := uuid.New()
	reader := nodeReader{
		rootID: {
			NodeID: rootID, RepositoryID: repositoryID, Name: "", Kind: "directory", Lifecycle: "active",
		},
		directoryID: {
			NodeID: directoryID, RepositoryID: repositoryID,
			ParentNodeID: uuid.NullUUID{UUID: rootID, Valid: true},
			Name:         "album", Kind: "directory", Lifecycle: "active",
		},
		fileID: {
			NodeID: fileID, RepositoryID: repositoryID,
			ParentNodeID: uuid.NullUUID{UUID: directoryID, Valid: true},
			Name:         "photo.jpg", Kind: "file", Lifecycle: "active",
		},
	}
	projected, err := ProjectPath(context.Background(), reader, repositoryID, fileID)
	if err != nil {
		t.Fatal(err)
	}
	if projected != "album/photo.jpg" {
		t.Fatalf("ProjectPath = %q", projected)
	}
}
