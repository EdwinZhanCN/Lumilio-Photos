//go:build darwin && cgo

package changefeed

/*
#cgo LDFLAGS: -framework CoreServices -framework CoreFoundation
#include <CoreServices/CoreServices.h>
#include <CoreFoundation/CoreFoundation.h>
#include <dispatch/dispatch.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
    char *path;
    uint64_t event_id;
    uint32_t flags;
} lumilio_fsevent;

typedef struct {
    lumilio_fsevent *events;
    size_t capacity;
    size_t count;
    uint64_t through;
    int has_more;
    int overflow;
    int gap;
    uint64_t latest;
    dispatch_semaphore_t history_done;
} lumilio_fsevent_batch;

static void lumilio_fsevent_callback(
    ConstFSEventStreamRef stream,
    void *info,
    size_t count,
    void *event_paths,
    const FSEventStreamEventFlags flags[],
    const FSEventStreamEventId ids[]
) {
    (void)stream;
    lumilio_fsevent_batch *batch = (lumilio_fsevent_batch *)info;
    char **paths = (char **)event_paths;
    for (size_t index = 0; index < count; index++) {
        FSEventStreamEventFlags flag = flags[index];
        if (flag & kFSEventStreamEventFlagHistoryDone) {
            dispatch_semaphore_signal(batch->history_done);
        }
        if (flag & (kFSEventStreamEventFlagUserDropped |
                    kFSEventStreamEventFlagKernelDropped)) {
            batch->overflow = 1;
        }
        if (flag & (kFSEventStreamEventFlagEventIdsWrapped |
                    kFSEventStreamEventFlagRootChanged |
                    kFSEventStreamEventFlagUnmount)) {
            batch->gap = 1;
        }
        if (ids[index] > batch->through ||
            (flag & (kFSEventStreamEventFlagHistoryDone |
                     kFSEventStreamEventFlagMount))) {
            continue;
        }
        if (batch->count >= batch->capacity) {
            batch->has_more = 1;
            continue;
        }
        lumilio_fsevent *event = &batch->events[batch->count++];
        event->path = strdup(paths[index]);
        event->event_id = ids[index];
        event->flags = (uint32_t)flag;
    }
}

static void lumilio_dispatch_barrier(void *unused) {
    (void)unused;
}

static int lumilio_read_fsevents(
    dev_t device,
    const char *root,
    uint64_t after,
    uint64_t through,
    size_t capacity,
    lumilio_fsevent_batch *batch
) {
    memset(batch, 0, sizeof(*batch));
    batch->capacity = capacity;
    batch->through = through;
    batch->history_done = dispatch_semaphore_create(0);
    batch->events = (lumilio_fsevent *)calloc(capacity, sizeof(lumilio_fsevent));
    if (batch->events == NULL) return -1;

    CFStringRef root_string = CFStringCreateWithCString(NULL, root, kCFStringEncodingUTF8);
    if (root_string == NULL) return -2;
    const void *values[] = { root_string };
    CFArrayRef paths = CFArrayCreate(NULL, values, 1, &kCFTypeArrayCallBacks);
    FSEventStreamContext context = {0, batch, NULL, NULL, NULL};
    FSEventStreamRef stream = FSEventStreamCreateRelativeToDevice(
        NULL,
        &lumilio_fsevent_callback,
        &context,
        device,
        paths,
        (FSEventStreamEventId)after,
        0.01,
        kFSEventStreamCreateFlagNoDefer |
        kFSEventStreamCreateFlagWatchRoot |
        kFSEventStreamCreateFlagFileEvents
    );
    CFRelease(paths);
    CFRelease(root_string);
    if (stream == NULL) return -3;

    dispatch_queue_t queue = dispatch_queue_create("photos.lumilio.roe.fsevents", DISPATCH_QUEUE_SERIAL);
    FSEventStreamSetDispatchQueue(stream, queue);
    if (!FSEventStreamStart(stream)) {
        FSEventStreamInvalidate(stream);
        FSEventStreamRelease(stream);
        dispatch_release(queue);
        return -4;
    }
    FSEventStreamFlushSync(stream);
    long history_status = dispatch_semaphore_wait(
        batch->history_done,
        dispatch_time(DISPATCH_TIME_NOW, 2 * NSEC_PER_SEC)
    );
    dispatch_sync_f(queue, NULL, &lumilio_dispatch_barrier);
    batch->latest = (uint64_t)FSEventStreamGetLatestEventId(stream);
    FSEventStreamStop(stream);
    FSEventStreamInvalidate(stream);
    FSEventStreamRelease(stream);
    dispatch_release(queue);
    dispatch_release(batch->history_done);
    batch->history_done = NULL;
    // Advancing through a cursor without the HistoryDone boundary could skip
    // events that the daemon had not replayed yet. Treat a timeout as cursor
    // loss and let the controller require an authoritative verification.
    return history_status == 0 ? 0 : -5;
}

static int lumilio_snapshot_fsevents(
    dev_t device,
    const char *root,
    lumilio_fsevent_batch *batch
) {
    // This time lookup is only a conservative lower bound. The device stream
    // and HistoryDone barrier below advance it through events that have not yet
    // reached the on-disk journal files.
    CFAbsoluteTime now = CFAbsoluteTimeGetCurrent() + kCFAbsoluteTimeIntervalSince1970;
    uint64_t baseline = (uint64_t)FSEventsGetLastEventIdForDeviceBeforeTime(device, now);
    return lumilio_read_fsevents(device, root, baseline, UINT64_MAX, 1, batch);
}

static void lumilio_free_fsevents(lumilio_fsevent_batch *batch) {
    if (batch == NULL || batch->events == NULL) return;
    for (size_t index = 0; index < batch->count; index++) {
        free(batch->events[index].path);
    }
    free(batch->events);
    batch->events = NULL;
}

static char *lumilio_fsevents_device_uuid(dev_t device) {
    CFUUIDRef uuid = FSEventsCopyUUIDForDevice(device);
    if (uuid == NULL) return NULL;
    CFStringRef text = CFUUIDCreateString(NULL, uuid);
    CFRelease(uuid);
    if (text == NULL) return NULL;
    CFIndex length = CFStringGetMaximumSizeForEncoding(CFStringGetLength(text), kCFStringEncodingUTF8) + 1;
    char *result = (char *)calloc((size_t)length, 1);
    if (result != NULL && !CFStringGetCString(text, result, length, kCFStringEncodingUTF8)) {
        free(result);
        result = NULL;
    }
    CFRelease(text);
    return result;
}
*/
import "C"

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/google/uuid"

	"server/internal/db/repo"
)

