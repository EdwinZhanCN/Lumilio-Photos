//go:build linux

package changefeed

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"github.com/google/uuid"
	"golang.org/x/sys/unix"

	"server/internal/db/repo"
)

func TestInotifyCapturesAfterWatchAndFailsClosedOnOverflowAndFreshInstance(t *testing.T) {
	repository := repo.Repository{RepoID: uuid.New(), Path: t.TempDir()}
	feed := newNative().(*inotifyFeed)
	t.Cleanup(func() { _ = feed.Close() })
	start, err := feed.Snapshot(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(repository.Path, "album")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	// The root watch reports the directory creation independently. Advance past
	// it before installing the child watch so its earlier notification cannot be
	// mistaken for the later photo write.
	if _, err := waitForInotifyPath(context.Background(), feed, repository, start, "album"); err != nil {
		t.Fatal(err)
	}
	if err := feed.WatchDirectory(context.Background(), repository, "album"); err != nil {
		t.Fatal(err)
	}
	beforeWrite, err := feed.Snapshot(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "photo.jpg"), []byte("photo"), 0o600); err != nil {
		t.Fatal(err)
	}
	through, err := waitForInotifyPath(context.Background(), feed, repository, beforeWrite, "album/photo.jpg")
	if err != nil {
		t.Fatal(err)
	}

	feed.mu.Lock()
	session := feed.sessions[repository.RepoID]
	feed.mu.Unlock()
	session.mu.Lock()
	session.overflow = true
	session.mu.Unlock()
	if _, err := feed.Read(context.Background(), repository, start, through, 32); !errors.Is(err, ErrCursorInvalid) {
		t.Fatalf("overflow read error = %v", err)
	}
	if err := feed.Close(); err != nil {
		t.Fatal(err)
	}

	restarted := newNative().(*inotifyFeed)
	t.Cleanup(func() { _ = restarted.Close() })
	fresh, err := restarted.Snapshot(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.JournalIdentity == start.JournalIdentity {
		t.Fatal("fresh inotify instance reused the prior ephemeral cursor identity")
	}
	if _, err := restarted.Read(context.Background(), repository, start, fresh, 32); !errors.Is(err, ErrCursorInvalid) {
		t.Fatalf("fresh-instance read error = %v", err)
	}
}

func waitForInotifyPath(
	ctx context.Context,
	feed *inotifyFeed,
	repository repo.Repository,
	after Checkpoint,
	wantPath string,
) (Checkpoint, error) {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		through, err := feed.Snapshot(ctx, repository)
		if err != nil {
			return Checkpoint{}, err
		}
		if !after.SamePosition(through) {
			batch, err := feed.Read(ctx, repository, after, through, 32)
			if err != nil {
				return Checkpoint{}, err
			}
			for _, event := range batch.Events {
				if event.Path == wantPath {
					return batch.Next, nil
				}
			}
			after = batch.Next
			continue
		}
		select {
		case <-feed.Notifications():
		case <-time.After(20 * time.Millisecond):
		}
	}
	return Checkpoint{}, fmt.Errorf("inotify did not report %q before timeout", wantPath)
}

func TestInotifyNativeOverflowRecordInvalidatesCursorAndWakesController(t *testing.T) {
	repositoryID := uuid.New()
	feed := &inotifyFeed{notifications: make(chan uuid.UUID, 1)}
	session := &inotifySession{repositoryID: repositoryID}
	data := make([]byte, unix.SizeofInotifyEvent)
	record := (*unix.InotifyEvent)(unsafe.Pointer(&data[0]))
	record.Wd = -1
	record.Mask = unix.IN_Q_OVERFLOW

	feed.consumeLocked(session, data)
	if !session.overflow {
		t.Fatal("kernel overflow record left inotify cursor healthy")
	}
	select {
	case notified := <-feed.notifications:
		if notified != repositoryID {
			t.Fatalf("overflow notification = %s, want %s", notified, repositoryID)
		}
	default:
		t.Fatal("kernel overflow record did not wake controller")
	}
}

func TestInotifyIgnoresAttributeOnlyNoise(t *testing.T) {
	repositoryID := uuid.New()
	feed := &inotifyFeed{notifications: make(chan uuid.UUID, 1)}
	session := &inotifySession{
		repositoryID: repositoryID, repositoryPath: t.TempDir(),
		watches: map[int]string{1: ""},
	}
	data := make([]byte, unix.SizeofInotifyEvent)
	record := (*unix.InotifyEvent)(unsafe.Pointer(&data[0]))
	record.Wd = 1
	record.Mask = unix.IN_ATTRIB

	feed.consumeLocked(session, data)
	if session.sequence != 0 || len(session.events) != 0 {
		t.Fatalf("attribute-only event advanced feed: sequence=%d events=%+v", session.sequence, session.events)
	}
	select {
	case notified := <-feed.notifications:
		t.Fatalf("attribute-only event notified repository %s", notified)
	default:
	}
}

func TestInotifyIgnoresLumilioStorageProbeFiles(t *testing.T) {
	repository := repo.Repository{RepoID: uuid.New(), Path: t.TempDir()}
	feed := newNative().(*inotifyFeed)
	t.Cleanup(func() { _ = feed.Close() })
	start, err := feed.Snapshot(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}

	for _, pattern := range []string{".lumilio_permission_test-*", ".lumilio_case_probe-a*"} {
		probe, createErr := os.CreateTemp(repository.Path, pattern)
		if createErr != nil {
			t.Fatal(createErr)
		}
		name := probe.Name()
		if closeErr := probe.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		if removeErr := os.Remove(name); removeErr != nil {
			t.Fatal(removeErr)
		}
	}

	select {
	case notified := <-feed.Notifications():
		t.Fatalf("Lumilio storage probe notified repository %s", notified)
	case <-time.After(time.Second):
	}
	through, err := feed.Snapshot(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}
	if !start.SamePosition(through) {
		t.Fatalf("Lumilio storage probes advanced inotify cursor from %q to %q", start.Cursor, through.Cursor)
	}
}
