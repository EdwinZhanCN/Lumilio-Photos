package controller

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage"
	"server/internal/storage/roe/locations"
	"server/internal/storage/roe/materializer"
	"server/internal/storage/roe/nodegraph"
)

func processHash(ctx context.Context, fixture *controllerFixture, preparer *materializer.HashPreparer, applier *materializer.HashApplier, nodeID uuid.UUID, revision int64) (materializer.Result, error) {
	prepared, err := preparer.PrepareHash(ctx, nodeID, revision)
	if err != nil || prepared == nil {
		return materializer.Result{Code: materializer.ResultStale, NodeID: nodeID, Revision: revision}, err
	}
	var result materializer.Result
	err = fixture.database.WithTx(ctx, catalogtx.OperationRepositoryMaterializeHash, func(tx *sql.Tx, queries *repo.Queries) error {
		applied, err := applier.ApplyHash(ctx, tx, *prepared)
		if err != nil {
			return err
		}
		result = applied
		if result.Code != materializer.ResultBound && result.Code != materializer.ResultNoop {
			return nil
		}
		return service.ApplyAssetActivationTx(ctx, tx, queries, result.RepositoryID, result.NodeID, result.AssetID, result.ContentID)
	})
	return result, err
}

func drainHashEffects(t *testing.T, fixture *controllerFixture) []materializer.Result {
	t.Helper()
	files := storage.NewRepositoryFSFactory(nil, fixture.database.Queries)
	preparer := materializer.NewHashPreparer(fixture.database.ReaderQueries, fixture.database.ReaderSQL, files)
	applier := materializer.NewHashApplier()
	results := make([]materializer.Result, 0)
	for turn := 0; turn < 100; turn++ {
		candidates, err := fixture.database.ReaderQueries.ListRepositoryMaterializationCandidates(fixture.ctx, repo.ListRepositoryMaterializationCandidatesParams{
			RepositoryID: fixture.repository.RepoID, Limit: 8,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(candidates) == 0 {
			return results
		}
		for _, candidate := range candidates {
			result, err := processHash(fixture.ctx, fixture, preparer, applier, candidate.NodeID, candidate.ObservationRevision)
			results = append(results, result)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Fatal("repository materialization candidates did not drain")
	return nil
}

func TestExactCopiesCreateOneOwnerContentAssetAndMultipleLocations(t *testing.T) {
	fixture := newControllerFixture(t, 8)
	contents := []byte("identical original bytes")
	fixture.writeMedia(t, "copies/one.jpg", contents)
	fixture.writeMedia(t, "copies/two.jpg", contents)
	receipt, err := fixture.commands.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	fixture.runToTerminal(t, receipt.OperationID)
	results := drainHashEffects(t, fixture)
	bound := 0
	newAssets := 0
	for _, result := range results {
		if result.Code == materializer.ResultBound {
			bound++
		}
		if result.NewAsset {
			newAssets++
		}
	}
	if bound != 2 || newAssets != 1 {
		t.Fatalf("hash results bound=%d new_assets=%d: %+v", bound, newAssets, results)
	}
	var contentsCount, assetsCount, locationsCount, mediaItems, pipelineStates int
	row := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT
		  (SELECT count(*) FROM content_objects),
		  (SELECT count(*) FROM assets),
		  (SELECT count(*) FROM asset_locations WHERE unbound_observation_revision IS NULL),
		  (SELECT count(*) FROM media_items),
		  (SELECT count(*) FROM asset_pipeline_state)`)
	if err := row.Scan(&contentsCount, &assetsCount, &locationsCount, &mediaItems, &pipelineStates); err != nil {
		t.Fatal(err)
	}
	if contentsCount != 1 || assetsCount != 1 || locationsCount != 2 || mediaItems != 1 || pipelineStates != 3 {
		t.Fatalf("content=%d assets=%d locations=%d media_items=%d pipeline_states=%d",
			contentsCount, assetsCount, locationsCount, mediaItems, pipelineStates)
	}
	var assetID uuid.UUID
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, "SELECT asset_id FROM assets").Scan(&assetID); err != nil {
		t.Fatal(err)
	}
	activeLocations, err := fixture.database.ReaderQueries.ListActiveAssetLocations(fixture.ctx, assetID)
	if err != nil {
		t.Fatal(err)
	}
	firstNode, err := fixture.database.ReaderQueries.GetRepositoryNode(fixture.ctx, repo.GetRepositoryNodeParams{
		RepositoryID: fixture.repository.RepoID, NodeID: activeLocations[0].NodeID,
	})
	if err != nil {
		t.Fatal(err)
	}
	firstPath, err := nodegraph.ProjectPath(fixture.ctx, fixture.database.ReaderQueries, fixture.repository.RepoID, firstNode.NodeID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(fixture.repositoryPath, filepath.FromSlash(firstPath))); err != nil {
		t.Fatal(err)
	}
	resolver := locations.NewResolver(
		fixture.database.ReaderQueries,
		fixture.database.ReaderSQL,
		storage.NewRepositoryFSFactory(nil, fixture.database.Queries),
	)
	opened, err := resolver.OpenAsset(fixture.ctx, assetID)
	if err != nil {
		t.Fatalf("resolve remaining exact copy: %v", err)
	}
	resolvedContents, readErr := io.ReadAll(opened.File)
	closeErr := opened.Close()
	if readErr != nil || closeErr != nil {
		t.Fatal(errors.Join(readErr, closeErr))
	}
	if string(resolvedContents) != string(contents) {
		t.Fatalf("resolved contents = %q, want %q", resolvedContents, contents)
	}
}

func TestHashMutationPublishesNewRevisionWithoutStaleBinding(t *testing.T) {
	fixture := newControllerFixture(t, 8)
	fixture.writeMedia(t, "replace.jpg", []byte("before"))
	receipt, err := fixture.commands.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	fixture.runToTerminal(t, receipt.OperationID)
	if err := os.WriteFile(filepath.Join(fixture.repositoryPath, "replace.jpg"), []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	results := drainHashEffects(t, fixture)
	if len(results) < 2 || results[0].Code != materializer.ResultStale {
		t.Fatalf("mutation results = %+v, want stale then rebound", results)
	}
	var activeLocations, contentObjects int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT
		  (SELECT count(*) FROM asset_locations WHERE unbound_observation_revision IS NULL),
		  (SELECT count(*) FROM content_objects)`).Scan(&activeLocations, &contentObjects); err != nil {
		t.Fatal(err)
	}
	if activeLocations != 1 || contentObjects != 1 {
		t.Fatalf("active locations=%d content objects=%d", activeLocations, contentObjects)
	}
}

func TestSharedContentSeparatesAssetsByResolvedOwner(t *testing.T) {
	fixture := newControllerFixture(t, 8)
	contents := []byte("shared exact content")
	fixture.writeMedia(t, "owner-one.jpg", contents)
	first, err := fixture.commands.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	fixture.runToTerminal(t, first.OperationID)
	drainHashEffects(t, fixture)

	secondOwner, err := fixture.database.Queries.CreateUser(fixture.ctx, repo.CreateUserParams{
		Username: "second-owner", Password: "unused", DisplayName: "Second Owner",
		Role: "user", WebauthnUserHandle: []byte("second-owner-handle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	updatedRepository, err := fixture.database.Queries.UpdateRepository(fixture.ctx, repo.UpdateRepositoryParams{
		RepoID: fixture.repository.RepoID, Name: fixture.repository.Name,
		Config: fixture.repository.Config, DefaultOwnerID: &secondOwner.UserID,
		UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.repository = updatedRepository
	fixture.writeMedia(t, "owner-two.jpg", contents)
	second, err := fixture.commands.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	fixture.runToTerminal(t, second.OperationID)
	drainHashEffects(t, fixture)

	var contentsCount, assetsCount, ownersCount, locationsCount int
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT
		  (SELECT count(*) FROM content_objects),
		  (SELECT count(*) FROM assets),
		  (SELECT count(DISTINCT owner_id) FROM assets),
		  (SELECT count(*) FROM asset_locations WHERE unbound_observation_revision IS NULL)`).
		Scan(&contentsCount, &assetsCount, &ownersCount, &locationsCount); err != nil {
		t.Fatal(err)
	}
	if contentsCount != 1 || assetsCount != 2 || ownersCount != 2 || locationsCount != 2 {
		t.Fatalf("content=%d assets=%d owners=%d locations=%d",
			contentsCount, assetsCount, ownersCount, locationsCount)
	}
}

func TestRescanPreservesExistingNodeOwnerWhenDefaultChanges(t *testing.T) {
	fixture := newControllerFixture(t, 8)
	initialOwnerID := fixture.repository.DefaultOwnerID
	if initialOwnerID == nil {
		t.Fatal("fixture repository has no default owner")
	}
	uploadedOwner, err := fixture.database.Queries.CreateUser(fixture.ctx, repo.CreateUserParams{
		Username: "uploaded-owner", Password: "unused", DisplayName: "Uploaded Owner",
		Role: "user", WebauthnUserHandle: []byte("uploaded-owner-handle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	updatedRepository, err := fixture.database.Queries.UpdateRepository(fixture.ctx, repo.UpdateRepositoryParams{
		RepoID: fixture.repository.RepoID, Name: fixture.repository.Name,
		Config: fixture.repository.Config, DefaultOwnerID: &uploadedOwner.UserID,
		UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.repository = updatedRepository
	fixture.writeMedia(t, "uploaded.jpg", []byte("initial contents"))
	first, err := fixture.commands.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	fixture.runToTerminal(t, first.OperationID)
	drainHashEffects(t, fixture)

	updatedRepository, err = fixture.database.Queries.UpdateRepository(fixture.ctx, repo.UpdateRepositoryParams{
		RepoID: fixture.repository.RepoID, Name: fixture.repository.Name,
		Config: fixture.repository.Config, DefaultOwnerID: initialOwnerID,
		UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.repository = updatedRepository
	fixture.writeMedia(t, "uploaded.jpg", []byte("rescanned contents"))
	second, err := fixture.commands.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	fixture.runToTerminal(t, second.OperationID)
	drainHashEffects(t, fixture)

	var activeOwnerID int32
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT asset.owner_id
		FROM asset_locations location
		JOIN assets asset ON asset.asset_id = location.asset_id
		WHERE location.unbound_observation_revision IS NULL
		  AND asset.original_filename = 'uploaded.jpg'`).Scan(&activeOwnerID); err != nil {
		t.Fatal(err)
	}
	if activeOwnerID != uploadedOwner.UserID {
		t.Fatalf("active asset owner = %d, want original node owner %d", activeOwnerID, uploadedOwner.UserID)
	}
}
