//go:build windows

package changefeed

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sys/windows"

	"server/internal/db/repo"
)

func TestWindowsNativeFeedUsesUSNOrRDCWAndFailsClosedOnCursorLoss(t *testing.T) {
	feed := newNative().(*windowsFeed)
	t.Cleanup(func() { _ = feed.Close() })
	repository := repo.Repository{RepoID: uuid.New(), Path: t.TempDir()}
	start, err := feed.Snapshot(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}
	if start.AdapterKind != "usn" && start.AdapterKind != "rdcw" {
		t.Fatalf("Windows adapter = %q", start.AdapterKind)
	}
	// Create until RDCW reports a wakeup. CreateFile returning only proves the
	// directory handle exists; the watcher goroutine may not have armed its
	// first blocking ReadDirectoryChangesW call yet on a loaded hosted runner.
	wakeupDeadline := time.Now().Add(10 * time.Second)
	for attempt := 0; ; attempt++ {
		name := fmt.Sprintf("windows-change-%d.jpg", attempt)
		if err := os.WriteFile(filepath.Join(repository.Path, name), []byte("change"), 0o600); err != nil {
			t.Fatal(err)
		}
		select {
		case <-feed.Notifications():
			goto woke
		case <-time.After(25 * time.Millisecond):
			if time.Now().After(wakeupDeadline) {
				t.Fatal("ReadDirectoryChangesW emitted no wakeup")
			}
		}
	}

woke:

	deadline := time.Now().Add(10 * time.Second)
	var batch Batch
	for time.Now().Before(deadline) {
		through, snapshotErr := feed.Snapshot(context.Background(), repository)
		if snapshotErr != nil {
			t.Fatal(snapshotErr)
		}
		if through.AdapterKind != start.AdapterKind {
			// USN availability may change only by failing closed into a new
			// RDCW identity; the old cursor cannot be reused.
			if _, err := feed.Read(context.Background(), repository, start, through, 32); !errors.Is(err, ErrCursorInvalid) {
				t.Fatalf("adapter fallback error = %v", err)
			}
			return
		}
		if !start.SamePosition(through) {
			batch, err = feed.Read(context.Background(), repository, start, through, 32)
			if err == nil && len(batch.Events) > 0 {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range batch.Events {
		if strings.HasPrefix(event.Path, "windows-change-") || event.Recursive {
			found = true
		}
	}
	if !found {
		t.Fatalf("Windows native batch = %+v", batch.Events)
	}

	foreign := start
	foreign.JournalIdentity = "invalid-journal"
	if _, err := feed.Read(context.Background(), repository, foreign, batch.Next, 32); !errors.Is(err, ErrCursorInvalid) {
		t.Fatalf("foreign Windows cursor error = %v", err)
	}
}

func TestUSNTruncationWindowIsRejectedBeforeReadingRecords(t *testing.T) {
	checkpoint := Checkpoint{
		AdapterKind: "usn", Cursor: []byte("1"), VolumeIdentity: "volume",
		VolumeKind: "local", JournalIdentity: "0000000000000001", Health: HealthHealthy,
	}
	// Identity mismatch is the same fail-closed boundary used for a replaced
	// or truncated journal before any record can be interpreted.
	through := checkpoint
	through.JournalIdentity = "0000000000000002"
	feed := newNative().(*windowsFeed)
	t.Cleanup(func() { _ = feed.Close() })
	if _, err := feed.Read(context.Background(), repo.Repository{RepoID: uuid.New(), Path: t.TempDir()}, checkpoint, through, 1); !errors.Is(err, ErrCursorInvalid) {
		t.Fatalf("USN journal replacement error = %v", err)
	}
}

func TestResolvedUSNPathFiltersVolumeWideAndPrivateEvents(t *testing.T) {
	root := t.TempDir()
	repository := repo.Repository{RepoID: uuid.New(), Path: filepath.Join(root, "repository")}
	if err := os.Mkdir(repository.Path, 0o700); err != nil {
		t.Fatal(err)
	}

	if relative, relevant := resolvedUSNUserPath(repository, repository.Path, "photo.jpg"); !relevant || relative != "photo.jpg" {
		t.Fatalf("repository USN path = %q, relevant %t", relative, relevant)
	}
	if relative, relevant := resolvedUSNUserPath(repository, root, "outside.jpg"); relevant || relative != "" {
		t.Fatalf("volume-wide USN path = %q, relevant %t", relative, relevant)
	}
	if relative, relevant := resolvedUSNUserPath(repository, filepath.Join(repository.Path, ".lumilio"), "state"); relevant || relative != "" {
		t.Fatalf("private USN path = %q, relevant %t", relative, relevant)
	}
}

func TestWindowsVolumeKindClassification(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		filesystem string
		driveType  uint32
		want       string
	}{
		{name: "fixed NTFS", path: `C:\\photos`, filesystem: "NTFS", driveType: windows.DRIVE_FIXED, want: "local"},
		{name: "removable", path: `E:\\photos`, filesystem: "exFAT", driveType: windows.DRIVE_REMOVABLE, want: "removable"},
		{name: "network", path: `Z:\\photos`, filesystem: "NTFS", driveType: windows.DRIVE_REMOTE, want: "network"},
		{name: "UNC", path: `\\\\server\\share\\photos`, filesystem: "NTFS", driveType: windows.DRIVE_UNKNOWN, want: "network"},
		{name: "unsupported fixed", path: `D:\\photos`, filesystem: "exFAT", driveType: windows.DRIVE_FIXED, want: "unsupported"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := windowsVolumeKind(test.path, test.filesystem, test.driveType); got != test.want {
				t.Fatalf("volume kind = %q, want %q", got, test.want)
			}
		})
	}
}
