package app

import (
	"context"
	"fmt"

	"server/config"
	"server/internal/db"
	dbbackup "server/internal/db/backup"
	"server/internal/storage"
	"server/internal/version"
)

// preflightRepositoryObservationCutover establishes the recovery boundary for
// an existing path-keyed catalog. run calls it before constructing or starting
// River and before constructing any source materializer, so this process owns
// no in-flight staging commit while the callback runs.
func preflightRepositoryObservationCutover(
	ctx context.Context,
	database *db.DB,
	appConfig config.AppConfig,
	plan db.DestructiveMigrationPlan,
	logf dbbackup.Logf,
) error {
	if database == nil || database.SQL == nil {
		return fmt.Errorf("cutover catalog is unavailable")
	}
	if plan.FromApplicationMigration <= 0 || plan.ToApplicationMigration <= plan.FromApplicationMigration {
		return fmt.Errorf("invalid destructive migration plan: %+v", plan)
	}
	info, err := db.InspectCatalog(ctx, database.Path)
	if err != nil {
		return fmt.Errorf("inspect pre-cutover catalog: %w", err)
	}
	if info.ApplicationMigration != plan.FromApplicationMigration {
		return fmt.Errorf(
			"pre-cutover catalog migration = %d, planned from %d",
			info.ApplicationMigration,
			plan.FromApplicationMigration,
		)
	}
	metadata := dbbackup.SnapshotMetadata{
		AppVersion:          version.Version,
		ConfigSchemaVersion: appConfig.SchemaVersion,
	}
	snapshot, err := dbbackup.CreateSnapshot(
		ctx,
		database.ReaderSQL,
		appConfig.StorageConfig.BackupsDir(),
		dbbackup.CutoverPointPrefix,
		metadata,
		logf,
	)
	if err != nil {
		return fmt.Errorf("create pre-cutover Online Backup: %w", err)
	}
	manifest, verified, err := dbbackup.ValidateSnapshot(ctx, snapshot.Path, dbbackup.Compatibility{
		LibraryID:               info.LibraryID,
		ConfigSchemaVersion:     appConfig.SchemaVersion,
		MaxApplicationMigration: plan.FromApplicationMigration,
		MaxRiverMigration:       info.RiverMigration,
	})
	if err != nil {
		return fmt.Errorf("validate pre-cutover Online Backup: %w", err)
	}
	if manifest.ApplicationMigration != plan.FromApplicationMigration ||
		verified.ApplicationMigration != plan.FromApplicationMigration {
		return fmt.Errorf("pre-cutover Online Backup captured the wrong application migration")
	}

	repositories, err := database.ReaderQueries.ListActiveRepositories(ctx)
	if err != nil {
		return fmt.Errorf("list Repositories for staging preflight: %w", err)
	}
	files := storage.NewRepositoryFSFactory(nil, database.Queries)
	staging := storage.NewStagingManager(files)
	quarantined := 0
	for _, repository := range repositories {
		moves, err := staging.QuarantineUnresolvedStaging(ctx, repository)
		if err != nil {
			return fmt.Errorf(
				"quarantine uncertain staging for Repository %s: %w",
				repository.RepoID,
				err,
			)
		}
		quarantined += len(moves)
	}
	if logf != nil {
		logf(
			"cutover: verified backup %s and quarantined %d uncertain staging files",
			snapshot.Path,
			quarantined,
		)
	}
	return nil
}
