package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/testutil"

	"github.com/google/uuid"
)

func TestFaceRebuildConvergesThroughBoundedPublishes(t *testing.T) {
	ctx := context.Background()
	database := openFaceClusteringDatabase(t, ctx)
	repositoryID := seedFaceClusteringRepository(t, ctx, database)
	modelID := "fixture-face-v1"
	faceSize := int64(100)
	for index := 0; index < 3; index++ {
		assetID := uuid.New()
		if _, err := testutil.InsertAssetOccurrence(ctx, database.SQL, testutil.AssetOccurrenceParams{
			AssetID: assetID, RepositoryID: repositoryID, OwnerID: 1,
			Filename: fmt.Sprintf("face-%d.jpg", index), MIMEType: "image/jpeg", FileSize: 1,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := database.Queries.CreateFaceResult(ctx, repo.CreateFaceResultParams{
			AssetID: assetID, ModelID: modelID, TotalFaces: 1,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := database.Queries.CreateFaceItem(ctx, repo.CreateFaceItemParams{
			AssetID: assetID, BoundingBox: dbtypes.JSON(`{"x":0,"y":0,"width":10,"height":10}`),
			Confidence: 0.95, FaceSize: &faceSize,
			Embedding: dbtypes.NewVector([]float32{1, 0, 0}), EmbeddingModel: &modelID,
			IsPrimary: true, RepositoryID: repositoryID,
		}); err != nil {
			t.Fatal(err)
		}
	}

	ownerID := int32(1)
	service := NewFaceService(database.Queries, nil, database.SQL, database.Writer)
	result, err := service.RebuildFaceClusters(ctx, uuid.NullUUID{UUID: repositoryID, Valid: true}, &ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if result.CandidateFaces != 3 || result.ClusteredFaces != 3 || result.ClustersCreated != 1 {
		t.Fatalf("face rebuild result = %+v, want three faces in one new cluster", result)
	}
	assignments, err := database.Queries.GetFaceClusterAssignmentsForScope(ctx, repo.GetFaceClusterAssignmentsForScopeParams{
		RepositoryID: uuid.NullUUID{UUID: repositoryID, Valid: true}, OwnerID: &ownerID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(assignments) != 3 {
		t.Fatalf("face assignments = %d, want 3", len(assignments))
	}
	for _, assignment := range assignments[1:] {
		if assignment.ClusterID != assignments[0].ClusterID {
			t.Fatalf("face assignments span clusters: %+v", assignments)
		}
	}
}

func TestFaceRebuildReleasesWriterBeforeDerivedConvergence(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	database := openFaceClusteringDatabase(t, ctx)

	service := NewFaceService(database.Queries, nil, database.SQL, database.Writer).(*faceService)
	rebuildOutsideWriter := make(chan struct{}, 1)
	releaseRebuild := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseRebuild) }) }
	defer release()
	service.afterRebuildReset = func() {
		rebuildOutsideWriter <- struct{}{}
		<-releaseRebuild
	}

	rebuildResult := make(chan error, 1)
	go func() {
		_, rebuildErr := service.RebuildFaceClusters(ctx, uuid.NullUUID{}, nil)
		rebuildResult <- rebuildErr
	}()
	select {
	case <-rebuildOutsideWriter:
	case <-ctx.Done():
		t.Fatalf("face rebuild did not reach derived convergence: %v", ctx.Err())
	}

	writeCtx, writeCancel := context.WithTimeout(ctx, 100*time.Millisecond)
	_, writeErr := database.Queries.SetBootstrapPhase(writeCtx, "ready")
	writeCancel()
	if writeErr != nil {
		t.Fatalf("foreground writer waited behind face rebuild compute: %v", writeErr)
	}

	release()
	if err := <-rebuildResult; err != nil {
		t.Fatalf("face rebuild failed after convergence resumed: %v", err)
	}
}

func openFaceClusteringDatabase(t *testing.T, ctx context.Context) *db.DB {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(dir, "face-rebuild.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(context.Background()) })
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return database
}

func seedFaceClusteringRepository(t *testing.T, ctx context.Context, database *db.DB) uuid.UUID {
	t.Helper()
	rootID := uuid.New()
	repositoryID := uuid.New()
	if _, err := database.SQL.ExecContext(ctx, `
INSERT INTO users (
  user_id, username, password, created_at, updated_at, display_name, role, webauthn_user_handle
) VALUES (1, 'face-owner', 'unused', 1, 1, 'Face Owner', 'admin', x'01');
INSERT INTO repository_roots (
  root_id, name, path, kind, created_at, updated_at
) VALUES (?, 'Face root', '/face', 'default', 1, 1);
INSERT INTO repositories (
  repo_id, name, path, created_at, updated_at, default_owner_id, role, root_id
) VALUES (?, 'Faces', '/face/repository', 1, 1, 1, 'primary', ?);`, rootID, repositoryID, rootID); err != nil {
		t.Fatal(err)
	}
	return repositoryID
}
