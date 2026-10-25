package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// StorageViewAssetCounter loads cheap catalog counts for the admin storage view.
type StorageViewAssetCounter interface {
	CountActiveAssetsByRepository(ctx context.Context, repositoryID uuid.UUID) (int64, error)
}

type StorageLifecycleCounter interface {
	CountRepositoryLifecycle(context.Context, uuid.UUID) (repo.CountRepositoryLifecycleRow, error)
}

// StorageViewScanReader loads latest verification summaries when available.
type StorageViewScanReader interface {
	GetLatestScan(ctx context.Context, repositoryID string) (repo.RepositoryScan, error)
}

// StorageHandler serves authenticated storage selectors and the admin read model.
type StorageHandler struct {
	repoManager  storage.RepositoryManager
	assetCounter StorageViewAssetCounter
	scanReader   StorageViewScanReader
	recovery     StorageRecoveryService
}

func NewStorageHandler(
	repoManager storage.RepositoryManager,
	assetCounter StorageViewAssetCounter,
	scanReader StorageViewScanReader,
) *StorageHandler {
	return &StorageHandler{
		repoManager:  repoManager,
		assetCounter: assetCounter,
		scanReader:   scanReader,
	}
}

// GetStorageTargets returns admission-projected browse/upload selectors for any
// authenticated user. Foreground list reads never probe disks.
// @Summary List storage targets
// @Description Return repository selectors with read and upload admission. Omits paths, Storage Locations, capacity, and verification.
// @Tags storage
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.StorageTargetsResponseDTO
// @Failure 401 {object} api.ProblemResponse
// @Router /api/v1/storage/targets [get]
func (h *StorageHandler) GetStorageTargets(c *gin.Context) {
	if h == nil || h.repoManager == nil {
		api.WriteProblem(c, api.Internal(errors.New("repository manager unavailable")))
		return
	}
	repositories, err := h.repoManager.ListRepositories()
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	targets := make([]dto.StorageTargetDTO, 0, len(repositories))
	for _, repository := range repositories {
		if repository == nil {
			continue
		}
		read := storage.OpenOriginalAdmission(*repository)
		upload := storage.UploadAdmission(*repository, storage.WriteFacts{})
		targets = append(targets, dto.StorageTargetDTO{
			ID:     repository.RepoID.String(),
			Name:   repository.Name,
			Role:   storageTargetRole(repository.Role),
			Read:   admissionDecisionDTO(read),
			Upload: admissionDecisionDTO(upload),
		})
	}
	api.JSONOK(c, dto.StorageTargetsResponseDTO{Targets: targets})
}