type fseventsFeed struct {
	mu            sync.Mutex
	repositories  map[uuid.UUID]fseventsWatchedRepository
	notifications chan uuid.UUID
	stop          chan struct{}
	done          chan struct{}
	closeOnce     sync.Once
}

type fseventsWatchedRepository struct {
	repository      repo.Repository
	lastID          uint64
	journalIdentity string
}

type fseventsVolume struct {
	device          C.dev_t
	repositoryRoot  string
	journalIdentity string
	volumeIdentity  string
	volumeKind      string
}

func newNative() Feed {
	feed := &fseventsFeed{
		repositories:  make(map[uuid.UUID]fseventsWatchedRepository),
		notifications: make(chan uuid.UUID, 256), stop: make(chan struct{}), done: make(chan struct{}),
	}
	go feed.notificationLoop()
	return feed
}

func (feed *fseventsFeed) Notifications() <-chan uuid.UUID { return feed.notifications }

func (feed *fseventsFeed) Close() error {
	feed.closeOnce.Do(func() {
		close(feed.stop)
		<-feed.done
		close(feed.notifications)
	})
	return nil
}

func (feed *fseventsFeed) Snapshot(_ context.Context, repository repo.Repository) (Checkpoint, error) {
	volume, err := fseventsVolumeForPath(repository.Path)
	if err != nil {
		volumeIdentity, volumeKind := repositoryVolume(repository)
		return Checkpoint{AdapterKind: "fsevents", VolumeIdentity: volumeIdentity, VolumeKind: volumeKind, Health: HealthUnavailable}, err
	}
	current, err := fseventsSnapshotCursor(volume)
	if err != nil {
		return Checkpoint{
			AdapterKind: "fsevents", VolumeIdentity: volume.volumeIdentity,
			VolumeKind: volume.volumeKind, JournalIdentity: volume.journalIdentity,
			Health: HealthGap,
		}, err
	}
	feed.mu.Lock()
	feed.repositories[repository.RepoID] = fseventsWatchedRepository{
		repository: repository, lastID: current, journalIdentity: volume.journalIdentity,
	}
	feed.mu.Unlock()
	return Checkpoint{
		AdapterKind: "fsevents", Cursor: []byte(strconv.FormatUint(current, 10)),
		VolumeIdentity: volume.volumeIdentity, VolumeKind: volume.volumeKind,
		JournalIdentity: volume.journalIdentity, Health: HealthHealthy,
	}, nil
}

