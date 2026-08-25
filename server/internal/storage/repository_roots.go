package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	ErrRepositoryRootOffline      = errors.New("storage location is offline")
	ErrRepositoryRootInvalid      = errors.New("storage location is invalid")
	ErrRepositoryRootOverlap      = errors.New("storage location overlaps another registered location")
	ErrRepositoryRootNotRemovable = errors.New("default storage location cannot be removed")
	ErrRepositoryRootInUse        = errors.New("storage location still contains registered repositories")
)

// RepositoryRootConflictError reports a portable .lumilioroot identity that is
// already registered at another path. The host must not infer whether the
// directory moved or was copied.
type RepositoryRootConflictError struct {
	RootID         string
	RegisteredPath string
	RequestedPath  string
	Actions        []string
}

func (e *RepositoryRootConflictError) Error() string {
	return fmt.Sprintf("storage location %s is already registered at %s", e.RootID, e.RegisteredPath)
}

// EnsureDefaultRepositoryRoot initializes or reopens the configured default
// Storage Location. The marker remains disk-authoritative for identity.
func (rm *DefaultRepositoryManager) EnsureDefaultRepositoryRoot(ctx context.Context, path string, requests ...LifecycleRequest) (*repo.RepositoryRoot, error) {
	cleanPath, err := CanonicalizeRepositoryPath(path)
	if err != nil {
		return nil, fmt.Errorf("canonicalize default storage location: %w", err)
	}
	existingDefault, defaultErr := rm.queries.GetDefaultRepositoryRoot(ctx)
	if defaultErr != nil && !errors.Is(defaultErr, sql.ErrNoRows) {
		return nil, fmt.Errorf("load default storage location: %w", defaultErr)
	}
	if defaultErr == nil && existingDefault.Path != cleanPath {
		request := LifecycleRequest{Actor: "server:config", ConfirmationType: "portable_identity_match"}
		if len(requests) > 0 {
			request = requests[0]
		}
		return rm.switchDefaultRepositoryRoot(ctx, existingDefault, cleanPath, request)
	}
	if defaultErr == nil {
		info, statErr := os.Stat(cleanPath)
		if errors.Is(statErr, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: registered default Storage Location is missing at %s", ErrRepositoryRootOffline, cleanPath)
		}
		if statErr != nil || !info.IsDir() {
			return nil, fmt.Errorf("%w: registered default Storage Location is unavailable at %s", ErrRepositoryRootOffline, cleanPath)
		}
		if !rootcfg.Exists(cleanPath) {
			return nil, fmt.Errorf("%w: registered default Storage Location marker is missing at %s", ErrRepositoryRootInvalid, cleanPath)
		}
	} else if err := os.MkdirAll(cleanPath, 0o755); err != nil {
		return nil, fmt.Errorf("create default storage location: %w", err)
	}
	if err := rm.claimRuntimeStoragePath(ctx, "root", cleanPath); err != nil {
		return nil, err
	}
	if repocfg.IsRepositoryRoot(cleanPath) {
		return nil, fmt.Errorf("%w: a repository cannot also be a storage location", ErrRepositoryRootInvalid)
	}

	createdMarker := defaultErr != nil && !rootcfg.Exists(cleanPath)
	var config *rootcfg.RootConfig
	if createdMarker {
		config = rootcfg.New("Default storage")
	} else if config, err = rootcfg.Load(cleanPath); err != nil {
		return nil, err
	}
	if defaultErr == nil && config.ID != existingDefault.RootID.String() {
		return nil, fmt.Errorf("%w: configured default path contains a different .lumilioroot identity", ErrRepositoryRootInvalid)
	}

	targetID := config.ID
	operation, replay, err := rm.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: "ensure-default:" + uuid.NewSHA1(uuid.NameSpaceURL, []byte(cleanPath)).String(),
		Kind:      lifecycleKindCreateStorageLocation,
		Payload: createStorageLocationOperationPayload{
			Path: cleanPath, Name: config.Name, Kind: dbtypes.RepositoryRootKindDefault,
		},
		Actor:        "server:bootstrap",
		TargetType:   "storage_location",
		TargetID:     &targetID,
		RollbackData: createStorageLocationRollbackData{Path: cleanPath},
	})
	if err != nil {
		return nil, err
	}
	if replay {
		if err := lifecycleReplayError(operation); err != nil {
			return nil, err
		}
		registered, err := rm.queries.GetDefaultRepositoryRoot(ctx)
		if err != nil {
			return nil, err
		}
		diskConfig, err := rootcfg.Load(cleanPath)
		if err != nil || diskConfig.ID != registered.RootID.String() {
			return nil, fmt.Errorf("%w: default Storage Location identity is invalid", ErrRepositoryRootInvalid)
		}
		return &registered, nil
	}
	rollback := createStorageLocationRollbackData{Path: cleanPath, MarkerCreated: createdMarker}
	if createdMarker {
		if err := config.Save(cleanPath); err != nil {
			_ = rm.failLifecycleOperation(ctx, operation.OperationID, true, err,
				createStorageLocationRollbackData{Path: cleanPath})
			return nil, err
		}
		if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseFilesystemApplied, rollback); err != nil {
			_ = os.Remove(filepath.Join(cleanPath, rootcfg.FileName))
			_ = rm.failLifecycleOperation(ctx, operation.OperationID, true, err,
				createStorageLocationRollbackData{Path: cleanPath})
			return nil, err
		}
	}
	registered, err := rm.registerRepositoryRoot(ctx, cleanPath, config, dbtypes.RepositoryRootKindDefault, false)
	if err != nil {
		if createdMarker {
			_ = os.Remove(filepath.Join(cleanPath, rootcfg.FileName))
		}
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, true, err,
			createStorageLocationRollbackData{Path: cleanPath})
		return nil, err
	}
	if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseCatalogCommitted, rollback); err != nil {
		return nil, fmt.Errorf("default Storage Location registered but journal commit phase failed: %w", err)
	}
	if err := rm.completeLifecycleOperation(ctx, operation.OperationID,
		createStorageLocationOperationResult{RootID: registered.RootID.String()}); err != nil {
		return nil, fmt.Errorf("default Storage Location registered but journal completion failed: %w", err)
	}
	return registered, nil
}

