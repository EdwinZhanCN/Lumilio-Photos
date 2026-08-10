package cloud

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/internal/db/repo"
	"server/internal/sourcing"
	"server/internal/storage"

	"github.com/google/uuid"
)

type cursorTestProvider struct{}

func (cursorTestProvider) Name() ProviderKind { return ProviderICloud }

func (cursorTestProvider) List(context.Context, uuid.UUID, *Cursor, map[string]string) (*Page, error) {
	return &Page{
		Assets: []ReleaseAsset{{
			Provider: ProviderICloud, RemoteKey: "remote/photo.jpg", Filename: "photo.jpg",
			Size: 5, MIME: "image/jpeg", ETag: "etag-1", ModifiedAt: time.Now().UTC(),
		}},
		Cursor: &Cursor{Value: "next-page"},
	}, nil
}

type scopeRecordingProvider struct{ received map[string]string }

func (*scopeRecordingProvider) Name() ProviderKind { return ProviderICloud }

func (provider *scopeRecordingProvider) List(_ context.Context, _ uuid.UUID, _ *Cursor, scope map[string]string) (*Page, error) {
	provider.received = make(map[string]string, len(scope))
	for key, value := range scope {
		provider.received[key] = value
	}
	return &Page{}, nil
}

func (*scopeRecordingProvider) Download(context.Context, uuid.UUID, string, io.Writer) (int64, error) {
	panic("empty scoped listing must not download")
}

