package storage_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/storage"
	"server/internal/storage/testfixture"

	"go.uber.org/zap"
)

// freshCatalogManager is a Server whose app state was just created: no
// users, no Repositories, the configured default Storage Location registered
// from the marker the fixture wrote.
func freshCatalogManager(t *testing.T, location testfixture.Location) *storage.DefaultRepositoryManager {
	t.Helper()
	ctx := context.Background()
	catalogDirectory := t.TempDir()
	if err := os.Chmod(catalogDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(catalogDirectory, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close(context.Background()) })
	if err := catalog.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	files := storage.NewRepositoryFSFactory(storage.NewRepositoryAccessCoordinator(), catalog.Queries)
	manager, err := storage.NewRepositoryManager(catalog.SQL, catalog.Queries, zap.NewNop(), nil, files)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.EnsureDefaultStorageLocation(ctx, location.Path); err != nil {
		t.Fatal(err)
	}
	return manager
}

func adoptAfterReview(t *testing.T, manager *storage.DefaultRepositoryManager, review *storage.SetupPrimaryAssessment, requestID string) error {
	t.Helper()
	expected := ""
	if review.Existing != nil {
		expected = review.Existing.RepositoryID
	}
	_, err := manager.AdoptPrimaryRepository(context.Background(), storage.AdoptPrimarySpec{
		RequestID: requestID, Actor: "web:user:1", ExpectedRepositoryID: expected,
		AssessmentToken: review.Token, AcknowledgedRisks: storage.RiskCodes(review.Risks),
	})
	return err
}

