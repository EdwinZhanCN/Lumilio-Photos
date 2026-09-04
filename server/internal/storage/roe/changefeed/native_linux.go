//go:build linux

package changefeed

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/google/uuid"
	"golang.org/x/sys/unix"

	"server/internal/db/repo"
)

const inotifyEventHistory = 65536
const inotifyPendingMoveLimit = 1024

const inotifyMutationMask = uint32(unix.IN_CREATE | unix.IN_MODIFY | unix.IN_CLOSE_WRITE |
	unix.IN_DELETE | unix.IN_MOVED_FROM | unix.IN_MOVED_TO | unix.IN_DELETE_SELF |
	unix.IN_MOVE_SELF | unix.IN_UNMOUNT)

type inotifyFeed struct {
	mu            sync.Mutex
	sessions      map[uuid.UUID]*inotifySession
	notifications chan uuid.UUID
	closed        bool
}

type inotifySession struct {
	mu             sync.Mutex
	repositoryID   uuid.UUID
	repositoryPath string
	fd             int
	instanceID     string
	volumeIdentity string
	volumeKind     string
	sequence       uint64
	oldestSequence uint64
	events         []sequencedEvent
	watches        map[int]string
	watchByPath    map[string]int
	pendingMoves   map[uint32]string
	overflow       bool
	closed         bool
}

func newNative() Feed {
	return &inotifyFeed{
		sessions:      make(map[uuid.UUID]*inotifySession),
		notifications: make(chan uuid.UUID, 256),
	}
}

func (feed *inotifyFeed) Notifications() <-chan uuid.UUID { return feed.notifications }

