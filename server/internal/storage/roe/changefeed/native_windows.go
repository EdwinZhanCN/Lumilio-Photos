//go:build windows

package changefeed

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/google/uuid"
	"golang.org/x/sys/windows"

	"server/internal/db/repo"
)

const (
	fsctlQueryUSNJournal = 0x000900f4
	fsctlReadUSNJournal  = 0x000900bb

	usnReasonDataOverwrite      = 0x00000001
	usnReasonDataExtend         = 0x00000002
	usnReasonDataTruncation     = 0x00000004
	usnReasonNamedDataOverwrite = 0x00000010
	usnReasonNamedDataExtend    = 0x00000020
	usnReasonNamedDataTruncate  = 0x00000040
	usnReasonFileCreate         = 0x00000100
	usnReasonFileDelete         = 0x00000200
	usnReasonEAChange           = 0x00000400
	usnReasonSecurityChange     = 0x00000800
	usnReasonRenameOldName      = 0x00001000
	usnReasonRenameNewName      = 0x00002000
	usnReasonIndexableChange    = 0x00004000
	usnReasonBasicInfoChange    = 0x00008000
	usnReasonHardLinkChange     = 0x00010000
	usnReasonCompressionChange  = 0x00020000
	usnReasonEncryptionChange   = 0x00040000
	usnReasonObjectIDChange     = 0x00080000
	usnReasonReparsePointChange = 0x00100000
	usnReasonStreamChange       = 0x00200000

	fileIDType                  = 0
	fileNameNormalizedVolumeDOS = 0
	rdcwHistory                 = 65536
)

var openFileByID = windows.NewLazySystemDLL("kernel32.dll").NewProc("OpenFileById")

type windowsFeed struct {
	mu            sync.Mutex
	sessions      map[uuid.UUID]*rdcwSession
	notifications chan uuid.UUID
	closed        bool
}

type rdcwSession struct {
	mu             sync.Mutex
	repositoryID   uuid.UUID
	repositoryPath string
	handle         windows.Handle
	instanceID     string
	volumeIdentity string
	volumeKind     string
	sequence       uint64
	oldestSequence uint64
	events         []sequencedEvent
	overflow       bool
	closed         bool
}

type usnJournalData struct {
	JournalID       uint64
	FirstUSN        int64
	NextUSN         int64
	LowestValidUSN  int64
	MaxUSN          int64
	MaximumSize     uint64
	AllocationDelta uint64
}

type readUSNJournalData struct {
	StartUSN          int64
	ReasonMask        uint32
	ReturnOnlyOnClose uint32
	Timeout           uint64
	BytesToWaitFor    uint64
	JournalID         uint64
}

type fileIDDescriptor struct {
	Size uint32
	Type uint32
	ID   int64
}

type usnRecord struct {
	Length              uint32
	MajorVersion        uint16
	MinorVersion        uint16
	FileReference       uint64
	ParentFileReference uint64
	USN                 int64
	Timestamp           int64
	Reason              uint32
	SourceInfo          uint32
	SecurityID          uint32
	FileAttributes      uint32
	FileNameLength      uint16
	FileNameOffset      uint16
}

func newNative() Feed {
	return &windowsFeed{sessions: make(map[uuid.UUID]*rdcwSession), notifications: make(chan uuid.UUID, 256)}
}

func (feed *windowsFeed) Notifications() <-chan uuid.UUID { return feed.notifications }

func (feed *windowsFeed) Snapshot(ctx context.Context, repository repo.Repository) (Checkpoint, error) {
	if checkpoint, err := usnSnapshot(repository); err == nil {
		// RDCW remains active as the low-latency wakeup source; USN is the
		// persisted replay/cursor authority.
		_ = feed.ensureRDCW(repository)
		return checkpoint, nil
	}
	return feed.rdcwSnapshot(ctx, repository)
}