func (rm *DefaultRepositoryManager) switchDefaultRepositoryRoot(
	ctx context.Context,
	existing repo.RepositoryRoot,
	newPath string,
	request LifecycleRequest,
) (*repo.RepositoryRoot, error) {
	requestID := "switch-default:" + existing.RootID.String() + ":" + uuid.NewSHA1(uuid.NameSpaceURL, []byte(newPath)).String()
	if strings.TrimSpace(request.RequestID) != "" {
		requestID = request.RequestID
	}
	actor := strings.TrimSpace(request.Actor)
	if actor == "" {
		actor = "server:config"
	}
	confirmation := strings.TrimSpace(request.ConfirmationType)
	if confirmation == "" {
		confirmation = "portable_identity_match"
	}
	repositories, err := rm.queries.ListRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("inspect default Storage Location switch impact: %w", err)
	}
	var repositoryCount int64
	for _, repository := range repositories {
		if repository.RootID == existing.RootID {
			repositoryCount++
		}
	}
	targetID := existing.RootID.String()
	operation, replay, err := rm.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: requestID, Kind: lifecycleKindSwitchDefaultStorage,
		Payload: switchDefaultStorageOperationPayload{
			RootID: existing.RootID.String(), OldPath: existing.Path, NewPath: newPath,
			ConfirmationType: confirmation, RepositoryCount: repositoryCount,
		},
		Actor: actor, ActorUserID: request.ActorUserID, HostInstanceID: request.HostInstanceID,
		TargetType: "storage_location", TargetID: &targetID,
	})
	if err != nil {
		return nil, err
	}
	if replay {
		if err := lifecycleReplayError(operation); err != nil {
			return nil, err
		}
		root, err := rm.queries.GetRepositoryRoot(ctx, existing.RootID)
		if err != nil {
			return nil, err
		}
		return &root, nil
	}
	if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseFilesystemApplied, nil); err != nil {
		return nil, err
	}
	root, err := rm.relocateRepositoryRoot(ctx, existing.RootID.String(), newPath, true)
	if err != nil {
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, false, err, nil)
		return nil, err
	}
	if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseCatalogCommitted, nil); err != nil {
		return nil, err
	}
	if err := rm.completeLifecycleOperation(ctx, operation.OperationID,
		switchDefaultStorageOperationResult{
			RootID: root.RootID.String(), RepositoryCount: repositoryCount, FilesPreserved: true,
		}); err != nil {
		return nil, err
	}
	return root, nil
}

