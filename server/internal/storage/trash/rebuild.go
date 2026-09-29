package trash

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"server/internal/db/dbtypes"
	"server/internal/storage"
	"server/internal/storage/scan"
)

// RebuildAll rebuilds the Trash of every reachable repository from its info
// sidecars; see Rebuild.
func (s *Service) RebuildAll(ctx context.Context) (int, error) {
	repositories, err := s.reader.ListRepositories(ctx)
	if err != nil {
		return 0, fmt.Errorf("list repositories for trash rebuild: %w", err)
	}
	total := 0
	var failures []error
	for _, repository := range repositories {
		if repository.Reachability != dbtypes.RepositoryReachabilityActive {
			continue
		}
		adopted, err := s.Rebuild(ctx, repository.RepoID)
		total += adopted
		if err != nil {
			failures = append(failures, err)
		}
	}
	return total, errors.Join(failures...)
}

// Rebuild records every trashed file of a repository that the catalog does
// not know, from the info sidecars in its trash. The trash is
// self-describing, so a catalog rebuilt from disk, or a repository opened into
// a new catalog, lists the same Trash. A sidecar in a format this build does
// not read is reported and left alone; a sidecar whose file is gone is
// ignored.
func (s *Service) Rebuild(ctx context.Context, repositoryID uuid.UUID) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// A move a crash interrupted has a sidecar before its catalog commit;
	// recovery owns it, so the rebuild waits until no journal is open.
	operations, err := s.reader.ListIncompleteLifecycleOperations(ctx)
	if err != nil {
		return 0, err
	}
	for _, operation := range operations {
		if operation.Kind == storage.LifecycleKindTrashAssets || operation.Kind == storage.LifecycleKindRestoreAssets {
			return 0, nil
		}
	}
	repository, err := s.reader.GetRepository(ctx, repositoryID)
	if err != nil {
		return 0, err
	}
	if repository.DefaultOwnerID == nil || *repository.DefaultOwnerID <= 0 {
		return 0, fmt.Errorf("repository %s has no owner for trashed files", repositoryID)
	}
	opened, err := s.openRepositories(ctx, []uuid.UUID{repositoryID})
	if err != nil {
		return 0, err
	}
	defer opened.close()
	files := opened[repositoryID]
	if exists, err := files.Exists(storage.TrashInfoDirectory()); err != nil || !exists {
		return 0, err
	}
	sidecars, err := files.ListPrivateFiles(ctx, storage.TrashInfoDirectory())
	if err != nil {
		return 0, fmt.Errorf("list trash sidecars: %w", err)
	}
	var records []scan.TrashRecord
	adopted := 0
	flush := func() error {
		if len(records) == 0 {
			return nil
		}
		count, err := s.scanner.AdoptTrash(ctx, repositoryID, *repository.DefaultOwnerID, records)
		adopted += count
		records = records[:0]
		return err
	}
	for _, file := range sidecars {
		if path.Dir(file.Path.String()) != storage.TrashInfoDirectory().String() || !strings.HasSuffix(file.Path.String(), ".json") {
			continue
		}
		data, err := files.ReadPrivateFile(file.Path)
		if err != nil {
			return adopted, err
		}
		sidecar, err := ParseSidecar(data)
		if err != nil {
			s.logger.Warn("trash sidecar is not readable by this build; left as is",
				zap.String("repository_id", repositoryID.String()), zap.String("path", file.Path.String()), zap.Error(err))
			continue
		}
		trashID := uuid.MustParse(sidecar.TrashID)
		trashed, err := storage.TrashFilePath(trashID, sidecar.Name())
		if err != nil {
			continue
		}
		if exists, err := files.Exists(trashed); err != nil || !exists {
			continue
		}
		records = append(records, scan.TrashRecord{
			TrashID: trashID, OriginalPath: sidecar.OriginalPath, HashAlgorithm: sidecar.HashAlgorithm,
			ContentHash: sidecar.ContentHash, Size: sidecar.Size, MtimeNs: sidecar.MtimeNs, DeletedAt: sidecar.DeletedAt,
		})
		if len(records) == purgeBatch {
			if err := flush(); err != nil {
				return adopted, err
			}
		}
	}
	return adopted, flush()
}
