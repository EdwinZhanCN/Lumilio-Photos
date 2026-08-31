// Package materializer turns stable node observations into immutable content,
// owner/content Assets, and versioned Locations. Full content I/O completes
// before the short catalog CAS transaction begins.
package materializer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"

	"github.com/google/uuid"

	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/roe/nodegraph"
	fileutil "server/internal/utils/file"
)

type ResultCode string

const (
	ResultBound ResultCode = "bound"
	ResultStale ResultCode = "stale"
	ResultNoop  ResultCode = "noop"
)

type Result struct {
	Code         ResultCode
	RepositoryID uuid.UUID
	NodeID       uuid.UUID
	AssetID      uuid.UUID
	ContentID    uuid.UUID
	Revision     int64
	NewAsset     bool
}

// HashPreparation is the immutable result of one bounded node read. It is
// deliberately free of a database capability: a macro may prepare it, but
// only the commit coordinator may apply it.
type HashPreparation struct {
	Node          repo.RepositoryNode
	OwnerID       int32
	ProjectedPath string
	Observation   storage.FileObservation
	AssetType     string
	MimeType      string
	Reobserve     bool
}

type HashReader interface {
	GetRepositoryObservationForNodeRevision(context.Context, repo.GetRepositoryObservationForNodeRevisionParams) (repo.RepositoryObservation, error)
	GetRepositoryNode(context.Context, repo.GetRepositoryNodeParams) (repo.RepositoryNode, error)
	GetRepository(context.Context, uuid.UUID) (repo.Repository, error)
}

type QueryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type HashPreparer struct {
	reader   HashReader
	queryRow QueryRower
	files    *storage.RepositoryFSFactory
}

func NewHashPreparer(reader HashReader, queryRow QueryRower, files *storage.RepositoryFSFactory) *HashPreparer {
	return &HashPreparer{reader: reader, queryRow: queryRow, files: files}
}

func (m *HashPreparer) PrepareHash(ctx context.Context, nodeID uuid.UUID, expectedRevision int64) (*HashPreparation, error) {
	if m == nil || m.reader == nil || m.queryRow == nil || m.files == nil {
		return nil, errors.New("repository hash materializer unavailable")
	}
	if nodeID == uuid.Nil || expectedRevision <= 0 {
		return nil, errors.New("node ID and expected revision are required")
	}
	node, err := m.getNodeByID(ctx, nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if node.Lifecycle != "active" || node.Kind == "directory" || node.ObservationRevision != expectedRevision {
		return nil, nil
	}
	fact, err := m.reader.GetRepositoryObservationForNodeRevision(ctx, repo.GetRepositoryObservationForNodeRevisionParams{
		RepositoryID: node.RepositoryID,
		MappedNodeID: uuid.NullUUID{UUID: node.NodeID, Valid: true}, Revision: expectedRevision,
	})
	if err != nil {
		return nil, fmt.Errorf("load resolved observation owner: %w", err)
	}
	if fact.ResolvedOwnerID == nil || *fact.ResolvedOwnerID <= 0 || *fact.ResolvedOwnerID > int64(^uint32(0)>>1) {
		return nil, errors.New("repository observation has no valid resolved owner")
	}
	resolvedOwnerID := int32(*fact.ResolvedOwnerID)
	projectedPath, err := nodegraph.ProjectPath(ctx, m.reader, node.RepositoryID, node.NodeID)
	if err != nil {
		return nil, fmt.Errorf("project repository node path: %w", err)
	}
	repositoryPath, err := storage.ParseUserMediaPath(projectedPath)
	if err != nil {
		return nil, err
	}
	validation := fileutil.ValidateFile(path.Base(projectedPath), "")
	if !validation.Valid {
		return nil, fmt.Errorf("unsupported repository media: %s", validation.ErrorReason)
	}
	repository, err := m.reader.GetRepository(ctx, node.RepositoryID)
	if err != nil {
		return nil, fmt.Errorf("load repository: %w", err)
	}
	repositoryFS, err := m.files.OpenContext(ctx, repository)
	if err != nil {
		return nil, err
	}
	observation, inspectErr := repositoryFS.InspectMedia(ctx, repositoryPath, storage.HashFull)
	closeErr := repositoryFS.Close()
	if inspectErr != nil {
		if errors.Is(inspectErr, storage.ErrRepositoryFileUnstable) {
			return m.prepareReobserve(ctx, node, repository, repositoryPath, resolvedOwnerID)
		}
		return nil, errors.Join(inspectErr, closeErr)
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if node.StabilityToken == nil || observation.ObservationToken != *node.StabilityToken {
		return &HashPreparation{Node: node, OwnerID: resolvedOwnerID, ProjectedPath: projectedPath, Observation: observation, Reobserve: true}, nil
	}
	if observation.ContentHash == nil {
		return nil, errors.New("stable full hash did not produce content identity")
	}
	return &HashPreparation{Node: node, OwnerID: resolvedOwnerID, ProjectedPath: projectedPath, Observation: observation, AssetType: string(validation.AssetType), MimeType: validation.MimeType}, nil
}

func (m *HashPreparer) prepareReobserve(
	ctx context.Context,
	node repo.RepositoryNode,
	repository repo.Repository,
	repositoryPath storage.RepositoryPath,
	resolvedOwnerID int32,
) (*HashPreparation, error) {
	repositoryFS, err := m.files.OpenContext(ctx, repository)
	if err != nil {
		return nil, err
	}
	observation, inspectErr := repositoryFS.InspectMedia(ctx, repositoryPath, storage.HashNone)
	closeErr := repositoryFS.Close()
	if inspectErr != nil || closeErr != nil {
		return nil, errors.Join(inspectErr, closeErr)
	}
	return &HashPreparation{Node: node, OwnerID: resolvedOwnerID, Observation: observation, Reobserve: true}, nil
}

func (m *HashPreparer) getNodeByID(ctx context.Context, nodeID uuid.UUID) (repo.RepositoryNode, error) {
	row := m.queryRow.QueryRowContext(ctx, `
		SELECT repository_id
		FROM repository_nodes
		WHERE node_id = ?`, nodeID)
	var repositoryID uuid.UUID
	if err := row.Scan(&repositoryID); err != nil {
		return repo.RepositoryNode{}, err
	}
	return m.reader.GetRepositoryNode(ctx, repo.GetRepositoryNodeParams{
		RepositoryID: repositoryID, NodeID: nodeID,
	})
}