// GetStorageView returns the administrator storage read model.
// @Summary Get admin storage view
// @Description Return Storage Locations, repositories, and capacity groups for administration.
// @Tags storage
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.StorageViewResponseDTO
// @Failure 401 {object} api.ProblemResponse
// @Failure 403 {object} api.ProblemResponse
// @Router /api/v1/storage/view [get]
func (h *StorageHandler) GetStorageView(c *gin.Context) {
	if h == nil || h.repoManager == nil {
		api.WriteProblem(c, api.Internal(errors.New("repository manager unavailable")))
		return
	}
	ctx := c.Request.Context()
	storageLocations, err := h.repoManager.ListStorageLocations(ctx)
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}
	repositories, err := h.repoManager.ListRepositories()
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}

	locations := make([]dto.StorageLocationViewDTO, 0, len(storageLocations))
	for _, storageLocation := range storageLocations {
		impact, impactErr := h.repoManager.PreviewStorageLocationRemoval(ctx, storageLocation.StorageLocationID.String())
		if impactErr != nil {
			api.WriteProblem(c, api.Internal(impactErr))
			return
		}
		locationView := dto.StorageLocationViewDTO{
			ID:              storageLocation.StorageLocationID.String(),
			Name:            storageLocation.Name,
			Kind:            string(storageLocation.Kind),
			CanRemove:       impact.CanRemove,
			BlockingReason:  impact.BlockingReason,
			FilesPreserved:  impact.FilesPreserved,
			RepositoryCount: impact.RepositoryCount,
		}
		locationPolicy, policyErr := h.repoManager.StorageLocationActionPolicyFor(ctx, storageLocation)
		if policyErr != nil {
			api.WriteProblem(c, api.Internal(fmt.Errorf("derive storage location actions: %w", policyErr)))
			return
		}
		locationView.Actions, locationView.UnavailableActions = storageLocationActionDTOs(locationPolicy)
		locations = append(locations, locationView)
	}

	capacityGroups, repoGroupIDs, repoStorage := buildStorageCapacityGroups(repositories)
	repoViews := make([]dto.StorageRepositoryViewDTO, 0, len(repositories))
	for index, repository := range repositories {
		if repository == nil {
			continue
		}
		view := dto.StorageRepositoryViewDTO{
			ID:                repository.RepoID.String(),
			Name:              repository.Name,
			Role:              storageTargetRole(repository.Role),
			StorageLocationID: repository.StorageLocationID.String(),
			Reachability:      string(repository.Reachability),
			WritePolicy: dto.StorageRepositoryWritePolicyDTO{
				Activity:    string(repository.Activity),
				PauseReason: repository.PauseReason,
			},
			Activity: string(repository.Activity),
		}
		policy, policyErr := h.repoManager.RepositoryActionPolicyFor(ctx, *repository)
		if policyErr != nil {
			api.WriteProblem(c, api.Internal(fmt.Errorf("derive repository actions: %w", policyErr)))
			return
		}
		view.Actions, view.UnavailableActions = repositoryActionDTOs(policy)
		if groupID, ok := repoGroupIDs[index]; ok {
			view.CapacityGroupID = &groupID
		}
		if sampled, ok := repoStorage[index]; ok {
			view.MountPath = sampled.MountPath
			view.Filesystem = sampled.Filesystem
		}
		if h.assetCounter != nil {
			// Every non-deleted Asset counts, whatever its processing state,
			// so the column agrees with the removal-impact dialog.
			count, countErr := h.assetCounter.CountActiveAssetsByRepository(ctx, repository.RepoID)
			if countErr != nil {
				api.WriteProblem(c, api.Internal(fmt.Errorf("count repository assets: %w", countErr)))
				return
			}
			view.AssetCount = &count
		}
		if counter, ok := h.assetCounter.(StorageLifecycleCounter); ok {
			counts, err := counter.CountRepositoryLifecycle(ctx, repository.RepoID)
			if err != nil {
				api.WriteProblem(c, api.Internal(err))
				return
			}
			view.MissingCount, view.TrashCount, view.TrashBytes = counts.MissingCount, counts.TrashCount, counts.TrashBytes
			view.TrashHeldCount = counts.TrashHeldCount
		}
		if h.recovery != nil && repository.Reachability == dbtypes.RepositoryReachabilityActive {
			// The inventory lives in the Repository's receipts; a Repository
			// that cannot be read right now simply shows no pending items.
			if inventory, err := h.recovery.Inventory(ctx, repository.RepoID); err == nil {
				for _, item := range inventory.Items {
					if item.Present && len(item.Actions) > 0 {
						view.RecoveryItemCount++
					}
				}
			}
		}
		if h.scanReader != nil {
			if scanRun, scanErr := h.scanReader.GetLatestScan(ctx, repository.RepoID.String()); scanErr == nil {
				view.Verification = &dto.StorageRepositoryVerificationSummaryDTO{
					OperationID: scanRun.ScanID.String(),
					Status:      scanRun.Status,
					Mode:        scanRun.Trigger,
				}
			} else if !errors.Is(scanErr, sql.ErrNoRows) {
				api.WriteProblem(c, api.Internal(fmt.Errorf("load verification summary: %w", scanErr)))
				return
			}
		}
		repoViews = append(repoViews, view)
	}

	api.JSONOK(c, dto.StorageViewResponseDTO{
		StorageLocations: locations,
		Repositories:     repoViews,
		CapacityGroups:   capacityGroups,
		ObservedAt:       time.Now().UTC(),
	})
}

