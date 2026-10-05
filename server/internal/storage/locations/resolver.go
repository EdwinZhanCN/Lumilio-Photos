// Package locations resolves logical Assets to a present physical file
// immediately before media I/O. It holds the RepositoryFS lifecycle lease for
// the lifetime of the returned capability and falls through unavailable
// copies without changing catalog state.
package locations

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"

	"server/internal/db/repo"
	"server/internal/storage"
)

var ErrAssetUnavailable = errors.New("asset has no available present file")

type Reader interface {
	ListPresentRepositoryEntriesForAsset(context.Context, uuid.NullUUID) ([]repo.RepositoryEntry, error)
	GetRepository(context.Context, uuid.UUID) (repo.Repository, error)
}

type Resolver struct {
	reader Reader
	files  *storage.RepositoryFSFactory
}

func NewResolver(reader Reader, files *storage.RepositoryFSFactory) *Resolver {
	return &Resolver{reader: reader, files: files}
}

// OpenedMedia is one opened original. Entry is the scan index row the file
// was resolved from; its stat tuple is what the catalog last proved.
type OpenedMedia struct {
	File       *os.File
	Repository *storage.RepositoryFS
	Catalog    repo.Repository
	Entry      repo.RepositoryEntry
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

// MatchesCatalog reports whether a fresh observation of the opened file still
// has the stat tuple the scan index recorded for it.
func (opened *OpenedMedia) MatchesCatalog(observation storage.FileObservation) bool {
	entry := opened.Entry
	return entry.Size == observation.Size && entry.MtimeNs == observation.ModTimeNS &&
		equalInt64(entry.CtimeNs, observation.ChangeTimeNS) && equalString(entry.FileID, observation.FileIdentity)
}

func equalInt64(left, right *int64) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

func equalString(left, right *string) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

func (r *Resolver) OpenAsset(ctx context.Context, assetID uuid.UUID) (*OpenedMedia, error) {
	if r == nil || r.reader == nil || r.files == nil {
		return nil, ErrAssetUnavailable
	}
	entries, err := r.reader.ListPresentRepositoryEntriesForAsset(ctx, uuid.NullUUID{UUID: assetID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list present asset entries: %w", err)
	}
	var unavailable error
	for _, entry := range entries {
		repositoryPath, err := storage.ParseUserMediaPath(entry.Path)
		if err != nil {
			unavailable = errors.Join(unavailable, err)
			continue
		}
		repository, err := r.reader.GetRepository(ctx, entry.RepositoryID)
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
		return &OpenedMedia{File: file, Repository: repositoryFS, Catalog: repository, Entry: entry, Path: repositoryPath}, nil
	}
	if unavailable != nil {
		return nil, fmt.Errorf("%w: %w", ErrAssetUnavailable, unavailable)
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