func (feed *windowsFeed) Read(
	ctx context.Context,
	repository repo.Repository,
	after Checkpoint,
	through Checkpoint,
	limit int,
) (Batch, error) {
	switch after.AdapterKind {
	case "usn":
		return readUSN(ctx, repository, after, through, limit)
	case "rdcw":
		return feed.readRDCW(repository, after, through, limit)
	default:
		return Batch{}, ErrCursorInvalid
	}
}

func (feed *windowsFeed) WatchDirectory(context.Context, repo.Repository, string) error {
	// ReadDirectoryChangesW watches the complete subtree from one handle; USN
	// replay is volume-journal based. Neither needs per-directory registration.
	return nil
}

func (feed *windowsFeed) Close() error {
	feed.mu.Lock()
	defer feed.mu.Unlock()
	if feed.closed {
		return nil
	}
	feed.closed = true
	var result error
	for _, session := range feed.sessions {
		session.mu.Lock()
		if !session.closed {
			session.closed = true
			if err := windows.CloseHandle(session.handle); err != nil {
				result = errors.Join(result, err)
			}
		}
		session.mu.Unlock()
	}
	feed.sessions = nil
	close(feed.notifications)
	return result
}

func usnSnapshot(repository repo.Repository) (Checkpoint, error) {
	volume, err := openRepositoryVolume(repository.Path)
	if err != nil {
		return Checkpoint{}, err
	}
	defer windows.CloseHandle(volume.handle)
	journal, err := queryUSNJournal(volume.handle)
	if err != nil {
		return Checkpoint{}, err
	}
	return Checkpoint{
		AdapterKind: "usn", Cursor: []byte(strconv.FormatInt(journal.NextUSN, 10)),
		VolumeIdentity: volume.identity, VolumeKind: volume.kind,
		JournalIdentity: fmt.Sprintf("%016x", journal.JournalID), Health: HealthHealthy,
	}, nil
}