func TestCloudImportPassesRemoteScopeToProvider(t *testing.T) {
	provider := &scopeRecordingProvider{}
	source := NewCloudImportSource(CloudImportSourceConfig{
		Provider: provider, State: &cursorTestState{}, Repository: repo.Repository{RepoID: uuid.New()},
		RemoteScope: map[string]string{"album": "Favorites"},
	})
	if err := source.ForEach(context.Background(), func(sourcing.IngestSource) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if provider.received["album"] != "Favorites" {
		t.Fatalf("provider remote scope = %v", provider.received)
	}
}

func (cursorTestProvider) Download(_ context.Context, _ uuid.UUID, _ string, destination io.Writer) (int64, error) {
	written, err := destination.Write([]byte("photo"))
	return int64(written), err
}

type cursorTestState struct {
	saved []string
}

func (*cursorTestState) GetCursor(context.Context, uuid.UUID, ProviderKind) (string, error) {
	return "", nil
}

func (state *cursorTestState) SaveCursor(_ context.Context, _ uuid.UUID, _ ProviderKind, cursor string) error {
	state.saved = append(state.saved, cursor)
	return nil
}

func (*cursorTestState) IsFileSynced(context.Context, uuid.UUID, ProviderKind, string, string) (bool, error) {
	return false, nil
}

func (*cursorTestState) MarkFileSynced(context.Context, uuid.UUID, ProviderKind, string, string, uuid.UUID) error {
	return nil
}

type cursorTestStaging struct {
	directory string
}

type rejectingCapacityGuard struct {
	expected uint64
}

func (guard *rejectingCapacityGuard) CheckRepositoryWriteCapacity(_ context.Context, _ string, expected uint64) (storage.CapacityDecision, error) {
	guard.expected = expected
	return storage.CapacityDecision{Allowed: false, ExpectedBytes: expected}, storage.ErrInsufficientSpace
}

type stagedCapacityGuard struct {
	calls    int
	rejectAt int
}

func (guard *stagedCapacityGuard) CheckRepositoryWriteCapacity(_ context.Context, _ string, expected uint64) (storage.CapacityDecision, error) {
	guard.calls++
	if guard.rejectAt > 0 && guard.calls == guard.rejectAt {
		return storage.CapacityDecision{Allowed: false, ExpectedBytes: expected}, storage.ErrInsufficientSpace
	}
	return storage.CapacityDecision{Allowed: true, ExpectedBytes: expected}, nil
}

type unknownSizeProvider struct{}

func (unknownSizeProvider) Name() ProviderKind { return ProviderICloud }

func (unknownSizeProvider) List(context.Context, uuid.UUID, *Cursor, map[string]string) (*Page, error) {
	return &Page{
		Assets: []ReleaseAsset{{
			Provider: ProviderICloud, RemoteKey: "remote/unknown.jpg", Filename: "unknown.jpg",
			Size: 0, MIME: "image/jpeg", ETag: "unknown-etag", ModifiedAt: time.Now().UTC(),
		}},
		Cursor: &Cursor{Value: "unknown-complete"},
	}, nil
}

func (unknownSizeProvider) Download(_ context.Context, _ uuid.UUID, _ string, destination io.Writer) (int64, error) {
	payload := make([]byte, capacityResampleBytes+1)
	for index := range payload {
		payload[index] = byte(index % 251)
	}
	written, err := destination.Write(payload)
	return int64(written), err
}

type recoveryTestStaging struct {
	directory string
	failed    int
}

func (staging *recoveryTestStaging) CreateStagingFile(repository repo.Repository, filename string) (*storage.StagingFile, *storage.RepositoryFile, error) {
	path := filepath.Join(staging.directory, uuid.NewString()+"-"+filename)
	opened, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return &storage.StagingFile{
		ID: uuid.NewString(), RepositoryID: repository.RepoID, PrivatePath: path, Filename: filename,
	}, &storage.RepositoryFile{File: opened}, nil
}

func (staging *recoveryTestStaging) MoveStagingToFailed(_ repo.Repository, staged *storage.StagingFile) error {
	staging.failed++
	return os.Rename(staged.PrivatePath, staged.PrivatePath+".failed")
}

func (staging *recoveryTestStaging) RemoveStagingFile(_ repo.Repository, staged *storage.StagingFile) error {
	return os.Remove(staged.PrivatePath)
}

func TestCapacitySamplingWriterStopsUnknownSizeDownloadAtReserveBoundary(t *testing.T) {
	guard := &rejectingCapacityGuard{}
	writer := &capacitySamplingWriter{
		ctx: context.Background(), repositoryID: uuid.NewString(), guard: guard, destination: io.Discard,
	}
	written, err := writer.Write(make([]byte, capacityResampleBytes+1))
	if !errors.Is(err, storage.ErrInsufficientSpace) {
		t.Fatalf("continuous capacity error = %v, want ErrInsufficientSpace", err)
	}
	if int64(written) != capacityResampleBytes {
		t.Fatalf("bytes written before capacity pause = %d, want %d", written, capacityResampleBytes)
	}
	if guard.expected != 0 {
		t.Fatalf("continuous sample expected bytes = %d, want unknown-size sample 0", guard.expected)
	}
}

func (staging cursorTestStaging) CreateStagingFile(repository repo.Repository, filename string) (*storage.StagingFile, *storage.RepositoryFile, error) {
	opened, err := os.CreateTemp(staging.directory, "cloud-*")
	if err != nil {
		return nil, nil, err
	}
	return &storage.StagingFile{
		ID: uuid.NewString(), RepositoryID: repository.RepoID,
		PrivatePath: ".lumilio/staging/incoming/" + filepath.Base(opened.Name()), Filename: filename,
	}, &storage.RepositoryFile{File: opened}, nil
}

func TestCloudCapacityPreflightStopsBeforeStagingAndCursorAdvance(t *testing.T) {
	state := &cursorTestState{}
	guard := &rejectingCapacityGuard{}
	source := NewCloudImportSource(CloudImportSourceConfig{
		Provider: cursorTestProvider{}, State: state,
		Repository: repo.Repository{RepoID: uuid.New()},
		Staging:    cursorTestStaging{directory: ""}, CapacityGuard: guard,
	})
	err := source.ForEach(context.Background(), func(sourcing.IngestSource) error {
		t.Fatal("capacity-rejected asset reached materializer")
		return nil
	})
	if !errors.Is(err, storage.ErrInsufficientSpace) {
		t.Fatalf("ForEach error = %v, want ErrInsufficientSpace", err)
	}
	if guard.expected != 5 {
		t.Fatalf("capacity expected bytes = %d, want 5", guard.expected)
	}
	if len(state.saved) != 0 {
		t.Fatalf("capacity rejection advanced cursor: %v", state.saved)
	}
}

func TestUnknownSizeCapacityPausePreservesRetryIntegrity(t *testing.T) {
	repository := repo.Repository{RepoID: uuid.New()}
	state := &cursorTestState{}
	firstStaging := &recoveryTestStaging{directory: t.TempDir()}
	firstGuard := &stagedCapacityGuard{rejectAt: 2} // allow unknown-size preflight, reject the first continuous sample
	first := NewCloudImportSource(CloudImportSourceConfig{
		Provider: unknownSizeProvider{}, State: state, Repository: repository,
		Staging: firstStaging, CapacityGuard: firstGuard,
	})
	if err := first.ForEach(context.Background(), func(sourcing.IngestSource) error {
		t.Fatal("partially downloaded unknown-size asset reached the materializer")
		return nil
	}); !errors.Is(err, storage.ErrInsufficientSpace) {
		t.Fatalf("first import error = %v, want ErrInsufficientSpace", err)
	}
	if firstStaging.failed != 1 || len(state.saved) != 0 {
		t.Fatalf("paused unknown-size import failed=%d cursor=%v", firstStaging.failed, state.saved)
	}

	secondStaging := &recoveryTestStaging{directory: t.TempDir()}
	second := NewCloudImportSource(CloudImportSourceConfig{
		Provider: unknownSizeProvider{}, State: state, Repository: repository,
		Staging: secondStaging, CapacityGuard: &stagedCapacityGuard{},
	})
	consumed := 0
	if err := second.ForEach(context.Background(), func(candidate sourcing.IngestSource) error {
		consumed++
		data, err := os.ReadFile(candidate.StagingPath)
		if err != nil {
			return err
		}
		if int64(len(data)) != capacityResampleBytes+1 || data[capacityResampleBytes] != byte(capacityResampleBytes%251) {
			t.Fatalf("retried unknown-size payload is incomplete: size=%d tail=%d", len(data), data[capacityResampleBytes])
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if consumed != 1 || len(state.saved) != 1 || state.saved[0] != "unknown-complete" {
		t.Fatalf("retried unknown-size import consumed=%d cursor=%v", consumed, state.saved)
	}
}

func (cursorTestStaging) MoveStagingToFailed(repo.Repository, *storage.StagingFile) error {
	return nil
}

func (cursorTestStaging) RemoveStagingFile(repo.Repository, *storage.StagingFile) error {
	return nil
}

func TestCloudCursorAdvancesOnlyAfterCandidateAcknowledgement(t *testing.T) {
	repository := repo.Repository{RepoID: uuid.New()}
	consumeErr := errors.New("materializer did not commit")
	for _, test := range []struct {
		name       string
		consumeErr error
		wantSaved  int
	}{
		{name: "committed", wantSaved: 1},
		{name: "not committed", consumeErr: consumeErr, wantSaved: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := &cursorTestState{}
			source := NewCloudImportSource(CloudImportSourceConfig{
				Provider: cursorTestProvider{}, State: state, Repository: repository,
				Staging: cursorTestStaging{directory: t.TempDir()},
			})
			calls := 0
			err := source.ForEach(context.Background(), func(candidate sourcing.IngestSource) error {
				calls++
				if candidate.StagingPath == "" || candidate.Kind != sourcing.IngestSourceCloud {
					t.Fatalf("invalid cloud candidate: %+v", candidate)
				}
				return test.consumeErr
			})
			if !errors.Is(err, test.consumeErr) {
				t.Fatalf("ForEach error = %v, want %v", err, test.consumeErr)
			}
			if calls != 1 || len(state.saved) != test.wantSaved {
				t.Fatalf("calls=%d saved=%v, want calls=1 saved=%d", calls, state.saved, test.wantSaved)
			}
		})
	}
}

func TestCloudCancellationCleansOnlyUnclaimedStaging(t *testing.T) {
	repository := repo.Repository{RepoID: uuid.New()}
	for _, test := range []struct {
		name      string
		prepared  bool
		wantFiles int
	}{
		{name: "before prepared asset", prepared: false, wantFiles: 0},
		{name: "after prepared asset", prepared: true, wantFiles: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			state := &cursorTestState{}
			staging := &recoveryTestStaging{directory: directory}
			source := NewCloudImportSource(CloudImportSourceConfig{
				Provider: cursorTestProvider{}, State: state, Repository: repository, Staging: staging,
			})
			err := source.ForEach(context.Background(), func(sourcing.IngestSource) error {
				return sourcing.WithStagingOwnership(context.Canceled, test.prepared)
			})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("cancel error = %v", err)
			}
			entries, readErr := os.ReadDir(directory)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if len(entries) != test.wantFiles {
				t.Fatalf("staging files after cancel = %d, want %d", len(entries), test.wantFiles)
			}
			if len(state.saved) != 0 {
				t.Fatalf("cancel advanced cursor: %v", state.saved)
			}
		})
	}
}