func (*fseventsFeed) Read(
	ctx context.Context,
	repository repo.Repository,
	after Checkpoint,
	through Checkpoint,
	limit int,
) (Batch, error) {
	if err := ctx.Err(); err != nil {
		return Batch{}, err
	}
	if !after.Valid() || !through.Valid() || !after.SameIdentity(through) {
		return Batch{}, ErrCursorInvalid
	}
	afterID, err := strconv.ParseUint(string(after.Cursor), 10, 64)
	if err != nil {
		return Batch{}, ErrCursorInvalid
	}
	throughID, err := strconv.ParseUint(string(through.Cursor), 10, 64)
	if err != nil || throughID < afterID {
		return Batch{}, ErrCursorInvalid
	}
	if afterID == throughID {
		return Batch{Next: through, Done: true}, nil
	}
	volume, err := fseventsVolumeForPath(repository.Path)
	if err != nil || volume.volumeIdentity != after.VolumeIdentity ||
		volume.journalIdentity != after.JournalIdentity {
		return Batch{}, fmt.Errorf("%w: FSEvents device identity changed", ErrCursorInvalid)
	}
	if limit <= 0 {
		limit = 1
	}
	root := C.CString(volume.repositoryRoot)
	defer C.free(unsafe.Pointer(root))
	var native C.lumilio_fsevent_batch
	status := C.lumilio_read_fsevents(
		volume.device, root, C.uint64_t(afterID), C.uint64_t(throughID), C.size_t(limit+1), &native,
	)
	defer C.lumilio_free_fsevents(&native)
	if status != 0 {
		return Batch{}, fmt.Errorf("read FSEvents history: native status %d", int(status))
	}
	if native.overflow != 0 {
		next := through
		next.Health = HealthOverflow
		return Batch{Next: next}, ErrCursorInvalid
	}
	if native.gap != 0 {
		next := through
		next.Health = HealthGap
		return Batch{Next: next}, ErrCursorInvalid
	}

	count := int(native.count)
	if count > limit {
		count = limit
	}
	events := make([]Event, 0, count)
	lastID := afterID
	nativeEvents := unsafe.Slice((*C.lumilio_fsevent)(unsafe.Pointer(native.events)), int(native.count))
	for index := 0; index < count; index++ {
		candidate := nativeEvents[index]
		id := uint64(candidate.event_id)
		lastID = id
		relative, pathErr := fseventsUserPath(repository, volume.repositoryRoot, C.GoString(candidate.path))
		if pathErr != nil {
			// Private .lumilio activity and paths outside the current repository
			// are intentionally absent from the observation stream.
			continue
		}
		flags := uint32(candidate.flags)
		kind := fseventsKind(flags)
		events = append(events, Event{
			Key:  fmt.Sprintf("fsevents:%d:%08x:%s", id, flags, relative),
			Kind: kind, Path: relative,
			Recursive: flags&uint32(C.kFSEventStreamEventFlagMustScanSubDirs) != 0,
			Cursor:    []byte(strconv.FormatUint(id, 10)),
		})
	}
	hasMore := native.has_more != 0 || int(native.count) > limit
	next := through
	if hasMore {
		if lastID == afterID {
			return Batch{}, fmt.Errorf("FSEvents page contained no applicable cursor progress")
		}
		next.Cursor = []byte(strconv.FormatUint(lastID, 10))
	}
	return Batch{Events: events, Next: next, Done: !hasMore}, nil
}