// AddRepositoryRoot registers a native-host-authorized directory as an
// external Storage Location. The directory must already exist; the server never
// turns a missing mount path into a new directory.
func (rm *DefaultRepositoryManager) AddRepositoryRoot(ctx context.Context, path, name string, requests ...LifecycleRequest) (*repo.RepositoryRoot, error) {
	cleanPath, err := rm.validateRepositoryRootPath(path)
	if err != nil {
		return nil, err
	}
	if err := rm.claimRuntimeStoragePath(ctx, "root", cleanPath); err != nil {
		return nil, err
	}
	rootName := strings.TrimSpace(name)
	if rootName == "" {
		rootName = filepath.Base(cleanPath)
	}
	createdMarker := !rootcfg.Exists(cleanPath)
	var config *rootcfg.RootConfig
	if createdMarker {
		config = rootcfg.New(rootName)
	} else if config, err = rootcfg.Load(cleanPath); err != nil {
		return nil, err
	}
	targetID := config.ID
	request := LifecycleRequest{}
	if len(requests) > 0 {
		request = requests[0]
	}
	operation, replay, err := rm.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: request.RequestID, Kind: lifecycleKindCreateStorageLocation,
		Payload: createStorageLocationOperationPayload{
			Path: cleanPath, Name: rootName, Kind: dbtypes.RepositoryRootKindExternal,
		},
		Actor: request.Actor, ActorUserID: request.ActorUserID, HostInstanceID: request.HostInstanceID, TargetType: "storage_location", TargetID: &targetID,
		RollbackData: createStorageLocationRollbackData{Path: cleanPath},
	})
	if err != nil {
		return nil, err
	}
	if replay {
		if err := lifecycleReplayError(operation); err != nil {
			return nil, err
		}
		if operation.TargetID == nil {
			return nil, fmt.Errorf("%w: completed Storage Location operation has no target", ErrLifecycleRecoveryRequired)
		}
		rootID, err := uuid.Parse(*operation.TargetID)
		if err != nil {
			return nil, fmt.Errorf("%w: completed Storage Location operation has an invalid target", ErrLifecycleRecoveryRequired)
		}
		root, err := rm.queries.GetRepositoryRoot(ctx, rootID)
		if err != nil {
			return nil, fmt.Errorf("load completed Storage Location result: %w", err)
		}
		return &root, nil
	}
	failPrepared := func(cause error, markerCreated bool) (*repo.RepositoryRoot, error) {
		rollback := createStorageLocationRollbackData{Path: cleanPath, MarkerCreated: markerCreated}
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, true, cause, rollback)
		return nil, cause
	}

	if existing, err := rm.queries.GetRepositoryRootByPath(ctx, cleanPath); err == nil {
		if config.ID != existing.RootID.String() {
			return failPrepared(fmt.Errorf("%w: database and .lumilioroot identities differ", ErrRepositoryRootInvalid), false)
		}
		if err := rm.completeLifecycleOperation(ctx, operation.OperationID,
			createStorageLocationOperationResult{RootID: existing.RootID.String()}); err != nil {
			return nil, err
		}
		return &existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return failPrepared(fmt.Errorf("find storage location by path: %w", err), false)
	}
	if err := rm.rejectOverlappingRepositoryRoot(ctx, cleanPath); err != nil {
		return failPrepared(err, false)
	}
	if createdMarker {
		if err := config.Save(cleanPath); err != nil {
			return failPrepared(err, false)
		}
		rollback := createStorageLocationRollbackData{Path: cleanPath, MarkerCreated: true}
		if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseFilesystemApplied, rollback); err != nil {
			_ = os.Remove(filepath.Join(cleanPath, rootcfg.FileName))
			return failPrepared(err, false)
		}
	}
	registered, err := rm.registerRepositoryRoot(ctx, cleanPath, config, dbtypes.RepositoryRootKindExternal, false)
	if err != nil {
		if createdMarker {
			_ = os.Remove(filepath.Join(cleanPath, rootcfg.FileName))
		}
		return failPrepared(err, false)
	}
	rollback := createStorageLocationRollbackData{Path: cleanPath, MarkerCreated: createdMarker}
	if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseCatalogCommitted, rollback); err != nil {
		return nil, fmt.Errorf("Storage Location registered but journal commit phase failed: %w", err)
	}
	if err := rm.completeLifecycleOperation(ctx, operation.OperationID,
		createStorageLocationOperationResult{RootID: registered.RootID.String()}); err != nil {
		return nil, fmt.Errorf("Storage Location registered but journal completion failed: %w", err)
	}
	return registered, nil
}

// RelocateRepositoryRoot reconnects an existing external Storage Location at
// a new native-host-authorized path. The marker at the requested path must
// carry the registered identity; this never rewrites or guesses identity.
func (rm *DefaultRepositoryManager) RelocateRepositoryRoot(ctx context.Context, id, path string, requests ...LifecycleRequest) (*repo.RepositoryRoot, error) {
	rootID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return nil, fmt.Errorf("invalid storage location id: %w", err)
	}
	registered, err := rm.queries.GetRepositoryRoot(ctx, rootID)
	if err != nil {
		return nil, err
	}
	cleanPath, err := CanonicalizeRepositoryPath(path)
	if err != nil {
		return nil, err
	}
	targetID := rootID.String()
	request := firstLifecycleRequest(requests)
	oldPath := registered.Path
	if requestID := strings.TrimSpace(request.RequestID); requestID != "" {
		if existing, lookupErr := rm.queries.GetLifecycleOperationByRequestID(ctx, requestID); lookupErr == nil {
			var existingPayload switchDefaultStorageOperationPayload
			if existing.Kind != lifecycleKindRelocateStorage || json.Unmarshal(existing.Payload, &existingPayload) != nil ||
				existingPayload.RootID != rootID.String() || existingPayload.NewPath != cleanPath {
				return nil, ErrLifecycleRequestConflict
			}
			oldPath = existingPayload.OldPath
		} else if !errors.Is(lookupErr, sql.ErrNoRows) {
			return nil, fmt.Errorf("find Storage Location relocate request: %w", lookupErr)
		}
	}
	operation, replay, err := rm.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: request.RequestID,
		Kind:      lifecycleKindRelocateStorage,
		Payload:   switchDefaultStorageOperationPayload{RootID: rootID.String(), OldPath: oldPath, NewPath: cleanPath},
		Actor:     request.Actor, ActorUserID: request.ActorUserID, HostInstanceID: request.HostInstanceID,
		TargetType: "storage_location", TargetID: &targetID,
	})
	if err != nil {
		return nil, err
	}
	if replay {
		if err := lifecycleReplayError(operation); err != nil {
			return nil, err
		}
		root, err := rm.queries.GetRepositoryRoot(ctx, rootID)
		return &root, err
	}
	root, err := rm.relocateRepositoryRoot(ctx, id, cleanPath, false)
	if err != nil {
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, false, err, nil)
		return nil, err
	}
	if err := rm.completeLifecycleOperation(ctx, operation.OperationID, createStorageLocationOperationResult{RootID: rootID.String()}); err != nil {
		return nil, err
	}
	return root, nil
}

