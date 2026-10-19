package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"server/internal/api/dto"
	"server/internal/storage/testfixture"
)

// seedLeftoverLocation copies the P0 leftover-primary tree (a Default
// Storage Location with its root marker and a primary/ Repository whose
// catalog was lost) into the storage path a fresh install is configured with.
func seedLeftoverLocation(tb testing.TB, storageDir string) testfixture.Repository {
	tb.Helper()
	location, primary := testfixture.NewLeftoverPrimary(tb)
	if err := os.CopyFS(storageDir, os.DirFS(location.Path)); err != nil {
		tb.Fatal(err)
	}
	primary.Path = filepath.Join(storageDir, "primary")
	return primary
}

func treeHashes(tb testing.TB, root string) map[string]string {
	tb.Helper()
	hashes := map[string]string{}
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, name)
		sum := sha256.Sum256(data)
		hashes[filepath.ToSlash(relative)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		tb.Fatal(err)
	}
	return hashes
}

func (s *blackboxServer) primaryCount() int {
	s.t.Helper()
	var view struct {
		Repositories []struct {
			Role      string `json:"role"`
			IsPrimary bool   `json:"is_primary"`
		} `json:"repositories"`
	}
	s.mustJSON(http.MethodGet, "/api/v1/storage/view", nil, &view)
	count := 0
	for _, row := range view.Repositories {
		if row.Role == "primary" || row.IsPrimary {
			count++
		}
	}
	return count
}