func (feed *fseventsFeed) notificationLoop() {
	defer close(feed.done)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-feed.stop:
			return
		case <-ticker.C:
			feed.mu.Lock()
			watched := make([]fseventsWatchedRepository, 0, len(feed.repositories))
			for _, repository := range feed.repositories {
				watched = append(watched, repository)
			}
			feed.mu.Unlock()
			for _, repository := range watched {
				volume, volumeErr := fseventsVolumeForPath(repository.repository.Path)
				if volumeErr != nil || volume.journalIdentity != repository.journalIdentity {
					select {
					case feed.notifications <- repository.repository.RepoID:
						feed.mu.Lock()
						latest, ok := feed.repositories[repository.repository.RepoID]
						if ok && latest.journalIdentity == repository.journalIdentity {
							delete(feed.repositories, repository.repository.RepoID)
						}
						feed.mu.Unlock()
					default:
					}
					continue
				}
				current, snapshotErr := fseventsSnapshotCursor(volume)
				if snapshotErr != nil {
					select {
					case feed.notifications <- repository.repository.RepoID:
					default:
					}
					continue
				}
				if repository.lastID >= current {
					continue
				}
				hasUserChange, err := fseventsHasUserChange(repository.repository, volume, repository.lastID, current)
				delivered := false
				if err != nil || hasUserChange {
					select {
					case feed.notifications <- repository.repository.RepoID:
						delivered = true
					default:
					}
				} else {
					delivered = true
				}
				if delivered {
					feed.mu.Lock()
					latest, ok := feed.repositories[repository.repository.RepoID]
					if ok && latest.lastID <= repository.lastID {
						latest.lastID = current
						feed.repositories[repository.repository.RepoID] = latest
					}
					feed.mu.Unlock()
				}
			}
		}
	}
}

func fseventsHasUserChange(
	repository repo.Repository,
	volume fseventsVolume,
	afterID, throughID uint64,
) (bool, error) {
	root := C.CString(volume.repositoryRoot)
	defer C.free(unsafe.Pointer(root))
	var native C.lumilio_fsevent_batch
	status := C.lumilio_read_fsevents(
		volume.device, root, C.uint64_t(afterID), C.uint64_t(throughID), 64, &native,
	)
	defer C.lumilio_free_fsevents(&native)
	if status != 0 {
		return false, fmt.Errorf("poll FSEvents history: native status %d", int(status))
	}
	if native.overflow != 0 || native.gap != 0 || native.has_more != 0 {
		return true, nil
	}
	events := unsafe.Slice((*C.lumilio_fsevent)(unsafe.Pointer(native.events)), int(native.count))
	for _, event := range events {
		if _, err := fseventsUserPath(repository, volume.repositoryRoot, C.GoString(event.path)); err == nil {
			return true, nil
		}
	}
	return false, nil
}

func fseventsSnapshotCursor(volume fseventsVolume) (uint64, error) {
	root := C.CString(volume.repositoryRoot)
	defer C.free(unsafe.Pointer(root))
	var native C.lumilio_fsevent_batch
	status := C.lumilio_snapshot_fsevents(volume.device, root, &native)
	defer C.lumilio_free_fsevents(&native)
	if status != 0 {
		return 0, fmt.Errorf("snapshot FSEvents history: native status %d", int(status))
	}
	if native.overflow != 0 || native.gap != 0 {
		return 0, fmt.Errorf("%w: FSEvents reported a snapshot gap", ErrCursorInvalid)
	}
	return uint64(native.latest), nil
}

func fseventsRoot(repositoryPath string) string {
	absolute, err := filepath.Abs(repositoryPath)
	if err != nil {
		absolute = filepath.Clean(repositoryPath)
	}
	evaluated, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return absolute
	}
	return filepath.Clean(evaluated)
}