func (rm *DefaultRepositoryManager) relocateRepositoryRoot(ctx context.Context, id, path string, allowDefault bool) (*repo.RepositoryRoot, error) {
	rootID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return nil, fmt.Errorf("invalid storage location id: %w", err)
	}
	registered, err := rm.queries.GetRepositoryRoot(ctx, rootID)
	if err != nil {
		return nil, err
	}
	if registered.Kind != dbtypes.RepositoryRootKindExternal && !allowDefault {
		return nil, ErrRepositoryRootNotRemovable
	}

	cleanPath, err := rm.validateRepositoryRootPath(path)
	if err != nil {
		return nil, err
	}
	config, err := rootcfg.Load(cleanPath)
	if err != nil {
		return nil, err
	}
	if config.ID != rootID.String() {
		return nil, fmt.Errorf("%w: selected directory has a different .lumilioroot identity", ErrRepositoryRootInvalid)
	}
	if registered.Path != cleanPath {
		if originalConfig, originalErr := rootcfg.Load(registered.Path); originalErr == nil && originalConfig.ID == rootID.String() {
			return nil, &RepositoryRootConflictError{
				RootID: rootID.String(), RegisteredPath: registered.Path, RequestedPath: cleanPath,
				Actions: []string{},
			}
		}
	}
	if err := rm.claimRuntimeStoragePath(ctx, "root", cleanPath); err != nil {
		return nil, err
	}
	if err := rm.rejectOverlappingRepositoryRootExcept(ctx, cleanPath, rootID); err != nil {
		return nil, err
	}
	if rm.database == nil {
		return nil, errors.New("repository catalog transaction is unavailable")
	}
	coordinator := rm.files.AccessCoordinator()
	releaseRoot, err := coordinator.AcquireRootMutationContext(ctx, rootID)
	if err != nil {
		return nil, fmt.Errorf("%w: Storage Location is busy: %v", ErrRepositoryBusy, err)
	}
	defer releaseRoot()

	type repositoryMove struct {
		repository repo.Repository
		path       string
		config     *repocfg.RepositoryConfig
	}
	repositories, err := rm.queries.ListRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("list repositories for storage location relocate: %w", err)
	}
	moves := make([]repositoryMove, 0)
	repositoryIDs := make([]uuid.UUID, 0)
	for _, repository := range repositories {
		if repository.RootID != rootID {
			continue
		}
		requestedRepositoryPath, moveErr := relocatedRepositoryPath(registered.Path, cleanPath, repository.Path)
		if moveErr != nil {
			return nil, moveErr
		}
		repositoryConfig, loadErr := repocfg.LoadConfigFromFile(requestedRepositoryPath)
		if loadErr != nil {
			return nil, fmt.Errorf("validate repository after Storage Location move: %w", loadErr)
		}
		if repositoryConfig.ID != repository.RepoID.String() {
			return nil, fmt.Errorf("%w: repository identity differs at %s", ErrRepositoryRootInvalid, requestedRepositoryPath)
		}
		if occupying, findErr := rm.queries.GetRepositoryByPath(ctx, requestedRepositoryPath); findErr == nil && occupying.RepoID != repository.RepoID {
			return nil, fmt.Errorf("%w: %s", ErrRepositoryExistsAtPath, requestedRepositoryPath)
		} else if findErr != nil && !errors.Is(findErr, sql.ErrNoRows) {
			return nil, fmt.Errorf("check repository destination: %w", findErr)
		}
		moves = append(moves, repositoryMove{
			repository: repository,
			path:       requestedRepositoryPath,
			config:     repositoryConfig,
		})
		if err := rm.claimRuntimeStoragePath(ctx, "repository", requestedRepositoryPath); err != nil {
			return nil, err
		}
		repositoryIDs = append(repositoryIDs, repository.RepoID)
	}

	releaseRepositories, err := coordinator.AcquireMutationsContext(ctx, repositoryIDs)
	if err != nil {
		return nil, fmt.Errorf("%w: child repository is busy: %v", ErrRepositoryBusy, err)
	}
	defer releaseRepositories()

	// Commit the maintenance barrier before changing any paths. HTTP reads and
	// every BeginRepositoryActivity caller can now observe/refuse this root and
	// its children, instead of maintenance existing only inside the final tx.
	maintenanceTx, err := rm.writer.BeginTx(ctx, catalogtx.OperationRepositoryRootRelocateMaintenance, nil)
	if err != nil {
		return nil, fmt.Errorf("begin Storage Location maintenance: %w", err)
	}
	maintenanceQueries := rm.queries.WithTx(maintenanceTx.Raw())
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := maintenanceQueries.UpdateRepositoryRootFromDisk(ctx, repo.UpdateRepositoryRootFromDiskParams{
		RootID: rootID, Name: registered.Name,
		Status: dbtypes.RepositoryRootStatusMaintenance, UpdatedAt: now,
	}); err != nil {
		_ = maintenanceTx.Rollback()
		return nil, fmt.Errorf("enter Storage Location maintenance: %w", err)
	}
	for _, move := range moves {
		if _, err := maintenanceQueries.BeginRepositoryMaintenance(ctx, repo.BeginRepositoryMaintenanceParams{
			RepoID: move.repository.RepoID, UpdatedAt: now,
		}); err != nil {
			_ = maintenanceTx.Rollback()
			if errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("%w: child repository %s has active work", ErrRepositoryBusy, move.repository.Name)
			}
			return nil, fmt.Errorf("enter child repository maintenance: %w", err)
		}
	}
	if err := maintenanceTx.Commit(); err != nil {
		return nil, fmt.Errorf("commit Storage Location maintenance: %w", err)
	}
	maintenanceCommitted := true
	defer func() {
		if !maintenanceCommitted {
			return
		}
		background := context.Background()
		restoreNow := dbtypes.NewTimestamp(time.Now().UTC())
		_, _ = rm.queries.UpdateRepositoryRootFromDisk(background, repo.UpdateRepositoryRootFromDiskParams{
			RootID: rootID, Name: registered.Name, Status: registered.Status, UpdatedAt: restoreNow,
		})
		for _, move := range moves {
			_, _ = rm.queries.EndRepositoryMaintenance(background, repo.EndRepositoryMaintenanceParams{
				RepoID: move.repository.RepoID, Reachability: move.repository.Reachability,
				Activity: move.repository.Activity, UpdatedAt: restoreNow,
			})
		}
	}()

	tx, err := rm.writer.BeginTx(ctx, catalogtx.OperationRepositoryRootRelocate, nil)
	if err != nil {
		return nil, fmt.Errorf("begin Storage Location relocate: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	queries := rm.queries.WithTx(tx.Raw())
	now = dbtypes.NewTimestamp(time.Now().UTC())
	for _, move := range moves {
		updated, updateErr := queries.UpdateRepositoryPath(ctx, repo.UpdateRepositoryPathParams{
			RepoID: move.repository.RepoID, Path: move.path, RootID: rootID,
			Reachability: dbtypes.RepositoryReachabilityMaintenance, UpdatedAt: now,
		})
		if updateErr != nil {
			return nil, fmt.Errorf("relocate repository with Storage Location: %w", updateErr)
		}
		if _, updateErr := queries.UpdateRepository(ctx, repo.UpdateRepositoryParams{
			RepoID: updated.RepoID, Name: move.config.Name, Config: *move.config,
			DefaultOwnerID: updated.DefaultOwnerID, UpdatedAt: now,
		}); updateErr != nil {
			return nil, fmt.Errorf("refresh relocated repository config: %w", updateErr)
		}
		if _, updateErr := queries.EndRepositoryMaintenance(ctx, repo.EndRepositoryMaintenanceParams{
			RepoID: updated.RepoID, Reachability: dbtypes.RepositoryReachabilityActive,
			Activity: dbtypes.RepositoryActivityIdle, UpdatedAt: now,
		}); updateErr != nil {
			return nil, fmt.Errorf("leave relocated repository maintenance: %w", updateErr)
		}
	}
	root, err := queries.UpsertRepositoryRoot(ctx, repo.UpsertRepositoryRootParams{
		RootID: rootID, Name: config.Name, Path: cleanPath,
		Kind: registered.Kind, Status: dbtypes.RepositoryRootStatusActive,
		MountFingerprint: InspectStoragePath(cleanPath).MountFingerprint,
		CreatedAt:        registered.CreatedAt, UpdatedAt: now,
	})
	if err != nil {
		return nil, fmt.Errorf("relocate Storage Location row: %w", err)
	}
	root, err = queries.UpdateRepositoryRootMountFingerprint(ctx, repo.UpdateRepositoryRootMountFingerprintParams{
		RootID: rootID, MountFingerprint: InspectStoragePath(cleanPath).MountFingerprint, UpdatedAt: now,
	})
	if err != nil {
		return nil, fmt.Errorf("record relocated Storage Location mount fingerprint: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit Storage Location relocate: %w", err)
	}
	maintenanceCommitted = false
	return &root, nil
}

func (rm *DefaultRepositoryManager) validateRepositoryRootPath(path string) (string, error) {
	cleanPath, err := CanonicalizeRepositoryPath(path)
	if err != nil {
		return "", fmt.Errorf("canonicalize storage location: %w", err)
	}
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", ErrRepositoryRootOffline, cleanPath)
		}
		return "", fmt.Errorf("stat storage location: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%w: path is not a directory", ErrRepositoryRootInvalid)
	}
	if repocfg.IsRepositoryRoot(cleanPath) {
		return "", fmt.Errorf("%w: choose attach repository for a .lumiliorepo directory", ErrRepositoryRootInvalid)
	}
	if isInsidePhotosLibrary(cleanPath) {
		return "", fmt.Errorf("%w: a Photos library bundle cannot be a storage location", ErrRepositoryRootInvalid)
	}
	if nested, parent, err := rm.isNestedRepository(cleanPath); err != nil {
		return "", err
	} else if nested {
		return "", fmt.Errorf("%w: path is inside repository %s", ErrRepositoryRootInvalid, parent)
	}
	return cleanPath, nil
}

