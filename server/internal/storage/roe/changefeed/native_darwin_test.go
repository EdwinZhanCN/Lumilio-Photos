//go:build darwin && cgo

package changefeed

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"server/internal/db/repo"
)

func TestFSEventsReplaysRepositoryRelativeChangesAndRejectsForeignJournal(t *testing.T) {
	feed := newNative().(*fseventsFeed)
	t.Cleanup(func() { _ = feed.Close() })
	// The workspace may be an external or virtual volume without an FSEvents
	// journal UUID. Prefer the host-native temporary volume, while allowing a
	// native-profile run to select a disposable APFS mount explicitly.
	repositoryPath := t.TempDir()
	var err error
	if profileRoot := os.Getenv("LUMILIO_ROE_NATIVE_TEST_ROOT"); profileRoot != "" {
		repositoryPath, err = os.MkdirTemp(profileRoot, ".native-fsevents-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(repositoryPath) })
	}
	repositoryPath, err = filepath.Abs(repositoryPath)
	if err != nil {
		t.Fatal(err)
	}
	repository := repo.Repository{RepoID: uuid.New(), Path: repositoryPath}
	start, err := feed.Snapshot(context.Background(), repository)
	if err != nil {
		t.Skipf("FSEvents journal unavailable in this test environment: %v", err)
	}
	filename := filepath.Join(repository.Path, "native-change.jpg")
	if err := os.WriteFile(filename, []byte("native change"), 0o600); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(10 * time.Second)
	var batch Batch
	lastThrough := start
	found := false
	for time.Now().Before(deadline) {
		through, snapshotErr := feed.Snapshot(context.Background(), repository)
		if snapshotErr != nil {
			t.Fatal(snapshotErr)
		}
		lastThrough = through
		if !start.SamePosition(through) {
			batch, err = feed.Read(context.Background(), repository, start, through, 16)
			if err == nil {
				for _, event := range batch.Events {
					if event.Path == "native-change.jpg" {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatalf("FSEvents batch = %+v, start = %q, through = %q, want native-change.jpg", batch.Events, start.Cursor, lastThrough.Cursor)
	}

	foreign := start
	foreign.JournalIdentity = "different-volume-journal"
	if _, err := feed.Read(context.Background(), repository, foreign, batch.Next, 16); !errors.Is(err, ErrCursorInvalid) {
		t.Fatalf("foreign FSEvents journal error = %v", err)
	}
}

func TestFSEventsRelativeRootUsesDeviceNamespace(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name       string
		repository string
		mount      string
		want       string
	}{
		{name: "volume root", repository: "/Volumes/Photos", mount: "/Volumes/Photos", want: ""},
		{name: "nested removable repository", repository: "/Volumes/Photos/Library/2026", mount: "/Volumes/Photos", want: "Library/2026"},
		{name: "root volume", repository: "/Users/photographer/Pictures", mount: "/", want: "Users/photographer/Pictures"},
		{name: "APFS Data firmlink", repository: "/Users/photographer/Pictures", mount: "/System/Volumes/Data", want: "Users/photographer/Pictures"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := fseventsRelativeRoot(test.repository, test.mount)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("relative root = %q, want %q", got, test.want)
			}
		})
	}
	if _, err := fseventsRelativeRoot("/Users/photos", "/Volumes/External"); err == nil {
		t.Fatal("repository outside device mount unexpectedly accepted")
	}
}

func TestFSEventsUserPathNormalizesDeviceRelativeEvents(t *testing.T) {
	t.Parallel()
	repository := repo.Repository{RepoID: uuid.New(), Path: "/Volumes/Photos/Library"}
	for _, test := range []struct {
		name       string
		nativePath string
		want       string
		wantError  bool
	}{
		{name: "root event", nativePath: "Library", want: ""},
		{name: "nested file", nativePath: "Library/Trips/day.jpg", want: "Trips/day.jpg"},
		{name: "absolute callback compatibility", nativePath: "/Volumes/Photos/Library/day.jpg", want: "day.jpg"},
		{name: "outside repository", nativePath: "Other/day.jpg", wantError: true},
		{name: "Lumilio probe", nativePath: "Library/.lumilio_case_probe-1", wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := fseventsUserPath(repository, "Library", test.nativePath)
			if test.wantError {
				if err == nil {
					t.Fatalf("path = %q, want rejection", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("path = %q, want %q", got, test.want)
			}
		})
	}
}