func fseventsUserPath(repository repo.Repository, repositoryRoot, nativePath string) (string, error) {
	if filepath.IsAbs(nativePath) {
		return relativeUserPath(repository, nativePath)
	}
	cleanedRoot := strings.TrimPrefix(path.Clean(filepath.ToSlash(repositoryRoot)), "/")
	if cleanedRoot == "." {
		cleanedRoot = ""
	}
	cleanedNative := strings.TrimPrefix(path.Clean(filepath.ToSlash(nativePath)), "/")
	relative := cleanedNative
	if cleanedRoot != "" {
		var err error
		relative, err = filepath.Rel(cleanedRoot, cleanedNative)
		if err != nil || relative == ".." || strings.HasPrefix(relative, "../") {
			return "", fmt.Errorf("FSEvents path %q is outside repository", nativePath)
		}
	}
	if relative == "." || relative == "" {
		return "", nil
	}
	if !validUserRelativePath(relative) {
		return "", fmt.Errorf("FSEvents path %q is not portable user media", nativePath)
	}
	return relative, nil
}

func fseventsKind(flags uint32) EventKind {
	switch {
	case flags&uint32(C.kFSEventStreamEventFlagItemRemoved) != 0:
		return EventRemove
	case flags&uint32(C.kFSEventStreamEventFlagItemRenamed) != 0:
		return EventRename
	case flags&uint32(C.kFSEventStreamEventFlagItemCreated) != 0:
		return EventCreate
	default:
		return EventModify
	}
}

func fseventsJournalIdentityForDevice(device C.dev_t) (string, error) {
	value := C.lumilio_fsevents_device_uuid(device)
	if value == nil {
		return "", fmt.Errorf("FSEvents journal identity is unavailable for device %d", uint64(device))
	}
	defer C.free(unsafe.Pointer(value))
	return strings.ToLower(C.GoString(value)), nil
}

func fseventsVolumeForPath(repositoryPath string) (fseventsVolume, error) {
	root := fseventsRoot(repositoryPath)
	info, err := os.Stat(root)
	if err != nil {
		return fseventsVolume{volumeKind: "unknown"}, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fseventsVolume{volumeKind: "unknown"}, fmt.Errorf("repository stat has no Darwin device identity")
	}
	volume := fseventsVolume{device: C.dev_t(stat.Dev), volumeKind: "local"}
	var filesystem syscall.Statfs_t
	if err := syscall.Statfs(root, &filesystem); err != nil {
		return volume, err
	}
	mountPath := darwinStatfsString(filesystem.Mntonname[:])
	filesystemKind := strings.ToLower(darwinStatfsString(filesystem.Fstypename[:]))
	switch filesystemKind {
	case "afpfs", "nfs", "smbfs", "webdav":
		volume.volumeKind = "network"
	default:
		if strings.HasPrefix(strings.ToLower(filepath.Clean(mountPath)), "/volumes/") {
			volume.volumeKind = "removable"
		}
	}
	volume.repositoryRoot, err = fseventsRelativeRoot(root, mountPath)
	if err != nil {
		return volume, err
	}
	volume.journalIdentity, err = fseventsJournalIdentityForDevice(volume.device)
	if err != nil {
		return volume, err
	}
	volume.volumeIdentity = "darwin-volume:" + volume.journalIdentity
	return volume, nil
}

func fseventsRelativeRoot(repositoryPath, mountPath string) (string, error) {
	relative, err := filepath.Rel(filepath.Clean(mountPath), filepath.Clean(repositoryPath))
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		if relative == "." {
			return "", nil
		}
		return filepath.ToSlash(relative), nil
	}
	// APFS firmlinks expose Data-volume paths (for example /Users and
	// /private) in the root namespace even though statfs reports the underlying
	// /System/Volumes/Data mount. FSEvents addresses those paths from the Data
	// volume root, so their root-relative spelling is the absolute path without
	// its leading separator.
	if filepath.Clean(mountPath) == "/System/Volumes/Data" && filepath.IsAbs(repositoryPath) {
		return strings.TrimPrefix(filepath.ToSlash(filepath.Clean(repositoryPath)), "/"), nil
	}
	return "", fmt.Errorf("repository %q is outside mounted volume %q", repositoryPath, mountPath)
}

func darwinStatfsString(value []int8) string {
	bytes := make([]byte, 0, len(value))
	for _, character := range value {
		if character == 0 {
			break
		}
		bytes = append(bytes, byte(character))
	}
	return string(bytes)
}

func platformRepositoryVolume(repositoryPath string) (string, string, error) {
	volume, err := fseventsVolumeForPath(repositoryPath)
	return volume.volumeIdentity, volume.volumeKind, err
}
