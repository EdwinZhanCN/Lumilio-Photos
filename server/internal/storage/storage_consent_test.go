package storage_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/storage"
	"server/internal/storage/testfixture"

	"go.uber.org/zap"
)

// consentHarness is the R1 Server evidence of the storage plan: a real catalog
// manager with a default and an external Storage Location, whose target facts
// are injected through the P1 observer so removable and network storage are
// reproduced without hardware.
type consentHarness struct {
	manager  *storage.DefaultRepositoryManager
	observer *testfixture.Observer
	external storageLocationRef
}

type storageLocationRef struct {
	id   string
	path string
}

func newConsentHarness(t *testing.T) *consentHarness {
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
	observer := &testfixture.Observer{}
	manager.SetObservationBackend(observer)

	base := t.TempDir()
	defaultPath := filepath.Join(base, "default")
	defaultLocation, err := manager.EnsureDefaultStorageLocation(ctx, defaultPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.CreateRepository(ctx, storage.CreateRepositorySpec{
		RequestID: "consent-primary", Actor: "server:test", Name: "Primary", Role: dbtypes.RepoRolePrimary,
		StorageLocationID: defaultLocation.StorageLocationID.String(),
	}); err != nil {
		t.Fatal(err)
	}
	externalPath := filepath.Join(base, "external")
	if err := os.Mkdir(externalPath, 0o755); err != nil {
		t.Fatal(err)
	}
	externalLocation, err := manager.AddStorageLocation(ctx, externalPath, "External disk")
	if err != nil {
		t.Fatal(err)
	}
	return &consentHarness{
		manager: manager, observer: observer,
		external: storageLocationRef{id: externalLocation.StorageLocationID.String(), path: externalPath},
	}
}

// markRemovable injects the facts of an external disk for every path below
// the external Storage Location; everything else keeps the real filesystem.
func (h *consentHarness) markRemovable(t *testing.T) {
	t.Helper()
	external := canonicalPath(t, h.external.path)
	h.observer.MountInfoFunc = func(p string) (storage.MountFacts, error) {
		facts, err := storage.OSStorageObserver{}.MountInfo(p)
		if p == external || filepath.Dir(p) == external {
			facts.Removable = true
			facts.MountID, facts.MountSource, facts.Device = "external-1", "/dev/external1", "external1"
			facts.MountPath = external
		}
		return facts, err
	}
}

func (h *consentHarness) assess(t *testing.T, directory string) *storage.CreateTargetAssessment {
	t.Helper()
	assessment, err := h.manager.AssessCreateTarget(context.Background(), storage.AssessCreateTargetRequest{
		Intent: storage.IntentCreateRepository, StorageLocationID: h.external.id, DirectoryName: directory,
	})
	if err != nil {
		t.Fatal(err)
	}
	return assessment
}

func (h *consentHarness) spec(requestID, directory string) storage.CreateRepositorySpec {
	return storage.CreateRepositorySpec{
		RequestID: requestID, Actor: "web:user:1", Name: "Archive " + directory, DirectoryName: directory,
		Role: dbtypes.RepoRoleRegular, StorageLocationID: h.external.id,
	}
}

func (h *consentHarness) auditStages(t *testing.T, requestID string) []string {
	t.Helper()
	events, err := h.manager.ListLifecycleAudit(context.Background(), storage.LifecycleAuditFilter{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	var stages []string
	for _, event := range events {
		if event.RequestID == requestID && event.Result == storage.AuditResultRejected {
			stages = append(stages, event.FailureStage)
		}
	}
	return stages
}

func TestAssessCreateTargetReviewsTheActualTargetWithInjectedFacts(t *testing.T) {
	h := newConsentHarness(t)
	h.markRemovable(t)

	assessment := h.assess(t, "archive")
	if !reflect.DeepEqual(assessment.Risks, []storage.StorageRisk{storage.RiskRemovable, storage.RiskNonDefault}) {
		t.Fatalf("risks = %+v, want removable and non-default", assessment.Risks)
	}
	if !assessment.Assessment.Capabilities.ReviewCreate || assessment.Problem() != "" || assessment.Token == "" {
		t.Fatalf("assessment = %+v", assessment)
	}
	if assessment.StorageLocationID.String() != h.external.id || assessment.StorageLocationKind != dbtypes.StorageLocationKindExternal || assessment.DirectoryName != "archive" {
		t.Fatalf("target identity = %+v", assessment)
	}
	if again := h.assess(t, "archive"); again.Token != assessment.Token {
		t.Fatal("re-assessing unchanged facts minted a different token")
	}

	// Capacity fluctuation alone keeps the review valid.
	h.observer.StatFSFunc = func(p string) (storage.VolumeFacts, error) {
		facts, err := storage.OSStorageObserver{}.StatFS(p)
		facts.AvailableBytes, facts.TotalBytes = 1, 2
		return facts, err
	}
	if changed := h.assess(t, "archive"); changed.Token != assessment.Token {
		t.Fatal("a capacity-only change invalidated the review")
	}
	h.observer.StatFSFunc = nil

	// A different target, or a changed mount, is a different review.
	if other := h.assess(t, "other"); other.Token == assessment.Token {
		t.Fatal("two targets shared one consent token")
	}
	previous := h.observer.MountInfoFunc
	h.observer.MountInfoFunc = func(p string) (storage.MountFacts, error) {
		facts, err := previous(p)
		if facts.Removable {
			facts.Device = "replacement-disk"
		}
		return facts, err
	}
	if replaced := h.assess(t, "archive"); replaced.Token == assessment.Token {
		t.Fatal("a replaced disk kept the old consent token")
	}

	// The default Location is reviewed without the non-default risk.
	defaults, err := h.manager.AssessCreateTarget(context.Background(), storage.AssessCreateTargetRequest{
		Intent: storage.IntentCreateRepository, DirectoryName: "local",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, risk := range defaults.Risks {
		if risk == storage.RiskNonDefault {
			t.Fatalf("default Location reported the non-default risk: %+v", defaults.Risks)
		}
	}
	if defaults.StorageLocationKind != dbtypes.StorageLocationKindDefault {
		t.Fatalf("default review kind = %q", defaults.StorageLocationKind)
	}
}

func TestCreateRepositoryBindsAcknowledgementToTheAssessedTarget(t *testing.T) {
	h := newConsentHarness(t)
	h.markRemovable(t)
	ctx := context.Background()
	target := filepath.Join(h.external.path, "archive")
	assertNothingCreated := func() {
		t.Helper()
		if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("a refused create mutated the disk: %v", err)
		}
	}

	// No review: the command refuses with the risks and the token to retry with.
	var consent *storage.StorageRiskConfirmationError
	if _, err := h.manager.CreateRepository(ctx, h.spec("create-unreviewed", "archive")); !errors.As(err, &consent) {
		t.Fatalf("unreviewed create error = %v, want StorageRiskConfirmationError", err)
	}
	assertNothingCreated()
	review := h.assess(t, "archive")
	if consent.Token != review.Token || !reflect.DeepEqual(consent.Risks, review.Risks) {
		t.Fatalf("command refusal %+v disagrees with the review %+v", consent, review)
	}

	// A token for another target never carries over.
	otherReview := h.assess(t, "other")
	stale := h.spec("create-stale", "archive")
	stale.AssessmentToken, stale.AcknowledgedRisks = otherReview.Token, storage.RiskCodes(review.Risks)
	if _, err := h.manager.CreateRepository(ctx, stale); !errors.Is(err, storage.ErrStorageAssessmentStale) {
		t.Fatalf("foreign token error = %v, want ErrStorageAssessmentStale", err)
	}
	assertNothingCreated()

	// The right token with an incomplete acknowledgement is not consent.
	partial := h.spec("create-partial", "archive")
	partial.AssessmentToken, partial.AcknowledgedRisks = review.Token, []string{string(storage.RiskRemovable)}
	if _, err := h.manager.CreateRepository(ctx, partial); !errors.As(err, &consent) || consent.Token != review.Token {
		t.Fatalf("partial acknowledgement error = %v, want a refusal with the same token", err)
	}
	assertNothingCreated()

	// Facts changed after the review: the acknowledgement is invalidated.
	previous := h.observer.MountInfoFunc
	h.observer.MountInfoFunc = func(p string) (storage.MountFacts, error) {
		facts, err := previous(p)
		if facts.Removable {
			facts.Device = "replacement-disk"
		}
		return facts, err
	}
	changed := h.spec("create-changed", "archive")
	changed.AssessmentToken, changed.AcknowledgedRisks = review.Token, storage.RiskCodes(review.Risks)
	if _, err := h.manager.CreateRepository(ctx, changed); !errors.Is(err, storage.ErrStorageAssessmentStale) {
		t.Fatalf("changed-facts error = %v, want ErrStorageAssessmentStale", err)
	}
	assertNothingCreated()
	h.observer.MountInfoFunc = previous

	// Capacity drift alone does not invalidate the acknowledgement.
	h.observer.StatFSFunc = func(p string) (storage.VolumeFacts, error) {
		facts, err := storage.OSStorageObserver{}.StatFS(p)
		facts.AvailableBytes = 42
		return facts, err
	}
	accepted := h.spec("create-accepted", "archive")
	accepted.AssessmentToken, accepted.AcknowledgedRisks = review.Token, storage.RiskCodes(review.Risks)
	result, err := h.manager.CreateRepository(ctx, accepted)
	if err != nil {
		t.Fatalf("acknowledged create failed: %v", err)
	}
	if !reflect.DeepEqual(result.AcknowledgedRisks, review.Risks) {
		t.Fatalf("result risks = %+v, want %+v", result.AcknowledgedRisks, review.Risks)
	}
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		t.Fatalf("acknowledged create did not register the target: %v", err)
	}

	for requestID, want := range map[string]string{
		"create-unreviewed": "risk_confirmation", "create-stale": "assessment_stale",
		"create-partial": "risk_confirmation", "create-changed": "assessment_stale",
	} {
		if stages := h.auditStages(t, requestID); len(stages) != 1 || stages[0] != want {
			t.Fatalf("%s rejected audit stages = %v, want [%s]", requestID, stages, want)
		}
	}
	events, err := h.manager.ListLifecycleAudit(ctx, storage.LifecycleAuditFilter{TargetType: "repository", TargetID: result.Repository.RepoID.String(), Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Result != storage.AuditResultSucceeded || events[0].ConfirmationType != "storage_risk" {
		t.Fatalf("success audit = %+v, want one storage_risk confirmation", events)
	}
}

func TestCreateRepositoryRefusesUnsupportedStorageBeforeAnyConsent(t *testing.T) {
	h := newConsentHarness(t)
	ctx := context.Background()
	external := canonicalPath(t, h.external.path)
	h.observer.StatFSFunc = func(p string) (storage.VolumeFacts, error) {
		facts, err := storage.OSStorageObserver{}.StatFS(p)
		if p == external || filepath.Dir(p) == external {
			facts.Filesystem = "cifs"
		}
		return facts, err
	}

	review := h.assess(t, "share")
	if review.Assessment.Classification.Support != storage.StorageUnsupported || review.Problem() != storage.ProblemUnsupportedFilesystem || review.Assessment.Capabilities.ReviewCreate {
		t.Fatalf("network target review = %+v", review.Assessment.Classification)
	}
	spec := h.spec("create-network", "share")
	spec.AssessmentToken, spec.AcknowledgedRisks = review.Token, []string{"network_filesystem"}
	if _, err := h.manager.CreateRepository(ctx, spec); !errors.Is(err, storage.ErrStorageUnsupportedFilesystem) {
		t.Fatalf("network create error = %v, want ErrStorageUnsupportedFilesystem: a confirmation never enables network storage", err)
	}
	if _, err := os.Stat(filepath.Join(h.external.path, "share")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refused network create mutated the disk: %v", err)
	}
	if stages := h.auditStages(t, "create-network"); len(stages) != 1 || stages[0] != "storage_support" {
		t.Fatalf("network refusal audit stages = %v", stages)
	}

	// Permission denial on the target is unknown support, never consent-able.
	h.observer.StatFSFunc = nil
	h.observer.StatFunc = func(p string) (os.FileInfo, error) {
		if filepath.Dir(p) == h.external.path || filepath.Dir(p) == external {
			return nil, os.ErrPermission
		}
		return os.Stat(p)
	}
	denied := h.spec("create-denied", "locked")
	if _, err := h.manager.CreateRepository(ctx, denied); !errors.Is(err, storage.ErrStoragePermissionDenied) {
		t.Fatalf("denied target error = %v, want ErrStoragePermissionDenied", err)
	}
}

func TestSetupPrimaryIntentReviewsTheDefaultPrimaryTarget(t *testing.T) {
	h := newConsentHarness(t)
	assessment, err := h.manager.AssessCreateTarget(context.Background(), storage.AssessCreateTargetRequest{Intent: storage.IntentSetupPrimary})
	if err != nil {
		t.Fatal(err)
	}
	if assessment.DirectoryName != "primary" || assessment.StorageLocationKind != dbtypes.StorageLocationKindDefault || assessment.Intent != storage.IntentSetupPrimary {
		t.Fatalf("setup review = %+v", assessment)
	}
	// The primary already exists in this harness, so the review reports it.
	if assessment.ExistingRepositoryID() == "" || assessment.Assessment.Capabilities.ReviewCreate {
		t.Fatalf("existing primary not reported: %+v", assessment)
	}
	if _, err := h.manager.AssessCreateTarget(context.Background(), storage.AssessCreateTargetRequest{Intent: storage.IntentSetupPrimary, StorageLocationID: h.external.id}); !errors.Is(err, storage.ErrPathNotAllowed) {
		t.Fatalf("setup_primary on an external Location = %v, want ErrPathNotAllowed", err)
	}
	if _, err := h.manager.AssessCreateTarget(context.Background(), storage.AssessCreateTargetRequest{Intent: storage.IntentCreateRepository, DirectoryName: "bad/name"}); !errors.Is(err, storage.ErrInvalidRepositoryDirectory) {
		t.Fatalf("invalid folder review = %v, want ErrInvalidRepositoryDirectory", err)
	}
}
