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
	"server/internal/storage/marker"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	ErrStorageLocationOffline      = errors.New("storage location is offline")
	ErrStorageLocationInvalid      = errors.New("storage location is invalid")
	ErrStorageLocationOverlap      = errors.New("storage location overlaps another registered location")
	ErrStorageLocationNotRemovable = errors.New("default storage location cannot be removed")
	ErrStorageLocationInUse        = errors.New("storage location still contains registered repositories")
)

// StorageLocationConflictError reports a portable .lumilioroot identity that is
// already registered at another path. The host must not infer whether the
// directory moved or was copied.
// ErrStorageLocationOriginalOnline reports that the registered Storage
// Location is still online at its registered path, so there is nothing to
// reconnect; a same-identity tree elsewhere is a copy, never a move.
var ErrStorageLocationOriginalOnline = errors.New("registered storage location is still online at its original path")

type StorageLocationConflictError struct {
	StorageLocationID string
	RegisteredPath    string
	RequestedPath     string
	Actions           []string
}

func (e *StorageLocationConflictError) Error() string {
	return fmt.Sprintf("storage location %s is already registered at %s", e.StorageLocationID, e.RegisteredPath)
}

// EnsureDefaultStorageLocation initializes or reopens the configured default
// Storage Location. The marker remains disk-authoritative for identity.
func (rm *DefaultRepositoryManager) EnsureDefaultStorageLocation(ctx context.Context, path string, requests ...LifecycleRequest) (*repo.StorageLocation, error) {
	cleanPath, err := CanonicalizeRepositoryPath(path)
	if err != nil {
		return nil, fmt.Errorf("canonicalize default storage location: %w", err)
	}
	existingDefault, defaultErr := rm.queries.GetDefaultStorageLocation(ctx)
	if defaultErr != nil && !errors.Is(defaultErr, sql.ErrNoRows) {
		return nil, fmt.Errorf("load default storage location: %w", defaultErr)
	}
	if defaultErr == nil && existingDefault.Path != cleanPath {
		request := LifecycleRequest{Actor: "server:config", ConfirmationType: "portable_identity_match"}
		if len(requests) > 0 {
			request = requests[0]
		}
		fresh, err := rm.freshDefaultSwitchEligible(ctx, existingDefault, cleanPath)
		if err != nil {
			return nil, err
		}
		if fresh {
			return rm.replaceUnusedDefaultStorageLocation(ctx, existingDefault, cleanPath, request)
		}
		return rm.switchDefaultStorageLocation(ctx, existingDefault, cleanPath, request)
	}
	if defaultErr == nil {
		info, statErr := os.Stat(cleanPath)
		if errors.Is(statErr, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: registered default Storage Location is missing at %s", ErrStorageLocationOffline, cleanPath)
		}
		if statErr != nil || !info.IsDir() {
			return nil, fmt.Errorf("%w: registered default Storage Location is unavailable at %s", ErrStorageLocationOffline, cleanPath)
		}
		if !rootcfg.Exists(cleanPath) {
			return nil, fmt.Errorf("%w: registered default Storage Location marker is missing at %s", ErrStorageLocationInvalid, cleanPath)
		}
	} else if err := os.MkdirAll(cleanPath, 0o755); err != nil {
		return nil, fmt.Errorf("create default storage location: %w", err)
	} else if err := applyDirectoryMode(cleanPath, 0o755); err != nil {
		return nil, fmt.Errorf("protect default storage location: %w", err)
	}
	if err := rm.claimRuntimeStoragePath(ctx, "storage_location", cleanPath); err != nil {
		return nil, err
	}
	if repocfg.IsStorageLocation(cleanPath) {
		return nil, fmt.Errorf("%w: a repository cannot also be a storage location", ErrStorageLocationInvalid)
	}

	createdMarker := defaultErr != nil && !rootcfg.Exists(cleanPath)
	var config *rootcfg.RootConfig
	if createdMarker {
		config = rootcfg.New("Default storage")
	} else if config, err = rm.identities().LoadLocation(cleanPath); err != nil {
		// A corrupt or newer root marker is preserved as it is: never replaced
		// at startup, reported so setup can diagnose it or choose elsewhere.
		return nil, fmt.Errorf("%w: %v", ErrStorageLocationInvalid, err)
	}
	if defaultErr == nil && config.ID != existingDefault.StorageLocationID.String() {
		return nil, fmt.Errorf("%w: configured default path contains a different .lumilioroot identity", ErrStorageLocationInvalid)
	}

	targetID := config.ID
	operation, replay, err := rm.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: "ensure-default:" + uuid.NewSHA1(uuid.NameSpaceURL, []byte(cleanPath)).String(),
		Kind:      lifecycleKindCreateStorageLocation,
		Payload: createStorageLocationOperationPayload{
			Path: cleanPath, Name: config.Name, Kind: dbtypes.StorageLocationKindDefault,
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
		registered, err := rm.queries.GetDefaultStorageLocation(ctx)
		if err != nil {
			return nil, err
		}
		diskConfig, err := rm.identities().LoadLocation(cleanPath)
		if err != nil || diskConfig.ID != registered.StorageLocationID.String() {
			return nil, fmt.Errorf("%w: default Storage Location identity is invalid", ErrStorageLocationInvalid)
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
	registered, err := rm.registerStorageLocation(ctx, cleanPath, config, dbtypes.StorageLocationKindDefault, false)
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
		createStorageLocationOperationResult{StorageLocationID: registered.StorageLocationID.String()}); err != nil {
		return nil, fmt.Errorf("default Storage Location registered but journal completion failed: %w", err)
	}
	return registered, nil
}

func (rm *DefaultRepositoryManager) switchDefaultStorageLocation(
	ctx context.Context,
	existing repo.StorageLocation,
	newPath string,
	request LifecycleRequest,
) (*repo.StorageLocation, error) {
	requestID := "switch-default:" + existing.StorageLocationID.String() + ":" + uuid.NewSHA1(uuid.NameSpaceURL, []byte(newPath)).String()
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
		if repository.StorageLocationID == existing.StorageLocationID {
			repositoryCount++
		}
	}
	targetID := existing.StorageLocationID.String()
	operation, replay, err := rm.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: requestID, Kind: lifecycleKindSwitchDefaultStorage,
		Payload: switchDefaultStorageOperationPayload{
			StorageLocationID: existing.StorageLocationID.String(), OldPath: existing.Path, NewPath: newPath,
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
		storageLocation, err := rm.queries.GetStorageLocation(ctx, existing.StorageLocationID)
		if err != nil {
			return nil, err
		}
		return &storageLocation, nil
	}
	if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseFilesystemApplied, nil); err != nil {
		return nil, err
	}
	storageLocation, err := rm.reconnectStorageLocation(ctx, existing.StorageLocationID.String(), newPath, true)
	if err != nil {
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, false, err, nil)
		return nil, err
	}
	if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseCatalogCommitted, nil); err != nil {
		return nil, err
	}
	if err := rm.completeLifecycleOperation(ctx, operation.OperationID,
		switchDefaultStorageOperationResult{
			StorageLocationID: storageLocation.StorageLocationID.String(), RepositoryCount: repositoryCount, FilesPreserved: true,
		}); err != nil {
		return nil, err
	}
	return storageLocation, nil
}