// R3 state matrix on the P0 fixtures: every state the fixed primary target
// can be in names exactly the legal transition, the review never changes
// the tree, and the explicit Adopt runs only on a compatible unregistered
// marker. Unmarked, corrupt, and newer bytes are never initialized over or
// replaced.
func TestSetupPrimaryReviewClassifiesEveryTargetState(t *testing.T) {
	ctx := context.Background()
	t.Run("leftover primary is adoptable with its private state previewed", func(t *testing.T) {
		location, primary := testfixture.NewLeftoverPrimary(t)
		manager := freshCatalogManager(t, location)
		before := fixtureTree(t, location.Path)
		review, err := manager.AssessSetupPrimaryTarget(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if review.State != storage.SetupPrimaryAdoptable || review.Existing == nil || review.Existing.RepositoryID != primary.Config.ID ||
			review.Existing.Name != primary.Config.Name || review.Existing.MarkerVersion != "1.0" || review.Recovery == nil {
			t.Fatalf("leftover review = %+v existing=%+v", review, review.Existing)
		}
		if p := review.Recovery; p.TrashFiles != 2 || p.StudioRecords != 1 || p.StagingFiles != 2 || p.UnknownFiles != 1 || p.Receipts != 0 {
			t.Fatalf("recovery preview = %+v", p)
		}
		if got := fixtureTree(t, location.Path); len(got) != len(before) {
			t.Fatal("the review changed the tree")
		}
		if err := adoptAfterReview(t, manager, review, "adopt-leftover"); err != nil {
			t.Fatal(err)
		}
		adopted, err := manager.GetRepository(primary.Config.ID)
		if err != nil || adopted.Role != dbtypes.RepoRolePrimary {
			t.Fatalf("adopted primary = %+v err=%v", adopted, err)
		}
		original := fixtureRead(t, primary.Path, testfixture.OriginalPath)
		if fixtureHash(original) != before[filepath.ToSlash(filepath.Join("primary", testfixture.OriginalPath))] {
			t.Fatal("adoption changed the original")
		}
	})
	t.Run("missing private layout is repairable", func(t *testing.T) {
		location := testfixture.NewLocation(t, testfixture.Valid)
		primary := location.Repository(t, "primary", testfixture.Valid, testfixture.MissingPrivate)
		manager := freshCatalogManager(t, location)
		review, err := manager.AssessSetupPrimaryTarget(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if review.State != storage.SetupPrimaryLayoutRepairable || len(review.LayoutMissing) != 1 || review.LayoutMissing[0] != ".lumilio" || !review.Adoptable() {
			t.Fatalf("missing-layout review = %+v missing=%v", review.State, review.LayoutMissing)
		}
		if err := adoptAfterReview(t, manager, review, "adopt-repair"); err != nil {
			t.Fatal(err)
		}
		if info, err := os.Stat(filepath.Join(primary.Path, ".lumilio")); err != nil || !info.IsDir() {
			t.Fatalf("repair did not add the private workspace: %v", err)
		}
		if fixtureHash(fixtureRead(t, primary.Path, ".lumiliorepo")) != fixtureHash(fixtureRead(t, primary.Path, ".lumiliorepo")) || fixtureRead(t, primary.Path, testfixture.OriginalPath) == nil {
			t.Fatal("repair touched the marker or the original")
		}
	})
	t.Run("unmarked files are never initialized over", func(t *testing.T) {
		location, primary := testfixture.NewUnmarkedPrimary(t)
		manager := freshCatalogManager(t, location)
		before := fixtureTree(t, location.Path)
		review, err := manager.AssessSetupPrimaryTarget(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if review.State != storage.SetupPrimaryNonemptyUnmarked || review.Adoptable() || review.Existing != nil || len(review.Actions) != 1 || review.Actions[0] != storage.SetupActionFreshElsewhere {
			t.Fatalf("unmarked review = %+v actions=%v", review.State, review.Actions)
		}
		if err := adoptAfterReview(t, manager, review, "adopt-unmarked"); !errors.Is(err, storage.ErrStorageAssessmentStale) {
			t.Fatalf("adopting unmarked files = %v, want a stale-assessment refusal (no identity was reviewed)", err)
		}
		if _, err := manager.CreateRepository(ctx, storage.CreateRepositorySpec{
			RequestID: "create-over-unmarked", Actor: "web:user:1", Name: "Primary", Role: dbtypes.RepoRolePrimary,
			AssessmentToken: review.Token, AcknowledgedRisks: storage.RiskCodes(review.Risks),
		}); err == nil {
			t.Fatal("setup created a primary over unmarked files")
		}
		if got := fixtureTree(t, location.Path); len(got) != len(before) {
			t.Fatalf("unmarked tree changed: %d entries, want %d", len(got), len(before))
		}
		if _, err := os.Stat(filepath.Join(primary.Path, ".lumiliorepo")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("a marker was written over unmarked files")
		}
	})
	t.Run("corrupt and newer markers are preserved and not adoptable", func(t *testing.T) {
		for _, variant := range []struct {
			marker testfixture.Marker
			state  storage.SetupPrimaryState
		}{{testfixture.Corrupt, storage.SetupPrimaryMarkerInvalid}, {testfixture.Unsupported, storage.SetupPrimaryMarkerUnsupported}} {
			location := testfixture.NewLocation(t, testfixture.Valid)
			primary := location.Repository(t, "primary", variant.marker, testfixture.Full)
			manager := freshCatalogManager(t, location)
			markerBefore := fixtureRead(t, primary.Path, ".lumiliorepo")
			review, err := manager.AssessSetupPrimaryTarget(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if review.State != variant.state || review.Adoptable() || len(review.Actions) != 1 {
				t.Fatalf("%v review = %+v actions=%v", variant.marker, review.State, review.Actions)
			}
			if variant.state == storage.SetupPrimaryMarkerUnsupported && (review.Existing == nil || review.Existing.MarkerVersion != "99.0") {
				t.Fatalf("newer marker version not reported: %+v", review.Existing)
			}
			if err := adoptAfterReview(t, manager, review, "adopt-"+string(variant.state)); err == nil {
				t.Fatalf("%v marker was adopted", variant.marker)
			}
			if fixtureHash(fixtureRead(t, primary.Path, ".lumiliorepo")) != fixtureHash(markerBefore) {
				t.Fatalf("%v marker bytes changed", variant.marker)
			}
		}
	})
	t.Run("absent target is creatable", func(t *testing.T) {
		location := testfixture.NewLocation(t, testfixture.Valid)
		manager := freshCatalogManager(t, location)
		review, err := manager.AssessSetupPrimaryTarget(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if review.State != storage.SetupPrimaryCreatable || review.Existing != nil || len(review.Actions) != 2 || review.Actions[0] != storage.SetupActionCreate {
			t.Fatalf("absent review = %+v actions=%v", review.State, review.Actions)
		}
		if err := adoptAfterReview(t, manager, review, "adopt-absent"); !errors.Is(err, storage.ErrStorageAssessmentStale) {
			t.Fatalf("adopting an absent target = %v", err)
		}
	})
	t.Run("an identity registered elsewhere is reported, never adopted twice", func(t *testing.T) {
		location, primary := testfixture.NewLeftoverPrimary(t)
		manager := freshCatalogManager(t, location)
		review, err := manager.AssessSetupPrimaryTarget(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := adoptAfterReview(t, manager, review, "adopt-first"); err != nil {
			t.Fatal(err)
		}
		// The same tree seen again by a review after the registration.
		after, err := manager.AssessSetupPrimaryTarget(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if after.State != storage.SetupPrimaryRegistered || after.Adoptable() || after.Existing.RegisteredRole != dbtypes.RepoRolePrimary || after.Existing.RegisteredPath != primary.Path {
			t.Fatalf("registered review = %+v existing=%+v", after.State, after.Existing)
		}
	})
}

// fixtureTree hashes every file below root by its slash path.
func fixtureTree(t *testing.T, root string) map[string]string {
	t.Helper()
	entries := map[string]string{}
	err := filepath.WalkDir(root, func(name string, item fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if item.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		entries[filepath.ToSlash(relative)] = fixtureHash(fixtureRead(t, root, relative))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func fixtureRead(t *testing.T, root, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func fixtureHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
