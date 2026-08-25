package processors

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/dbtypes/status"
	"server/internal/db/repo"
	"server/internal/settings"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riversqlite"
)

func TestSelectiveRetryStatusAndJobsAreAtomicAndIdempotent(t *testing.T) {
	ctx := context.Background()
	catalogDir := t.TempDir()
	if err := os.Chmod(catalogDir, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(catalogDir, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close(context.Background()) })
	if err := catalog.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	owner, err := catalog.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "retry-owner", Password: "unused", DisplayName: "Retry Owner", Role: "admin",
		WebauthnUserHandle: []byte("retry-owner"),
	})
	if err != nil {
		t.Fatal(err)
	}

	repositoryID := uuid.New()
	repositoryPath := t.TempDir()
	repositoryConfig := repocfg.NewRepositoryConfig("retry")
	repositoryConfig.ID = repositoryID.String()
	if err := repositoryConfig.SaveConfigToFile(repositoryPath); err != nil {
		t.Fatal(err)
	}
	rootID := uuid.New()
	rootConfig := rootcfg.New("retry root")
	rootConfig.ID = rootID.String()
	if err := rootConfig.Save(filepath.Dir(repositoryPath)); err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	if _, err := catalog.Queries.UpsertRepositoryRoot(ctx, repo.UpsertRepositoryRootParams{
		RootID: rootID, Name: "retry root", Path: filepath.Dir(repositoryPath), Kind: dbtypes.RepositoryRootKindExternal,
		Status: dbtypes.RepositoryRootStatusActive, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Queries.CreateRepository(ctx, repo.CreateRepositoryParams{
		RepoID: repositoryID, Name: "retry", Path: repositoryPath, Config: *repositoryConfig, Role: dbtypes.RepoRoleRegular,
		Reachability: dbtypes.RepositoryReachabilityActive, Activity: dbtypes.RepositoryActivityIdle,
		CreatedAt: now, UpdatedAt: now, RootID: rootID,
	}); err != nil {
		t.Fatal(err)
	}
	assetStatus := status.AssetStatus{State: status.StateFailed, Tasks: map[string]status.TaskStatus{
		"metadata_asset":  {State: status.TaskFailed},
		"thumbnail_asset": {State: status.TaskFailed},
	}}
	encodedStatus, err := assetStatus.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	assetID := uuid.New()
	content, err := catalog.Queries.InsertContentObject(ctx, repo.InsertContentObjectParams{
		ContentID: uuid.New(), HashAlgorithm: "blake3-v1", FullHash: strings.Repeat("c", 64),
		FileSize: 1, CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	rating := int64(0)
	asset, err := catalog.Queries.CreateAsset(ctx, repo.CreateAssetParams{
		AssetID: assetID, OwnerID: &owner.UserID, ContentID: content.ContentID,
		Type: string(dbtypes.AssetTypePhoto), OriginalFilename: "retry.jpg", MimeType: "image/jpeg",
		TakenTime: now, SpecificMetadata: dbtypes.SpecificMetadata([]byte("{}")), Rating: &rating, Status: encodedStatus,
	})
	if err != nil {
		t.Fatal(err)
	}
	queueClient, err := river.NewClient(riversqlite.New(catalog.SQL), &river.Config{})
	if err != nil {
		t.Fatal(err)
	}
	processor := &AssetProcessor{database: catalog.SQL, writer: catalog.Writer, queries: catalog.Queries, queueClient: queueClient}
	tasks := []string{"metadata_asset", "thumbnail_asset"}
	effectID := uuid.New()

	run := func() error {
		tx, err := catalog.SQL.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		queries := catalog.Queries.WithTx(tx)
		if err := queries.MutateAssetStatus(ctx, assetID, func(current status.AssetStatus) (status.AssetStatus, error) {
			current.EnsureTasks(tasks)
			for _, task := range tasks {
				current.MarkTaskPending(task, "retry")
			}
			return current, nil
		}); err != nil {
			return err
		}
		if err := processor.enqueueRetryTasks(ctx, tx, &asset, dbtypes.AssetTypePhoto, tasks, settings.ML{}, effectID); err != nil {
			return err
		}
		return tx.Commit()
	}

	forced := errors.New("forced second insert failure")
	processor.beforeRetryInsert = func(queue string) error {
		if queue == "thumbnail_asset" {
			return forced
		}
		return nil
	}
	if err := run(); !errors.Is(err, forced) {
		t.Fatalf("retry failure = %v, want forced insertion failure", err)
	}
	assertProcessorCount(t, catalog.SQL, "SELECT count(*) FROM river_job", 0)
	rolledBack, err := catalog.Queries.GetAssetByID(ctx, assetID)
	if err != nil {
		t.Fatal(err)
	}
	rolledBackStatus, err := status.FromJSON(rolledBack.Status)
	if err != nil {
		t.Fatal(err)
	}
	if rolledBackStatus.Tasks["metadata_asset"].State != status.TaskFailed {
		t.Fatalf("status mutation survived rolled-back insert: %+v", rolledBackStatus.Tasks)
	}

	processor.beforeRetryInsert = nil
	if err := run(); err != nil {
		t.Fatal(err)
	}
	if err := run(); err != nil {
		t.Fatal(err)
	}
	assertProcessorCount(t, catalog.SQL, "SELECT count(*) FROM river_job WHERE queue IN ('metadata_asset','thumbnail_asset')", 2)
}

func assertProcessorCount(t *testing.T, database *sql.DB, query string, want int) {
	t.Helper()
	var got int
	if err := database.QueryRow(query).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("count for %q = %d, want %d", query, got, want)
	}
}
