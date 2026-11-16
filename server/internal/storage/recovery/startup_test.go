package recovery

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"server/internal/catalog/repo"
	"server/internal/storage"
	"server/internal/storage/layout"
)

// onDisk is where a private path of the repository lives.
func onDisk(t *testing.T, repository *repo.Repository, p layout.Path) string {
	t.Helper()
	local, err := p.Local()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(repository.Path, local)
}

func writePrivate(t *testing.T, repository *repo.Repository, p layout.Path, body string) {
	t.Helper()
	target := onDisk(t, repository, p)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func must(p layout.Path, err error) layout.Path {
	if err != nil {
		panic(err)
	}
	return p
}

// At startup a repository gets its missing layout directories back, and what
// the last run left in work/ moves under a startup receipt. An upload session
// that a pending staging commit names, or whose manifest lets the browser
// resume it, stays; a second start finds nothing left to do.
func TestPrepareRepositoryRepairsTheLayoutAndQuarantinesDeadWork(t *testing.T) {
	f := newFixture(t)
	repository, _ := f.openLeftover(nil)

	dead, resumable, committed := uuid.New(), uuid.New(), uuid.New()
	tempFile := must(layout.TempDir().Child("interrupted-write"))
	deadChunk := must(layout.UploadChunk(dead, 0))
	resumableChunk := must(layout.UploadChunk(resumable, 0))
	committedFile := must(layout.UploadFile(committed, "IMG.jpg"))
	writePrivate(t, repository, tempFile, "temp")
	writePrivate(t, repository, deadChunk, "dead chunk")
	writePrivate(t, repository, must(layout.UploadManifest(resumable)), "{}")
	writePrivate(t, repository, resumableChunk, "resumable chunk")
	writePrivate(t, repository, committedFile, "staged")
	if _, err := f.database.SQL.ExecContext(f.ctx, `
		INSERT INTO repository_staging_commits (commit_id, repository_id, owner_id, source_kind, staging_path,
		  original_filename, mime_type, full_hash, file_size, status, created_at, updated_at)
		VALUES (?, ?, ?, 'upload', ?, 'IMG.jpg', 'image/jpeg', ?, 6, 'prepared', 1, 1)`,
		uuid.NewString(), repository.RepoID, f.owner, committedFile.String(), strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	cache := must(layout.ClassDir(layout.Cache))
	if err := os.RemoveAll(onDisk(t, repository, cache)); err != nil {
		t.Fatal(err)
	}

	result, err := f.recovery.PrepareRepository(f.ctx, *repository)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(result.Repaired, cache.String()) {
		t.Errorf("repaired = %v; want %s", result.Repaired, cache)
	}
	if info, err := os.Stat(onDisk(t, repository, cache)); err != nil || !info.IsDir() {
		t.Errorf("%s was not recreated: %v", cache, err)
	}
	if result.ReceiptID == "" || result.Quarantined != 2 {
		t.Fatalf("result = %+v; want two files under a receipt", result)
	}

	inventory, err := f.recovery.Inventory(f.ctx, repository.RepoID)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []layout.Path{tempFile, deadChunk} {
		item, ok := itemAt(inventory, path.String())
		if !ok || item.ReceiptID != result.ReceiptID || item.Disposition != storage.DispositionMoved || !item.Present {
			t.Errorf("%s: item %+v, listed %t; want moved and present under the startup receipt", path, item, ok)
		}
		if _, err := os.Stat(onDisk(t, repository, path)); !os.IsNotExist(err) {
			t.Errorf("%s is still in work/: %v", path, err)
		}
	}
	if _, err := os.Stat(onDisk(t, repository, must(layout.UploadSession(dead)))); !os.IsNotExist(err) {
		t.Errorf("the dead session directory stayed: %v", err)
	}
	for _, kept := range []layout.Path{resumableChunk, committedFile} {
		if _, err := os.Stat(onDisk(t, repository, kept)); err != nil {
			t.Errorf("%s was not kept: %v", kept, err)
		}
	}

	again, err := f.recovery.PrepareRepository(f.ctx, *repository)
	if err != nil || again.ReceiptID != "" || again.Quarantined != 0 || len(again.Repaired) != 0 {
		t.Fatalf("second start = %+v, %v; want nothing to do", again, err)
	}
}

// A start that wrote its receipt but stopped before the moves is finished by
// the next one, under the same receipt.
func TestPrepareRepositoryFinishesAnInterruptedStartup(t *testing.T) {
	f := newFixture(t)
	repository, _ := f.openLeftover(nil)
	leftover := must(layout.TempDir().Child("left-over"))
	writePrivate(t, repository, leftover, "bytes")
	receiptID := uuid.New()
	destination := must(layout.RecoveryMoved(receiptID, leftover))
	files, err := f.files.OpenContext(f.ctx, *repository)
	if err != nil {
		t.Fatal(err)
	}
	err = storage.WriteRecoveryReceipt(files, storage.RecoveryReceipt{
		Format: storage.RecoveryReceiptFormat, ReceiptID: receiptID.String(), Event: storage.RecoveryEventStartup,
		RecordedAt: f.recovery.now(), SourceRepositoryID: repository.RepoID.String(), RepositoryID: repository.RepoID.String(),
		Items: []storage.RecoveryItem{{
			ItemID: storage.RecoveryItemID(receiptID.String(), leftover.String()), Class: storage.PrivateClassStaging,
			Path: leftover.String(), Size: 5, Disposition: storage.DispositionMoved, MovedTo: destination.String(),
		}},
	})
	_ = files.Close()
	if err != nil {
		t.Fatal(err)
	}

	result, err := f.recovery.PrepareRepository(f.ctx, *repository)
	if err != nil {
		t.Fatal(err)
	}
	if result.ReceiptID != "" {
		t.Fatalf("a new receipt %s was written for a file an earlier one already lists", result.ReceiptID)
	}
	if data, err := os.ReadFile(onDisk(t, repository, destination)); err != nil || string(data) != "bytes" {
		t.Fatalf("the recorded move was not finished: %q, %v", data, err)
	}
	if _, err := os.Stat(onDisk(t, repository, leftover)); !os.IsNotExist(err) {
		t.Fatalf("the source stayed in work/: %v", err)
	}
}
