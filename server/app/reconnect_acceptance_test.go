package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"server/internal/api/dto"
	"server/internal/storage"
)

// R2 over the wired runtime: a Repository or a whole external Storage
// Location moved while Lumilio was not looking is reconnected through the
// native task the Web starts and the Desktop host approves. Every outcome
// after the picker is visible on the task: success, a reasoned refusal, or
// a review; a silent no-op is the regression.

// reconnectHarness is a blackbox Server plus the in-process native control
// plane the Desktop host uses to approve a task with a chosen folder.
type reconnectHarness struct {
	*blackboxServer
	control RepositoryControl
}

func startReconnectHarness(t *testing.T) *reconnectHarness {
	t.Helper()
	harness := &reconnectHarness{}
	harness.blackboxServer = startBlackboxServerWith(t, blackboxOptions{repositoryControl: harness.capture})
	if harness.control == nil {
		t.Fatal("the native control plane was not handed to the harness")
	}
	return harness
}

func (h *reconnectHarness) capture(control RepositoryControl) {
	if control != nil {
		h.control = control
	}
}

// move is the external move of a tree the Server registered. On Linux and
// macOS it happens while the Server runs and holds the tree's runtime lock,
// the case the ownership rekey exists for. Windows refuses to rename a
// directory while any handle inside it is open, so there the move happens
// with the Server stopped, the way an operator moves a folder Explorer
// reports as in use, and the Server restarts on the same catalog.
func (h *reconnectHarness) move(from, to string) {
	h.t.Helper()
	if runtime.GOOS != "windows" {
		if err := os.Rename(from, to); err != nil {
			h.t.Fatal(err)
		}
		return
	}
	root, storageDir := h.root, h.storageDir
	h.stop()
	if err := os.Rename(from, to); err != nil {
		h.t.Fatal(err)
	}
	h.control = nil
	h.blackboxServer = startBlackboxServerWith(h.t, blackboxOptions{
		root: root, storageDir: storageDir, login: true, skipPrimary: true, repositoryControl: h.capture,
	})
	if h.control == nil {
		h.t.Fatal("the native control plane was not handed to the restarted harness")
	}
}

// externalDisk grants an external Storage Location the way the Desktop host
// does after a native folder selection.
func (h *reconnectHarness) externalDisk(name string) StorageLocationInfo {
	h.t.Helper()
	path := filepath.Join(h.root, name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		h.t.Fatal(err)
	}
	info, _, err := h.control.AddStorageLocation(context.Background(), "acceptance-"+name, path, name)
	if err != nil {
		h.t.Fatalf("add external Storage Location %s: %v", name, err)
	}
	return info
}

// createIn creates an acknowledged regular Repository under a Storage
// Location through the reviewed create journey.
func (h *reconnectHarness) createIn(location StorageLocationInfo, name, directory string) blackboxRepository {
	h.t.Helper()
	var review dto.StorageAssessmentDTO
	h.mustJSON(http.MethodPost, "/api/v1/storage/assessments", map[string]any{
		"intent": "create_repository", "storage_location_id": location.ID, "directory_name": directory,
	}, &review)
	var created struct {
		Repository blackboxRepository `json:"repository"`
	}
	h.mustJSON(http.MethodPost, "/api/v1/storage/repositories", map[string]any{
		"name": name, "directory_name": directory, "storage_location_id": location.ID, "storage_strategy": "flat",
		"assessment_token": review.Token, "acknowledged_risks": review.Risks,
	}, &created)
	if created.Repository.ID == "" || !strings.HasPrefix(created.Repository.Path, canonicalDir(h.t, location.Path)) {
		h.t.Fatalf("created repository = %+v under %s", created.Repository, location.Path)
	}
	h.waitIdle(created.Repository)
	return created.Repository
}