func readUSN(
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
	startUSN, err := strconv.ParseInt(string(after.Cursor), 10, 64)
	if err != nil {
		return Batch{}, ErrCursorInvalid
	}
	throughUSN, err := strconv.ParseInt(string(through.Cursor), 10, 64)
	if err != nil || throughUSN < startUSN {
		return Batch{}, ErrCursorInvalid
	}
	journalID, err := strconv.ParseUint(after.JournalIdentity, 16, 64)
	if err != nil {
		return Batch{}, ErrCursorInvalid
	}
	volume, err := openRepositoryVolume(repository.Path)
	if err != nil {
		return Batch{}, err
	}
	defer windows.CloseHandle(volume.handle)
	journal, err := queryUSNJournal(volume.handle)
	if err != nil || journal.JournalID != journalID || startUSN < journal.LowestValidUSN {
		return Batch{}, ErrCursorInvalid
	}
	if startUSN == throughUSN {
		return Batch{Next: through, Done: true}, nil
	}
	if limit <= 0 {
		limit = 1
	}
	input := readUSNJournalData{StartUSN: startUSN, ReasonMask: 0xffffffff, JournalID: journalID}
	buffer := make([]byte, 1024*1024)
	var returned uint32
	err = windows.DeviceIoControl(
		volume.handle, fsctlReadUSNJournal,
		(*byte)(unsafe.Pointer(&input)), uint32(unsafe.Sizeof(input)),
		&buffer[0], uint32(len(buffer)), &returned, nil,
	)
	if err != nil {
		return Batch{}, fmt.Errorf("read NTFS USN journal: %w", err)
	}
	if returned < 8 {
		return Batch{}, ErrCursorInvalid
	}
	reportedNext := int64(binary.LittleEndian.Uint64(buffer[:8]))
	if reportedNext > throughUSN {
		reportedNext = throughUSN
	}
	events := make([]Event, 0, limit)
	nextUSN := startUSN
	oldNames := make(map[uint64]string)
	for offset := 8; offset+int(unsafe.Sizeof(usnRecord{})) <= int(returned); {
		record := (*usnRecord)(unsafe.Pointer(&buffer[offset]))
		if record.Length < uint32(unsafe.Sizeof(usnRecord{})) || offset+int(record.Length) > int(returned) {
			return Batch{}, ErrCursorInvalid
		}
		if record.MajorVersion != 2 {
			offset += int(record.Length)
			nextUSN = max(nextUSN, record.USN+1)
			continue
		}
		if record.USN >= throughUSN || len(events) == limit {
			break
		}
		nameStart := offset + int(record.FileNameOffset)
		nameEnd := nameStart + int(record.FileNameLength)
		if nameStart < offset || nameEnd > offset+int(record.Length) || record.FileNameLength%2 != 0 {
			return Batch{}, ErrCursorInvalid
		}
		nameUnits := unsafe.Slice((*uint16)(unsafe.Pointer(&buffer[nameStart])), int(record.FileNameLength)/2)
		name := string(utf16.Decode(nameUnits))
		parentPath, pathErr := pathForFileID(volume.handle, record.ParentFileReference)
		relative := ""
		if pathErr == nil {
			var relevant bool
			relative, relevant = resolvedUSNUserPath(repository, parentPath, name)
			if !relevant {
				// The USN journal is volume-wide. A successfully resolved path
				// outside this repository (including its private control tree) is
				// irrelevant and must not turn every volume event into a root scan.
				offset += int(record.Length)
				nextUSN = max(nextUSN, record.USN+1)
				continue
			}
		}
		if pathErr != nil {
			// A deleted or inaccessible parent cannot be safely classified as
			// outside this repository. A recursive root verifier preserves
			// correctness without interpreting the unresolved native identity.
			relative = ""
		}
		oldPath := ""
		if record.Reason&usnReasonRenameOldName != 0 {
			oldNames[record.FileReference] = relative
		}
		if record.Reason&usnReasonRenameNewName != 0 {
			oldPath = oldNames[record.FileReference]
		}
		kind := usnKind(record.Reason)
		events = append(events, Event{
			Key:  fmt.Sprintf("usn:%016x:%d:%d:%08x", journalID, record.USN, record.FileReference, record.Reason),
			Kind: kind, Path: relative, OldPath: oldPath, Recursive: pathErr != nil,
			Cursor: []byte(strconv.FormatInt(record.USN+1, 10)),
		})
		nextUSN = record.USN + 1
		offset += int(record.Length)
	}
	if len(events) == 0 {
		nextUSN = max(nextUSN, reportedNext)
	}
	if nextUSN > throughUSN {
		nextUSN = throughUSN
	}
	next := through
	done := nextUSN >= throughUSN
	if !done {
		next.Cursor = []byte(strconv.FormatInt(nextUSN, 10))
	}
	return Batch{Events: events, Next: next, Done: done}, nil
}

func resolvedUSNUserPath(repository repo.Repository, parentPath, name string) (string, bool) {
	relative, err := relativeUserPath(repository, filepath.Join(parentPath, name))
	return relative, err == nil
}

type openedVolume struct {
	handle   windows.Handle
	root     string
	identity string
	kind     string
}

