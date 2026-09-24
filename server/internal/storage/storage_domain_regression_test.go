package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/rootcfg"
)

// F09: a Storage Location is an authorization scope, not an I/O health gate.
// Every case leaves the child path, marker, and original bytes intact.
func TestStorageDomainHealthyChildSurvivesParentFailure(t *testing.T) {
	for _, failure := range []string{"missing_marker", "invalid_marker", "replaced_marker", "offline", "error", "maintenance"} {
		t.Run(failure, func(t *testing.T) {
			_, manager := newCatalogRepositoryManager(t)
			ctx := context.Background()
			rootPath := filepath.Join(t.TempDir(), "storage")
			initializeDefaultStorageForTest(t, manager, rootPath)
			repository, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
			if err != nil {
				t.Fatal(err)
			}
			const original = "original media must survive storage registration faults"
			if err := os.WriteFile(filepath.Join(repository.Path, "original.jpg"), []byte(original), 0o644); err != nil {
				t.Fatal(err)
			}
			switch failure {
			case "missing_marker":
				err = os.Remove(filepath.Join(rootPath, rootcfg.FileName))
			case "invalid_marker":
				err = os.WriteFile(filepath.Join(rootPath, rootcfg.FileName), []byte("invalid marker"), 0o644)
			case "replaced_marker":
				err = rootcfg.New("Another Storage Location").Save(rootPath)
			default:
				storageLocation, loadErr := manager.queries.GetStorageLocation(ctx, repository.StorageLocationID)
				if loadErr != nil {
					t.Fatal(loadErr)
				}
				_, err = manager.queries.UpdateStorageLocationFromDisk(ctx, repo.UpdateStorageLocationFromDiskParams{
					StorageLocationID: storageLocation.StorageLocationID, Name: storageLocation.Name, Status: dbtypes.StorageLocationStatus(failure),
					UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
				})
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := manager.queries.GetStorageLocation(ctx, repository.StorageLocationID)
			if err != nil {
				t.Fatal(err)
			}
			files, err := manager.files.OpenContext(ctx, repository)
			if err != nil {
				t.Fatalf("healthy Repository denied by parent %s: %v", failure, err)
			}
			defer files.Close()
			mediaPath, err := ParseUserMediaPath("original.jpg")
			if err != nil {
				t.Fatal(err)
			}
			media, err := files.OpenMedia(mediaPath)
			if err != nil {
				t.Fatal(err)
			}
			defer media.Close()
			data, err := io.ReadAll(media)
			if err != nil || string(data) != original {
				t.Fatalf("read original = %q, %v", data, err)
			}
			after, err := manager.queries.GetStorageLocation(ctx, repository.StorageLocationID)
			if err != nil || after.Status != before.Status || after.UpdatedAt != before.UpdatedAt {
				t.Fatalf("foreground media read changed parent registration: before=%+v after=%+v error=%v", before, after, err)
			}
		})
	}
}

// F09: a process holding the Primary Repository does not own its siblings.
func TestStorageDomainLockConflictIsIsolatedToRepository(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	initializeDefaultStorageForTest(t, manager, filepath.Join(t.TempDir(), "storage"))
	primary, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "unrelated-repository", Actor: "test", Name: "Unrelated",
		DirectoryName: "unrelated", Role: dbtypes.RepoRoleRegular, StorageLocationID: primary.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	holder := startStorageLockProcess(t, primary.Path, "repository")
	claimCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	release, err := manager.AcquireRuntimeStorageOwnership(claimCtx)
	if err != nil {
		t.Fatalf("one Repository lock prevented runtime ownership of healthy siblings: %v", err)
	}
	defer release()
	files, err := manager.files.OpenContext(ctx, *created.Repository)
	if err != nil {
		t.Fatalf("healthy sibling is inaccessible: %v", err)
	}
	if err := files.Close(); err != nil {
		t.Fatal(err)
	}
	blocked, err := manager.queries.GetRepository(ctx, primary.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	if files, err := manager.files.Open(blocked); err == nil {
		_ = files.Close()
		t.Fatal("Repository held by another process admitted I/O")
	}
	if err := holder.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = holder.Wait()
	if err := manager.ReconcileAll(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := manager.queries.GetRepository(ctx, primary.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	files, err = manager.files.Open(recovered)
	if err != nil {
		t.Fatalf("released Repository ownership did not recover: %v", err)
	}
	if err := files.Close(); err != nil {
		t.Fatal(err)
	}
}

// F09: Docker-shaped independence — one Storage Location, two direct-child
// Repositories. Parent marker/status faults and ReconcileStorageLocations must
// not deny healthy siblings; only the affected child path becomes unavailable.
func TestStorageDomainDockerShapedSiblingIndependence(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	rootPath := filepath.Join(t.TempDir(), "docker-storageLocation")
	initializeDefaultStorageForTest(t, manager, rootPath)
	defaultStorageLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	primary, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sibling, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "docker-sibling", Actor: "test", Name: "Sibling",
		DirectoryName: "sibling", Role: dbtypes.RepoRoleRegular,
		StorageLocationID: defaultStorageLocation.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	const primaryOriginal = "primary original bytes"
	const siblingOriginal = "sibling original bytes"
	if err := os.WriteFile(filepath.Join(primary.Path, "primary.jpg"), []byte(primaryOriginal), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sibling.Repository.Path, "sibling.jpg"), []byte(siblingOriginal), 0o644); err != nil {
		t.Fatal(err)
	}

	assertRepositoryOpensOriginal := func(t *testing.T, repository repo.Repository, relativePath, want string) {
		t.Helper()
		files, err := manager.files.OpenContext(ctx, repository)
		if err != nil {
			t.Fatalf("open repository %s: %v", repository.Path, err)
		}
		defer files.Close()
		mediaPath, err := ParseUserMediaPath(relativePath)
		if err != nil {
			t.Fatal(err)
		}
		media, err := files.OpenMedia(mediaPath)
		if err != nil {
			t.Fatal(err)
		}
		defer media.Close()
		data, err := io.ReadAll(media)
		if err != nil || string(data) != want {
			t.Fatalf("read %s = %q, %v", relativePath, data, err)
		}
	}

	for _, failure := range parentStorageLocationFailures {
		t.Run("parent_"+failure, func(t *testing.T) {
			_, isolated := newCatalogRepositoryManager(t)
			isolatedRoot := filepath.Join(t.TempDir(), "docker-storageLocation")
			initializeDefaultStorageForTest(t, isolated, isolatedRoot)
			isolatedPrimary, err := isolated.queries.GetPrimaryRepositoryRecord(ctx)
			if err != nil {
				t.Fatal(err)
			}
			isolatedDefault, err := isolated.queries.GetDefaultStorageLocation(ctx)
			if err != nil {
				t.Fatal(err)
			}
			isolatedSibling, err := isolated.CreateRepository(ctx, CreateRepositorySpec{
				RequestID: "docker-sibling-" + failure, Actor: "test", Name: "Sibling",
				DirectoryName: "sibling", Role: dbtypes.RepoRoleRegular,
				StorageLocationID: isolatedDefault.StorageLocationID.String(),
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(isolatedPrimary.Path, "primary.jpg"), []byte(primaryOriginal), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(isolatedSibling.Repository.Path, "sibling.jpg"), []byte(siblingOriginal), 0o644); err != nil {
				t.Fatal(err)
			}
			failParentStorageLocationForTest(t, isolated, isolatedRoot, isolatedPrimary, failure)
			if err := isolated.ReconcileStorageLocations(ctx); err != nil {
				t.Fatal(err)
			}
			openOriginal := func(repository repo.Repository, relativePath, want string) {
				files, err := isolated.files.OpenContext(ctx, repository)
				if err != nil {
					t.Fatalf("open repository %s: %v", repository.Path, err)
				}
				defer files.Close()
				mediaPath, err := ParseUserMediaPath(relativePath)
				if err != nil {
					t.Fatal(err)
				}
				media, err := files.OpenMedia(mediaPath)
				if err != nil {
					t.Fatal(err)
				}
				defer media.Close()
				data, err := io.ReadAll(media)
				if err != nil || string(data) != want {
					t.Fatalf("read %s = %q, %v", relativePath, data, err)
				}
			}
			openOriginal(isolatedPrimary, "primary.jpg", primaryOriginal)
			openOriginal(*isolatedSibling.Repository, "sibling.jpg", siblingOriginal)
		})
	}

	t.Run("lost_child_path", func(t *testing.T) {
		lostPath := sibling.Repository.Path
		if err := os.Rename(lostPath, lostPath+".lost"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = os.Chmod(lostPath+".lost", 0o755)
			_ = os.Rename(lostPath+".lost", lostPath)
		})
		if err := manager.ReconcileAll(ctx); err != nil {
			t.Fatal(err)
		}
		assertRepositoryOpensOriginal(t, primary, "primary.jpg", primaryOriginal)
		if _, err := manager.files.OpenContext(ctx, *sibling.Repository); err == nil {
			t.Fatal("lost sibling path unexpectedly opened")
		}
	})
}

// F09: desktop-shaped fixture — one Storage Location per volume, Repositories as
// child directories. Losing the volume makes every child path unreadable. Catalog
// reachability follows those paths; Location status is a derived summary of the
// missing volume, not a parent probe that vetoes still-healthy children.
func TestStorageDomainDesktopShapedVolumeLossMarksEveryChildUnavailable(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	rootPath := filepath.Join(t.TempDir(), "desktop-volume")
	initializeDefaultStorageForTest(t, manager, rootPath)
	defaultStorageLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	primary, err := manager.queries.GetPrimaryRepositoryRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sibling, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "desktop-sibling", Actor: "test", Name: "Desktop Sibling",
		DirectoryName: "sibling", Role: dbtypes.RepoRoleRegular,
		StorageLocationID: defaultStorageLocation.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	primaryMedia := filepath.Join(primary.Path, "primary.jpg")
	siblingMedia := filepath.Join(sibling.Repository.Path, "sibling.jpg")
	if err := os.WriteFile(primaryMedia, []byte("desktop primary original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(siblingMedia, []byte("desktop sibling original"), 0o644); err != nil {
		t.Fatal(err)
	}
	primaryChecksum, err := sha256FileHex(primaryMedia)
	if err != nil {
		t.Fatal(err)
	}
	siblingChecksum, err := sha256FileHex(siblingMedia)
	if err != nil {
		t.Fatal(err)
	}

	lostVolume := rootPath + ".unplugged"
	if err := os.Rename(rootPath, lostVolume); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Rename(lostVolume, rootPath)
	})

	if err := manager.ReconcileAll(ctx); err != nil {
		t.Fatal(err)
	}
	if err := manager.ReconcileStorageLocations(ctx); err != nil {
		t.Fatal(err)
	}

	for _, repositoryID := range []string{primary.RepoID.String(), sibling.Repository.RepoID.String()} {
		record, err := manager.GetRepository(repositoryID)
		if err != nil {
			t.Fatalf("load repository %s: %v", repositoryID, err)
		}
		if record.Reachability != dbtypes.RepositoryReachabilityOffline {
			t.Fatalf("repository %s reachability = %q, want offline after volume loss",
				repositoryID, record.Reachability)
		}
		if _, err := manager.files.OpenContext(ctx, *record); err == nil {
			t.Fatalf("repository %s opened after its volume path disappeared", repositoryID)
		}
	}

	storageLocation, err := manager.GetStorageLocation(ctx, defaultStorageLocation.StorageLocationID.String())
	if err != nil {
		t.Fatal(err)
	}
	if storageLocation.Status != dbtypes.StorageLocationStatusOffline {
		t.Fatalf("storage location status = %q, want offline total loss", storageLocation.Status)
	}
	runtimeStatus, err := manager.StorageRuntimeStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if runtimeStatus.State != StorageRuntimeStateActive {
		t.Fatalf("instance runtime = %+v, want active after initialized volume loss", runtimeStatus)
	}

	lostPrimary := filepath.Join(lostVolume, filepath.Base(primary.Path), "primary.jpg")
	lostSibling := filepath.Join(lostVolume, "sibling", "sibling.jpg")
	afterPrimary, err := sha256FileHex(lostPrimary)
	if err != nil || afterPrimary != primaryChecksum {
		t.Fatalf("primary original checksum = %s, %v, want %s", afterPrimary, err, primaryChecksum)
	}
	afterSibling, err := sha256FileHex(lostSibling)
	if err != nil || afterSibling != siblingChecksum {
		t.Fatalf("sibling original checksum = %s, %v, want %s", afterSibling, err, siblingChecksum)
	}
}

// F03: create/open validates live Storage Location identity, not cached status.
func TestCreateOpenIgnoresCachedStorageLocationStatus(t *testing.T) {
	for _, cachedStatus := range []dbtypes.StorageLocationStatus{
		dbtypes.StorageLocationStatusOffline,
		dbtypes.StorageLocationStatusError,
		dbtypes.StorageLocationStatusMaintenance,
	} {
		t.Run(string(cachedStatus), func(t *testing.T) {
			_, manager := newCatalogRepositoryManager(t)
			ctx := context.Background()
			rootPath := filepath.Join(t.TempDir(), "default")
			initializeDefaultStorageForTest(t, manager, rootPath)
			defaultStorageLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
			if err != nil {
				t.Fatal(err)
			}
			_, err = manager.queries.UpdateStorageLocationFromDisk(ctx, repo.UpdateStorageLocationFromDiskParams{
				StorageLocationID: defaultStorageLocation.StorageLocationID, Name: defaultStorageLocation.Name, Status: cachedStatus,
				UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = manager.CreateRepository(ctx, CreateRepositorySpec{
				RequestID: "cached-status-" + string(cachedStatus), Actor: "test", Name: "Child",
				DirectoryName: "child-" + string(cachedStatus), Role: dbtypes.RepoRoleRegular,
				StorageLocationID: defaultStorageLocation.StorageLocationID.String(),
			}); err != nil {
				t.Fatalf("create rejected by cached status %s: %v", cachedStatus, err)
			}
			candidates, err := manager.ListDefaultRepositoryCandidates(ctx)
			if err != nil {
				t.Fatalf("list candidates rejected by cached status %s: %v", cachedStatus, err)
			}
			dirName := "child-" + string(cachedStatus)
			if candidate, ok := candidateByDirectoryName(candidates, dirName); !ok || candidate.Classification != RepositoryCandidateRegistered {
				t.Fatalf("registered candidate for %s = %#v", dirName, candidate)
			}
		})
	}
}

func TestCreateOpenRejectsMissingStorageLocationMarkerNotCachedStatus(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	rootPath := filepath.Join(t.TempDir(), "default")
	initializeDefaultStorageForTest(t, manager, rootPath)
	defaultStorageLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(rootPath, rootcfg.FileName)); err != nil {
		t.Fatal(err)
	}
	_, err = manager.queries.UpdateStorageLocationFromDisk(ctx, repo.UpdateStorageLocationFromDiskParams{
		StorageLocationID: defaultStorageLocation.StorageLocationID, Name: defaultStorageLocation.Name, Status: dbtypes.StorageLocationStatusActive,
		UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "missing-marker", Actor: "test", Name: "Blocked",
		DirectoryName: "blocked", Role: dbtypes.RepoRoleRegular, StorageLocationID: defaultStorageLocation.StorageLocationID.String(),
	})
	if !errors.Is(err, ErrStorageLocationInvalid) {
		t.Fatalf("create error = %v, want ErrStorageLocationInvalid", err)
	}
	if errors.Is(err, ErrStorageLocationOffline) {
		t.Fatalf("create misclassified missing marker as offline: %v", err)
	}
}

// F10: rename does not take a Storage Location read lease that would block behind
// an in-flight Location mutation while siblings remain writable.
func TestRenameRepositoryDoesNotRequireStorageLocationReadLease(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	rootPath := filepath.Join(t.TempDir(), "default")
	initializeDefaultStorageForTest(t, manager, rootPath)
	defaultStorageLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sibling, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "rename-sibling", Actor: "test", Name: "Sibling",
		DirectoryName: "sibling", Role: dbtypes.RepoRoleRegular, StorageLocationID: defaultStorageLocation.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	failParentStorageLocationForTest(t, manager, rootPath, *sibling.Repository, "missing_marker")

	releaseStorageLocation, err := manager.files.AccessCoordinator().AcquireStorageLocationMutationContext(ctx, defaultStorageLocation.StorageLocationID)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseStorageLocation()

	renamed, err := manager.RenameRepository(ctx, sibling.Repository.RepoID.String(), "Renamed Sibling",
		LifecycleRequest{RequestID: "rename-without-storageLocation-read", Actor: "test"})
	if err != nil {
		t.Fatalf("rename blocked behind Storage Location mutation lease: %v", err)
	}
	if renamed.Name != "Renamed Sibling" {
		t.Fatalf("renamed repository = %+v", renamed)
	}
}

func candidateByDirectoryName(candidates []RepositoryCandidate, directoryName string) (RepositoryCandidate, bool) {
	for _, candidate := range candidates {
		if candidate.DirectoryName == directoryName {
			return candidate, true
		}
	}
	return RepositoryCandidate{}, false
}
