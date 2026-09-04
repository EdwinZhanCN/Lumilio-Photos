// Package artifact owns the one immutable publication contract for derived
// pipeline files inside a registered repository.
package artifact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"server/internal/storage"
)

const privateRoot = ".lumilio/artifacts"

type Identity struct{ SourceFence, Stage, PipelineVersion, Name string }

func (i Identity) validate() error {
	for _, value := range []string{i.SourceFence, i.Stage, i.PipelineVersion, i.Name} {
		if value == "" || value == "." || value == ".." || strings.ContainsAny(value, `/\\`) {
			return errors.New("artifact identity components must be non-empty path segments")
		}
	}
	return nil
}

func (i Identity) Path() (storage.RepositoryPath, error) {
	if err := i.validate(); err != nil {
		return storage.RepositoryPath{}, err
	}
	return storage.ParsePrivateRepositoryPath(path.Join(privateRoot, i.SourceFence, i.Stage, i.PipelineVersion, i.Name))
}

type Published struct {
	Path, SHA256 string
	Size         int64
}

type Store struct{ files *storage.RepositoryFS }

func NewStore(files *storage.RepositoryFS) (*Store, error) {
	if files == nil {
		return nil, errors.New("artifact store requires a repository filesystem")
	}
	return &Store{files: files}, nil
}

// Publish consumes the complete candidate on every attempt. RepositoryFS then
// atomically creates the immutable name. Once that name exists, its first
// complete file is canonical for the logical identity: pipeline tools may
// recompute byte-different but equivalent output after a crash, so a retry
// adopts the already-published artifact instead of replacing it.
func (s *Store) Publish(ctx context.Context, identity Identity, reader io.Reader) (Published, error) {
	if ctx == nil {
		return Published{}, errors.New("artifact publish context is nil")
	}
	if reader == nil {
		return Published{}, errors.New("artifact reader is nil")
	}
	if err := ctx.Err(); err != nil {
		return Published{}, err
	}
	if s == nil || s.files == nil {
		return Published{}, errors.New("artifact store is not configured")
	}
	finalPath, err := identity.Path()
	if err != nil {
		return Published{}, err
	}
	directory, err := storage.ParsePrivateRepositoryPath(path.Dir(finalPath.String()))
	if err != nil {
		return Published{}, err
	}
	if err := s.files.MkdirAllPrivate(directory, 0o755); err != nil {
		return Published{}, fmt.Errorf("create artifact directory: %w", err)
	}
	digest := sha256.New()
	written, err := s.files.WritePrivateFileImmutable(finalPath, io.TeeReader(&contextReader{ctx: ctx, reader: reader}, digest), 0o644)
	if err != nil {
		if errors.Is(err, storage.ErrRepositoryImmutableConflict) {
			existing, inspectErr := s.inspectPublished(ctx, finalPath)
			if inspectErr != nil {
				return Published{}, fmt.Errorf("adopt existing immutable artifact: %w", inspectErr)
			}
			return existing, nil
		}
		return Published{}, fmt.Errorf("publish immutable artifact: %w", err)
	}
	return Published{Path: finalPath.String(), SHA256: hex.EncodeToString(digest.Sum(nil)), Size: written}, nil
}

func (s *Store) inspectPublished(ctx context.Context, repositoryPath storage.RepositoryPath) (Published, error) {
	opened, err := s.files.OpenPrivate(repositoryPath)
	if err != nil {
		return Published{}, err
	}
	defer opened.Close()
	info, err := opened.Stat()
	if err != nil {
		return Published{}, err
	}
	if !info.Mode().IsRegular() {
		return Published{}, fmt.Errorf("%w: %s is not a regular file", storage.ErrRepositoryImmutableConflict, repositoryPath.String())
	}
	digest := sha256.New()
	size, err := io.Copy(digest, &contextReader{ctx: ctx, reader: opened})
	if err != nil {
		return Published{}, err
	}
	if size == 0 {
		return Published{}, fmt.Errorf("%w: %s is empty", storage.ErrRepositoryImmutableConflict, repositoryPath.String())
	}
	return Published{Path: repositoryPath.String(), SHA256: hex.EncodeToString(digest.Sum(nil)), Size: size}, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

// RemoveOrphans deletes only unreferenced files older than the grace period
// from the canonical artifact subtree. Catalog paths use the same normalized
// repository-relative form as Published.Path.
func (s *Store) RemoveOrphans(ctx context.Context, referenced map[string]struct{}, grace time.Duration, now time.Time) (int, error) {
	if ctx == nil {
		return 0, errors.New("artifact cleanup context is nil")
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if s == nil || s.files == nil {
		return 0, errors.New("artifact store is not configured")
	}
	if grace < 0 {
		return 0, errors.New("artifact cleanup grace must be non-negative")
	}
	root, err := storage.ParsePrivateRepositoryPath(privateRoot)
	if err != nil {
		return 0, err
	}
	files, err := s.files.ListPrivateFiles(ctx, root)
	if err != nil {
		return 0, err
	}
	cutoff := now.Add(-grace)
	removed := 0
	for _, file := range files {
		if _, keep := referenced[file.Path.String()]; keep || file.ModTime.After(cutoff) {
			continue
		}
		if err := s.files.RemovePrivate(file.Path); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}