func admissionDecisionDTO(decision storage.AdmissionDecision) dto.AdmissionDecisionDTO {
	reasons := decision.Reasons
	if reasons == nil {
		reasons = []string{}
	}
	return dto.AdmissionDecisionDTO{Allowed: decision.Allowed, Reasons: reasons}
}

func storageTargetRole(role dbtypes.RepoRole) string {
	switch role {
	case dbtypes.RepoRolePrimary:
		return "primary"
	default:
		return "regular"
	}
}

// buildStorageCapacityGroups samples every Repository path once and returns the
// capacity pools, each Repository's pool id, and the sampled per-Repository
// storage facts the admin read model needs to name the backing storage.
func buildStorageCapacityGroups(repositories []*repo.Repository) ([]dto.StorageCapacityGroupDTO, map[int]string, map[int]storage.StoragePathInfo) {
	infos := make([]storage.StoragePathInfo, len(repositories))
	repoStorage := make(map[int]storage.StoragePathInfo, len(repositories))
	for index, repository := range repositories {
		if repository == nil {
			continue
		}
		infos[index] = storage.InspectStoragePathReadOnly(repository.Path)
		repoStorage[index] = infos[index]
	}
	grouped := storage.GroupCapacityByBackingStorage(infos)
	capacityGroups := make([]dto.StorageCapacityGroupDTO, 0, len(grouped))
	repoGroupIDs := make(map[int]string, len(repositories))
	for groupIndex, group := range grouped {
		groupID := strconv.Itoa(groupIndex)
		capacityGroups = append(capacityGroups, dto.StorageCapacityGroupDTO{
			ID:             groupID,
			GroupingKnown:  group.GroupingKnown,
			CapacityKnown:  group.CapacityKnown,
			TotalBytes:     group.TotalBytes,
			AvailableBytes: group.AvailableBytes,
		})
		for _, memberIndex := range group.MemberIndices {
			repoGroupIDs[memberIndex] = groupID
		}
	}
	return capacityGroups, repoGroupIDs, repoStorage
}

// storageLocationActionDTOs is repositoryActionDTOs for the kind-aware
// Storage Location policy; both lists are always present.
func storageLocationActionDTOs(policy storage.StorageLocationActionPolicy) ([]string, []dto.RepositoryActionBlockDTO) {
	actions := make([]string, 0, len(policy.Verdicts))
	blocked := make([]dto.RepositoryActionBlockDTO, 0)
	for _, verdict := range policy.Verdicts {
		if verdict.Allowed() {
			actions = append(actions, string(verdict.Action))
			continue
		}
		blocked = append(blocked, dto.RepositoryActionBlockDTO{
			Action: string(verdict.Action), Blockers: storage.BlockerCodes(verdict.Blockers),
			Alternatives: storage.AlternativeCodes(verdict.Alternatives),
		})
	}
	return actions, blocked
}

// repositoryActionDTOs splits the role-aware policy into the admitted action
// codes and the explained withheld ones. Both lists are always present so the
// Web never confuses "no actions" with "actions unknown".
func repositoryActionDTOs(policy storage.RepositoryActionPolicy) ([]string, []dto.RepositoryActionBlockDTO) {
	actions := make([]string, 0, len(policy.Verdicts))
	blocked := make([]dto.RepositoryActionBlockDTO, 0)
	for _, verdict := range policy.Verdicts {
		if verdict.Allowed() {
			actions = append(actions, string(verdict.Action))
			continue
		}
		blocked = append(blocked, dto.RepositoryActionBlockDTO{
			Action: string(verdict.Action), Blockers: storage.BlockerCodes(verdict.Blockers),
			Alternatives: storage.AlternativeCodes(verdict.Alternatives),
		})
	}
	return actions, blocked
}