func canonicalDir(t testing.TB, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// nativeTask starts a task from the Web side and returns it.
func (h *reconnectHarness) nativeTask(body map[string]any) dto.HostActionDTO {
	h.t.Helper()
	var task dto.HostActionDTO
	h.mustJSON(http.MethodPost, "/api/v1/storage/native-tasks", body, &task)
	if task.ID == "" || task.Status != "pending" {
		h.t.Fatalf("created native task = %+v", task)
	}
	return task
}

// approve is the Desktop half: the host picks a folder for the pending task
// and executes it in process, then the Web half answers a risk review with
// the exact codes it was shown. It returns the task as the Web reads it.
func (h *reconnectHarness) approve(task dto.HostActionDTO, selectedPath string) dto.HostActionDTO {
	h.t.Helper()
	ctx := context.Background()
	pending, err := h.control.ListPendingHostActions(ctx)
	if err != nil {
		h.t.Fatal(err)
	}
	nonce := ""
	for _, candidate := range pending {
		if candidate.ID == task.ID {
			nonce = candidate.Nonce
		}
	}
	if nonce == "" {
		h.t.Fatalf("task %s is not pending on the host: %+v", task.ID, pending)
	}
	// A refusal surfaces as the task's recorded outcome; the error is the
	// host's copy of it and is read from the task instead.
	_, _ = h.control.ExecuteHostAction(ctx, task.ID, nonce, "desktop-acceptance", selectedPath, nil)
	current := h.task(task.ID)
	if current.Status == "needs_decision" && current.Result != nil && current.Result.Conflict != nil && current.Result.Conflict.Type == "storage_risk" {
		// A refused command answers the resolve with its Problem and records
		// the same outcome on the task, which is what the Web shows.
		body, err := json.Marshal(map[string]any{
			"resolution": "confirm_risk", "acknowledged_risks": current.Result.Conflict.RiskWarnings,
		})
		if err != nil {
			h.t.Fatal(err)
		}
		_, _ = h.do(http.MethodPost, "/api/v1/storage/native-tasks/"+task.ID+"/resolve", bytes.NewReader(body), "application/json")
		current = h.task(task.ID)
	}
	return current
}

func (h *reconnectHarness) task(id string) dto.HostActionDTO {
	h.t.Helper()
	var task dto.HostActionDTO
	h.mustJSON(http.MethodGet, "/api/v1/storage/native-tasks/"+id, nil, &task)
	return task
}

func (h *reconnectHarness) view() dto.StorageViewResponseDTO {
	h.t.Helper()
	var view dto.StorageViewResponseDTO
	h.mustJSON(http.MethodGet, "/api/v1/storage/view", nil, &view)
	return view
}

func (h *reconnectHarness) viewRepository(id string) dto.StorageRepositoryViewDTO {
	h.t.Helper()
	for _, row := range h.view().Repositories {
		if row.ID == id {
			return row
		}
	}
	h.t.Fatalf("repository %s is not in the storage view", id)
	return dto.StorageRepositoryViewDTO{}
}

func (h *reconnectHarness) viewLocation(id string) dto.StorageLocationViewDTO {
	h.t.Helper()
	for _, row := range h.view().StorageLocations {
		if row.ID == id {
			return row
		}
	}
	h.t.Fatalf("storage location %s is not in the storage view", id)
	return dto.StorageLocationViewDTO{}
}

// repositoryPath reads the registered path from the catalog through the
// in-process manager; the HTTP read model deliberately carries no paths.
func (h *reconnectHarness) repositoryPath(id string) string {
	h.t.Helper()
	repository, err := h.manager().GetRepository(id)
	if err != nil {
		h.t.Fatal(err)
	}
	return repository.Path
}

func (h *reconnectHarness) manager() *storage.DefaultRepositoryManager {
	h.t.Helper()
	control, ok := h.control.(*repositoryControl)
	if !ok {
		h.t.Fatalf("native control is %T", h.control)
	}
	manager, ok := control.manager.(*storage.DefaultRepositoryManager)
	if !ok {
		h.t.Fatalf("repository manager is %T", control.manager)
	}
	return manager
}

// originalHashes hashes every original file below root: the private
// `.lumilio/` tree and the OS lock files are coordination state, not media.
func originalHashes(tb testing.TB, root string) map[string]string {
	tb.Helper()
	hashes := map[string]string{}
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".lumilio" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".lock") {
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

// copyTree copies a Repository tree the way a user duplicates a folder,
// leaving out the OS lock files the running Server holds (Windows refuses
// to read a locked region, and a copy carries no lock anyway).
func copyTree(tb testing.TB, from, to string) {
	tb.Helper()
	err := filepath.WalkDir(from, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(from, name)
		if err != nil {
			return err
		}
		target := filepath.Join(to, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if strings.HasSuffix(entry.Name(), ".lock") {
			return nil
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		tb.Fatal(err)
	}
}

func sameHashes(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for name, sum := range a {
		if b[name] != sum {
			return false
		}
	}
	return true
}

func problemType(task dto.HostActionDTO) string {
	if task.Problem == nil {
		return ""
	}
	return task.Problem.Type
}

// acceptanceDenyingObserver reads the host filesystem except below one path,
// which fails with permission denied: a mounted disk the Server may not read.
type acceptanceDenyingObserver struct {
	storage.OSStorageObserver
	denied string
}

func (o *acceptanceDenyingObserver) blocks(p string) bool {
	clean := filepath.Clean(p)
	return clean == o.denied || strings.HasPrefix(clean, o.denied+string(filepath.Separator))
}

func (o *acceptanceDenyingObserver) ReadFile(p string) ([]byte, error) {
	if o.blocks(p) {
		return nil, &fs.PathError{Op: "open", Path: p, Err: fs.ErrPermission}
	}
	return o.OSStorageObserver.ReadFile(p)
}

func (o *acceptanceDenyingObserver) Stat(p string) (fs.FileInfo, error) {
	if o.blocks(p) {
		return nil, &fs.PathError{Op: "stat", Path: p, Err: fs.ErrPermission}
	}
	return o.OSStorageObserver.Stat(p)
}

func (h *reconnectHarness) setObserver(observer storage.StorageObserver) {
	h.t.Helper()
	h.manager().SetObservationBackend(observer)
}

// A regular Repository whose disk was moved is reconnected through picker →
// review → command: the identity, the Assets, the album membership and the
// bytes survive, the catalog paths change atomically, and the scan is
// re-queued. An online same-identity copy, a tree with another identity and
// an unreadable original each refuse with their specific reason on the task.
func TestMovedRegularRepositoryReconnectsThroughTheNativeTask(t *testing.T) {
	h := startReconnectHarness(t)
	diskA := h.externalDisk("disk-a")
	diskB := h.externalDisk("disk-b")
	travel := h.createIn(diskA, "Travel", "travel")
	h.upload(travel, "beach.jpg", blackboxJPEG(t, 120, "beach"))
	h.waitProcessingSettled()
	assets := h.listByFilename("beach.jpg")
	if len(assets) != 1 {
		t.Fatalf("uploaded asset = %+v", assets)
	}
	asset := assets[0]
	album := h.createAlbum("Trips")
	h.addToAlbum(album, asset.AssetID)
	before := originalHashes(t, travel.Path)
	if len(before) == 0 {
		t.Fatal("no originals were written")
	}

	// The disk moves while Lumilio is not looking.
	movedPath := filepath.Join(diskB.Path, "travel")
	h.move(travel.Path, movedPath)
	task := h.approve(h.nativeTask(map[string]any{"kind": "locate_repository", "repository_id": travel.ID}), movedPath)
	if task.Status != "succeeded" || task.Result == nil || task.Result.RepositoryID != travel.ID {
		t.Fatalf("reconnect task = %+v problem=%+v refusal=%+v", task, task.Problem, task.Refusal)
	}
	row := h.viewRepository(travel.ID)
	if row.StorageLocationID != diskB.ID || row.Reachability != "active" {
		t.Fatalf("reconnected row = %+v", row)
	}
	canonicalMoved := canonicalDir(t, movedPath)
	if got := h.repositoryPath(travel.ID); got != canonicalMoved {
		t.Fatalf("repository path after reconnect = %q, want %q", got, canonicalMoved)
	}
	for _, action := range row.Actions {
		if action == "reconnect" {
			t.Fatalf("a reconnected Repository still offers reconnect: %v", row.Actions)
		}
	}
	// The scan the reconnect fenced finishes against the new path; the Asset,
	// its album and its bytes are the same.
	h.waitIdle(blackboxRepository{ID: travel.ID, Name: travel.Name, Path: canonicalMoved})
	h.waitProcessingSettled()
	if status := h.assetStatus(asset.AssetID); status != http.StatusOK {
		t.Fatalf("asset after reconnect = %d", status)
	}
	if listed := h.listByFilename("beach.jpg"); len(listed) != 1 || listed[0].AssetID != asset.AssetID {
		t.Fatalf("asset identity after reconnect = %+v, want %s", listed, asset.AssetID)
	}
	inAlbum := false
	for _, member := range h.albumAssets(album) {
		inAlbum = inAlbum || member.AssetID == asset.AssetID
	}
	if !inAlbum {
		t.Fatal("album membership was lost by the reconnect")
	}
	if after := originalHashes(t, canonicalMoved); !sameHashes(before, after) {
		t.Fatalf("reconnect changed original bytes: before=%v after=%v", before, after)
	}

	// A same-identity copy while the original is online is never a move.
	copyPath := filepath.Join(diskA.Path, "travel-copy")
	copyTree(t, canonicalMoved, copyPath)
	refused := h.approve(h.nativeTask(map[string]any{"kind": "locate_repository", "repository_id": travel.ID}), copyPath)
	if refused.Status != "failed" || !strings.HasSuffix(problemType(refused), "storage/original-online") {
		t.Fatalf("online-copy task = %+v problem=%+v", refused, refused.Problem)
	}
	if got := h.repositoryPath(travel.ID); got != canonicalMoved {
		t.Fatalf("refused reconnect moved the row to %q", got)
	}

	// A tree carrying another identity is refused once the original is
	// positively gone, and the row is untouched.
	other := h.createIn(diskA, "Other", "other")
	awayPath := filepath.Join(diskB.Path, "travel-away")
	h.move(canonicalMoved, awayPath)
	mismatch := h.approve(h.nativeTask(map[string]any{"kind": "locate_repository", "repository_id": travel.ID}), other.Path)
	if mismatch.Status != "failed" || !strings.HasSuffix(problemType(mismatch), "storage/identity-mismatch") {
		t.Fatalf("wrong-identity task = %+v problem=%+v", mismatch, mismatch.Problem)
	}
	if got := h.repositoryPath(travel.ID); got != canonicalMoved {
		t.Fatalf("refused reconnect moved the row to %q", got)
	}

	// An original that cannot be read authorizes nothing: the task names the
	// access problem instead of treating the denial as absence.
	if err := os.MkdirAll(canonicalMoved, 0o755); err != nil {
		t.Fatal(err)
	}
	h.setObserver(&acceptanceDenyingObserver{denied: canonicalMoved})
	denied := h.approve(h.nativeTask(map[string]any{"kind": "locate_repository", "repository_id": travel.ID}), awayPath)
	h.setObserver(storage.OSStorageObserver{})
	if denied.Status != "failed" || !strings.HasSuffix(problemType(denied), "storage/action-unavailable") ||
		denied.Refusal == nil || len(denied.Refusal.Blockers) != 1 || denied.Refusal.Blockers[0] != "original_inaccessible" ||
		len(denied.Refusal.Alternatives) != 1 || denied.Refusal.Alternatives[0] != "check_access" {
		t.Fatalf("denied-original task = %+v problem=%+v refusal=%+v", denied, denied.Problem, denied.Refusal)
	}
	if err := os.Remove(canonicalMoved); err != nil {
		t.Fatal(err)
	}

	// With the original positively gone, the same destination reconnects.
	again := h.approve(h.nativeTask(map[string]any{"kind": "locate_repository", "repository_id": travel.ID}), awayPath)
	if again.Status != "succeeded" {
		t.Fatalf("second reconnect task = %+v problem=%+v refusal=%+v", again, again.Problem, again.Refusal)
	}
	if got := h.repositoryPath(travel.ID); got != canonicalDir(t, awayPath) {
		t.Fatalf("repository path after second reconnect = %q", got)
	}
	if after := originalHashes(t, canonicalDir(t, awayPath)); !sameHashes(before, after) {
		t.Fatalf("second reconnect changed original bytes: before=%v after=%v", before, after)
	}
}

// A whole external Storage Location moved with its Repositories reconnects
// every child or nothing: a child missing at the destination refuses the
// task and leaves every row at its old path; a complete tree re-points the
// Location and each child atomically. The storage view offers the Location
// reconnect only while the original is positively unavailable and never on
// the Default Storage Location.
func TestMovedStorageLocationReconnectsEveryChildOrNothing(t *testing.T) {
	h := startReconnectHarness(t)
	diskC := h.externalDisk("disk-c")
	alpha := h.createIn(diskC, "Alpha", "alpha")
	beta := h.createIn(diskC, "Beta", "beta")
	h.upload(alpha, "alpha.jpg", blackboxJPEG(t, 60, "alpha"))
	h.upload(beta, "beta.jpg", blackboxJPEG(t, 200, "beta"))
	h.waitProcessingSettled()
	before := originalHashes(t, diskC.Path)

	view := h.view()
	for _, location := range view.StorageLocations {
		blocked := map[string][]string{}
		for _, entry := range location.UnavailableActions {
			blocked[entry.Action] = entry.Blockers
		}
		switch location.ID {
		case diskC.ID:
			if len(location.Actions) != 0 || blocked["reconnect"][0] != "original_available" || blocked["detach"][0] != "registered_repositories" {
				t.Fatalf("online external Location actions = %+v", location)
			}
		default:
			if location.Kind != "default" || len(location.Actions) != 0 || blocked["reconnect"][0] != "default_protected" || blocked["detach"][0] != "default_protected" {
				t.Fatalf("default Location actions = %+v", location)
			}
		}
	}

	movedPath := filepath.Join(h.root, "disk-c-moved")
	h.move(diskC.Path, movedPath)
	if offline := h.viewLocation(diskC.ID); len(offline.Actions) != 1 || offline.Actions[0] != "reconnect" {
		t.Fatalf("moved external Location actions = %+v", offline)
	}

	// One child is not where the Location layout says it is: nothing moves.
	if err := os.Rename(filepath.Join(movedPath, "beta"), filepath.Join(movedPath, "beta-elsewhere")); err != nil {
		t.Fatal(err)
	}
	partial := h.approve(h.nativeTask(map[string]any{"kind": "locate_storage_location", "storage_location_id": diskC.ID}), movedPath)
	if partial.Status != "failed" {
		t.Fatalf("partial Location reconnect task = %+v", partial)
	}
	for _, repository := range []blackboxRepository{alpha, beta} {
		if got := h.repositoryPath(repository.ID); got != repository.Path {
			t.Fatalf("partial Location reconnect moved %s to %q", repository.Name, got)
		}
	}
	if row := h.viewLocation(diskC.ID); len(row.Actions) != 1 || row.Actions[0] != "reconnect" {
		t.Fatalf("Location after refused reconnect = %+v", row)
	}

	// The complete tree reconnects the Location and both children together.
	if err := os.Rename(filepath.Join(movedPath, "beta-elsewhere"), filepath.Join(movedPath, "beta")); err != nil {
		t.Fatal(err)
	}
	complete := h.approve(h.nativeTask(map[string]any{"kind": "locate_storage_location", "storage_location_id": diskC.ID}), movedPath)
	if complete.Status != "succeeded" || complete.Result == nil || complete.Result.StorageLocationID != diskC.ID {
		t.Fatalf("Location reconnect task = %+v problem=%+v refusal=%+v", complete, complete.Problem, complete.Refusal)
	}
	canonicalMoved := canonicalDir(t, movedPath)
	for _, repository := range []blackboxRepository{alpha, beta} {
		want := filepath.Join(canonicalMoved, filepath.Base(repository.Path))
		if got := h.repositoryPath(repository.ID); got != want {
			t.Fatalf("%s path after Location reconnect = %q, want %q", repository.Name, got, want)
		}
		row := h.viewRepository(repository.ID)
		if row.StorageLocationID != diskC.ID || row.Reachability != "active" {
			t.Fatalf("%s row after Location reconnect = %+v", repository.Name, row)
		}
	}
	if row := h.viewLocation(diskC.ID); len(row.Actions) != 0 {
		t.Fatalf("reconnected Location still offers %v", row.Actions)
	}
	if after := originalHashes(t, canonicalMoved); !sameHashes(before, after) {
		t.Fatalf("Location reconnect changed original bytes: before=%v after=%v", before, after)
	}
	// The children scan again against their new paths and stay idle.
	deadline := time.Now().Add(2 * time.Minute)
	for _, repository := range []blackboxRepository{alpha, beta} {
		h.waitIdle(blackboxRepository{ID: repository.ID, Name: repository.Name})
		if time.Now().After(deadline) {
			t.Fatal("children did not settle after the Location reconnect")
		}
	}
}