// freshDefaultSwitchEligible decides whether a configured default path that
// differs from the registration is a fresh default rather than a move. It is
// legal only before setup completes: no Primary Repository, no Repository
// registered under the default, and no running operation on it (smaller
// decision "Fresh default during setup"). A path carrying the registered
// identity is always an identity-preserving move.
func (rm *DefaultRepositoryManager) freshDefaultSwitchEligible(ctx context.Context, existing repo.StorageLocation, newPath string) (bool, error) {
	if reading := rootcfg.ReadMarker(rm.observerOrDefault(), newPath); reading.State == marker.Valid && reading.UUID == existing.StorageLocationID.String() {
		return false, nil
	}
	primaries, err := rm.queries.CountPrimaryRepositories(ctx)
	if err != nil {
		return false, fmt.Errorf("count primary repositories: %w", err)
	}
	if primaries > 0 {
		return false, nil
	}
	repositories, err := rm.queries.ListRepositories(ctx)
	if err != nil {
		return false, fmt.Errorf("inspect default Storage Location children: %w", err)
	}
	for _, repository := range repositories {
		if repository.StorageLocationID == existing.StorageLocationID {
			return false, nil
		}
	}
	operations, err := rm.queries.ListIncompleteLifecycleOperations(ctx)
	if err != nil {
		return false, fmt.Errorf("inspect pending default Storage Location operations: %w", err)
	}
	for _, operation := range operations {
		if operation.TargetType == "storage_location" && operation.TargetID != nil && *operation.TargetID == existing.StorageLocationID.String() {
			return false, nil
		}
	}
	return true, nil
}

