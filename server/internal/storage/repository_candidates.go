package storage

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"
)

const (
	RepositoryCandidateRegistered       = "registered_repository"
	RepositoryCandidateExisting         = "existing_repository"
	RepositoryCandidateEmptyWritable    = "empty_writable"
	RepositoryCandidateNonemptyUnmarked = "nonempty_unmarked"
	RepositoryCandidateMarkerInvalid    = "marker_invalid"
	RepositoryCandidateIdentityError    = "identity_error"
	RepositoryCandidateUnavailable      = "unavailable"
)

func (rm *DefaultRepositoryManager) ListDefaultRepositoryCandidates(ctx context.Context) ([]RepositoryCandidate, error) {
	storageLocation, err := rm.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		return nil, fmt.Errorf("load default Storage Location: %w", err)
	}
	entries, err := os.ReadDir(storageLocation.Path)
	if err != nil {
		return nil, fmt.Errorf("list default Storage Location: %w", err)
	}
	repositories, err := rm.queries.ListRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("list registered repositories: %w", err)
	}
	byPath := make(map[string]repo.Repository, len(repositories))
	byID := make(map[string]repo.Repository, len(repositories))
	for _, repository := range repositories {
		byPath[filepath.Clean(repository.Path)] = repository
		byID[repository.RepoID.String()] = repository
	}

	candidates := make([]RepositoryCandidate, 0, len(entries))
	for _, entry := range entries {
		if entry.Name() == ".lumilioroot" || !entry.IsDir() {
			continue
		}
		candidate := RepositoryCandidate{DirectoryName: entry.Name()}
		path, pathErr := CanonicalizeRepositoryPath(filepath.Join(storageLocation.Path, entry.Name()))
		if pathErr != nil || !pathIsDirectChild(storageLocation.Path, path) {
			candidate.Classification = RepositoryCandidateUnavailable
			candidates = append(candidates, candidate)
			continue
		}
		pathInfo := InspectStoragePath(path)
		candidate.Writable = pathInfo.Writable
		candidate.CapacityKnown = pathInfo.CapacityKnown
		candidate.TotalBytes = pathInfo.TotalBytes
		candidate.AvailableBytes = pathInfo.AvailableBytes
		candidate.Filesystem = pathInfo.Filesystem
		if registered, ok := byPath[filepath.Clean(path)]; ok {
			candidate.Classification = RepositoryCandidateRegistered
			candidate.RepositoryID = registered.RepoID.String()
			candidate.Name = registered.Name
			candidates = append(candidates, candidate)
			continue
		}
		markerPath := filepath.Join(path, ".lumiliorepo")
		if _, markerErr := os.Stat(markerPath); markerErr == nil {
			config, loadErr := repocfg.LoadConfigFromFile(path)
			validation, validationErr := rm.validateRepository(path)
			if loadErr != nil || validationErr != nil || validation == nil || !validation.Valid {
				candidate.Classification = RepositoryCandidateMarkerInvalid
			} else if registered, ok := byID[config.ID]; ok {
				candidate.Classification = RepositoryCandidateIdentityError
				candidate.RepositoryID = registered.RepoID.String()
				candidate.Name = registered.Name
				candidate.Actions = repositoryConflictActions(registered.Path, config.ID)
			} else {
				candidate.Classification = RepositoryCandidateExisting
				candidate.RepositoryID = config.ID
				candidate.Name = config.Name
				candidate.CanOpen = true
			}
			candidates = append(candidates, candidate)
			continue
		} else if !errors.Is(markerErr, os.ErrNotExist) {
			candidate.Classification = RepositoryCandidateUnavailable
			candidates = append(candidates, candidate)
			continue
		}
		children, readErr := os.ReadDir(path)
		if readErr != nil {
			candidate.Classification = RepositoryCandidateUnavailable
			candidates = append(candidates, candidate)
			continue
		}
		candidate.Writable = candidate.Writable && rm.checkDirectoryPermissions(path) == nil
		candidate.MountPoint, _ = isLinuxMountPoint(path)
		if len(children) == 0 && candidate.Writable {
			candidate.Classification = RepositoryCandidateEmptyWritable
			candidate.CanCreate = runtime.GOOS != "linux" || candidate.MountPoint
		} else {
			candidate.Classification = RepositoryCandidateNonemptyUnmarked
		}
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return strings.ToLower(candidates[i].DirectoryName) < strings.ToLower(candidates[j].DirectoryName)
	})
	return candidates, nil
}