func openRepositoryVolume(repositoryPath string) (openedVolume, error) {
	root, volumeName, serial, filesystem, driveType, err := repositoryVolumeInfo(repositoryPath)
	if err != nil {
		return openedVolume{}, err
	}
	openName := strings.TrimSuffix(volumeName, `\`)
	pointer, err := windows.UTF16PtrFromString(openName)
	if err != nil {
		return openedVolume{}, err
	}
	handle, err := windows.CreateFile(pointer, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return openedVolume{}, fmt.Errorf("open repository volume for USN: %w", err)
	}
	kind := windowsVolumeKind(repositoryPath, filesystem, driveType)
	return openedVolume{
		handle: handle, root: root,
		identity: fmt.Sprintf("windows-volume:%s:%08x", strings.ToLower(volumeName), serial), kind: kind,
	}, nil
}

func queryUSNJournal(handle windows.Handle) (usnJournalData, error) {
	var journal usnJournalData
	var returned uint32
	err := windows.DeviceIoControl(handle, fsctlQueryUSNJournal, nil, 0,
		(*byte)(unsafe.Pointer(&journal)), uint32(unsafe.Sizeof(journal)), &returned, nil)
	if err != nil {
		return usnJournalData{}, fmt.Errorf("query NTFS USN journal: %w", err)
	}
	if returned < uint32(unsafe.Sizeof(journal)) {
		return usnJournalData{}, ErrCursorInvalid
	}
	return journal, nil
}

func pathForFileID(volume windows.Handle, id uint64) (string, error) {
	descriptor := fileIDDescriptor{Size: uint32(unsafe.Sizeof(fileIDDescriptor{})), Type: fileIDType, ID: int64(id)}
	handleValue, _, callErr := openFileByID.Call(
		uintptr(volume), uintptr(unsafe.Pointer(&descriptor)), windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		0, windows.FILE_FLAG_BACKUP_SEMANTICS,
	)
	if handleValue == uintptr(windows.InvalidHandle) || handleValue == 0 {
		if callErr != nil {
			return "", callErr
		}
		return "", syscall.EINVAL
	}
	handle := windows.Handle(handleValue)
	defer windows.CloseHandle(handle)
	buffer := make([]uint16, 32768)
	length, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), fileNameNormalizedVolumeDOS)
	if err != nil {
		return "", err
	}
	if length == 0 || int(length) >= len(buffer) {
		return "", syscall.ENAMETOOLONG
	}
	value := windows.UTF16ToString(buffer[:length])
	value = strings.TrimPrefix(value, `\\?\`)
	return value, nil
}

func usnKind(reason uint32) EventKind {
	switch {
	case reason&usnReasonFileDelete != 0 || reason&usnReasonRenameOldName != 0:
		return EventRemove
	case reason&usnReasonRenameNewName != 0:
		return EventRename
	case reason&usnReasonFileCreate != 0:
		return EventCreate
	default:
		return EventModify
	}
}

func (feed *windowsFeed) ensureRDCW(repository repo.Repository) error {
	_, err := feed.rdcwSnapshot(context.Background(), repository)
	return err
}

func (feed *windowsFeed) rdcwSnapshot(_ context.Context, repository repo.Repository) (Checkpoint, error) {
	volumeIdentity, volumeKind := repositoryVolume(repository)
	feed.mu.Lock()
	defer feed.mu.Unlock()
	if feed.closed {
		return Checkpoint{}, errors.New("ReadDirectoryChangesW feed is closed")
	}
	session := feed.sessions[repository.RepoID]
	if session != nil {
		session.mu.Lock()
		reset := session.closed || session.overflow || session.repositoryPath != repository.Path ||
			session.volumeIdentity != volumeIdentity
		session.mu.Unlock()
		if reset {
			feed.closeRDCWLocked(session)
			delete(feed.sessions, repository.RepoID)
			session = nil
		}
	}
	if session == nil {
		var err error
		session, err = feed.startRDCWLocked(repository, volumeIdentity, volumeKind)
		if err != nil {
			return Checkpoint{AdapterKind: "rdcw", VolumeIdentity: volumeIdentity, VolumeKind: volumeKind, Health: HealthUnavailable}, err
		}
		feed.sessions[repository.RepoID] = session
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.checkpoint(), nil
}

func (feed *windowsFeed) readRDCW(repository repo.Repository, after, through Checkpoint, limit int) (Batch, error) {
	if !after.Valid() || !through.Valid() || !after.SameIdentity(through) {
		return Batch{}, ErrCursorInvalid
	}
	from, err := strconv.ParseUint(string(after.Cursor), 10, 64)
	if err != nil {
		return Batch{}, ErrCursorInvalid
	}
	to, err := strconv.ParseUint(string(through.Cursor), 10, 64)
	if err != nil || to < from {
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
		from+1 < session.oldestSequence {
		return Batch{}, ErrCursorInvalid
	}
	if limit <= 0 {
		limit = 1
	}
	events := make([]Event, 0, limit)
	nextSequence := from
	remaining := false
	for _, candidate := range session.events {
		if candidate.sequence <= from || candidate.sequence > to {
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

func (feed *windowsFeed) startRDCWLocked(repository repo.Repository, volumeIdentity, volumeKind string) (*rdcwSession, error) {
	pointer, err := windows.UTF16PtrFromString(repository.Path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(pointer, windows.FILE_LIST_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, fmt.Errorf("open repository for ReadDirectoryChangesW: %w", err)
	}
	session := &rdcwSession{
		repositoryID: repository.RepoID, repositoryPath: repository.Path, handle: handle,
		instanceID: uuid.NewString(), volumeIdentity: volumeIdentity, volumeKind: volumeKind,
		oldestSequence: 1,
	}
	go feed.readRDCWLoop(session)
	return session, nil
}

func (feed *windowsFeed) closeRDCWLocked(session *rdcwSession) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if !session.closed {
		session.closed = true
		_ = windows.CloseHandle(session.handle)
	}
}

func (feed *windowsFeed) readRDCWLoop(session *rdcwSession) {
	buffer := make([]byte, 256*1024)
	mask := uint32(windows.FILE_NOTIFY_CHANGE_FILE_NAME | windows.FILE_NOTIFY_CHANGE_DIR_NAME |
		windows.FILE_NOTIFY_CHANGE_ATTRIBUTES | windows.FILE_NOTIFY_CHANGE_SIZE |
		windows.FILE_NOTIFY_CHANGE_LAST_WRITE | windows.FILE_NOTIFY_CHANGE_CREATION |
		windows.FILE_NOTIFY_CHANGE_SECURITY)
	for {
		var returned uint32
		err := windows.ReadDirectoryChanges(session.handle, &buffer[0], uint32(len(buffer)), true, mask, &returned, nil, 0)
		session.mu.Lock()
		if session.closed {
			session.mu.Unlock()
			return
		}
		if err != nil || returned == 0 {
			session.overflow = true
			session.mu.Unlock()
			select {
			case feed.notifications <- session.repositoryID:
			default:
			}
			return
		}
		feed.consumeRDCWLocked(session, buffer[:returned])
		session.mu.Unlock()
	}
}

func (feed *windowsFeed) consumeRDCWLocked(session *rdcwSession, data []byte) {
	oldName := ""
	for offset := uint32(0); int(offset)+12 <= len(data); {
		native := (*windows.FileNotifyInformation)(unsafe.Pointer(&data[offset]))
		nameStart := int(offset) + int(unsafe.Offsetof(native.FileName))
		nameEnd := nameStart + int(native.FileNameLength)
		if native.FileNameLength%2 != 0 || nameEnd > len(data) {
			session.overflow = true
			return
		}
		units := unsafe.Slice((*uint16)(unsafe.Pointer(&data[nameStart])), int(native.FileNameLength)/2)
		relative := filepath.ToSlash(string(utf16.Decode(units)))
		absolute := filepath.Join(session.repositoryPath, filepath.FromSlash(relative))
		userPath, err := relativeUserPath(repo.Repository{RepoID: session.repositoryID, Path: session.repositoryPath}, absolute)
		if err == nil {
			oldPath := ""
			kind := EventModify
			switch native.Action {
			case windows.FILE_ACTION_ADDED:
				kind = EventCreate
			case windows.FILE_ACTION_REMOVED:
				kind = EventRemove
			case windows.FILE_ACTION_RENAMED_OLD_NAME:
				kind = EventRemove
				oldName = userPath
			case windows.FILE_ACTION_RENAMED_NEW_NAME:
				kind = EventRename
				oldPath = oldName
				oldName = ""
			}
			session.sequence++
			event := Event{
				Key:  fmt.Sprintf("rdcw:%s:%d:%d:%s", session.instanceID, session.sequence, native.Action, userPath),
				Kind: kind, Path: userPath, OldPath: oldPath,
			}
			session.events = append(session.events, sequencedEvent{sequence: session.sequence, event: event})
			if len(session.events) > rdcwHistory {
				dropped := session.events[0].sequence
				session.events = session.events[1:]
				session.oldestSequence = dropped + 1
			}
			select {
			case feed.notifications <- session.repositoryID:
			default:
			}
		}
		if native.NextEntryOffset == 0 {
			return
		}
		if native.NextEntryOffset < 12 || int(offset+native.NextEntryOffset) > len(data) {
			session.overflow = true
			return
		}
		offset += native.NextEntryOffset
	}
}

func (session *rdcwSession) checkpoint() Checkpoint {
	health := HealthHealthy
	if session.overflow {
		health = HealthOverflow
	}
	return Checkpoint{
		AdapterKind: "rdcw", Cursor: []byte(strconv.FormatUint(session.sequence, 10)),
		VolumeIdentity: session.volumeIdentity, VolumeKind: session.volumeKind,
		JournalIdentity: session.instanceID, Health: health,
	}
}

func repositoryVolumeInfo(repositoryPath string) (root, volumeName string, serial uint32, filesystem string, driveType uint32, resultErr error) {
	pathPointer, err := windows.UTF16PtrFromString(repositoryPath)
	if err != nil {
		return "", "", 0, "", windows.DRIVE_UNKNOWN, err
	}
	rootBuffer := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumePathName(pathPointer, &rootBuffer[0], uint32(len(rootBuffer))); err != nil {
		return "", "", 0, "", windows.DRIVE_UNKNOWN, err
	}
	root = windows.UTF16ToString(rootBuffer)
	rootPointer, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return "", "", 0, "", windows.DRIVE_UNKNOWN, err
	}
	driveType = windows.GetDriveType(rootPointer)
	volumeBuffer := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumeNameForVolumeMountPoint(rootPointer, &volumeBuffer[0], uint32(len(volumeBuffer))); err != nil {
		return "", "", 0, "", driveType, err
	}
	volumeName = windows.UTF16ToString(volumeBuffer)
	filesystemBuffer := make([]uint16, 64)
	if err := windows.GetVolumeInformation(rootPointer, nil, 0, &serial, nil, nil, &filesystemBuffer[0], uint32(len(filesystemBuffer))); err != nil {
		return "", "", 0, "", driveType, err
	}
	filesystem = windows.UTF16ToString(filesystemBuffer)
	return root, volumeName, serial, filesystem, driveType, nil
}

func platformRepositoryVolume(repositoryPath string) (string, string, error) {
	_, volumeName, serial, filesystem, driveType, err := repositoryVolumeInfo(repositoryPath)
	if err != nil {
		return "", "unknown", err
	}
	kind := windowsVolumeKind(repositoryPath, filesystem, driveType)
	return fmt.Sprintf("windows-volume:%s:%08x", strings.ToLower(volumeName), serial), kind, nil
}

func windowsVolumeKind(repositoryPath, filesystem string, driveType uint32) string {
	switch {
	case driveType == windows.DRIVE_REMOTE || strings.HasPrefix(strings.ToLower(repositoryPath), `\\`):
		return "network"
	case driveType == windows.DRIVE_REMOVABLE || driveType == windows.DRIVE_CDROM:
		return "removable"
	case !strings.EqualFold(filesystem, "NTFS") && !strings.EqualFold(filesystem, "ReFS"):
		return "unsupported"
	default:
		return "local"
	}
}

var _ = usnReasonDataOverwrite | usnReasonDataExtend | usnReasonDataTruncation |
	usnReasonNamedDataOverwrite | usnReasonNamedDataExtend | usnReasonNamedDataTruncate |
	usnReasonEAChange | usnReasonSecurityChange | usnReasonIndexableChange |
	usnReasonBasicInfoChange | usnReasonHardLinkChange | usnReasonCompressionChange |
	usnReasonEncryptionChange | usnReasonObjectIDChange | usnReasonReparsePointChange |
	usnReasonStreamChange