// replaceUnusedDefaultStorageLocation registers a fresh Default Storage
// Location at newPath and drops the unused registration at the old path.
// The old tree is not touched: its marker and whatever it holds stay for an
// explicit open, adopt, or import later. A folder that already carries a
// compatible, unregistered root marker keeps that identity; a Repository,
// a corrupt or newer marker, or an identity registered elsewhere is refused.
func (rm *DefaultRepositoryManager) replaceUnusedDefaultStorageLocation(ctx context.Context, existing repo.StorageLocation, newPath string, request LifecycleRequest) (*repo.StorageLocation, error) {
	if err := os.MkdirAll(newPath, 0o755); err != nil {
		return nil, fmt.Errorf("create default storage location: %w", err)
	}
	if err := rm.claimRuntimeStoragePath(ctx, "storage_location", newPath); err != nil {
		return nil, err
	}
	if repocfg.IsStorageLocation(newPath) {
		return nil, fmt.Errorf("%w: a repository cannot also be a storage location", ErrStorageLocationInvalid)
	}
	var config *rootcfg.RootConfig
	createdMarker := false
	switch reading := rootcfg.ReadMarker(rm.observerOrDefault(), newPath); reading.State {
	case marker.Absent:
		config = rootcfg.New("Default storage")
		createdMarker = true
	case marker.Valid:
		loaded, err := rm.identities().LoadLocation(newPath)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrStorageLocationInvalid, err)
		}
		if _, err := rm.queries.GetStorageLocation(ctx, uuid.MustParse(loaded.ID)); err == nil {
			return nil, fmt.Errorf("%w: the selected folder carries a Storage Location identity that is already registered", ErrStorageLocationInvalid)
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("find storage location by id: %w", err)
		}
		config = loaded
	default:
		return nil, fmt.Errorf("%w: the selected folder carries a .lumilioroot marker this build cannot adopt (%s)", ErrStorageLocationInvalid, reading.State)
	}
	actor := strings.TrimSpace(request.Actor)
	if actor == "" {
		actor = "server:config"
	}
	requestID := strings.TrimSpace(request.RequestID)
	if requestID == "" {
		requestID = "fresh-default:" + existing.StorageLocationID.String() + ":" + uuid.NewSHA1(uuid.NameSpaceURL, []byte(newPath)).String()
	}
	targetID := existing.StorageLocationID.String()
	rollback := createStorageLocationRollbackData{Path: newPath, MarkerCreated: createdMarker}
	operation, replay, err := rm.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: requestID, Kind: lifecycleKindSwitchDefaultStorage,
		Payload: switchDefaultStorageOperationPayload{
			StorageLocationID: targetID, OldPath: existing.Path, NewPath: newPath,
			ConfirmationType: "fresh_default", RepositoryCount: 0, NewStorageLocationID: config.ID,
		},
		Actor: actor, ActorUserID: request.ActorUserID, HostInstanceID: request.HostInstanceID,
		TargetType: "storage_location", TargetID: &targetID, RollbackData: rollback,
	})
	if err != nil {
		return nil, err
	}
	if replay {
		if err := lifecycleReplayError(operation); err != nil {
			return nil, err
		}
		registered, err := rm.queries.GetStorageLocation(ctx, uuid.MustParse(config.ID))
		if err != nil {
			return nil, fmt.Errorf("%w: fresh default Storage Location is not registered", ErrStorageLocationInvalid)
		}
		return &registered, nil
	}
	fail := func(cause error) (*repo.StorageLocation, error) {
		if createdMarker {
			_ = os.Remove(filepath.Join(newPath, rootcfg.FileName))
		}
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, true, cause, rollback)
		return nil, cause
	}
	if createdMarker {
		if err := config.Save(newPath); err != nil {
			return fail(err)
		}
	}
	if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseFilesystemApplied, rollback); err != nil {
		return fail(err)
	}
	removed, err := rm.queries.DeleteUnusedDefaultStorageLocation(ctx, existing.StorageLocationID)
	if err != nil {
		return fail(fmt.Errorf("release unused default Storage Location: %w", err))
	}
	if removed == 0 {
		return fail(fmt.Errorf("%w: the registered default Storage Location is in use", ErrStorageLocationInvalid))
	}
	registered, err := rm.registerStorageLocation(ctx, newPath, config, dbtypes.StorageLocationKindDefault, false)
	if err != nil {
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, false, err, rollback)
		return nil, err
	}
	if err := rm.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseCatalogCommitted, rollback); err != nil {
		return nil, fmt.Errorf("fresh default Storage Location registered but journal commit phase failed: %w", err)
	}
	if err := rm.completeLifecycleOperation(ctx, operation.OperationID,
		switchDefaultStorageOperationResult{StorageLocationID: registered.StorageLocationID.String(), RepositoryCount: 0, FilesPreserved: true}); err != nil {
		return nil, fmt.Errorf("fresh default Storage Location registered but journal completion failed: %w", err)
	}
	rm.logger.Info("default Storage Location started fresh elsewhere; the previous tree is untouched",
		zap.String("operation", "storage_location.fresh_default"),
		zap.String("previous_path", existing.Path), zap.String("path", newPath),
		zap.String("previous_storage_location_id", existing.StorageLocationID.String()),
		zap.String("storage_location_id", registered.StorageLocationID.String()))
	return registered, nil
}

// recoverFreshDefaultStorageOperation finishes a fresh-default switch a
// crash interrupted. A new registration that exists is completed (the old,
// unused row is released if it is still there); a new marker on disk with
// the old row already gone is rolled forward; otherwise the created marker
// is removed and the operation records its rollback.
func (rm *DefaultRepositoryManager) recoverFreshDefaultStorageOperation(ctx context.Context, operation repo.LifecycleOperation, payload switchDefaultStorageOperationPayload) error {
	oldID, err := uuid.Parse(payload.StorageLocationID)
	if err != nil {
		return fmt.Errorf("%w: invalid default Storage Location identity", ErrLifecycleRecoveryRequired)
	}
	newID, err := uuid.Parse(payload.NewStorageLocationID)
	if err != nil {
		return fmt.Errorf("%w: invalid fresh default Storage Location identity", ErrLifecycleRecoveryRequired)
	}
	rollback := createStorageLocationRollbackData{}
	if operation.RollbackData != nil {
		if err := json.Unmarshal(*operation.RollbackData, &rollback); err != nil {
			return fmt.Errorf("%w: decode fresh default rollback data: %v", ErrLifecycleRecoveryRequired, err)
		}
	}
	complete := func() error {
		return rm.completeRecoveredLifecycleOperation(ctx, operation.OperationID,
			switchDefaultStorageOperationResult{StorageLocationID: newID.String(), FilesPreserved: true})
	}
	if _, err := rm.queries.GetStorageLocation(ctx, newID); err == nil {
		if _, err := rm.queries.DeleteUnusedDefaultStorageLocation(ctx, oldID); err != nil {
			return fmt.Errorf("release unused default Storage Location: %w", err)
		}
		return complete()
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("recover fresh default Storage Location: %w", err)
	}
	_, oldErr := rm.queries.GetStorageLocation(ctx, oldID)
	diskConfig, diskErr := rm.identities().LoadLocation(payload.NewPath)
	if errors.Is(oldErr, sql.ErrNoRows) {
		if diskErr != nil || diskConfig.ID != newID.String() {
			return fmt.Errorf("%w: fresh default Storage Location marker is missing at %s", ErrLifecycleRecoveryRequired, payload.NewPath)
		}
		if _, err := rm.registerStorageLocation(ctx, payload.NewPath, diskConfig, dbtypes.StorageLocationKindDefault, false); err != nil {
			return fmt.Errorf("%w: roll forward fresh default Storage Location: %v", ErrLifecycleRecoveryRequired, err)
		}
		return complete()
	}
	if oldErr != nil {
		return fmt.Errorf("recover fresh default Storage Location: %w", oldErr)
	}
	if rollback.MarkerCreated && diskErr == nil && diskConfig.ID == newID.String() {
		if err := os.Remove(filepath.Join(payload.NewPath, rootcfg.FileName)); err != nil && !errors.Is(err, os.ErrNotExist) {
			_ = rm.recoverLifecycleOperationFailure(ctx, operation.OperationID, false, err, rollback)
			return fmt.Errorf("%w: remove fresh default marker: %v", ErrLifecycleRecoveryRequired, err)
		}
	}
	return rm.recoverLifecycleOperationFailure(ctx, operation.OperationID, true, errors.New("rolled back incomplete fresh default Storage Location switch"), rollback)
}

