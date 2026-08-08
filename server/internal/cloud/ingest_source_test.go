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

func (cursorTestProvider) List(context.Context, uuid.UUID, *Cursor) (*Page, error) {
	return &Page{
		Assets: []ReleaseAsset{{
			Provider: ProviderICloud, RemoteKey: "remote/photo.jpg", Filename: "photo.jpg",
			Size: 5, MIME: "image/jpeg", ETag: "etag-1", ModifiedAt: time.Now().UTC(),
		}},
		Cursor: &Cursor{Value: "next-page"},
	}, nil
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

func (cursorTestStaging) MoveStagingToFailed(repo.Repository, *storage.StagingFile) error {
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