// Storage P5 / R3, through the public API: a fresh app state whose
// configured Default Storage Location still holds the earlier install's
// Primary Repository. After the administrator is created and before
// readiness, the setup review offers Adopt and fresh-elsewhere, the public
// status carries no host fact, create keeps refusing, and the explicit Adopt
// keeps the identity and the originals, assigns the owner, creates exactly
// one primary, queues the scan, holds the inherited Trash, reattaches the
// Studio record, and reaches ready. The same request replays its result.
func TestLeftoverPrimaryIsAdoptedDuringSetup(t *testing.T) {
	var leftover testfixture.Repository
	server := startBlackboxServerWith(t, blackboxOptions{
		skipPrimary: true,
		seedStorage: func(tb testing.TB, storageDir string) { leftover = seedLeftoverLocation(tb, storageDir) },
	})
	before := treeHashes(t, server.storageDir)

	// Public status: phase only. No path, no token, no identity.
	token := server.token
	server.token = ""
	status, publicBody := server.do(http.MethodGet, "/api/v1/setup/status", nil, "")
	if status != http.StatusOK {
		t.Fatalf("public status = %d %s", status, publicBody)
	}
	for _, secret := range []string{server.storageDir, leftover.Config.ID, "token", "repository_defaults", "primary_target"} {
		if strings.Contains(string(publicBody), secret) {
			t.Fatalf("public setup status leaks %q: %s", secret, publicBody)
		}
	}
	var public dto.SetupStatusDTO
	if err := json.Unmarshal(publicBody, &public); err != nil || !public.AdminInitialized || public.PrimaryRepositoryInitialized || public.Initialized {
		t.Fatalf("public status = %+v err=%v", public, err)
	}
	if status, _ := server.do(http.MethodGet, "/api/v1/setup/primary-target", nil, ""); status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated primary target = %d, want 401", status)
	}
	if status, _ := server.do(http.MethodPost, "/api/v1/setup/primary-repository/adopt", strings.NewReader(`{}`), "application/json"); status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated adopt = %d, want 401", status)
	}
	server.token = token

	// The administrator's review names the one legal transition.
	var target dto.SetupPrimaryTargetDTO
	server.mustJSON(http.MethodGet, "/api/v1/setup/primary-target", nil, &target)
	if target.SetupComplete || target.NativeHost || target.DefaultStorage.State != "ready" || !target.DefaultStorage.Registered || target.Target == nil {
		t.Fatalf("primary target = %+v", target)
	}
	review := target.Target
	if review.State != "adoptable" || strings.Join(review.Actions, ",") != "adopt,fresh_elsewhere" || review.Existing == nil ||
		review.Existing.RepositoryID != leftover.Config.ID || review.Existing.Name != leftover.Config.Name || review.Assessment.Token == "" {
		t.Fatalf("review = %+v existing=%+v", review, review.Existing)
	}
	if r := review.Recovery; r == nil || r.TrashFiles != 2 || r.StudioRecords != 1 || r.StagingFiles != 2 || r.UnknownFiles != 1 {
		t.Fatalf("recovery preview = %+v", review.Recovery)
	}
	if encoded, _ := json.Marshal(target); strings.Contains(string(encoded), filepath.Join(server.storageDir, "primary")) {
		t.Fatalf("review exposes the Repository path: %s", encoded)
	}
	if after := treeHashes(t, server.storageDir); len(after) != len(before) {
		t.Fatal("the review changed the tree")
	}

	// Create still refuses the occupied target; adoption needs the exact review.
	if status, body := server.do(http.MethodPost, "/api/v1/setup/primary-repository",
		strings.NewReader(`{"name":"Fresh","assessment_token":"`+review.Assessment.Token+`"}`), "application/json"); status != http.StatusConflict || !strings.Contains(string(body), "existing_repository_found") {
		t.Fatalf("create over the leftover = %d %s", status, body)
	}
	if status, _ := server.do(http.MethodPost, "/api/v1/setup/primary-repository/adopt",
		strings.NewReader(`{"expected_repository_id":"`+leftover.Config.ID+`"}`), "application/json"); status != http.StatusBadRequest {
		t.Fatalf("adopt without a token = %d, want 400", status)
	}
	if status, body := server.do(http.MethodPost, "/api/v1/setup/primary-repository/adopt",
		strings.NewReader(`{"expected_repository_id":"`+uuid.NewString()+`","assessment_token":"`+review.Assessment.Token+`"}`), "application/json"); status != http.StatusConflict || !strings.Contains(string(body), "storage/assessment-stale") {
		t.Fatalf("adopt of another identity = %d %s", status, body)
	}
	if status, body := server.do(http.MethodPost, "/api/v1/setup/primary-repository/adopt",
		strings.NewReader(`{"expected_repository_id":"`+leftover.Config.ID+`","assessment_token":"v1.forged"}`), "application/json"); status != http.StatusConflict || !strings.Contains(string(body), "storage/assessment-stale") {
		t.Fatalf("adopt with a foreign token = %d %s", status, body)
	}
	server.mustJSON(http.MethodGet, "/api/v1/setup/status", nil, &public)
	if public.PrimaryRepositoryInitialized {
		t.Fatal("a refused adoption registered a primary")
	}

	// The explicit Adopt, bound to the review.
	server.extraHeaders = map[string]string{"Idempotency-Key": "adopt-" + uuid.NewString()}
	acknowledged, _ := json.Marshal(review.Assessment.Risks)
	var adopted dto.AdoptPrimaryRepositoryResponseDTO
	server.mustJSON(http.MethodPost, "/api/v1/setup/primary-repository/adopt", map[string]any{
		"expected_repository_id": leftover.Config.ID, "assessment_token": review.Assessment.Token,
		"acknowledged_risks": json.RawMessage(acknowledged),
	}, &adopted)
	if adopted.Repository.ID != leftover.Config.ID || !adopted.Repository.IsPrimary || adopted.Repository.Role != "primary" ||
		adopted.RecoveryReceiptID == "" || adopted.Recovery == nil || adopted.Recovery.TrashFiles != 2 || adopted.Replayed {
		t.Fatalf("adopted = %+v recovery=%+v", adopted, adopted.Recovery)
	}
	if adopted.Repository.DefaultOwnerID == nil {
		t.Fatal("adoption assigned no owner")
	}
	var replayed dto.AdoptPrimaryRepositoryResponseDTO
	server.mustJSON(http.MethodPost, "/api/v1/setup/primary-repository/adopt", map[string]any{
		"expected_repository_id": leftover.Config.ID, "assessment_token": review.Assessment.Token,
		"acknowledged_risks": json.RawMessage(acknowledged),
	}, &replayed)
	if !replayed.Replayed || replayed.Repository.ID != adopted.Repository.ID || replayed.RecoveryReceiptID != adopted.RecoveryReceiptID {
		t.Fatalf("replay = %+v", replayed)
	}
	server.extraHeaders = nil
	if status, _ := server.do(http.MethodPost, "/api/v1/setup/primary-repository/adopt",
		strings.NewReader(`{"expected_repository_id":"`+leftover.Config.ID+`","assessment_token":"`+review.Assessment.Token+`"}`), "application/json"); status != http.StatusConflict {
		t.Fatalf("adopt after setup = %d, want 409", status)
	}
	if server.primaryCount() != 1 {
		t.Fatalf("primary count = %d, want exactly one", server.primaryCount())
	}

	// Setup is complete and durable; the marker and the originals are untouched.
	server.mustJSON(http.MethodGet, "/api/v1/setup/status", nil, &public)
	if !public.Initialized || !public.PrimaryRepositoryInitialized || public.RuntimeState != "active" {
		t.Fatalf("status after adoption = %+v", public)
	}
	after := treeHashes(t, server.storageDir)
	for _, name := range []string{".lumilioroot", "primary/.lumiliorepo", "primary/" + testfixture.OriginalPath, "primary/" + testfixture.TrashPath} {
		if after[name] != before[name] {
			t.Fatalf("adoption rewrote %s", name)
		}
	}
	var completed dto.SetupPrimaryTargetDTO
	server.mustJSON(http.MethodGet, "/api/v1/setup/primary-target", nil, &completed)
	if !completed.SetupComplete || completed.Target != nil {
		t.Fatalf("primary target after adoption = %+v", completed)
	}

	// The queued scan proves the original, the Studio record reattaches, and
	// the inherited Trash is held: the P4 recovery journey, from adoption.
	repository := blackboxRepository{ID: adopted.Repository.ID, Name: adopted.Repository.Name, Path: adopted.Repository.Path}
	recoveryPath := "/api/v1/storage/repositories/" + url.PathEscape(repository.ID) + "/recovery"
	var inventory dto.RepositoryRecoveryDTO
	var original []blackboxAsset
	if !waitFor(2*time.Minute, func() bool {
		server.mustJSON(http.MethodGet, recoveryPath, nil, &inventory)
		original = server.listByFilename("original.png")
		for _, item := range inventory.Items {
			if item.Class == "studio" && item.Path == testfixture.StudioPath && item.Disposition == "reassociated" {
				return inventory.Trash.HeldCount == 1 && len(original) == 1
			}
		}
		return false
	}) {
		t.Fatalf("recovery inventory never settled after adoption: %+v (original: %+v)", inventory, original)
	}
	if len(inventory.Receipts) != 1 || inventory.Receipts[0].Event != "adopt" || inventory.Receipts[0].ID != adopted.RecoveryReceiptID {
		t.Fatalf("receipts = %+v", inventory.Receipts)
	}
	var sidecar dto.AssetSidecarResponseDTO
	server.mustJSON(http.MethodGet, "/api/v1/assets/"+original[0].AssetID+"/sidecar", nil, &sidecar)
	if !sidecar.Exists || sidecar.Sidecar.Adjustments.Exposure != 0.5 {
		t.Fatalf("reassociated sidecar = exists %t, exposure %v", sidecar.Exists, sidecar.Sidecar.Adjustments.Exposure)
	}
	trashed := server.listTrashedAssets(repository.ID)
	if len(trashed) != 1 || trashed[0].OriginalFilename != "deleted.png" {
		t.Fatalf("Trash lists %+v, want deleted.png", trashed)
	}
	var holds dto.AssetTrashHoldsDTO
	server.mustJSON(http.MethodGet, "/api/v1/assets/trash-holds?repository_id="+url.QueryEscape(repository.ID), nil, &holds)
	if holds.Count != 1 {
		t.Fatalf("trash holds = %+v", holds)
	}
}