// AddStorageLocation registers a native-host-authorized directory as an
// external Storage Location. The directory must already exist; the server never
// turns a missing mount path into a new directory.
func (rm *DefaultRepositoryManager) AddStorageLocation(ctx context.Context, path, name string, requests ...LifecycleRequest) (*repo.StorageLocation, error) {
	cleanPath, err := rm.validateStorageLocationPath(path)
	if err != nil {
		return nil, err
	}
	if err := rm.claimRuntimeStoragePath(ctx, "storage_location", cleanPath); err != nil {
		return nil, err
	}
	storageLocationName := strings.TrimSpace(name)
	if storageLocationName == "" {
		storageLocationName = filepath.Base(cleanPath)
	}
	createdMarker := !rootcfg.Exists(cleanPath)
	var config *rootcfg.RootConfig
	if createdMarker {
		config = rootcfg.New(storageLocationName)
	} else if config, err = rm.identities().LoadLocation(cleanPath); err != nil {
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
			Path: cleanPath, Name: storageLocationName, Kind: dbtypes.StorageLocationKindExternal,
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
		storageLocationID, err := uuid.Parse(*operation.TargetID)
		if err != nil {
			return nil, fmt.Errorf("%w: completed Storage Location operation has an invalid target", ErrLifecycleRecoveryRequired)
		}
		storageLocation, err := rm.queries.GetStorageLocation(ctx, storageLocationID)
		if err != nil {
			return nil, fmt.Errorf("load completed Storage Location result: %w", err)
		}
		return &storageLocation, nil
	}
	failPrepared := func(cause error, markerCreated bool) (*repo.StorageLocation, error) {
		rollback := createStorageLocationRollbackData{Path: cleanPath, MarkerCreated: markerCreated}
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, true, cause, rollback)
		return nil, cause
	}

	if existing, err := rm.queries.GetStorageLocationByPath(ctx, cleanPath); err == nil {
		if config.ID != existing.StorageLocationID.String() {
			return failPrepared(fmt.Errorf("%w: database and .lumilioroot identities differ", ErrStorageLocationInvalid), false)
		}
		if err := rm.completeLifecycleOperation(ctx, operation.OperationID,
			createStorageLocationOperationResult{StorageLocationID: existing.StorageLocationID.String()}); err != nil {
			return nil, err
		}
		return &existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return failPrepared(fmt.Errorf("find storage location by path: %w", err), false)
	}
	if err := rm.rejectOverlappingStorageLocation(ctx, cleanPath); err != nil {
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
	registered, err := rm.registerStorageLocation(ctx, cleanPath, config, dbtypes.StorageLocationKindExternal, false)
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
		createStorageLocationOperationResult{StorageLocationID: registered.StorageLocationID.String()}); err != nil {
		return nil, fmt.Errorf("Storage Location registered but journal completion failed: %w", err)
	}
	return registered, nil
}

// ReconnectStorageLocation re-points an existing external Storage Location,
// and every child Repository with it, at the tree that now carries its
// identity after an external move or remount. The marker at the requested
// path must carry the registered identity; this never rewrites or guesses
// identity and never moves data.
func (rm *DefaultRepositoryManager) ReconnectStorageLocation(ctx context.Context, id, path string, requests ...LifecycleRequest) (*repo.StorageLocation, error) {
	storageLocationID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return nil, fmt.Errorf("invalid storage location id: %w", err)
	}
	registered, err := rm.queries.GetStorageLocation(ctx, storageLocationID)
	if err != nil {
		return nil, err
	}
	cleanPath, err := CanonicalizeRepositoryPath(path)
	if err != nil {
		return nil, err
	}
	targetID := storageLocationID.String()
	request := firstLifecycleRequest(requests)
	oldPath := registered.Path
	replaying := false
	if requestID := strings.TrimSpace(request.RequestID); requestID != "" {
		if existing, lookupErr := rm.queries.GetLifecycleOperationByRequestID(ctx, requestID); lookupErr == nil {
			var existingPayload switchDefaultStorageOperationPayload
			if existing.Kind != lifecycleKindReconnectStorage || json.Unmarshal(existing.Payload, &existingPayload) != nil ||
				existingPayload.StorageLocationID != storageLocationID.String() || existingPayload.NewPath != cleanPath {
				return nil, ErrLifecycleRequestConflict
			}
			oldPath = existingPayload.OldPath
			replaying = true
		} else if !errors.Is(lookupErr, sql.ErrNoRows) {
			return nil, fmt.Errorf("find Storage Location reconnect request: %w", lookupErr)
		}
	}
	// The kind-aware policy on the projected row, before the destination is
	// looked at: the Default Storage Location is never reconnected on its
	// own, and a registered path that cannot be read admits nothing. A
	// replay skips it; its row is already at the new path.
	if !replaying && registered.Path != cleanPath {
		if err := rm.admitStorageLocationAction(ctx, registered, LocationActionReconnect); err != nil {
			var unavailable *StorageLocationActionUnavailableError
			if errors.As(err, &unavailable) && len(unavailable.Blockers) == 1 && unavailable.Blockers[0] == BlockerOriginalAvailable {
				return nil, fmt.Errorf("%w: %s", ErrStorageLocationOriginalOnline, registered.Path)
			}
			return nil, err
		}
	}
	operation, replay, err := rm.beginLifecycleOperation(ctx, lifecycleBeginInput{
		RequestID: request.RequestID,
		Kind:      lifecycleKindReconnectStorage,
		Payload:   switchDefaultStorageOperationPayload{StorageLocationID: storageLocationID.String(), OldPath: oldPath, NewPath: cleanPath},
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
		storageLocation, err := rm.queries.GetStorageLocation(ctx, storageLocationID)
		return &storageLocation, err
	}
	storageLocation, err := rm.reconnectStorageLocation(ctx, id, cleanPath, false)
	if err != nil {
		_ = rm.failLifecycleOperation(ctx, operation.OperationID, false, err, nil)
		return nil, err
	}
	if err := rm.completeLifecycleOperation(ctx, operation.OperationID, createStorageLocationOperationResult{StorageLocationID: storageLocationID.String()}); err != nil {
		return nil, err
	}
	return storageLocation, nil
}

