package storage

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"

	"github.com/google/uuid"
)

const minimumCapacitySafetyMargin uint64 = 512 << 20 // 512 MiB

// CapacitySafetyMargin is the write reserve retained on a repository's target
// volume. Exposing the policy calculation lets diagnostics explain the same
// threshold that the write gate enforces.
func CapacitySafetyMargin(totalBytes uint64) uint64 {
	margin := totalBytes / 20
	if margin < minimumCapacitySafetyMargin {
		return minimumCapacitySafetyMargin
	}
	return margin
}

var (
	ErrRepositoryReadOnly = errors.New("repository storage is read-only")
	ErrInsufficientSpace  = errors.New("repository storage has insufficient free space")
)

type CapacityDecision struct {
	Allowed        bool
	CapacityKnown  bool
	Writable       bool
	ExpectedBytes  uint64
	SafetyMargin   uint64
	RequiredBytes  uint64
	AvailableBytes uint64
	TotalBytes     uint64
	RepositoryID   string
	RepositoryPath string
}

// CheckRepositoryWriteCapacity is the single pre-write gate for sources whose
// size is known and the sampling point for unknown/streaming sources. The
// safety reserve is scoped to the target volume rather than a global minimum.
func (rm *DefaultRepositoryManager) CheckRepositoryWriteCapacity(ctx context.Context, repositoryID string, expectedBytes uint64) (CapacityDecision, error) {
	id, err := uuid.Parse(repositoryID)
	if err != nil {
		return CapacityDecision{}, fmt.Errorf("invalid repository ID: %w", err)
	}
	repository, err := rm.queries.GetRepository(ctx, id)
	if err != nil {
		return CapacityDecision{}, err
	}
	if err := rm.files.ValidateRepositoryParent(ctx, repository); err != nil {
		return CapacityDecision{}, err
	}
	info := InspectStoragePath(repository.Path)
	decision := capacityDecision(repository.RepoID.String(), repository.Path, info, expectedBytes)
	if !decision.Writable {
		return decision, ErrRepositoryReadOnly
	}
	if !decision.Allowed {
		_ = rm.pauseRepositoryForLowSpace(ctx, repository)
		return decision, fmt.Errorf("%w: need %d bytes including %d-byte safety margin, have %d",
			ErrInsufficientSpace, decision.RequiredBytes, decision.SafetyMargin, decision.AvailableBytes)
	}
	return decision, nil
}

func capacityDecision(repositoryID, path string, info StoragePathInfo, expectedBytes uint64) CapacityDecision {
	decision := CapacityDecision{
		Allowed: true, CapacityKnown: info.CapacityKnown, Writable: info.Writable,
		ExpectedBytes: expectedBytes, AvailableBytes: info.AvailableBytes, TotalBytes: info.TotalBytes,
		RepositoryID: repositoryID, RepositoryPath: path,
	}
	if !info.Writable {
		decision.Allowed = false
		return decision
	}
	if !info.CapacityKnown {
		return decision
	}
	margin := CapacitySafetyMargin(info.TotalBytes)
	decision.SafetyMargin = margin
	if expectedBytes > math.MaxUint64-margin {
		decision.RequiredBytes = math.MaxUint64
	} else {
		decision.RequiredBytes = expectedBytes + margin
	}
	decision.Allowed = decision.AvailableBytes >= decision.RequiredBytes
	return decision
}

func (rm *DefaultRepositoryManager) pauseRepositoryForLowSpace(ctx context.Context, repository repo.Repository) error {
	if repository.Activity == dbtypes.RepositoryActivityPaused && repository.PauseReason != "low_space" {
		return nil
	}
	_, err := rm.queries.PauseRepositoryForLowSpace(ctx, repo.PauseRepositoryForLowSpaceParams{
		RepoID:    repository.RepoID,
		UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	})
	return err
}

// ReconcileRepositoryCapacity is deliberately conservative: it pauses active
// work when the reserve is crossed, but never automatically resumes a paused
// repository because pause may also represent an administrator/lifecycle gate.
func (rm *DefaultRepositoryManager) ReconcileRepositoryCapacity(ctx context.Context) error {
	repositories, err := rm.queries.ListRepositories(ctx)
	if err != nil {
		return err
	}
	for _, repository := range repositories {
		if repository.Reachability != dbtypes.RepositoryReachabilityActive {
			continue
		}
		info := InspectStoragePath(repository.Path)
		decision := capacityDecision(repository.RepoID.String(), repository.Path, info, 0)
		if repository.Activity == dbtypes.RepositoryActivityPaused {
			if _, err := rm.resumeRepositoryAfterCapacityRecovery(ctx, repository, info); err != nil {
				return fmt.Errorf("resume repository %s after capacity recovery: %w", repository.RepoID, err)
			}
			continue
		}
		if decision.Writable && decision.Allowed {
			continue
		}
		if err := rm.pauseRepositoryForLowSpace(ctx, repository); err != nil {
			return fmt.Errorf("pause repository %s after capacity check: %w", repository.RepoID, err)
		}
	}
	return nil
}

func (rm *DefaultRepositoryManager) resumeRepositoryAfterCapacityRecovery(ctx context.Context, repository repo.Repository, info StoragePathInfo) (bool, error) {
	decision := capacityDecision(repository.RepoID.String(), repository.Path, info, 0)
	if repository.Activity != dbtypes.RepositoryActivityPaused || repository.PauseReason != "low_space" ||
		!decision.CapacityKnown || !decision.Writable || !decision.Allowed {
		return false, nil
	}
	if _, err := rm.queries.ResumeRepositoryAfterLowSpace(ctx, repo.ResumeRepositoryAfterLowSpaceParams{
		RepoID: repository.RepoID, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	}); err != nil {
		return false, err
	}
	return true, nil
}
