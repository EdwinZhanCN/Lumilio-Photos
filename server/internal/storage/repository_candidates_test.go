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
	storageLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	storageLocation, err = manager.queries.UpdateStorageLocationMountFingerprint(ctx, repo.UpdateStorageLocationMountFingerprintParams{
		StorageLocationID: storageLocation.StorageLocationID, MountFingerprint: "test-replaced-mount", UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	})
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, CreateRepositorySpec{
		RequestID: "candidate-original", Actor: "test", Name: "Archive",
		DirectoryName: "archive", Role: dbtypes.RepoRoleRegular, StorageLocationID: storageLocation.StorageLocationID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	movedPath := filepath.Join(storageLocation.Path, "archive-moved")
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
	// Candidates suggest folders; the risks of one come from the shared
	// review of that exact folder, and the acknowledgement is bound to it.
	review, err := manager.AssessCreateTarget(ctx, AssessCreateTargetRequest{Intent: IntentOpenRepository, DirectoryName: "archive-moved"})
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(RiskCodes(review.Risks), string(RiskMountChanged)) || review.Token == "" {
		t.Fatalf("moved candidate review = %+v, want mount_fingerprint_changed with a token", review)
	}
	if _, err := manager.ResolveDefaultRepositoryCandidate(
		ctx, "archive-moved", "update_location", nil,
		LifecycleRequest{RequestID: "candidate-relocate-rejected", Actor: "test"},
	); !errors.Is(err, ErrRepositoryRiskConfirmationRequired) {
		t.Fatalf("unreviewed relocation error = %v, want ErrRepositoryRiskConfirmationRequired", err)
	}
	relocated, err := manager.ResolveDefaultRepositoryCandidate(
		ctx, "archive-moved", "update_location", nil,
		LifecycleRequest{
			RequestID: "candidate-relocate", Actor: "web:admin", HostInstanceID: "web-server-1",
			AssessmentToken: review.Token, AcknowledgedRisks: RiskCodes(review.Risks),
		},
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

	copyPath := filepath.Join(storageLocation.Path, "archive-copy")
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
	// A review of another folder never admits this one: the token is bound
	// to the exact target.
	if _, err := manager.ResolveDefaultRepositoryCandidate(
		ctx, "archive-copy", "add_separate", nil,
		LifecycleRequest{RequestID: "candidate-copy-stale", Actor: "test", AssessmentToken: review.Token, AcknowledgedRisks: RiskCodes(review.Risks)},
	); !errors.Is(err, ErrStorageAssessmentStale) {
		t.Fatalf("copy with the moved folder's token = %v, want ErrStorageAssessmentStale", err)
	}
	copyReview, err := manager.AssessCreateTarget(ctx, AssessCreateTargetRequest{Intent: IntentOpenRepository, DirectoryName: "archive-copy"})
	if err != nil {
		t.Fatal(err)
	}
	copyRepository, err := manager.ResolveDefaultRepositoryCandidate(
		ctx, "archive-copy", "add_separate", nil,
		LifecycleRequest{RequestID: "candidate-copy", Actor: "test", AssessmentToken: copyReview.Token, AcknowledgedRisks: RiskCodes(copyReview.Risks)},
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
	storageLocation, err := manager.queries.GetDefaultStorageLocation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.queries.UpdateStorageLocationMountFingerprint(ctx, repo.UpdateStorageLocationMountFingerprintParams{
		StorageLocationID: storageLocation.StorageLocationID, MountFingerprint: "test-replaced-mount", UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
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
	if existing.Classification != RepositoryCandidateExisting || !existing.CanOpen {
		t.Fatalf("existing candidate = %#v", existing)
	}
	review, err := manager.AssessCreateTarget(ctx, AssessCreateTargetRequest{Intent: IntentOpenRepository, DirectoryName: "existing"})
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(RiskCodes(review.Risks), string(RiskMountChanged)) {
		t.Fatalf("existing candidate review risks = %v, want mount_fingerprint_changed", RiskCodes(review.Risks))
	}
	if _, err := manager.OpenDefaultRepositoryCandidate(ctx, "existing", nil, LifecycleRequest{
		RequestID: "candidate-open-rejected", Actor: "test",
	}); !errors.Is(err, ErrRepositoryRiskConfirmationRequired) {
		t.Fatalf("unreviewed open error = %v, want ErrRepositoryRiskConfirmationRequired", err)
	}
	// Acknowledging by code without the token, or with a token of other
	// facts, is refused the same way the create command refuses it.
	if _, err := manager.OpenDefaultRepositoryCandidate(ctx, "existing", nil, LifecycleRequest{
		RequestID: "candidate-open-unbound", Actor: "test", AcknowledgedRisks: RiskCodes(review.Risks),
	}); !errors.Is(err, ErrRepositoryRiskConfirmationRequired) {
		t.Fatalf("unbound acknowledgement error = %v, want ErrRepositoryRiskConfirmationRequired", err)
	}
	if _, err := manager.OpenDefaultRepositoryCandidate(ctx, "existing", nil, LifecycleRequest{
		RequestID: "candidate-open-stale", Actor: "test", AssessmentToken: "v1.not-this-target", AcknowledgedRisks: RiskCodes(review.Risks),
	}); !errors.Is(err, ErrStorageAssessmentStale) {
		t.Fatalf("stale token error = %v, want ErrStorageAssessmentStale", err)
	}
	rejectedEvents, err := manager.ListLifecycleAudit(ctx, LifecycleAuditFilter{TargetType: "repository", TargetID: "existing", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	stages := map[string]string{}
	for _, event := range rejectedEvents {
		if event.Result == AuditResultRejected {
			stages[event.RequestID] = event.FailureStage
		}
	}
	if stages["candidate-open-rejected"] != "risk_confirmation" || stages["candidate-open-unbound"] != "risk_confirmation" || stages["candidate-open-stale"] != "assessment_stale" {
		t.Fatalf("candidate rejection audits = %v", stages)
	}
	opened, err := manager.OpenDefaultRepositoryCandidate(ctx, "existing", nil, LifecycleRequest{
		RequestID: "candidate-open", Actor: "test", AssessmentToken: review.Token, AcknowledgedRisks: RiskCodes(review.Risks),
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

// A folder holding cloud placeholders is not a risk to acknowledge. The
// review samples the folder itself (deep traversal is the command's
// preflight), so it reports no refusal, and the open command's
// materialization preflight refuses the tree whatever was acknowledged.
func TestRepositoryCandidateWithPlaceholdersIsRefusedBeforeAnyConsent(t *testing.T) {
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
	review, err := manager.AssessCreateTarget(ctx, AssessCreateTargetRequest{Intent: IntentOpenRepository, DirectoryName: "placeholder"})
	if err != nil {
		t.Fatal(err)
	}
	if review.Problem() != "" || len(review.Risks) != 0 {
		t.Fatalf("placeholder review = problem %q risks %v, want a reviewable folder", review.Problem(), RiskCodes(review.Risks))
	}
	for _, request := range []LifecycleRequest{
		{RequestID: "placeholder-unreviewed", Actor: "web:admin"},
		{RequestID: "placeholder-acknowledged", Actor: "web:admin", AssessmentToken: review.Token, AcknowledgedRisks: RiskCodes(review.Risks)},
	} {
		if _, err := manager.OpenDefaultRepositoryCandidate(ctx, "placeholder", nil, request); !errors.Is(err, ErrUnavailableCloudPlaceholder) {
			t.Fatalf("%s: placeholder open error = %v, want ErrUnavailableCloudPlaceholder", request.RequestID, err)
		}
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