func (rm *DefaultRepositoryManager) registerRepositoryRoot(
	ctx context.Context,
	path string,
	config *rootcfg.RootConfig,
	kind dbtypes.RepositoryRootKind,
	allowMove bool,
) (*repo.RepositoryRoot, error) {
	rootID, err := uuid.Parse(config.ID)
	if err != nil {
		return nil, fmt.Errorf("parse storage location id: %w", err)
	}
	if registered, err := rm.queries.GetRepositoryRoot(ctx, rootID); err == nil {
		if registered.Path != path && !allowMove {
			actions := []string{"relocate"}
			if marker, markerErr := rootcfg.Load(registered.Path); markerErr == nil && marker.ID == config.ID {
				actions = nil
			}
			return nil, &RepositoryRootConflictError{
				RootID:         config.ID,
				RegisteredPath: registered.Path,
				RequestedPath:  path,
				Actions:        actions,
			}
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("find storage location by id: %w", err)
	}

	now := time.Now()
	createdAt := config.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	registered, err := rm.queries.UpsertRepositoryRoot(ctx, repo.UpsertRepositoryRootParams{
		RootID:           rootID,
		Name:             config.Name,
		Path:             path,
		Kind:             kind,
		Status:           dbtypes.RepositoryRootStatusActive,
		MountFingerprint: InspectStoragePath(path).MountFingerprint,
		CreatedAt:        dbtypes.NewTimestamp(createdAt),
		UpdatedAt:        dbtypes.NewTimestamp(now),
	})
	if err != nil {
		return nil, fmt.Errorf("register storage location: %w", err)
	}
	return &registered, nil
}

// ListRepositoryRoots returns the latest reconciled reachability projection.
// Startup and the portable background reconciler own disk inspection and
// writes; a foreground list must remain available while SQLite's writer is
// busy.
func (rm *DefaultRepositoryManager) ListRepositoryRoots(ctx context.Context) ([]repo.RepositoryRoot, error) {
	roots, err := rm.readerQueries.ListRepositoryRoots(ctx)
	if err != nil {
		return nil, fmt.Errorf("list storage locations: %w", err)
	}
	return roots, nil
}

func (rm *DefaultRepositoryManager) ReconcileRepositoryRoots(ctx context.Context) error {
	roots, err := rm.readerQueries.ListRepositoryRoots(ctx)
	if err != nil {
		return fmt.Errorf("list storage locations for reconcile: %w", err)
	}
	for _, root := range roots {
		if root.Status == dbtypes.RepositoryRootStatusMaintenance {
			continue
		}
		status := dbtypes.RepositoryRootStatusActive
		name := root.Name
		if info, statErr := os.Stat(root.Path); statErr != nil || !info.IsDir() {
			status = dbtypes.RepositoryRootStatusOffline
		} else if config, loadErr := rootcfg.Load(root.Path); loadErr != nil {
			status = dbtypes.RepositoryRootStatusError
		} else if config.ID != root.RootID.String() {
			status = dbtypes.RepositoryRootStatusError
		} else {
			name = config.Name
		}
		_, updateErr := rm.queries.UpdateRepositoryRootFromDisk(ctx, repo.UpdateRepositoryRootFromDiskParams{
			RootID:    root.RootID,
			Name:      name,
			Status:    status,
			UpdatedAt: dbtypes.NewTimestamp(time.Now()),
		})
		if updateErr != nil {
			return fmt.Errorf("update storage location status: %w", updateErr)
		}
	}
	return nil
}

func (rm *DefaultRepositoryManager) GetRepositoryRoot(ctx context.Context, id string) (*repo.RepositoryRoot, error) {
	rootID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return nil, fmt.Errorf("invalid storage location id: %w", err)
	}
	root, err := rm.queries.GetRepositoryRoot(ctx, rootID)
	if err != nil {
		return nil, err
	}
	return &root, nil
}

