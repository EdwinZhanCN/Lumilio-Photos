package storage

import (
	"context"
	"path/filepath"
	"testing"

	"server/internal/db/dbtypes"
	"server/internal/storage/repocfg"
)

// F09: the enqueue admission gate is the child's own reachability and activity,
// not its parent Storage Location's health.
func TestBeginRepositoryWorkIgnoresParentStorageLocationFailure(t *testing.T) {
	for _, failure := range parentStorageLocationFailures {
		t.Run(failure, func(t *testing.T) {
			_, manager := newCatalogRepositoryManager(t)
			ctx := context.Background()
			rootPath := filepath.Join(t.TempDir(), "default")
			initializeDefaultStorageForTest(t, manager, rootPath)
			repository, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
			if err != nil {
				t.Fatal(err)
			}
			failParentStorageLocationForTest(t, manager, rootPath, repository, failure)

			started, release, err := manager.BeginRepositoryWork(
				ctx, repository.RepoID.String(), dbtypes.RepositoryActivityScanning,
			)
			if err != nil {
				t.Fatalf("repository work denied by parent %s: %v", failure, err)
			}
			if started.RepoID != repository.RepoID || started.Activity != dbtypes.RepositoryActivityScanning {
				t.Fatalf("started repository work = %+v", started)
			}
			if err := release(); err != nil {
				t.Fatal(err)
			}
			idle, err := manager.queries.GetRepository(ctx, repository.RepoID)
			if err != nil || idle.Activity != dbtypes.RepositoryActivityIdle {
				t.Fatalf("repository activity after release = %+v, error = %v", idle, err)
			}
		})
	}
}

// F10/F09: recovery of an interrupted rename re-validates the repository's own
// marker identity and never requires a readable parent Storage Location.
func TestRenameRecoveryIgnoresParentStorageLocationFailure(t *testing.T) {
	for _, failure := range parentStorageLocationFailures {
		t.Run(failure, func(t *testing.T) {
			_, manager := newCatalogRepositoryManager(t)
			ctx := context.Background()
			rootPath := filepath.Join(t.TempDir(), "default")
			initializeDefaultStorageForTest(t, manager, rootPath)
			repository, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
			if err != nil {
				t.Fatal(err)
			}
			failParentStorageLocationForTest(t, manager, rootPath, repository, failure)

			diskConfig, err := repocfg.LoadConfigFromFile(repository.Path)
			if err != nil {
				t.Fatal(err)
			}
			targetID := repository.RepoID.String()
			crashConfig := *diskConfig
			crashConfig.Name = "Recovered Rename"
			operation, _, err := manager.beginLifecycleOperation(ctx, lifecycleBeginInput{
				RequestID: "rename-recovery-parent-independent-" + failure, Kind: lifecycleKindRenameRepository,
				Payload: renameRepositoryOperationPayload{
					RepositoryID: targetID, Path: repository.Path, NewName: crashConfig.Name,
				},
				Actor: "test", TargetType: "repository", TargetID: &targetID,
				RollbackData: renameRepositoryRollbackData{PreviousConfig: *diskConfig},
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := crashConfig.SaveConfigToFile(repository.Path); err != nil {
				t.Fatal(err)
			}
			if err := manager.updateLifecycleOperationPhase(ctx, operation.OperationID, lifecyclePhaseFilesystemApplied,
				renameRepositoryRollbackData{PreviousConfig: *diskConfig}); err != nil {
				t.Fatal(err)
			}

			if err := manager.RecoverLifecycleOperations(ctx); err != nil {
				t.Fatalf("rename recovery vetoed by parent %s: %v", failure, err)
			}
			recovered, err := manager.queries.GetRepository(ctx, repository.RepoID)
			if err != nil || recovered.Name != crashConfig.Name {
				t.Fatalf("recovered rename = %+v, error = %v", recovered, err)
			}
			recoveredDisk, err := repocfg.LoadConfigFromFile(repository.Path)
			if err != nil || recoveredDisk.Name != crashConfig.Name || recoveredDisk.ID != targetID {
				t.Fatalf("recovered marker = %+v, error = %v", recoveredDisk, err)
			}
			completed, err := manager.queries.GetLifecycleOperation(ctx, operation.OperationID)
			if err != nil || completed.Status != lifecycleStatusCompleted {
				t.Fatalf("recovered operation = %+v, error = %v", completed, err)
			}
		})
	}
}
