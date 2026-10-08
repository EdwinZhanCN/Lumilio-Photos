package scan

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
)

// Receipt acknowledges a scan request: a new queued scan, or the queued scan
// the request joined.
type Receipt struct {
	ScanID       uuid.UUID
	RepositoryID uuid.UUID
	Trigger      string
	Status       string
	Inserted     bool
	Coalesced    bool
}

// RequestScan queues a full scan of a repository and returns its receipt.
func (s *Scanner) RequestScan(ctx context.Context, repositoryID, trigger, requestedBy string) (Receipt, error) {
	id, err := uuid.Parse(repositoryID)
	if err != nil {
		return Receipt{}, err
	}
	if _, err := s.reader.GetRepository(ctx, id); err != nil {
		return Receipt{}, err
	}
	queued, coalesced, err := s.Request(ctx, id, trigger, "", requestedBy, time.Time{})
	if err != nil {
		return Receipt{}, err
	}
	return Receipt{
		ScanID: queued.ScanID, RepositoryID: id, Trigger: queued.Trigger, Status: queued.Status,
		Inserted: !coalesced, Coalesced: coalesced,
	}, nil
}

// RequestAllPeriodic queues a periodic full scan of every repository.
func (s *Scanner) RequestAllPeriodic(ctx context.Context) error {
	repositories, err := s.reader.ListRepositories(ctx)
	if err != nil {
		return err
	}
	var errs error
	for _, repository := range repositories {
		if _, _, err := s.Request(ctx, repository.RepoID, TriggerPeriodic, "", "", time.Time{}); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

func (s *Scanner) GetScan(ctx context.Context, repositoryID, scanID string) (repo.RepositoryScan, error) {
	repositoryUUID, scanUUID, err := parseScanIDs(repositoryID, scanID)
	if err != nil {
		return repo.RepositoryScan{}, err
	}
	scan, err := s.reader.GetRepositoryScan(ctx, scanUUID)
	if err == nil && scan.RepositoryID != repositoryUUID {
		return repo.RepositoryScan{}, sql.ErrNoRows
	}
	return scan, err
}

func (s *Scanner) GetLatestScan(ctx context.Context, repositoryID string) (repo.RepositoryScan, error) {
	id, err := uuid.Parse(repositoryID)
	if err != nil {
		return repo.RepositoryScan{}, err
	}
	return s.reader.GetLatestRepositoryScan(ctx, id)
}

func (s *Scanner) ListScans(ctx context.Context, repositoryID string, limit, offset int32) ([]repo.RepositoryScan, error) {
	id, err := uuid.Parse(repositoryID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.reader.ListRepositoryScans(ctx, repo.ListRepositoryScansParams{
		RepositoryID: id, Limit: int64(limit), Offset: int64(max(offset, 0)),
	})
}

// CancelScan asks a scan to stop at its next turn; a queued scan ends at
// once. Entries the scan already committed stay, and a cancelled scan marks
// nothing further missing.
func (s *Scanner) CancelScan(ctx context.Context, repositoryID, scanID string) (repo.RepositoryScan, error) {
	repositoryUUID, scanUUID, err := parseScanIDs(repositoryID, scanID)
	if err != nil {
		return repo.RepositoryScan{}, err
	}
	var scan repo.RepositoryScan
	err = s.writer.WithTx(ctx, catalogtx.OperationRepositoryScanFinish, func(_ *sql.Tx, queries *repo.Queries) error {
		now := dbtypes.NewTimestamp(s.config.Now())
		var err error
		scan, err = queries.RequestRepositoryScanCancellation(ctx, repo.RequestRepositoryScanCancellationParams{
			UpdatedAt: now, ScanID: scanUUID, RepositoryID: repositoryUUID,
		})
		if err != nil || scan.Status != StatusQueued {
			return err
		}
		if _, err := queries.FinishRepositoryScan(ctx, repo.FinishRepositoryScanParams{
			Status: StatusCancelled, FinishedAt: now, ScanID: scanUUID,
		}); err != nil {
			return err
		}
		scan, err = queries.GetRepositoryScan(ctx, scanUUID)
		return err
	})
	return scan, err
}

func parseScanIDs(repositoryID, scanID string) (uuid.UUID, uuid.UUID, error) {
	repositoryUUID, err := uuid.Parse(repositoryID)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	scanUUID, err := uuid.Parse(scanID)
	return repositoryUUID, scanUUID, err
}