// R3 variant: files without a marker at the fixed target are never
// initialized over. The review says so, create and adopt both refuse, the
// bytes stay, and the only way out is a fresh destination.
func TestUnmarkedPrimaryIsNeverInitializedOverDuringSetup(t *testing.T) {
	server := startBlackboxServerWith(t, blackboxOptions{
		skipPrimary: true,
		seedStorage: func(tb testing.TB, storageDir string) {
			location, _ := testfixture.NewUnmarkedPrimary(tb)
			if err := os.CopyFS(storageDir, os.DirFS(location.Path)); err != nil {
				tb.Fatal(err)
			}
		},
	})
	before := treeHashes(t, server.storageDir)
	var target dto.SetupPrimaryTargetDTO
	server.mustJSON(http.MethodGet, "/api/v1/setup/primary-target", nil, &target)
	if target.Target == nil || target.Target.State != "nonempty_unmarked" || strings.Join(target.Target.Actions, ",") != "fresh_elsewhere" || target.Target.Existing != nil {
		t.Fatalf("unmarked review = %+v", target.Target)
	}
	if status, body := server.do(http.MethodPost, "/api/v1/setup/primary-repository",
		strings.NewReader(`{"name":"Fresh","assessment_token":"`+target.Target.Assessment.Token+`"}`), "application/json"); status != http.StatusConflict {
		t.Fatalf("create over unmarked files = %d %s", status, body)
	}
	if status, body := server.do(http.MethodPost, "/api/v1/setup/primary-repository/adopt",
		strings.NewReader(`{"expected_repository_id":"`+uuid.NewString()+`","assessment_token":"`+target.Target.Assessment.Token+`"}`), "application/json"); status != http.StatusConflict {
		t.Fatalf("adopt of unmarked files = %d %s", status, body)
	}
	after := treeHashes(t, server.storageDir)
	if len(after) != len(before) {
		t.Fatalf("setup changed the unmarked tree: %v", after)
	}
	for name, sum := range before {
		if after[name] != sum {
			t.Fatalf("setup changed %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(server.storageDir, "primary", ".lumiliorepo")); !os.IsNotExist(err) {
		t.Fatal("a marker was written over unmarked files")
	}
	var status dto.SetupStatusDTO
	server.mustJSON(http.MethodGet, "/api/v1/setup/status", nil, &status)
	if status.PrimaryRepositoryInitialized {
		t.Fatal("setup completed over unmarked files")
	}
}

// R3 fresh-elsewhere: before the Primary Repository exists, restarting with a
// different storage.path registers a fresh Default Storage Location, leaves
// the previous tree untouched, and setup creates the primary there. An
// established default that goes offline later keeps the instance ready and
// is never recreated.
func TestFreshDefaultElsewhereBeforeThePrimaryExists(t *testing.T) {
	first := startBlackboxServerWith(t, blackboxOptions{
		skipPrimary: true,
		seedStorage: func(tb testing.TB, storageDir string) { seedLeftoverLocation(tb, storageDir) },
	})
	oldDir := first.storageDir
	oldBefore := treeHashes(t, oldDir)
	first.stop()

	freshDir := filepath.Join(first.root, "storage-fresh")
	second := startBlackboxServerWith(t, blackboxOptions{root: first.root, storageDir: freshDir, login: true, skipPrimary: true})
	var target dto.SetupPrimaryTargetDTO
	second.mustJSON(http.MethodGet, "/api/v1/setup/primary-target", nil, &target)
	if target.SetupComplete || target.DefaultStorage.State != "ready" || target.DefaultStorage.Path != freshDir || target.Target == nil || target.Target.State != "creatable" {
		t.Fatalf("fresh default review = %+v target=%+v", target.DefaultStorage, target.Target)
	}
	var created struct {
		Repository blackboxRepository `json:"repository"`
	}
	second.mustJSON(http.MethodPost, "/api/v1/setup/primary-repository", map[string]any{
		"name": "Fresh Primary", "storage_strategy": "flat", "assessment_token": target.Target.Assessment.Token,
		"acknowledged_risks": target.Target.Assessment.Risks,
	}, &created)
	if created.Repository.ID == "" || !strings.HasPrefix(created.Repository.Path, freshDir) {
		t.Fatalf("fresh primary = %+v", created.Repository)
	}
	if second.primaryCount() != 1 {
		t.Fatalf("primary count = %d", second.primaryCount())
	}
	oldAfter := treeHashes(t, oldDir)
	if len(oldAfter) != len(oldBefore) {
		t.Fatalf("fresh default touched the previous tree: %v", oldAfter)
	}
	for name, sum := range oldBefore {
		if oldAfter[name] != sum {
			t.Fatalf("fresh default changed %s in the previous tree", name)
		}
	}
	var status dto.SetupStatusDTO
	second.mustJSON(http.MethodGet, "/api/v1/setup/status", nil, &status)
	if !status.Initialized || status.RuntimeState != "active" {
		t.Fatalf("status after fresh setup = %+v", status)
	}
	second.stop()

	// The established default goes offline: the instance stays ready and
	// degraded, the folder is not recreated, and setup is not reopened.
	offlineDir := filepath.Join(first.root, "storage-fresh-unplugged")
	if err := os.Rename(freshDir, offlineDir); err != nil {
		t.Fatal(err)
	}
	third := startBlackboxServerWith(t, blackboxOptions{root: first.root, storageDir: freshDir, login: true, skipPrimary: true})
	third.mustJSON(http.MethodGet, "/api/v1/setup/status", nil, &status)
	if !status.Initialized || !status.PrimaryRepositoryInitialized || status.RuntimeState != "degraded" {
		t.Fatalf("status with the default offline = %+v", status)
	}
	if _, err := os.Stat(freshDir); !os.IsNotExist(err) {
		t.Fatalf("startup recreated the offline default: %v", err)
	}
	var offline dto.SetupPrimaryTargetDTO
	third.mustJSON(http.MethodGet, "/api/v1/setup/primary-target", nil, &offline)
	if !offline.SetupComplete || offline.DefaultStorage.State != "offline" || offline.Target != nil {
		t.Fatalf("primary target with the default offline = %+v", offline)
	}
	if status, _ := third.do(http.MethodPost, "/api/v1/setup/primary-repository", strings.NewReader(`{"name":"Again"}`), "application/json"); status != http.StatusConflict {
		t.Fatalf("create after completion = %d, want 409", status)
	}
}