func (rm *DefaultRepositoryManager) reconnectStorageLocation(ctx context.Context, id, path string, allowDefault bool) (*repo.StorageLocation, error) {
	storageLocationID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return nil, fmt.Errorf("invalid storage location id: %w", err)
	}
	registered, err := rm.queries.GetStorageLocation(ctx, storageLocationID)
	if err != nil {
		return nil, err
	}
	if registered.Kind != dbtypes.StorageLocationKindExternal && !allowDefault {
		return nil, ErrStorageLocationNotRemovable
	}

	cleanPath, err := rm.validateStorageLocationPath(path)
	if err != nil {
		return nil, err
	}
	config, err := rm.identities().LoadLocation(cleanPath)
	if err != nil {
		return nil, err
	}
	if config.ID != storageLocationID.String() {
		return nil, fmt.Errorf("%w: %w: selected directory has a different .lumilioroot identity", ErrStorageLocationInvalid, ErrRepositoryIDMismatch)
	}
	if registered.Path != cleanPath {
		// The registered path decides under the barrier: an original that is
		// online is a conflict, and one that cannot be read authorizes
		// nothing. Only a positively unavailable original is reconnected.
		switch rm.storageLocationOriginalState(registered.Path, storageLocationID.String()) {
		case OriginalOnline:
			return nil, fmt.Errorf("%w: %s", ErrStorageLocationOriginalOnline, registered.Path)
		case OriginalInaccessible:
			return nil, &StorageLocationActionUnavailableError{
				StorageLocationID: storageLocationID.String(), Action: LocationActionReconnect,
				Blockers:     []RepositoryActionBlocker{BlockerOriginalInaccessible},
				Alternatives: []RepositoryActionAlternative{AlternativeCheckAccess},
			}
		}
	}
	if err := rm.claimRuntimeStoragePath(ctx, "storage_location", cleanPath); err != nil {
		return nil, err
	}
	if err := rm.rejectOverlappingStorageLocationExcept(ctx, cleanPath, storageLocationID); err != nil {
		return nil, err
	}
	if rm.database == nil {
		return nil, errors.New("repository catalog transaction is unavailable")
	}
	coordinator := rm.files.AccessCoordinator()
	releaseStorageLocation, err := coordinator.AcquireStorageLocationMutationContext(ctx, storageLocationID)
	if err != nil {
		return nil, fmt.Errorf("%w: Storage Location is busy: %v", ErrRepositoryBusy, err)
	}
	defer releaseStorageLocation()

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
		if repository.StorageLocationID != storageLocationID {
			continue
		}
		requestedRepositoryPath, moveErr := relocatedRepositoryPath(registered.Path, cleanPath, repository.Path)
		if moveErr != nil {
			return nil, moveErr
		}
		// Every child must be where the Location layout says it is, carrying
		// its registered identity; one that is missing or different refuses
		// the whole reconnect before any row changes.
		repositoryConfig, loadErr := rm.identities().LoadRepository(requestedRepositoryPath)
		if loadErr != nil {
			return nil, fmt.Errorf("%w: %w: %s does not hold repository %s: %v", ErrStorageLocationInvalid, ErrRepositoryIDMismatch, requestedRepositoryPath, repository.RepoID, loadErr)
		}
		if repositoryConfig.ID != repository.RepoID.String() {
			return nil, fmt.Errorf("%w: %w: repository identity differs at %s", ErrStorageLocationInvalid, ErrRepositoryIDMismatch, requestedRepositoryPath)
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
	// every BeginRepositoryActivity caller can now observe/refuse this storageLocation and
	// its children, instead of maintenance existing only inside the final tx.
	maintenanceTx, err := rm.writer.BeginTx(ctx, catalogtx.OperationStorageLocationRelocateMaintenance, nil)
	if err != nil {
		return nil, fmt.Errorf("begin Storage Location maintenance: %w", err)
	}
	maintenanceQueries := rm.queries.WithTx(maintenanceTx.Raw())
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := maintenanceQueries.UpdateStorageLocationFromDisk(ctx, repo.UpdateStorageLocationFromDiskParams{
		StorageLocationID: storageLocationID, Name: registered.Name,
		Status: dbtypes.StorageLocationStatusMaintenance, UpdatedAt: now,
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
		_, _ = rm.queries.UpdateStorageLocationFromDisk(background, repo.UpdateStorageLocationFromDiskParams{
			StorageLocationID: storageLocationID, Name: registered.Name, Status: registered.Status, UpdatedAt: restoreNow,
		})
		for _, move := range moves {
			_, _ = rm.queries.EndRepositoryMaintenance(background, repo.EndRepositoryMaintenanceParams{
				RepoID: move.repository.RepoID, Reachability: move.repository.Reachability,
				Activity: move.repository.Activity, UpdatedAt: restoreNow,
			})
		}
	}()

	tx, err := rm.writer.BeginTx(ctx, catalogtx.OperationStorageLocationRelocate, nil)
	if err != nil {
		return nil, fmt.Errorf("begin Storage Location relocate: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	queries := rm.queries.WithTx(tx.Raw())
	now = dbtypes.NewTimestamp(time.Now().UTC())
	for _, move := range moves {
		updated, updateErr := queries.UpdateRepositoryPath(ctx, repo.UpdateRepositoryPathParams{
			RepoID: move.repository.RepoID, Path: move.path, StorageLocationID: storageLocationID,
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
		if updateErr := invalidateRepositoryObservationAfterRelocation(ctx, tx, move.repository.RepoID, move.path, now); updateErr != nil {
			return nil, updateErr
		}
		if _, updateErr := queries.EndRepositoryMaintenance(ctx, repo.EndRepositoryMaintenanceParams{
			RepoID: updated.RepoID, Reachability: dbtypes.RepositoryReachabilityActive,
			Activity: dbtypes.RepositoryActivityIdle, UpdatedAt: now,
		}); updateErr != nil {
			return nil, fmt.Errorf("leave relocated repository maintenance: %w", updateErr)
		}
	}
	storageLocation, err := queries.UpsertStorageLocation(ctx, repo.UpsertStorageLocationParams{
		StorageLocationID: storageLocationID, Name: config.Name, Path: cleanPath,
		Kind: registered.Kind, Status: dbtypes.StorageLocationStatusActive,
		MountFingerprint: InspectStoragePath(cleanPath).MountFingerprint,
		CreatedAt:        registered.CreatedAt, UpdatedAt: now,
	})
	if err != nil {
		return nil, fmt.Errorf("relocate Storage Location row: %w", err)
	}
	storageLocation, err = queries.UpdateStorageLocationMountFingerprint(ctx, repo.UpdateStorageLocationMountFingerprintParams{
		StorageLocationID: storageLocationID, MountFingerprint: InspectStoragePath(cleanPath).MountFingerprint, UpdatedAt: now,
	})
	if err != nil {
		return nil, fmt.Errorf("record relocated Storage Location mount fingerprint: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit Storage Location relocate: %w", err)
	}
	maintenanceCommitted = false
	return &storageLocation, nil
}

func (rm *DefaultRepositoryManager) validateStorageLocationPath(path string) (string, error) {
	cleanPath, err := CanonicalizeRepositoryPath(path)
	if err != nil {
		return "", fmt.Errorf("canonicalize storage location: %w", err)
	}
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", ErrStorageLocationOffline, cleanPath)
		}
		return "", fmt.Errorf("stat storage location: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%w: path is not a directory", ErrStorageLocationInvalid)
	}
	if repocfg.IsStorageLocation(cleanPath) {
		return "", fmt.Errorf("%w: choose attach repository for a .lumiliorepo directory", ErrStorageLocationInvalid)
	}
	if isInsidePhotosLibrary(cleanPath) {
		return "", fmt.Errorf("%w: a Photos library bundle cannot be a storage location", ErrStorageLocationInvalid)
	}
	if nested, parent, err := rm.isNestedRepository(cleanPath); err != nil {
		return "", err
	} else if nested {
		return "", fmt.Errorf("%w: path is inside repository %s", ErrStorageLocationInvalid, parent)
	}
	return cleanPath, nil
}

func (rm *DefaultRepositoryManager) registerStorageLocation(
	ctx context.Context,
	path string,
	config *rootcfg.RootConfig,
	kind dbtypes.StorageLocationKind,
	allowMove bool,
) (*repo.StorageLocation, error) {
	storageLocationID, err := uuid.Parse(config.ID)
	if err != nil {
		return nil, fmt.Errorf("parse storage location id: %w", err)
	}
	if registered, err := rm.queries.GetStorageLocation(ctx, storageLocationID); err == nil {
		if registered.Path != path && !allowMove {
			actions := []string{"relocate"}
			if marker, markerErr := rm.identities().LoadLocation(registered.Path); markerErr == nil && marker.ID == config.ID {
				actions = nil
			}
			return nil, &StorageLocationConflictError{
				StorageLocationID: config.ID,
				RegisteredPath:    registered.Path,
				RequestedPath:     path,
				Actions:           actions,
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
	registered, err := rm.queries.UpsertStorageLocation(ctx, repo.UpsertStorageLocationParams{
		StorageLocationID: storageLocationID,
		Name:              config.Name,
		Path:              path,
		Kind:              kind,
		Status:            dbtypes.StorageLocationStatusActive,
		MountFingerprint:  InspectStoragePath(path).MountFingerprint,
		CreatedAt:         dbtypes.NewTimestamp(createdAt),
		UpdatedAt:         dbtypes.NewTimestamp(now),
	})
	if err != nil {
		return nil, fmt.Errorf("register storage location: %w", err)
	}
	return &registered, nil
}

// ListStorageLocations returns the latest reconciled reachability projection.
// Startup and the portable background reconciler own disk inspection and
// writes; a foreground list must remain available while SQLite's writer is
// busy.
func (rm *DefaultRepositoryManager) ListStorageLocations(ctx context.Context) ([]repo.StorageLocation, error) {
	storageLocations, err := rm.readerQueries.ListStorageLocations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list storage locations: %w", err)
	}
	return storageLocations, nil
}

func (rm *DefaultRepositoryManager) ReconcileStorageLocations(ctx context.Context) error {
	storageLocations, err := rm.readerQueries.ListStorageLocations(ctx)
	if err != nil {
		return fmt.Errorf("list storage locations for reconcile: %w", err)
	}
	for _, storageLocation := range storageLocations {
		if storageLocation.Status == dbtypes.StorageLocationStatusMaintenance {
			continue
		}
		status := dbtypes.StorageLocationStatusActive
		name := storageLocation.Name
		if info, statErr := os.Stat(storageLocation.Path); statErr != nil || !info.IsDir() {
			status = dbtypes.StorageLocationStatusOffline
		} else if config, loadErr := rm.identities().LoadLocation(storageLocation.Path); loadErr != nil {
			status = dbtypes.StorageLocationStatusError
		} else if config.ID != storageLocation.StorageLocationID.String() {
			status = dbtypes.StorageLocationStatusError
		} else {
			name = config.Name
		}
		_, updateErr := rm.queries.UpdateStorageLocationFromDisk(ctx, repo.UpdateStorageLocationFromDiskParams{
			StorageLocationID: storageLocation.StorageLocationID,
			Name:              name,
			Status:            status,
			UpdatedAt:         dbtypes.NewTimestamp(time.Now()),
		})
		if updateErr != nil {
			return fmt.Errorf("update storage location status: %w", updateErr)
		}
	}
	return nil
}

func (rm *DefaultRepositoryManager) GetStorageLocation(ctx context.Context, id string) (*repo.StorageLocation, error) {
	storageLocationID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return nil, fmt.Errorf("invalid storage location id: %w", err)
	}
	storageLocation, err := rm.queries.GetStorageLocation(ctx, storageLocationID)
	if err != nil {
		return nil, err
	}
	return &storageLocation, nil
}

func (rm *DefaultRepositoryManager) PreviewStorageLocationRemoval(ctx context.Context, id string) (StorageLocationRemovalImpact, error) {
	storageLocationID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return StorageLocationRemovalImpact{}, fmt.Errorf("invalid storage location id: %w", err)
	}
	storageLocation, err := rm.queries.GetStorageLocation(ctx, storageLocationID)
	if err != nil {
		return StorageLocationRemovalImpact{}, err
	}
	impact := StorageLocationRemovalImpact{
		StorageLocationID: storageLocation.StorageLocationID.String(), StorageLocationName: storageLocation.Name, Kind: storageLocation.Kind, FilesPreserved: true,
	}
	if err := rm.readerDatabase.QueryRowContext(ctx,
		"SELECT count(*) FROM repositories WHERE storage_location_id = ?", storageLocationID,
	).Scan(&impact.RepositoryCount); err != nil {
		return StorageLocationRemovalImpact{}, fmt.Errorf("count Storage Location repositories: %w", err)
	}
	if err := rm.readerDatabase.QueryRowContext(ctx, `
		SELECT count(*) FROM lifecycle_operations
		WHERE target_type = 'storage_location' AND target_id = ? AND status = 'running'
	`, storageLocationID.String()).Scan(&impact.ActiveOperationCount); err != nil {
		return StorageLocationRemovalImpact{}, fmt.Errorf("count Storage Location operations: %w", err)
	}
	switch {
	case storageLocation.Kind != dbtypes.StorageLocationKindExternal:
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

func (rm *DefaultRepositoryManager) DeleteStorageLocation(ctx context.Context, id string, requests ...LifecycleRequest) error {
	storageLocationID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("invalid storage location id: %w", err)
	}
	storageLocation, err := rm.queries.GetStorageLocation(ctx, storageLocationID)
	if err != nil {
		return err
	}
	if storageLocation.Kind != dbtypes.StorageLocationKindExternal {
		return ErrStorageLocationNotRemovable
	}
	coordinator := rm.files.AccessCoordinator()
	releaseStorageLocation, err := coordinator.AcquireStorageLocationMutationContext(ctx, storageLocationID)
	if err != nil {
		return fmt.Errorf("%w: Storage Location is busy: %v", ErrRepositoryBusy, err)
	}
	defer releaseStorageLocation()
	impact, err := rm.PreviewStorageLocationRemoval(ctx, id)
	if err != nil {
		return err
	}
	if impact.RepositoryCount != 0 {
		return ErrStorageLocationInUse
	}
	if impact.ActiveOperationCount != 0 {
		return fmt.Errorf("%w: Storage Location has an active lifecycle operation", ErrRepositoryBusy)
	}
	tx, err := rm.writer.BeginTx(ctx, catalogtx.OperationStorageLocationDelete, nil)
	if err != nil {
		return fmt.Errorf("begin Storage Location removal: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	queries := rm.queries.WithTx(tx.Raw())
	deleted, err := queries.DeleteExternalStorageLocation(ctx, storageLocationID)
	if err != nil {
		return fmt.Errorf("remove storage location: %w", err)
	}
	if deleted == 0 {
		return ErrStorageLocationInUse
	}
	request := firstLifecycleRequest(requests)
	if _, err := recordLifecycleAuditWithQueries(ctx, queries, LifecycleAuditInput{
		Actor: request.Actor, ActorUserID: request.ActorUserID, HostInstanceID: request.HostInstanceID, RequestID: request.RequestID,
		Action: "remove_storage_location", TargetType: "storage_location", TargetID: id,
		Source: auditSourceForActor(request.Actor), ConfirmationType: "summary",
		OldPath: storageLocation.Path, Result: AuditResultSucceeded,
		Details: map[string]any{"storage_location_name": storageLocation.Name, "files_preserved": true},
	}); err != nil {
		return fmt.Errorf("audit Storage Location removal: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Storage Location removal: %w", err)
	}
	rm.repoAudit(storageLocation.Path).Operation("storage_location.remove",
		zap.String("storage_location_id", storageLocation.StorageLocationID.String()),
		zap.String("storage_location_name", storageLocation.Name),
		zap.String("preserved_path", storageLocation.Path),
	)
	return nil
}

func (rm *DefaultRepositoryManager) resolveStorageLocationForCreate(ctx context.Context, id string, role dbtypes.RepoRole) (*repo.StorageLocation, error) {
	var storageLocation repo.StorageLocation
	var err error
	if strings.TrimSpace(id) == "" {
		storageLocation, err = rm.queries.GetDefaultStorageLocation(ctx)
	} else {
		storageLocationID, parseErr := uuid.Parse(strings.TrimSpace(id))
		if parseErr != nil {
			return nil, fmt.Errorf("invalid storage location id: %w", parseErr)
		}
		storageLocation, err = rm.queries.GetStorageLocation(ctx, storageLocationID)
	}
	if err != nil {
		return nil, fmt.Errorf("load storage location: %w", err)
	}
	if normalizeRepoRole(role) == dbtypes.RepoRolePrimary && storageLocation.Kind != dbtypes.StorageLocationKindDefault {
		return nil, fmt.Errorf("%w: primary repository must use the default storage location", ErrPathNotAllowed)
	}
	if err := validateRegisteredStorageLocationMarker(storageLocation); err != nil {
		return nil, err
	}
	return &storageLocation, nil
}

func validateRegisteredStorageLocationMarker(storageLocation repo.StorageLocation) error {
	config, err := (LocalRepositoryIdentityDetector{}).LoadLocation(storageLocation.Path)
	if err != nil || config.ID != storageLocation.StorageLocationID.String() {
		return fmt.Errorf("%w: %s", ErrStorageLocationInvalid, storageLocation.Path)
	}
	return nil
}

func (rm *DefaultRepositoryManager) storageLocationIDForPath(ctx context.Context, path string) (uuid.UUID, error) {
	storageLocations, err := rm.queries.ListStorageLocations(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("list storage locations for repository association: %w", err)
	}
	for _, storageLocation := range storageLocations {
		if !pathIsDirectChild(storageLocation.Path, path) {
			continue
		}
		if err := validateRegisteredStorageLocationMarker(storageLocation); err != nil {
			return uuid.Nil, err
		}
		return storageLocation.StorageLocationID, nil
	}
	return uuid.Nil, fmt.Errorf(
		"%w: repository %s must be a direct child of a registered Storage Location",
		ErrPathNotAllowed,
		path,
	)
}

func (rm *DefaultRepositoryManager) resolveRepositoryAssociation(ctx context.Context, path string, requested []uuid.UUID) (uuid.UUID, error) {
	if len(requested) == 0 {
		return rm.storageLocationIDForPath(ctx, path)
	}
	if len(requested) != 1 {
		return uuid.Nil, fmt.Errorf("%w: exactly one Storage Location is required", ErrPathNotAllowed)
	}
	storageLocation, err := rm.queries.GetStorageLocation(ctx, requested[0])
	if err != nil {
		return uuid.Nil, fmt.Errorf("load repository Storage Location: %w", err)
	}
	if !pathIsDirectChild(storageLocation.Path, path) {
		return uuid.Nil, fmt.Errorf(
			"%w: repository %s must be a direct child of Storage Location %s",
			ErrPathNotAllowed,
			path,
			storageLocation.Path,
		)
	}
	if err := validateRegisteredStorageLocationMarker(storageLocation); err != nil {
		return uuid.Nil, err
	}
	return storageLocation.StorageLocationID, nil
}

func (rm *DefaultRepositoryManager) rejectOverlappingStorageLocation(ctx context.Context, requested string) error {
	return rm.rejectOverlappingStorageLocationExcept(ctx, requested, uuid.Nil)
}

func (rm *DefaultRepositoryManager) rejectOverlappingStorageLocationExcept(ctx context.Context, requested string, except uuid.UUID) error {
	storageLocations, err := rm.queries.ListStorageLocations(ctx)
	if err != nil {
		return fmt.Errorf("list storage locations for overlap check: %w", err)
	}
	for _, storageLocation := range storageLocations {
		if except != uuid.Nil && storageLocation.StorageLocationID == except {
			continue
		}
		if storageLocation.Path == requested || pathIsStrictlyInside(storageLocation.Path, requested) || pathIsStrictlyInside(requested, storageLocation.Path) {
			return fmt.Errorf("%w: %s and %s", ErrStorageLocationOverlap, requested, storageLocation.Path)
		}
	}
	return nil
}

func pathIsStrictlyInside(storageLocation, path string) bool {
	rel, err := filepath.Rel(storageLocation, path)
	if err != nil || rel == "." || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func pathIsDirectChild(storageLocation, path string) bool {
	relative, err := filepath.Rel(storageLocation, path)
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
		return "", fmt.Errorf("%w: repository %s is not a direct child of its registered Storage Location", ErrStorageLocationInvalid, repositoryPath)
	}
	return filepath.Join(newRoot, relative), nil
}

// StorageLocationWarnings returns non-fatal placement risks for the Desktop
// Control Panel to surface immediately after a native directory grant.
func StorageLocationWarnings(path string) []string {
	provider := cloudSyncProvider(path)
	if provider == "" {
		return nil
	}
	return []string{fmt.Sprintf(
		"%s is inside %s. Sync clients may evict originals or duplicate files.", path, provider,
	)}
}
