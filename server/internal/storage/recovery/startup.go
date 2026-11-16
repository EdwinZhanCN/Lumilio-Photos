package recovery

import (
	"context"
	"errors"
	"fmt"

	"server/internal/catalog/dbtypes"
	"server/internal/catalog/repo"
	"server/internal/storage"
	"server/internal/storage/layout"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// StartupResult is what [Service.PrepareRepository] did to one repository.
type StartupResult struct {
	// Repaired lists the missing layout directories it created.
	Repaired []string
	// ReceiptID names the startup receipt of the work it quarantined, or "".
	ReceiptID string
	// Quarantined counts the files moved under that receipt.
	Quarantined int
}

// PrepareRepositories runs [Service.PrepareRepository] on every reachable
// repository. It runs at startup, before uploads, workers, and requests
// resume, so nothing else writes to work/ meanwhile. A repository that
// fails is logged and tried again at the next start.
func (s *Service) PrepareRepositories(ctx context.Context) error {
	repositories, err := s.reader.ListRepositories(ctx)
	if err != nil {
		return fmt.Errorf("list repositories to prepare: %w", err)
	}
	var failures []error
	for _, repository := range repositories {
		if repository.Reachability != dbtypes.RepositoryReachabilityActive {
			continue
		}
		result, err := s.PrepareRepository(ctx, repository)
		if err != nil {
			s.logger.Warn("repository not prepared; the next start retries", zap.String("repository_id", repository.RepoID.String()), zap.Error(err))
			failures = append(failures, err)
			continue
		}
		if len(result.Repaired) > 0 || result.Quarantined > 0 {
			s.logger.Info("repository prepared", zap.String("repository_id", repository.RepoID.String()),
				zap.Strings("layout_repaired", result.Repaired), zap.String("receipt_id", result.ReceiptID),
				zap.Int("quarantined", result.Quarantined))
		}
	}
	return errors.Join(failures...)
}

// PrepareRepository validates the repository's layout, creates missing
// directories, and quarantines what an earlier run left in work/.
//
// An entry in work/ belongs to no operation once the Server restarts, except
// an upload session that a pending staging commit names or whose manifest
// lets the browser resume it. Every other file moves under a startup receipt
// in quarantine/recovery/, where the Storage page lists it for export or
// deletion; nothing is deleted. The receipt is written before the moves, and
// a later start finishes the moves an interrupted one recorded.
func (s *Service) PrepareRepository(ctx context.Context, repository repo.Repository) (StartupResult, error) {
	var result StartupResult
	files, err := s.files.OpenContext(ctx, repository)
	if err != nil {
		return result, err
	}
	defer files.Close()

	report, err := layout.Validate(repository.Path)
	if err != nil {
		return result, err
	}
	if !report.Repairable() {
		return result, fmt.Errorf("repository layout cannot be repaired: wrong kind %v, inaccessible %v", report.WrongKind, report.Inaccessible)
	}
	if len(report.Missing) > 0 {
		if err := layout.Init(repository.Path); err != nil {
			return result, fmt.Errorf("repair repository layout: %w", err)
		}
		result.Repaired = report.Missing
	}

	keep, err := s.resumableSessions(ctx, files, repository.RepoID)
	if err != nil {
		return result, err
	}
	planned, err := plannedMoves(ctx, files)
	if err != nil {
		return result, err
	}
	workDir, err := layout.ClassDir(layout.Work)
	if err != nil {
		return result, err
	}
	work, err := files.ListPrivateFiles(ctx, workDir)
	if err != nil {
		return result, err
	}
	receiptID := uuid.New()
	receipt := storage.RecoveryReceipt{
		Format: storage.RecoveryReceiptFormat, ReceiptID: receiptID.String(), Event: storage.RecoveryEventStartup,
		RecordedAt: s.now(), Actor: "system", SourceRepositoryID: repository.RepoID.String(), RepositoryID: repository.RepoID.String(),
		Items: []storage.RecoveryItem{},
	}
	moves := map[layout.Path]layout.Path{}
	for _, file := range work {
		if to, ok := planned[file.Path]; ok {
			moves[file.Path] = to
			continue
		}
		if staged, err := layout.ParseStaged(file.Path.String()); err == nil {
			if session, ok := layout.StagedSession(staged); ok && keep(session) {
				continue
			}
		}
		to, err := layout.RecoveryMoved(receiptID, file.Path)
		if err != nil {
			return result, err
		}
		moves[file.Path] = to
		receipt.Items = append(receipt.Items, storage.RecoveryItem{
			ItemID: storage.RecoveryItemID(receipt.ReceiptID, file.Path.String()), Class: storage.PrivateClassStaging,
			Path: file.Path.String(), Size: file.Size, MtimeNs: file.ModTime.UnixNano(),
			Disposition: storage.DispositionMoved, MovedTo: to.String(),
		})
	}
	if len(receipt.Items) > 0 {
		if err := storage.WriteRecoveryReceipt(files, receipt); err != nil {
			return result, fmt.Errorf("write startup receipt: %w", err)
		}
		result.ReceiptID, result.Quarantined = receipt.ReceiptID, len(receipt.Items)
	}
	for from, to := range moves {
		if err := quarantineWorkFile(files, from, to); err != nil {
			return result, err
		}
	}
	return result, nil
}

// resumableSessions reports the upload sessions something can still finish:
// one a pending staging commit names, or one with a manifest.
func (s *Service) resumableSessions(ctx context.Context, files *storage.RepositoryFS, repositoryID uuid.UUID) (func(uuid.UUID) bool, error) {
	pending, err := s.reader.ListPendingRepositoryStagingPaths(ctx, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("list pending staging commits: %w", err)
	}
	committed := map[uuid.UUID]bool{}
	for _, stored := range pending {
		if staged, err := layout.ParseStaged(stored); err == nil {
			if session, ok := layout.StagedSession(staged); ok {
				committed[session] = true
			}
		}
	}
	return func(session uuid.UUID) bool {
		if committed[session] {
			return true
		}
		manifest, err := layout.UploadManifest(session)
		if err != nil {
			return false
		}
		_, err = files.StatPrivate(manifest)
		return err == nil
	}, nil
}

// plannedMoves maps each work file an earlier receipt moved to where the
// receipt says it went, so a start that was interrupted is finished.
func plannedMoves(ctx context.Context, files *storage.RepositoryFS) (map[layout.Path]layout.Path, error) {
	receipts, _, err := storage.ListRecoveryReceipts(ctx, files)
	if err != nil {
		return nil, err
	}
	planned := map[layout.Path]layout.Path{}
	for _, receipt := range receipts {
		for _, item := range receipt.Items {
			if item.Disposition != storage.DispositionMoved || item.MovedTo == "" {
				continue
			}
			from, fromErr := layout.Parse(item.Path)
			to, toErr := layout.Parse(item.MovedTo)
			if fromErr == nil && toErr == nil {
				planned[from] = to
			}
		}
	}
	return planned, nil
}

// quarantineWorkFile moves one file and drops the upload session directory
// it emptied. A move a crash cut between link and unlink is finished.
func quarantineWorkFile(files *storage.RepositoryFS, from, to layout.Path) error {
	directory, err := to.Dir()
	if err != nil {
		return err
	}
	if err := files.MkdirAllPrivate(directory, 0o700); err != nil {
		return err
	}
	if same, err := files.SameFile(from, to); err == nil && same {
		err = files.RemovePrivate(from)
		if err != nil {
			return err
		}
	} else if err := files.MovePrivateFile(from, to); err != nil {
		return fmt.Errorf("quarantine %s: %w", from, err)
	}
	if _, err := layout.ParseStaged(from.String()); err == nil {
		if session, err := from.Dir(); err == nil {
			// A session directory that still holds files stays.
			_ = files.RemovePrivate(session)
		}
	}
	return nil
}