func (feed *inotifyFeed) Snapshot(_ context.Context, repository repo.Repository) (Checkpoint, error) {
	volumeIdentity, volumeKind := repositoryVolume(repository)
	feed.mu.Lock()
	defer feed.mu.Unlock()
	if feed.closed {
		return Checkpoint{}, errors.New("inotify feed is closed")
	}
	session := feed.sessions[repository.RepoID]
	if session != nil {
		session.mu.Lock()
		reset := session.closed || session.overflow || session.repositoryPath != repository.Path ||
			session.volumeIdentity != volumeIdentity
		session.mu.Unlock()
		if reset {
			feed.closeSessionLocked(session)
			delete(feed.sessions, repository.RepoID)
			session = nil
		}
	}
	if session == nil {
		var err error
		session, err = feed.startSessionLocked(repository, volumeIdentity, volumeKind)
		if err != nil {
			return Checkpoint{
				AdapterKind: "inotify", VolumeIdentity: volumeIdentity,
				VolumeKind: volumeKind, Health: HealthUnavailable,
			}, err
		}
		feed.sessions[repository.RepoID] = session
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.checkpoint(), nil
}

func (feed *inotifyFeed) Read(
	_ context.Context,
	repository repo.Repository,
	after Checkpoint,
	through Checkpoint,
	limit int,
) (Batch, error) {
	if !after.Valid() || !through.Valid() || !after.SameIdentity(through) {
		return Batch{}, ErrCursorInvalid
	}
	fromSequence, err := strconv.ParseUint(string(after.Cursor), 10, 64)
	if err != nil {
		return Batch{}, ErrCursorInvalid
	}
	throughSequence, err := strconv.ParseUint(string(through.Cursor), 10, 64)
	if err != nil || throughSequence < fromSequence {
		return Batch{}, ErrCursorInvalid
	}
	feed.mu.Lock()
	session := feed.sessions[repository.RepoID]
	feed.mu.Unlock()
	if session == nil {
		return Batch{}, ErrCursorInvalid
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed || session.overflow || session.instanceID != after.JournalIdentity ||
		session.volumeIdentity != after.VolumeIdentity || fromSequence+1 < session.oldestSequence {
		return Batch{}, ErrCursorInvalid
	}
	if limit <= 0 {
		limit = 1
	}
	events := make([]Event, 0, limit)
	nextSequence := fromSequence
	remaining := false
	for _, candidate := range session.events {
		if candidate.sequence <= fromSequence || candidate.sequence > throughSequence {
			continue
		}
		if len(events) == limit {
			remaining = true
			break
		}
		event := candidate.event
		event.Cursor = []byte(strconv.FormatUint(candidate.sequence, 10))
		events = append(events, event)
		nextSequence = candidate.sequence
	}
	next := through
	if remaining {
		next.Cursor = []byte(strconv.FormatUint(nextSequence, 10))
	}
	return Batch{Events: events, Next: next, Done: !remaining}, nil
}

func (feed *inotifyFeed) WatchDirectory(_ context.Context, repository repo.Repository, relativePath string) error {
	if _, err := feed.Snapshot(context.Background(), repository); err != nil {
		return err
	}
	feed.mu.Lock()
	session := feed.sessions[repository.RepoID]
	feed.mu.Unlock()
	if session == nil {
		return errors.New("inotify session disappeared")
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.addWatch(relativePath)
}

func (feed *inotifyFeed) Close() error {
	feed.mu.Lock()
	defer feed.mu.Unlock()
	if feed.closed {
		return nil
	}
	feed.closed = true
	var result error
	for _, session := range feed.sessions {
		if err := feed.closeSessionLocked(session); err != nil {
			result = errors.Join(result, err)
		}
	}
	feed.sessions = nil
	close(feed.notifications)
	return result
}

func (feed *inotifyFeed) startSessionLocked(
	repository repo.Repository,
	volumeIdentity, volumeKind string,
) (*inotifySession, error) {
	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
	if err != nil {
		return nil, fmt.Errorf("initialize inotify: %w", err)
	}
	session := &inotifySession{
		repositoryID: repository.RepoID, repositoryPath: repository.Path,
		fd: fd, instanceID: uuid.NewString(), volumeIdentity: volumeIdentity, volumeKind: volumeKind,
		oldestSequence: 1, watches: make(map[int]string), watchByPath: make(map[string]int),
		pendingMoves: make(map[uint32]string),
	}
	if err := session.addWatch(""); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	go feed.readLoop(session)
	return session, nil
}

func (feed *inotifyFeed) closeSessionLocked(session *inotifySession) error {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed {
		return nil
	}
	session.closed = true
	if err := unix.Close(session.fd); err != nil && !errors.Is(err, unix.EBADF) {
		return err
	}
	return nil
}

func (feed *inotifyFeed) readLoop(session *inotifySession) {
	buffer := make([]byte, 256*1024)
	for {
		poll := []unix.PollFd{{Fd: int32(session.fd), Events: unix.POLLIN}}
		_, err := unix.Poll(poll, 500)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			session.mu.Lock()
			feed.invalidateSessionLocked(session)
			session.mu.Unlock()
			return
		}
		session.mu.Lock()
		if session.closed {
			session.mu.Unlock()
			return
		}
		if poll[0].Revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
			feed.invalidateSessionLocked(session)
			session.mu.Unlock()
			return
		}
		if poll[0].Revents&unix.POLLIN == 0 {
			session.mu.Unlock()
			continue
		}
		count, readErr := unix.Read(session.fd, buffer)
		if readErr != nil {
			if errors.Is(readErr, unix.EAGAIN) || errors.Is(readErr, unix.EINTR) {
				session.mu.Unlock()
				continue
			}
			feed.invalidateSessionLocked(session)
			session.mu.Unlock()
			return
		}
		feed.consumeLocked(session, buffer[:count])
		session.mu.Unlock()
	}
}

func (feed *inotifyFeed) consumeLocked(session *inotifySession, data []byte) {
	for offset := 0; offset+unix.SizeofInotifyEvent <= len(data); {
		native := (*unix.InotifyEvent)(unsafe.Pointer(&data[offset]))
		recordSize := unix.SizeofInotifyEvent + int(native.Len)
		if recordSize <= 0 || offset+recordSize > len(data) {
			feed.invalidateSessionLocked(session)
			return
		}
		offset += recordSize
		if native.Mask&unix.IN_Q_OVERFLOW != 0 {
			feed.invalidateSessionLocked(session)
			continue
		}
		// Attribute-only notifications include access-time changes caused by
		// Lumilio reading directories and originals. Treating those reads as
		// mutations creates a self-sustaining observe/hash/observe loop. Content,
		// topology, and close-after-write signals remain authoritative hints.
		if native.Mask&inotifyMutationMask == 0 && native.Mask&unix.IN_IGNORED == 0 {
			continue
		}
		base, ok := session.watches[int(native.Wd)]
		if !ok {
			continue
		}
		nameBytes := data[offset-recordSize+unix.SizeofInotifyEvent : offset]
		name := string(bytes.TrimRight(nameBytes, "\x00"))
		relative := base
		if name != "" {
			relative = filepath.ToSlash(filepath.Join(base, name))
		}
		absolute := filepath.Join(session.repositoryPath, filepath.FromSlash(relative))
		userPath, err := relativeUserPath(repo.Repository{RepoID: session.repositoryID, Path: session.repositoryPath}, absolute)
		if err != nil {
			continue
		}
		oldPath := ""
		if native.Mask&unix.IN_MOVED_FROM != 0 && native.Cookie != 0 {
			if len(session.pendingMoves) >= inotifyPendingMoveLimit {
				// Pairing is only an optimization: the remove and create events
				// already dirty both parents. Bound unmatched move cookies so a
				// long-lived watcher cannot accumulate state indefinitely.
				clear(session.pendingMoves)
			}
			session.pendingMoves[native.Cookie] = userPath
		}
		if native.Mask&unix.IN_MOVED_TO != 0 && native.Cookie != 0 {
			oldPath = session.pendingMoves[native.Cookie]
			delete(session.pendingMoves, native.Cookie)
			if oldPath != "" {
				session.rewriteWatchPaths(oldPath, userPath)
			}
		}
		if native.Mask&unix.IN_IGNORED != 0 {
			delete(session.watchByPath, base)
			delete(session.watches, int(native.Wd))
			continue
		}
		kind := inotifyKind(native.Mask)
		session.sequence++
		event := Event{
			Key:  fmt.Sprintf("inotify:%s:%d:%d:%08x:%s", session.instanceID, session.sequence, native.Cookie, native.Mask, userPath),
			Kind: kind, Path: userPath, OldPath: oldPath,
		}
		session.events = append(session.events, sequencedEvent{sequence: session.sequence, event: event})
		if len(session.events) > inotifyEventHistory {
			dropped := session.events[0].sequence
			session.events = append(session.events[:0], session.events[1:]...)
			session.oldestSequence = dropped + 1
		}
		select {
		case feed.notifications <- session.repositoryID:
		default:
		}
		if native.Mask&(unix.IN_DELETE_SELF|unix.IN_MOVE_SELF|unix.IN_UNMOUNT) != 0 && base == "" {
			feed.invalidateSessionLocked(session)
		}
	}
}

func (feed *inotifyFeed) invalidateSessionLocked(session *inotifySession) {
	if session.closed {
		return
	}
	session.overflow = true
	select {
	case feed.notifications <- session.repositoryID:
	default:
	}
}

func (session *inotifySession) addWatch(relativePath string) error {
	relativePath = strings.Trim(strings.TrimSpace(filepath.ToSlash(relativePath)), "/")
	absolute := filepath.Join(session.repositoryPath, filepath.FromSlash(relativePath))
	mask := inotifyMutationMask | unix.IN_ONLYDIR | unix.IN_DONT_FOLLOW | unix.IN_EXCL_UNLINK
	wd, err := unix.InotifyAddWatch(session.fd, absolute, mask)
	if err != nil {
		return fmt.Errorf("watch repository directory %q: %w", relativePath, err)
	}
	for existingPath, existingWD := range session.watchByPath {
		if existingWD == wd && existingPath != relativePath {
			delete(session.watchByPath, existingPath)
		}
	}
	session.watches[wd] = relativePath
	session.watchByPath[relativePath] = wd
	return nil
}

func (session *inotifySession) rewriteWatchPaths(oldPrefix, newPrefix string) {
	for wd, current := range session.watches {
		if current != oldPrefix && !strings.HasPrefix(current, oldPrefix+"/") {
			continue
		}
		suffix := strings.TrimPrefix(current, oldPrefix)
		updated := strings.TrimPrefix(newPrefix+suffix, "/")
		delete(session.watchByPath, current)
		session.watches[wd] = updated
		session.watchByPath[updated] = wd
	}
}

func (session *inotifySession) checkpoint() Checkpoint {
	health := HealthHealthy
	if session.overflow {
		health = HealthOverflow
	}
	return Checkpoint{
		AdapterKind: "inotify", Cursor: []byte(strconv.FormatUint(session.sequence, 10)),
		VolumeIdentity: session.volumeIdentity, VolumeKind: session.volumeKind,
		JournalIdentity: session.instanceID, Health: health,
	}
}

func inotifyKind(mask uint32) EventKind {
	switch {
	case mask&(unix.IN_DELETE|unix.IN_DELETE_SELF|unix.IN_MOVED_FROM) != 0:
		return EventRemove
	case mask&unix.IN_MOVED_TO != 0:
		return EventRename
	case mask&unix.IN_CREATE != 0:
		return EventCreate
	default:
		return EventModify
	}
}

func platformRepositoryVolume(repositoryPath string) (string, string, error) {
	var stat unix.Stat_t
	if err := unix.Stat(repositoryPath, &stat); err != nil {
		return "", "unknown", err
	}
	var statfs unix.Statfs_t
	if err := unix.Statfs(repositoryPath, &statfs); err != nil {
		return "", "unknown", err
	}
	kind := "local"
	switch int64(statfs.Type) {
	case unix.NFS_SUPER_MAGIC, 0xff534d42: // CIFS_MAGIC_NUMBER
		kind = "network"
	}
	cleaned := strings.ToLower(filepath.Clean(repositoryPath))
	if strings.HasPrefix(cleaned, "/media/") || strings.HasPrefix(cleaned, "/run/media/") {
		kind = "removable"
	}
	return fmt.Sprintf("linux-device:%d:fs:%x", uint64(stat.Dev), uint64(statfs.Type)), kind, nil
}
