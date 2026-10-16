package scan

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/internal/db/repo"
	"server/internal/storage"
	fileutil "server/internal/utils/file"
)

// bindKnown indexes a file the way an upload commit does: the content hash
// comes from the handle that wrote it and a post-commit stat, so the scan
// index binds it without hashing (#222 "Uploads and cloud").
func (f *fixture) bindKnown(repository repo.Repository, relative string) KnownBinding {
	f.t.Helper()
	fsys, err := f.files.OpenContext(f.ctx, repository)
	if err != nil {
		f.t.Fatal(err)
	}
	defer fsys.Close()
	repositoryPath, err := storage.ParseUserMediaPath(relative)
	if err != nil {
		f.t.Fatal(err)
	}
	observation, err := fsys.InspectMedia(f.ctx, repositoryPath, storage.HashNone)
	if err != nil {
		f.t.Fatal(err)
	}
	validation := fileutil.ValidateFile(filepath.Base(relative), "")
	if !validation.Valid {
		f.t.Fatalf("%s is not a supported media file", relative)
	}
	binding, err := f.scanner.BindKnownContent(f.ctx, KnownContent{
		RepositoryID: repository.RepoID, OwnerID: f.owner, RelativePath: relative,
		AssetType: string(validation.AssetType), MimeType: validation.MimeType,
		FullHash: fileHash(f.t, f.path(repository, relative)), Observation: observation,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return binding
}

// #222: "An upload followed by a scan does not rehash." The uploaded files
// have just landed (their mtime is now, inside a production-sized settle
// window), which is exactly when a walk would otherwise defer or rehash them.
func TestUploadedContentIsNotRehashedOrDeferredByTheNextScan(t *testing.T) {
	f := newFixture(t, 30*time.Second)
	uploads := []string{"upload.jpg", "2026/trip/nested.jpg"}
	bindings := make(map[string]KnownBinding, len(uploads))
	for _, relative := range uploads {
		target := f.path(f.primary, relative)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte("uploaded "+relative), 0o644); err != nil {
			t.Fatal(err)
		}
		bindings[relative] = f.bindKnown(f.primary, relative)
	}
	before := make(map[string]repo.RepositoryEntry, len(uploads))
	for _, relative := range uploads {
		before[relative] = f.mustEntry(f.primary, relative, StatePresent)
	}
	f.resetObservations()

	scan := f.scan(f.primary)
	if scan.Status != StatusCompleted || scan.NewEntries+scan.Changed+scan.Deferred+scan.Missing+scan.Restored != 0 {
		t.Fatalf("scan after upload = %+v, want nothing new, changed, or deferred", scan)
	}
	if bound := f.hash(f.primary); bound != 0 {
		t.Fatalf("scan after upload rehashed %d files", bound)
	}
	if hashes := f.hashComputations(); hashes != 0 {
		t.Fatalf("scan after upload computed %d content hashes", hashes)
	}
	// The ancestors an upload creates record their first stat tuple; the
	// uploaded files themselves are not touched.
	for _, write := range f.entryWrites() {
		if write.Kind != KindDirectory {
			t.Fatalf("scan after upload wrote file entry rows: %+v", f.entryWrites())
		}
	}
	for _, relative := range uploads {
		after := f.mustEntry(f.primary, relative, StatePresent)
		if after.Revision != before[relative].Revision || after.AssetID.UUID != bindings[relative].AssetID {
			t.Fatalf("upload %s changed: %+v -> %+v", relative, before[relative], after)
		}
	}
	if _, err := f.database.ReaderQueries.GetQueuedRepositoryScan(f.ctx, f.primary.RepoID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("scan after upload queued a follow-up (err %v); an uploaded file is never inside the settle window", err)
	}
	if count := f.assetCount(); count != len(uploads) {
		t.Fatalf("assets = %d, want %d", count, len(uploads))
	}
}
