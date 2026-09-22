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
)

// StorageViewAssetCounter loads cheap catalog counts for the admin storage view.
type StorageViewAssetCounter interface {
	CountAssetsByStatusAndRepository(ctx context.Context, arg repo.CountAssetsByStatusAndRepositoryParams) (int64, error)
}

// StorageViewScanReader loads latest verification summaries when available.
type StorageViewScanReader interface {
	GetLatestScanRun(ctx context.Context, repositoryID string) (repo.RepositoryScanRun, error)
}

// StorageHandler serves authenticated storage selectors and the admin read model.
type StorageHandler struct {
	repoManager  storage.RepositoryManager
	assetCounter StorageViewAssetCounter
	scanReader   StorageViewScanReader
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
		locations = append(locations, dto.StorageLocationViewDTO{
			ID:              storageLocation.StorageLocationID.String(),
			Name:            storageLocation.Name,
			Kind:            string(storageLocation.Kind),
			CanRemove:       impact.CanRemove,
			BlockingReason:  impact.BlockingReason,
			FilesPreserved:  impact.FilesPreserved,
			RepositoryCount: impact.RepositoryCount,
		})
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
		if groupID, ok := repoGroupIDs[index]; ok {
			view.CapacityGroupID = &groupID
		}
		if sampled, ok := repoStorage[index]; ok {
			view.MountPath = sampled.MountPath
			view.Filesystem = sampled.Filesystem
		}
		if h.assetCounter != nil {
			count, countErr := h.assetCounter.CountAssetsByStatusAndRepository(ctx, repo.CountAssetsByStatusAndRepositoryParams{
				RepositoryID: repository.RepoID,
				Status:       dbtypes.JSON("ready"),
			})
			if countErr == nil {
				view.AssetCount = &count
			}
		}
		if h.scanReader != nil {
			if scanRun, scanErr := h.scanReader.GetLatestScanRun(ctx, repository.RepoID.String()); scanErr == nil {
				view.Verification = &dto.StorageRepositoryVerificationSummaryDTO{
					OperationID: scanRun.RunID.String(),
					Status:      scanRun.Status,
					Mode:        scanRun.Mode,
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