func (rm *DefaultRepositoryManager) OpenDefaultRepositoryCandidate(
	ctx context.Context,
	directoryName string,
	defaultOwnerID *int32,
	request LifecycleRequest,
) (*repo.Repository, error) {
	if err := ValidateRepositoryDirectoryName(directoryName); err != nil {
		return nil, err
	}
	storageLocation, err := rm.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		return nil, fmt.Errorf("load default Storage Location: %w", err)
	}
	path, err := resolveRepositoryCreatePath(storageLocation.Path, directoryName, dbtypes.RepoRoleRegular)
	if err != nil {
		return nil, err
	}
	request, err = rm.admitCandidateTarget(ctx, request, lifecycleKindOpenRepository, &storageLocation, directoryName, path)
	if err != nil {
		return nil, err
	}
	return rm.OpenRepository(ctx, path, defaultOwnerID, dbtypes.RepoRoleRegular, request)
}

// admitCandidateTarget is the candidate journeys' consent gate: the same
// classifier and the same token-bound acknowledgement the create commands
// use, applied to an existing direct child of the Default Storage Location.
// An unsupported or unobservable folder is refused before any consent; a
// risky one needs the acknowledgement of a review of exactly these facts.
// The admitted request records that its risks were acknowledged.
func (rm *DefaultRepositoryManager) admitCandidateTarget(ctx context.Context, request LifecycleRequest, action string, location *repo.StorageLocation, directoryName, path string) (LifecycleRequest, error) {
	target := rm.assessResolvedCreateTarget(ctx, IntentOpenRepository, location, directoryName, path)
	acknowledged, err := rm.admitAssessedTarget(ctx, consentAdmission{
		Actor: request.Actor, ActorUserID: request.ActorUserID, HostInstanceID: request.HostInstanceID,
		RequestID: request.RequestID, Action: action, TargetID: directoryName,
		AssessmentToken: request.AssessmentToken, AcknowledgedRisks: request.AcknowledgedRisks,
	}, target)
	if err != nil {
		return request, err
	}
	request.RiskConfirmation = len(acknowledged) > 0
	return request, nil
}

func (rm *DefaultRepositoryManager) ResolveDefaultRepositoryCandidate(
	ctx context.Context,
	directoryName string,
	resolution string,
	defaultOwnerID *int32,
	request LifecycleRequest,
) (*repo.Repository, error) {
	if err := ValidateRepositoryDirectoryName(directoryName); err != nil {
		return nil, err
	}
	storageLocation, err := rm.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		return nil, fmt.Errorf("load default Storage Location: %w", err)
	}
	path, err := resolveRepositoryCreatePath(storageLocation.Path, directoryName, dbtypes.RepoRoleRegular)
	if err != nil {
		return nil, err
	}
	action := "update_repository_location"
	if strings.TrimSpace(resolution) == "add_separate" {
		action = lifecycleKindRegisterRepositoryCopy
	}
	request, err = rm.admitCandidateTarget(ctx, request, action, &storageLocation, directoryName, path)
	if err != nil {
		return nil, err
	}
	config, err := repocfg.LoadConfigFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("load repository candidate marker: %w", err)
	}
	registered, err := rm.GetRepository(config.ID)
	if err != nil {
		return nil, fmt.Errorf("load registered repository identity: %w", err)
	}
	switch strings.TrimSpace(resolution) {
	case "update_location":
		request.ConfirmationType = "update_location"
		updated, relocateErr := rm.ReconnectRepository(ctx, registered.RepoID.String(), path, request)
		if relocateErr != nil {
			return nil, relocateErr
		}
		return updated, nil
	case "add_separate":
		return rm.RegisterRepositoryCopy(ctx, path, defaultOwnerID, dbtypes.RepoRoleRegular, request)
	default:
		return nil, fmt.Errorf("unsupported repository candidate resolution %q", resolution)
	}
}

func isLinuxMountPoint(path string) (bool, error) {
	if runtime.GOOS != "linux" {
		return false, nil
	}
	file, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return false, fmt.Errorf("read Linux mount information: %w", err)
	}
	defer file.Close()
	return mountInfoContainsPath(file, path)
}

func mountInfoContainsPath(reader io.Reader, path string) (bool, error) {
	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false, err
	}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 6 {
			continue
		}
		mountPath := decodeMountInfoPath(fields[4])
		if filepath.Clean(mountPath) == cleanPath {
			return true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func decodeMountInfoPath(value string) string {
	result := strings.NewReplacer(
		`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`,
	).Replace(value)
	// Linux may encode other octal bytes. Preserve invalid encodings rather than
	// guessing, but decode valid three-digit sequences for exact comparisons.
	for index := 0; index+3 < len(result); {
		if result[index] != '\\' {
			index++
			continue
		}
		decoded, err := strconv.ParseUint(result[index+1:index+4], 8, 8)
		if err != nil {
			index++
			continue
		}
		result = result[:index] + string(byte(decoded)) + result[index+4:]
		index++
	}
	return result
}

func existingEmptyRepositoryTarget(path string) (bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}