func (rm *DefaultRepositoryManager) PreviewRepositoryRootRemoval(ctx context.Context, id string) (RepositoryRootRemovalImpact, error) {
	rootID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return RepositoryRootRemovalImpact{}, fmt.Errorf("invalid storage location id: %w", err)
	}
	root, err := rm.queries.GetRepositoryRoot(ctx, rootID)
	if err != nil {
		return RepositoryRootRemovalImpact{}, err
	}
	impact := RepositoryRootRemovalImpact{
		RootID: root.RootID.String(), RootName: root.Name, Kind: root.Kind, FilesPreserved: true,
	}
	if err := rm.readerDatabase.QueryRowContext(ctx,
		"SELECT count(*) FROM repositories WHERE root_id = ?", rootID,
	).Scan(&impact.RepositoryCount); err != nil {
		return RepositoryRootRemovalImpact{}, fmt.Errorf("count Storage Location repositories: %w", err)
	}
	if err := rm.readerDatabase.QueryRowContext(ctx, `
		SELECT count(*) FROM lifecycle_operations
		WHERE target_type = 'storage_location' AND target_id = ? AND status = 'running'
	`, rootID.String()).Scan(&impact.ActiveOperationCount); err != nil {
		return RepositoryRootRemovalImpact{}, fmt.Errorf("count Storage Location operations: %w", err)
	}
	switch {
	case root.Kind != dbtypes.RepositoryRootKindExternal:
		impact.BlockingReason = "default_storage_location"
	case impact.RepositoryCount != 0:
		impact.BlockingReason = "registered_repositories"
	case impact.ActiveOperationCount != 0:
		impact.BlockingReason = "active_operation"
	default:
		impact.CanRemove = true
	}
	return impact, nil
}

