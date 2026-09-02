// Package locations resolves logical Assets to an active physical occurrence
// immediately before media I/O. It holds the RepositoryFS lifecycle lease for
// the lifetime of the returned capability and can fall through unavailable
// copies without changing catalog state.
package locations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"

	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/roe/nodegraph"
)

var ErrAssetUnavailable = errors.New("asset has no available active location")

type Resolver struct {
	reader   Reader
	queryRow QueryRower
	files    *storage.RepositoryFSFactory
}

type Reader interface {
	ListActiveAssetLocations(context.Context, uuid.UUID) ([]repo.AssetLocation, error)
	GetRepositoryNode(context.Context, repo.GetRepositoryNodeParams) (repo.RepositoryNode, error)
	GetRepository(context.Context, uuid.UUID) (repo.Repository, error)
}

type QueryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func NewResolver(reader Reader, queryRow QueryRower, files *storage.RepositoryFSFactory) *Resolver {
	return &Resolver{reader: reader, queryRow: queryRow, files: files}
}

type OpenedMedia struct {
	File       *os.File
	Repository *storage.RepositoryFS
	Catalog    repo.Repository
	Node       repo.RepositoryNode
	Location   repo.AssetLocation
	Path       storage.RepositoryPath
}

func (opened *OpenedMedia) Close() error {
	if opened == nil {
		return nil
	}
	var fileErr, repositoryErr error
	if opened.File != nil {
		fileErr = opened.File.Close()
		opened.File = nil
	}
	if opened.Repository != nil {
		repositoryErr = opened.Repository.Close()
		opened.Repository = nil
	}
	return errors.Join(fileErr, repositoryErr)
}

func (r *Resolver) OpenAsset(ctx context.Context, assetID uuid.UUID) (*OpenedMedia, error) {
	if r == nil || r.reader == nil || r.queryRow == nil || r.files == nil {
		return nil, ErrAssetUnavailable
	}
	locations, err := r.reader.ListActiveAssetLocations(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("list active asset locations: %w", err)
	}
	var unavailable error
	for _, location := range locations {
		node, err := r.nodeByID(ctx, location.NodeID)
		if err != nil || node.Lifecycle != "active" {
			unavailable = errors.Join(unavailable, err)
			continue
		}
		projectedPath, err := nodegraph.ProjectPath(ctx, r.reader, node.RepositoryID, node.NodeID)
		if err != nil {
			unavailable = errors.Join(unavailable, err)
			continue
		}
		repositoryPath, err := storage.ParseUserMediaPath(projectedPath)
		if err != nil {
			unavailable = errors.Join(unavailable, err)
			continue
		}
		repository, err := r.reader.GetRepository(ctx, node.RepositoryID)
		if err != nil {
			unavailable = errors.Join(unavailable, err)
			continue
		}
		repositoryFS, err := r.files.OpenContext(ctx, repository)
		if err != nil {
			unavailable = errors.Join(unavailable, err)
			continue
		}
		file, err := repositoryFS.OpenMedia(repositoryPath)
		if err != nil {
			_ = repositoryFS.Close()
			unavailable = errors.Join(unavailable, err)
			continue
		}
		return &OpenedMedia{
			File: file, Repository: repositoryFS, Catalog: repository, Node: node,
			Location: location, Path: repositoryPath,
		}, nil
	}
	if unavailable != nil {
		return nil, fmt.Errorf("%w: %v", ErrAssetUnavailable, unavailable)
	}
	return nil, ErrAssetUnavailable
}

func (r *Resolver) LocalAssetPath(ctx context.Context, assetID uuid.UUID) (*OpenedMedia, string, error) {
	opened, err := r.OpenAsset(ctx, assetID)
	if err != nil {
		return nil, "", err
	}
	localPath, err := opened.Repository.LocalMediaPath(opened.Path)
	if err != nil {
		_ = opened.Close()
		return nil, "", err
	}
	return opened, localPath, nil
}

func (r *Resolver) nodeByID(ctx context.Context, nodeID uuid.UUID) (repo.RepositoryNode, error) {
	var repositoryID uuid.UUID
	if err := r.queryRow.QueryRowContext(ctx,
		"SELECT repository_id FROM repository_nodes WHERE node_id = ?", nodeID,
	).Scan(&repositoryID); err != nil {
		return repo.RepositoryNode{}, err
	}
	return r.reader.GetRepositoryNode(ctx, repo.GetRepositoryNodeParams{
		RepositoryID: repositoryID, NodeID: nodeID,
	})
}
