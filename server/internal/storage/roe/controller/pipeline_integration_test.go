package controller

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/roe/locations"
	"server/internal/storage/roe/materializer"
	"server/internal/storage/roe/nodegraph"
	"server/internal/storage/roe/outbox"
)

func drainHashEffects(t *testing.T, fixture *controllerFixture) []materializer.Result {
	t.Helper()
	files := storage.NewRepositoryFSFactory(nil, fixture.database.Queries)
	hasher := materializer.NewHashMaterializer(fixture.database, files)
	drainer := outbox.New(fixture.database, outbox.Config{BatchSize: 8})
	results := make([]materializer.Result, 0)
	for turn := 0; turn < 100; turn++ {
		drainResult, err := drainer.DrainKind(fixture.ctx, "hash", func(ctx context.Context, effect repo.RepositoryOutbox) error {
			nodeID, err := uuid.Parse(effect.EntityID)
			if err != nil {
				return err
			}
			result, err := hasher.Process(ctx, nodeID, effect.ExpectedRevision)
			results = append(results, result)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		if drainResult.Claimed == 0 {
			return results
		}
	}
	t.Fatal("hash outbox did not drain")
	return nil
}

func TestRepositoryOutboxReclaimsExpiredLeaseAndRetriesIdempotentDelivery(t *testing.T) {
	fixture := newControllerFixture(t, 8)
	now := time.Now().UTC()
	effect, err := fixture.database.Queries.InsertRepositoryOutboxEffect(fixture.ctx, repo.InsertRepositoryOutboxEffectParams{
		OutboxID: uuid.New(), RepositoryID: fixture.repository.RepoID,
		EffectKey: "test:expired-lease", EffectKind: "controller",
		EntityID: uuid.NewString(), ExpectedRevision: 7, Payload: `{}`,
		CreatedAt: dbtypes.NewTimestamp(now),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.database.SQL.ExecContext(fixture.ctx, `
		UPDATE repository_outbox
		SET status = 'delivering', lease_id = 'crashed-worker',
		    lease_expires_at = ?, attempt_count = 1
		WHERE outbox_id = ?`, now.Add(-time.Second).UnixMicro(), effect.OutboxID); err != nil {
		t.Fatal(err)
	}

	drainer := outbox.New(fixture.database, outbox.Config{
		BatchSize: 1, Lease: time.Second, MaxAttempts: 4,
	})
	deliveryCalls := 0
	appliedEffects := map[string]struct{}{}
	deliver := func(_ context.Context, claimed repo.RepositoryOutbox) error {
		deliveryCalls++
		appliedEffects[claimed.EffectKey] = struct{}{}
		if deliveryCalls == 1 {
			return errors.New("injected crash after idempotent side effect")
		}
		return nil
	}

	first, err := drainer.DrainKind(fixture.ctx, "controller", deliver)
	if err != nil {
		t.Fatal(err)
	}
	if first.Claimed != 1 || first.Retrying != 1 || first.Delivered != 0 {
		t.Fatalf("first drain = %+v", first)
	}
	second, err := drainer.DrainKind(fixture.ctx, "controller", deliver)
	if err != nil {
		t.Fatal(err)
	}
	if second.Claimed != 1 || second.Delivered != 1 || second.Retrying != 0 {
		t.Fatalf("second drain = %+v", second)
	}
	third, err := drainer.DrainKind(fixture.ctx, "controller", deliver)
	if err != nil {
		t.Fatal(err)
	}
	if third.Claimed != 0 || deliveryCalls != 2 || len(appliedEffects) != 1 {
		t.Fatalf("terminal drain=%+v calls=%d applied=%d", third, deliveryCalls, len(appliedEffects))
	}

	var status string
	var attempts int64
	if err := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx,
		"SELECT status, attempt_count FROM repository_outbox WHERE outbox_id = ?", effect.OutboxID,
	).Scan(&status, &attempts); err != nil {
		t.Fatal(err)
	}
	if status != "delivered" || attempts != 3 {
		t.Fatalf("outbox status=%q attempts=%d, want delivered/3", status, attempts)
	}
}

func TestExactCopiesCreateOneOwnerContentAssetAndMultipleLocations(t *testing.T) {
	fixture := newControllerFixture(t, 8)
	contents := []byte("identical original bytes")
	fixture.writeMedia(t, "copies/one.jpg", contents)
	fixture.writeMedia(t, "copies/two.jpg", contents)
	receipt, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
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
	var contentsCount, assetsCount, locationsCount, processingEffects int
	row := fixture.database.ReaderSQL.QueryRowContext(fixture.ctx, `
		SELECT
		  (SELECT count(*) FROM content_objects),
		  (SELECT count(*) FROM assets),
		  (SELECT count(*) FROM asset_locations WHERE unbound_observation_revision IS NULL),
		  (SELECT count(*) FROM repository_outbox WHERE effect_kind = 'process_asset')`)
	if err := row.Scan(&contentsCount, &assetsCount, &locationsCount, &processingEffects); err != nil {
		t.Fatal(err)
	}
	if contentsCount != 1 || assetsCount != 1 || locationsCount != 2 || processingEffects != 1 {
		t.Fatalf("content=%d assets=%d locations=%d processing_effects=%d",
			contentsCount, assetsCount, locationsCount, processingEffects)
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
	resolver := locations.NewResolver(fixture.database, storage.NewRepositoryFSFactory(nil, fixture.database.Queries))
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
	receipt, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
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
	first, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
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
	second, err := fixture.controller.Request(fixture.ctx, fixture.repository.RepoID, "manual", "test", true)
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