func (rm *DefaultRepositoryManager) DeleteRepositoryRoot(ctx context.Context, id string, requests ...LifecycleRequest) error {
	rootID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("invalid storage location id: %w", err)
	}
	root, err := rm.queries.GetRepositoryRoot(ctx, rootID)
	if err != nil {
		return err
	}
	if root.Kind != dbtypes.RepositoryRootKindExternal {
		return ErrRepositoryRootNotRemovable
	}
	coordinator := rm.files.AccessCoordinator()
	releaseRoot, err := coordinator.AcquireRootMutationContext(ctx, rootID)
	if err != nil {
		return fmt.Errorf("%w: Storage Location is busy: %v", ErrRepositoryBusy, err)
	}
	defer releaseRoot()
	impact, err := rm.PreviewRepositoryRootRemoval(ctx, id)
	if err != nil {
		return err
	}
	if impact.RepositoryCount != 0 {
		return ErrRepositoryRootInUse
	}
	if impact.ActiveOperationCount != 0 {
		return fmt.Errorf("%w: Storage Location has an active lifecycle operation", ErrRepositoryBusy)
	}
	tx, err := rm.writer.BeginTx(ctx, catalogtx.OperationRepositoryRootDelete, nil)
	if err != nil {
		return fmt.Errorf("begin Storage Location removal: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	queries := rm.queries.WithTx(tx.Raw())
	deleted, err := queries.DeleteExternalRepositoryRoot(ctx, rootID)
	if err != nil {
		return fmt.Errorf("remove storage location: %w", err)
	}
	if deleted == 0 {
		return ErrRepositoryRootInUse
	}
	request := firstLifecycleRequest(requests)
	if _, err := recordLifecycleAuditWithQueries(ctx, queries, LifecycleAuditInput{
		Actor: request.Actor, ActorUserID: request.ActorUserID, HostInstanceID: request.HostInstanceID, RequestID: request.RequestID,
		Action: "remove_storage_location", TargetType: "storage_location", TargetID: id,
		Source: auditSourceForActor(request.Actor), ConfirmationType: "summary",
		OldPath: root.Path, Result: AuditResultSucceeded,
		Details: map[string]any{"storage_location_name": root.Name, "files_preserved": true},
	}); err != nil {
		return fmt.Errorf("audit Storage Location removal: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Storage Location removal: %w", err)
	}
	rm.repoAudit(root.Path).Operation("repository_root.remove",
		zap.String("storage_location_id", root.RootID.String()),
		zap.String("storage_location_name", root.Name),
		zap.String("preserved_path", root.Path),
	)
	return nil
}

func (rm *DefaultRepositoryManager) resolveRepositoryRootForCreate(ctx context.Context, id string, role dbtypes.RepoRole) (*repo.RepositoryRoot, error) {
	var root repo.RepositoryRoot
	var err error
	if strings.TrimSpace(id) == "" {
		root, err = rm.queries.GetDefaultRepositoryRoot(ctx)
	} else {
		rootID, parseErr := uuid.Parse(strings.TrimSpace(id))
		if parseErr != nil {
			return nil, fmt.Errorf("invalid storage location id: %w", parseErr)
		}
		root, err = rm.queries.GetRepositoryRoot(ctx, rootID)
	}
	if err != nil {
		return nil, fmt.Errorf("load storage location: %w", err)
	}
	if normalizeRepoRole(role) == dbtypes.RepoRolePrimary && root.Kind != dbtypes.RepositoryRootKindDefault {
		return nil, fmt.Errorf("%w: primary repository must use the default storage location", ErrPathNotAllowed)
	}
	if root.Status != dbtypes.RepositoryRootStatusActive {
		return nil, fmt.Errorf("%w: %s", ErrRepositoryRootOffline, root.Path)
	}
	config, err := rootcfg.Load(root.Path)
	if err != nil || config.ID != root.RootID.String() {
		return nil, fmt.Errorf("%w: %s", ErrRepositoryRootInvalid, root.Path)
	}
	return &root, nil
}

func (rm *DefaultRepositoryManager) repositoryRootIDForPath(ctx context.Context, path string) (uuid.UUID, error) {
	roots, err := rm.queries.ListRepositoryRoots(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("list storage locations for repository association: %w", err)
	}
	for _, root := range roots {
		if !pathIsDirectChild(root.Path, path) {
			continue
		}
		if root.Status != dbtypes.RepositoryRootStatusActive {
			return uuid.Nil, fmt.Errorf("%w: %s", ErrRepositoryRootOffline, root.Path)
		}
		config, loadErr := rootcfg.Load(root.Path)
		if loadErr != nil || config.ID != root.RootID.String() {
			return uuid.Nil, fmt.Errorf("%w: %s", ErrRepositoryRootInvalid, root.Path)
		}
		return root.RootID, nil
	}
	return uuid.Nil, fmt.Errorf(
		"%w: repository %s must be a direct child of an active registered Storage Location",
		ErrPathNotAllowed,
		path,
	)
}

func (rm *DefaultRepositoryManager) resolveRepositoryAssociation(ctx context.Context, path string, requested []uuid.UUID) (uuid.UUID, error) {
	if len(requested) == 0 {
		return rm.repositoryRootIDForPath(ctx, path)
	}
	if len(requested) != 1 {
		return uuid.Nil, fmt.Errorf("%w: exactly one Storage Location is required", ErrPathNotAllowed)
	}
	root, err := rm.queries.GetRepositoryRoot(ctx, requested[0])
	if err != nil {
		return uuid.Nil, fmt.Errorf("load repository Storage Location: %w", err)
	}
	if !pathIsDirectChild(root.Path, path) {
		return uuid.Nil, fmt.Errorf(
			"%w: repository %s must be a direct child of Storage Location %s",
			ErrPathNotAllowed,
			path,
			root.Path,
		)
	}
	if root.Status != dbtypes.RepositoryRootStatusActive {
		return uuid.Nil, fmt.Errorf("%w: %s", ErrRepositoryRootOffline, root.Path)
	}
	config, err := rootcfg.Load(root.Path)
	if err != nil || config.ID != root.RootID.String() {
		return uuid.Nil, fmt.Errorf("%w: %s", ErrRepositoryRootInvalid, root.Path)
	}
	return root.RootID, nil
}

func (rm *DefaultRepositoryManager) rejectOverlappingRepositoryRoot(ctx context.Context, requested string) error {
	return rm.rejectOverlappingRepositoryRootExcept(ctx, requested, uuid.Nil)
}

func (rm *DefaultRepositoryManager) rejectOverlappingRepositoryRootExcept(ctx context.Context, requested string, except uuid.UUID) error {
	roots, err := rm.queries.ListRepositoryRoots(ctx)
	if err != nil {
		return fmt.Errorf("list storage locations for overlap check: %w", err)
	}
	for _, root := range roots {
		if except != uuid.Nil && root.RootID == except {
			continue
		}
		if root.Path == requested || pathIsStrictlyInside(root.Path, requested) || pathIsStrictlyInside(requested, root.Path) {
			return fmt.Errorf("%w: %s and %s", ErrRepositoryRootOverlap, requested, root.Path)
		}
	}
	return nil
}

func pathIsStrictlyInside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func pathIsDirectChild(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == "." || filepath.IsAbs(relative) {
		return false
	}
	return relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator)) &&
		filepath.Dir(relative) == "."
}

func relocatedRepositoryPath(oldRoot, newRoot, repositoryPath string) (string, error) {
	relative, err := filepath.Rel(oldRoot, repositoryPath)
	if err != nil || !pathIsDirectChild(oldRoot, repositoryPath) {
		return "", fmt.Errorf("%w: repository %s is not a direct child of its registered Storage Location", ErrRepositoryRootInvalid, repositoryPath)
	}
	return filepath.Join(newRoot, relative), nil
}

// RepositoryRootWarnings returns non-fatal placement risks for the Desktop
// Control Panel to surface immediately after a native directory grant.
func RepositoryRootWarnings(path string) []string {
	provider := cloudSyncProvider(path)
	if provider == "" {
		return nil
	}
	return []string{fmt.Sprintf(
		"%s is inside %s. Sync clients may evict originals or duplicate files.", path, provider,
	)}
}
