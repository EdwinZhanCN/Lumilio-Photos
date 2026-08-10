package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"
)

func TestListDefaultRepositoryCandidatesClassifiesDirectChildren(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	rootPath := filepath.Join(t.TempDir(), "default")
	initializeDefaultStorageForTest(t, manager, rootPath)

	existingPath := filepath.Join(rootPath, "existing")
	if err := os.Mkdir(existingPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := manager.dirManager.CreateStructure(existingPath); err != nil {
		t.Fatal(err)
	}
	if err := repocfg.NewRepositoryConfig("Existing Archive").SaveConfigToFile(existingPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(rootPath, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(rootPath, "unmarked"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootPath, "unmarked", "file.txt"), []byte("media"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(rootPath, "invalid"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootPath, "invalid", ".lumiliorepo"), []byte("invalid: ["), 0o644); err != nil {
		t.Fatal(err)
	}

	candidates, err := manager.ListDefaultRepositoryCandidates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]RepositoryCandidate, len(candidates))
	for _, candidate := range candidates {
		byName[candidate.DirectoryName] = candidate
	}
	if byName["primary"].Classification != RepositoryCandidateRegistered {
		t.Fatalf("primary candidate = %#v", byName["primary"])
	}
	if candidate := byName["existing"]; candidate.Classification != RepositoryCandidateExisting || !candidate.CanOpen {
		t.Fatalf("existing candidate = %#v", candidate)
	}
	if candidate := byName["empty"]; candidate.Classification != RepositoryCandidateEmptyWritable {
		t.Fatalf("empty candidate = %#v", candidate)
	} else if runtime.GOOS != "linux" && !candidate.CanCreate {
		t.Fatalf("empty candidate should be creatable off Linux: %#v", candidate)
	}
	if byName["unmarked"].Classification != RepositoryCandidateNonemptyUnmarked {
		t.Fatalf("unmarked candidate = %#v", byName["unmarked"])
	}
	if byName["invalid"].Classification != RepositoryCandidateMarkerInvalid {
		t.Fatalf("invalid candidate = %#v", byName["invalid"])
	}
}

func TestResolveDefaultRepositoryCandidateSupportsMovedOriginalAndSeparateCopy(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	rootPath := filepath.Join(t.TempDir(), "default")
	initializeDefaultStorageForTest(t, manager, rootPath)
	root, err := manager.queries.GetDefaultRepositoryRoot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	root, err = manager.queries.UpdateRepositoryRootMountFingerprint(ctx, repo.UpdateRepositoryRootMountFingerprintParams{
		RootID: root.RootID, MountFingerprint: "test-replaced-mount", UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	})
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "candidate-original", Actor: "test", Name: "Archive",
		DirectoryName: "archive", Role: dbtypes.RepoRoleRegular, RootID: root.RootID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	movedPath := filepath.Join(root.Path, "archive-moved")
	if err := os.Rename(created.Repository.Path, movedPath); err != nil {
		t.Fatal(err)
	}
	candidates, err := manager.ListDefaultRepositoryCandidates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var movedCandidate RepositoryCandidate
	for _, candidate := range candidates {
		if candidate.DirectoryName == "archive-moved" {
			movedCandidate = candidate
			break
		}
	}
	if movedCandidate.Classification != RepositoryCandidateIdentityError ||
		!containsString(movedCandidate.Actions, "relocate") || !containsString(movedCandidate.Actions, "copy") {
		t.Fatalf("moved candidate = %#v", movedCandidate)
	}
	if !containsString(movedCandidate.RiskWarnings, "mount_fingerprint_changed") {
		t.Fatalf("moved candidate risks = %v, want mount_fingerprint_changed", movedCandidate.RiskWarnings)
	}
	if _, err := manager.ResolveDefaultRepositoryCandidate(
		ctx, "archive-moved", "update_location", nil,
		LifecycleRequest{RequestID: "candidate-relocate-rejected", Actor: "test"},
	); !errors.Is(err, ErrRepositoryRiskConfirmationRequired) {
		t.Fatalf("unconfirmed relocation error = %v, want ErrRepositoryRiskConfirmationRequired", err)
	}
	relocated, err := manager.ResolveDefaultRepositoryCandidate(
		ctx, "archive-moved", "update_location", nil,
		LifecycleRequest{RequestID: "candidate-relocate", Actor: "web:admin", HostInstanceID: "web-server-1", RiskConfirmation: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if relocated.Path != movedPath || relocated.RepoID != created.Repository.RepoID {
		t.Fatalf("relocated repository = %#v", relocated)
	}
	auditEvents, err := manager.ListLifecycleAudit(ctx, LifecycleAuditFilter{
		TargetType: "repository", TargetID: relocated.RepoID.String(), Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	var relocateAudit *LifecycleAuditEvent
	for index := range auditEvents {
		if auditEvents[index].RequestID == "candidate-relocate" {
			relocateAudit = &auditEvents[index]
			break
		}
	}
	if relocateAudit == nil || relocateAudit.Actor != "web:admin" || relocateAudit.HostInstanceID != "web-server-1" ||
		relocateAudit.ConfirmationType != "update_location" || relocateAudit.Result != AuditResultSucceeded {
		t.Fatalf("candidate relocation audit = %#v", relocateAudit)
	}

	copyPath := filepath.Join(root.Path, "archive-copy")
	if err := os.Mkdir(copyPath, 0o755); err != nil {
		t.Fatal(err)
	}
	config, err := repocfg.LoadConfigFromFile(movedPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := config.SaveConfigToFile(copyPath); err != nil {
		t.Fatal(err)
	}
	if err := manager.dirManager.CreateStructure(copyPath); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ResolveDefaultRepositoryCandidate(
		ctx, "archive-copy", "add_separate", nil,
		LifecycleRequest{RequestID: "candidate-copy-rejected", Actor: "test"},
	); !errors.Is(err, ErrRepositoryRiskConfirmationRequired) {
		t.Fatalf("unconfirmed copy error = %v, want ErrRepositoryRiskConfirmationRequired", err)
	}
	copyRepository, err := manager.ResolveDefaultRepositoryCandidate(
		ctx, "archive-copy", "add_separate", nil,
		LifecycleRequest{RequestID: "candidate-copy", Actor: "test", RiskConfirmation: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if copyRepository.RepoID == relocated.RepoID || copyRepository.Path != copyPath {
		t.Fatalf("separate copy = %#v", copyRepository)
	}
}

func TestOpenDefaultRepositoryCandidateRequiresMountRiskConfirmation(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	rootPath := filepath.Join(t.TempDir(), "default")
	initializeDefaultStorageForTest(t, manager, rootPath)
	root, err := manager.queries.GetDefaultRepositoryRoot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.queries.UpdateRepositoryRootMountFingerprint(ctx, repo.UpdateRepositoryRootMountFingerprintParams{
		RootID: root.RootID, MountFingerprint: "test-replaced-mount", UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(rootPath, "existing")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := manager.dirManager.CreateStructure(path); err != nil {
		t.Fatal(err)
	}
	if err := repocfg.NewRepositoryConfig("Existing").SaveConfigToFile(path); err != nil {
		t.Fatal(err)
	}

	candidates, err := manager.ListDefaultRepositoryCandidates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var existing RepositoryCandidate
	for _, candidate := range candidates {
		if candidate.DirectoryName == "existing" {
			existing = candidate
			break
		}
	}
	if !containsString(existing.RiskWarnings, "mount_fingerprint_changed") {
		t.Fatalf("existing candidate risks = %v, want mount_fingerprint_changed", existing.RiskWarnings)
	}
	if _, err := manager.OpenDefaultRepositoryCandidate(ctx, "existing", nil, LifecycleRequest{
		RequestID: "candidate-open-rejected", Actor: "test",
	}); !errors.Is(err, ErrRepositoryRiskConfirmationRequired) {
		t.Fatalf("unconfirmed open error = %v, want ErrRepositoryRiskConfirmationRequired", err)
	}
	rejectedEvents, err := manager.ListLifecycleAudit(ctx, LifecycleAuditFilter{
		TargetType: "repository", TargetID: "existing", Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rejectedEvents) != 1 || rejectedEvents[0].RequestID != "candidate-open-rejected" ||
		rejectedEvents[0].Result != AuditResultRejected || rejectedEvents[0].FailureStage != "risk_confirmation" {
		t.Fatalf("candidate rejection audit = %#v", rejectedEvents)
	}
	opened, err := manager.OpenDefaultRepositoryCandidate(ctx, "existing", nil, LifecycleRequest{
		RequestID: "candidate-open", Actor: "test", RiskConfirmation: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	canonicalPath, err := CanonicalizeRepositoryPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Path != canonicalPath {
		t.Fatalf("opened path = %q, want %q", opened.Path, canonicalPath)
	}
}

func TestRepositoryCandidateSurfacesPlaceholderRiskBeforeOpen(t *testing.T) {
	_, manager := newCatalogRepositoryManager(t)
	ctx := context.Background()
	rootPath := filepath.Join(t.TempDir(), "default")
	initializeDefaultStorageForTest(t, manager, rootPath)
	path := filepath.Join(rootPath, "placeholder")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := manager.dirManager.CreateStructure(path); err != nil {
		t.Fatal(err)
	}
	if err := repocfg.NewRepositoryConfig("Placeholder").SaveConfigToFile(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, ".photo.jpg.icloud"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	candidates, err := manager.ListDefaultRepositoryCandidates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var placeholder RepositoryCandidate
	for _, candidate := range candidates {
		if candidate.DirectoryName == "placeholder" {
			placeholder = candidate
			break
		}
	}
	if !containsString(placeholder.RiskWarnings, "unavailable_cloud_placeholder") {
		t.Fatalf("placeholder risks = %v", placeholder.RiskWarnings)
	}
	if _, err := manager.OpenDefaultRepositoryCandidate(ctx, "placeholder", nil, LifecycleRequest{
		RequestID: "placeholder-unconfirmed", Actor: "web:admin",
	}); !errors.Is(err, ErrRepositoryRiskConfirmationRequired) {
		t.Fatalf("unconfirmed placeholder error = %v, want risk confirmation", err)
	}
	if _, err := manager.OpenDefaultRepositoryCandidate(ctx, "placeholder", nil, LifecycleRequest{
		RequestID: "placeholder-confirmed", Actor: "web:admin", RiskConfirmation: true,
	}); !errors.Is(err, ErrUnavailableCloudPlaceholder) {
		t.Fatalf("confirmed unavailable placeholder error = %v", err)
	}
}

func TestMountInfoContainsPathDecodesEscapedNames(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux mount-info paths are only meaningful on Linux")
	}
	mountInfo := "36 25 0:32 / /data/archive\\040disk rw,relatime - ext4 /dev/sda rw\n"
	matched, err := mountInfoContainsPath(strings.NewReader(mountInfo), "/data/archive disk")
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Fatal("escaped Linux mount point was not matched")
	}
}
