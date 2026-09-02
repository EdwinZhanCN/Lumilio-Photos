package storage

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"
	fileutil "server/internal/utils/file"
	hashutil "server/internal/utils/hash"

	"github.com/google/uuid"
)

var (
	ErrRepositoryUnavailable       = ErrRepositoryOffline
	ErrRepositoryMarkerInvalid     = errors.New("repository marker is invalid")
	ErrRepositoryPermission        = errors.New("repository permission denied")
	ErrRepositoryFSClosed          = errors.New("repository filesystem is closed")
	ErrRepositoryFileUnstable      = errors.New("repository file changed during inspection")
	ErrRepositoryEntryUnsupported  = errors.New("repository entry is unsupported")
	ErrRepositoryImmutableConflict = errors.New("immutable repository file conflicts with existing content")
	ErrNestedRepository            = errors.New("nested repository boundary encountered")
)

type EntryKind string

const (
	EntryKindRegular   EntryKind = "regular"
	EntryKindSymlink   EntryKind = "symlink"
	EntryKindDirectory EntryKind = "directory"
)

type HashMode uint8

const (
	HashNone HashMode = iota
	HashQuick
	HashFull
	HashQuickAndFull
)

type FileObservation struct {
	RepositoryID        uuid.UUID
	Path                RepositoryPath
	EntryKind           EntryKind
	Size                int64
	ModTimeNS           int64
	ChangeTimeNS        *int64
	FileIdentityKind    *string
	FileIdentity        *string
	ObservationToken    string
	QuickFingerprint    *string
	QuickFingerprintVer *string
	ContentHash         *string
	ScanID              uuid.UUID
}

type WalkIssue struct {
	Path   string
	Reason string
	Err    error
}

type WalkOptions struct {
	ScanID uuid.UUID
	Settle time.Duration
	Now    time.Time
}

type WalkSummary struct {
	Observations  []FileObservation
	DeferredPaths []RepositoryPath
	Issues        []WalkIssue
	Skipped       int64
	Authoritative bool
	PartialReason string
}

// DirectoryReadOptions identifies one bounded verifier page. Offset counts raw
// directory entries, including markers and unsupported files, so a resumed
// frontier never depends on the number of catalog-worthy observations.
type DirectoryReadOptions struct {
	Directory string
	Offset    int64
	Limit     int
	ScanID    uuid.UUID
	Settle    time.Duration
	Now       time.Time
}

// DirectoryReadBatch is bounded by DirectoryReadOptions.Limit. Entries are
// positive observations only; Authoritative states whether the completed
// child set may later finalize absences.
type DirectoryReadBatch struct {
	Entries       []DirectoryReadEntry
	Issues        []WalkIssue
	NextOffset    int64
	Done          bool
	Authoritative bool
}

type DirectoryReadEntry struct {
	Observation FileObservation
	NextOffset  int64
}

type RepositoryFSFactory struct {
	access  *RepositoryAccessCoordinator
	queries *repo.Queries
}

func NewRepositoryFSFactory(access *RepositoryAccessCoordinator, queries *repo.Queries) *RepositoryFSFactory {
	if access == nil {
		access = NewRepositoryAccessCoordinator()
	}
	return &RepositoryFSFactory{access: access, queries: queries}
}

func (f *RepositoryFSFactory) AccessCoordinator() *RepositoryAccessCoordinator {
	if f == nil {
		return nil
	}
	return f.access
}

// ValidateRepositoryParent is the common pre-I/O identity gate. Repository
// reachability never overrides its parent Storage Location: an offline,
// maintenance, missing, or replaced root fails before any repository handle or
// staging writer is opened.
func (f *RepositoryFSFactory) ValidateRepositoryParent(ctx context.Context, repository repo.Repository) error {
	if f == nil || f.queries == nil {
		return nil
	}
	root, err := f.queries.GetRepositoryRoot(ctx, repository.RootID)
	if err != nil {
		return fmt.Errorf("%w: load parent Storage Location: %v", ErrRepositoryUnavailable, err)
	}
	if root.Status != dbtypes.RepositoryRootStatusActive {
		return fmt.Errorf("%w: parent Storage Location status=%s", ErrRepositoryUnavailable, root.Status)
	}
	marker, err := rootcfg.Load(root.Path)
	if err != nil || marker.ID != root.RootID.String() {
		_, _ = f.queries.UpdateRepositoryRootFromDisk(ctx, repo.UpdateRepositoryRootFromDiskParams{
			RootID: root.RootID, Name: root.Name, Status: dbtypes.RepositoryRootStatusError,
			UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
		})
		return fmt.Errorf("%w: parent Storage Location identity changed", ErrRepositoryUnavailable)
	}
	return nil
}

// Open verifies catalog reachability and the portable repository marker before
// returning any media capability.
func (f *RepositoryFSFactory) Open(repository repo.Repository) (*RepositoryFS, error) {
	return f.OpenContext(context.Background(), repository)
}

// OpenContext is Open with a bounded wait for the in-process repository lease.
// Request paths use it so lifecycle maintenance cannot leave an HTTP request
// blocked after its context has been cancelled.
func (f *RepositoryFSFactory) OpenContext(ctx context.Context, repository repo.Repository) (*RepositoryFS, error) {
	if f == nil || f.access == nil {
		return nil, fmt.Errorf("%w: factory is unavailable", ErrRepositoryUnavailable)
	}
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if f.queries == nil && repository.Reachability != dbtypes.RepositoryReachabilityActive {
		return nil, fmt.Errorf("%w: reachability=%s", ErrRepositoryUnavailable, repository.Reachability)
	}
	release, err := f.access.acquireReadContext(ctx, repository.RepoID)
	if err != nil {
		return nil, fmt.Errorf("%w: repository is busy: %v", ErrRepositoryBusy, err)
	}
	if f.queries != nil {
		current, err := f.queries.GetRepository(ctx, repository.RepoID)
		if err != nil {
			release()
			return nil, fmt.Errorf("%w: refresh repository location: %v", ErrRepositoryUnavailable, err)
		}
		repository = current
		if repository.Reachability != dbtypes.RepositoryReachabilityActive {
			release()
			return nil, fmt.Errorf("%w: reachability=%s", ErrRepositoryUnavailable, repository.Reachability)
		}
		if err := f.ValidateRepositoryParent(ctx, repository); err != nil {
			release()
			return nil, err
		}
	}
	root, err := os.OpenRoot(repository.Path)
	if err != nil {
		release()
		return nil, classifyRepositoryRootError(repository.Path, err)
	}
	fail := func(err error) (*RepositoryFS, error) {
		_ = root.Close()
		release()
		return nil, err
	}
	marker, err := root.ReadFile(".lumiliorepo")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fail(fmt.Errorf("%w: .lumiliorepo is missing", ErrRepositoryOffline))
		}
		return fail(classifyRepositoryRootError(".lumiliorepo", err))
	}
	config, err := repocfg.ParseConfig(marker)
	if err != nil {
		return fail(fmt.Errorf("%w: %v", ErrRepositoryMarkerInvalid, err))
	}
	markerID, err := uuid.Parse(config.ID)
	if err != nil {
		return fail(fmt.Errorf("%w: invalid marker UUID: %v", ErrRepositoryMarkerInvalid, err))
	}
	if markerID != repository.RepoID {
		return fail(fmt.Errorf("%w: marker=%s catalog=%s", ErrRepositoryIDMismatch, markerID, repository.RepoID))
	}
	return &RepositoryFS{
		repositoryID: repository.RepoID,
		root:         root,
		release:      release,
	}, nil
}

func classifyRepositoryRootError(name string, err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%w: %s: %v", ErrRepositoryOffline, name, err)
	case errors.Is(err, fs.ErrPermission):
		return fmt.Errorf("%w: %s: %v", ErrRepositoryPermission, name, err)
	default:
		return fmt.Errorf("%w: %s: %v", ErrRepositoryUnavailable, name, err)
	}
}

func classifyRepositoryEntryError(name string, err error) error {
	if errors.Is(err, fs.ErrPermission) {
		return fmt.Errorf("%w: %s: %v", ErrRepositoryPermission, name, err)
	}
	return fmt.Errorf("repository entry %s: %w", name, err)
}

// RepositoryFS owns one os.Root plus the lifecycle read lease associated with
// it. Close is idempotent and waits for in-flight methods.
type RepositoryFS struct {
	mu           sync.RWMutex
	repositoryID uuid.UUID
	root         *os.Root
	release      func()
	closed       bool
}

// RepositoryFile keeps the rooted filesystem lease alive for the lifetime of
// a staging file descriptor. Closing it releases both resources exactly once.
type RepositoryFile struct {
	*os.File
	repositoryFS *RepositoryFS
	closeOnce    sync.Once
	closeErr     error
}

func newRepositoryFile(file *os.File, repositoryFS *RepositoryFS) *RepositoryFile {
	return &RepositoryFile{File: file, repositoryFS: repositoryFS}
}

func (f *RepositoryFile) Close() error {
	if f == nil {
		return nil
	}
	f.closeOnce.Do(func() {
		fileErr := f.File.Close()
		rootErr := f.repositoryFS.Close()
		f.closeErr = errors.Join(fileErr, rootErr)
	})
	return f.closeErr
}

func (r *RepositoryFS) RepositoryID() uuid.UUID {
	if r == nil {
		return uuid.Nil
	}
	return r.repositoryID
}

// VerifyIdentity re-reads the marker through the held root immediately before
// a catalog mutation. It detects a replaced mount or repository identity
// change after the handle was opened.
func (r *RepositoryFS) VerifyIdentity() error {
	root, done, err := r.withRoot()
	if err != nil {
		return err
	}
	defer done()
	marker, err := root.ReadFile(".lumiliorepo")
	if err != nil {
		return classifyRepositoryRootError(".lumiliorepo", err)
	}
	config, err := repocfg.ParseConfig(marker)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRepositoryMarkerInvalid, err)
	}
	markerID, err := uuid.Parse(config.ID)
	if err != nil {
		return fmt.Errorf("%w: invalid marker UUID: %v", ErrRepositoryMarkerInvalid, err)
	}
	if markerID != r.repositoryID {
		return fmt.Errorf("%w: marker=%s catalog=%s", ErrRepositoryIDMismatch, markerID, r.repositoryID)
	}
	return nil
}

func (r *RepositoryFS) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	err := r.root.Close()
	if r.release != nil {
		r.release()
		r.release = nil
	}
	return err
}

func (r *RepositoryFS) withRoot() (*os.Root, func(), error) {
	if r == nil {
		return nil, nil, ErrRepositoryFSClosed
	}
	r.mu.RLock()
	if r.closed || r.root == nil {
		r.mu.RUnlock()
		return nil, nil, ErrRepositoryFSClosed
	}
	return r.root, r.mu.RUnlock, nil
}

func (r *RepositoryFS) OpenMedia(repositoryPath RepositoryPath) (*os.File, error) {
	if !repositoryPath.isUserMedia() {
		return nil, ErrRepositoryPathNamespace
	}
	local, err := repositoryPath.local()
	if err != nil {
		return nil, err
	}
	root, done, err := r.withRoot()
	if err != nil {
		return nil, err
	}
	defer done()
	opened, err := root.Open(local)
	if err != nil {
		return nil, classifyRepositoryEntryError(repositoryPath.String(), err)
	}
	info, err := opened.Stat()
	if err != nil {
		_ = opened.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		_ = opened.Close()
		return nil, fmt.Errorf("%w: %s is not a regular file", ErrRepositoryEntryUnsupported, repositoryPath.String())
	}
	return opened, nil
}

func (r *RepositoryFS) OpenPrivate(repositoryPath RepositoryPath) (*os.File, error) {
	if !repositoryPath.isPrivate() {
		return nil, ErrRepositoryPathNamespace
	}
	local, err := repositoryPath.local()
	if err != nil {
		return nil, err
	}
	root, done, err := r.withRoot()
	if err != nil {
		return nil, err
	}
	defer done()
	return root.Open(local)
}

func (r *RepositoryFS) OpenPrivateFile(repositoryPath RepositoryPath, flag int, perm fs.FileMode) (*os.File, error) {
	if !repositoryPath.isPrivate() {
		return nil, ErrRepositoryPathNamespace
	}
	local, err := repositoryPath.local()
	if err != nil {
		return nil, err
	}
	root, done, err := r.withRoot()
	if err != nil {
		return nil, err
	}
	defer done()
	return root.OpenFile(local, flag, perm)
}

func (r *RepositoryFS) ReadPrivateFile(repositoryPath RepositoryPath) ([]byte, error) {
	if !repositoryPath.isPrivate() {
		return nil, ErrRepositoryPathNamespace
	}
	local, err := repositoryPath.local()
	if err != nil {
		return nil, err
	}
	root, done, err := r.withRoot()
	if err != nil {
		return nil, err
	}
	defer done()
	return root.ReadFile(local)
}

// WritePrivateFileAtomic replaces one private-workspace file only after the
// complete stream is durable. A failed write leaves the previous file intact.
func (r *RepositoryFS) WritePrivateFileAtomic(repositoryPath RepositoryPath, reader io.Reader, perm fs.FileMode) (int64, error) {
	if !repositoryPath.isPrivate() {
		return 0, ErrRepositoryPathNamespace
	}
	destination, err := repositoryPath.local()
	if err != nil {
		return 0, err
	}
	temporaryPath, err := ParsePrivateRepositoryPath(repositoryPath.String() + ".tmp-" + uuid.NewString())
	if err != nil {
		return 0, err
	}
	temporary, err := temporaryPath.local()
	if err != nil {
		return 0, err
	}
	root, done, err := r.withRoot()
	if err != nil {
		return 0, err
	}
	defer done()
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return 0, err
	}
	cleanup := true
	defer func() {
		_ = file.Close()
		if cleanup {
			_ = root.Remove(temporary)
		}
	}()
	written, err := io.Copy(file, reader)
	if err != nil {
		return written, err
	}
	if written == 0 {
		return 0, io.ErrUnexpectedEOF
	}
	if err := file.Sync(); err != nil {
		return written, err
	}
	if err := file.Close(); err != nil {
		return written, err
	}
	if err := root.Rename(temporary, destination); err != nil {
		return written, err
	}
	cleanup = false
	return written, syncRootDirectory(root, path.Dir(repositoryPath.String()))
}

// WritePrivateFileImmutable publishes a private-workspace file without ever
// replacing an existing destination. A retry may reuse an existing regular
// file only when its complete content is byte-for-byte identical; a different
// file at the immutable path is a conflict and remains untouched.
func (r *RepositoryFS) WritePrivateFileImmutable(repositoryPath RepositoryPath, reader io.Reader, perm fs.FileMode) (int64, error) {
	if !repositoryPath.isPrivate() {
		return 0, ErrRepositoryPathNamespace
	}
	if reader == nil {
		return 0, errors.New("immutable file reader is required")
	}
	destination, err := repositoryPath.local()
	if err != nil {
		return 0, err
	}
	temporaryPath, err := ParsePrivateRepositoryPath(repositoryPath.String() + ".tmp-" + uuid.NewString())
	if err != nil {
		return 0, err
	}
	temporary, err := temporaryPath.local()
	if err != nil {
		return 0, err
	}
	root, done, err := r.withRoot()
	if err != nil {
		return 0, err
	}
	defer done()
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return 0, err
	}
	cleanup := true
	defer func() {
		_ = file.Close()
		if cleanup {
			_ = root.Remove(temporary)
		}
	}()

	digest := sha256.New()
	written, err := io.Copy(io.MultiWriter(file, digest), reader)
	if err != nil {
		return written, err
	}
	if written == 0 {
		return 0, io.ErrUnexpectedEOF
	}
	if err := file.Sync(); err != nil {
		return written, err
	}
	if err := file.Close(); err != nil {
		return written, err
	}

	if err := root.Link(temporary, destination); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return written, err
		}
		existingSize, existingDigest, compareErr := hashRootRegularFile(root, destination)
		if compareErr != nil {
			return written, compareErr
		}
		if existingSize != written || !equalDigest(existingDigest, digest.Sum(nil)) {
			return written, fmt.Errorf("%w: %s", ErrRepositoryImmutableConflict, repositoryPath.String())
		}
		return existingSize, nil
	}
	if err := syncRootDirectory(root, path.Dir(repositoryPath.String())); err != nil {
		return written, err
	}
	if err := root.Remove(temporary); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return written, err
	}
	cleanup = false
	if err := syncRootDirectory(root, path.Dir(repositoryPath.String())); err != nil {
		return written, err
	}
	return written, nil
}

func hashRootRegularFile(root *os.Root, name string) (int64, []byte, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return 0, nil, err
	}
	if !info.Mode().IsRegular() {
		return 0, nil, fmt.Errorf("%w: immutable destination is not a regular file", ErrRepositoryImmutableConflict)
	}
	file, err := root.Open(name)
	if err != nil {
		return 0, nil, err
	}
	defer file.Close()
	digest := sha256.New()
	size, err := io.Copy(digest, file)
	if err != nil {
		return 0, nil, err
	}
	return size, digest.Sum(nil), nil
}

func equalDigest(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var difference byte
	for index := range left {
		difference |= left[index] ^ right[index]
	}
	return difference == 0
}

func (r *RepositoryFS) StatPrivate(repositoryPath RepositoryPath) (fs.FileInfo, error) {
	if !repositoryPath.isPrivate() {
		return nil, ErrRepositoryPathNamespace
	}
	local, err := repositoryPath.local()
	if err != nil {
		return nil, err
	}
	root, done, err := r.withRoot()
	if err != nil {
		return nil, err
	}
	defer done()
	return root.Stat(local)
}

func (r *RepositoryFS) MkdirAllPrivate(repositoryPath RepositoryPath, perm fs.FileMode) error {
	return r.mkdirAll(repositoryPath, namespacePrivate, perm)
}

func (r *RepositoryFS) MkdirAllMedia(repositoryPath RepositoryPath, perm fs.FileMode) error {
	return r.mkdirAll(repositoryPath, namespaceUserMedia, perm)
}

func (r *RepositoryFS) mkdirAll(repositoryPath RepositoryPath, namespace repositoryNamespace, perm fs.FileMode) error {
	if repositoryPath.namespace != namespace {
		return ErrRepositoryPathNamespace
	}
	local, err := repositoryPath.local()
	if err != nil {
		return err
	}
	root, done, err := r.withRoot()
	if err != nil {
		return err
	}
	defer done()
	return root.MkdirAll(local, perm)
}

func (r *RepositoryFS) RemovePrivate(repositoryPath RepositoryPath) error {
	if !repositoryPath.isPrivate() {
		return ErrRepositoryPathNamespace
	}
	local, err := repositoryPath.local()
	if err != nil {
		return err
	}
	root, done, err := r.withRoot()
	if err != nil {
		return err
	}
	defer done()
	return root.Remove(local)
}

// PrivateFile is one regular application-owned file discovered below a
// validated private repository directory. Callers receive repository-relative
// paths only; the rooted filesystem capability never leaks an absolute path.
type PrivateFile struct {
	Path    RepositoryPath
	Size    int64
	ModTime time.Time
}

// ListPrivateFiles walks one private subtree without following or accepting
// non-regular entries. It is the bounded capability used by artifact garbage
// collection; ordinary media code must not inspect the application-owned tree.
func (r *RepositoryFS) ListPrivateFiles(ctx context.Context, directory RepositoryPath) ([]PrivateFile, error) {
	if ctx == nil {
		return nil, errors.New("private file walk context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !directory.isPrivate() {
		return nil, ErrRepositoryPathNamespace
	}
	root, done, err := r.withRoot()
	if err != nil {
		return nil, err
	}
	defer done()
	files := make([]PrivateFile, 0)
	err = fs.WalkDir(root.FS(), directory.String(), func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, fs.ErrNotExist) && name == directory.String() {
				return fs.SkipDir
			}
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%w: private entry %s is not a regular file", ErrRepositoryEntryUnsupported, name)
		}
		privatePath, err := ParsePrivateRepositoryPath(name)
		if err != nil {
			return err
		}
		files = append(files, PrivateFile{Path: privatePath, Size: info.Size(), ModTime: info.ModTime()})
		return nil
	})
	return files, err
}

func (r *RepositoryFS) StatMedia(repositoryPath RepositoryPath) (fs.FileInfo, error) {
	if !repositoryPath.isUserMedia() {
		return nil, ErrRepositoryPathNamespace
	}
	local, err := repositoryPath.local()
	if err != nil {
		return nil, err
	}
	root, done, err := r.withRoot()
	if err != nil {
		return nil, err
	}
	defer done()
	return root.Stat(local)
}

// LocalMediaPath is the only adapter for native tools that require a host
// filename. It verifies the rooted path immediately before deriving it.
func (r *RepositoryFS) LocalMediaPath(repositoryPath RepositoryPath) (string, error) {
	opened, err := r.OpenMedia(repositoryPath)
	if err != nil {
		return "", err
	}
	if err := opened.Close(); err != nil {
		return "", err
	}
	local, err := repositoryPath.local()
	if err != nil {
		return "", err
	}
	root, done, err := r.withRoot()
	if err != nil {
		return "", err
	}
	defer done()
	return filepath.Join(root.Name(), local), nil
}

// LocalPrivatePath is the private-workspace equivalent used only when a native
// tool requires a filename instead of an opened file.
func (r *RepositoryFS) LocalPrivatePath(repositoryPath RepositoryPath) (string, error) {
	opened, err := r.OpenPrivate(repositoryPath)
	if err != nil {
		return "", err
	}
	info, statErr := opened.Stat()
	closeErr := opened.Close()
	if statErr != nil {
		return "", statErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%w: %s is not a regular file", ErrRepositoryEntryUnsupported, repositoryPath.String())
	}
	local, err := repositoryPath.local()
	if err != nil {
		return "", err
	}
	root, done, err := r.withRoot()
	if err != nil {
		return "", err
	}
	defer done()
	return filepath.Join(root.Name(), local), nil
}

func (r *RepositoryFS) InspectMedia(ctx context.Context, repositoryPath RepositoryPath, mode HashMode) (FileObservation, error) {
	if err := ctx.Err(); err != nil {
		return FileObservation{}, err
	}
	if !repositoryPath.isUserMedia() {
		return FileObservation{}, ErrRepositoryPathNamespace
	}
	local, err := repositoryPath.local()
	if err != nil {
		return FileObservation{}, err
	}
	root, done, err := r.withRoot()
	if err != nil {
		return FileObservation{}, err
	}
	defer done()
	lstat, err := root.Lstat(local)
	if err != nil {
		return FileObservation{}, classifyRepositoryEntryError(repositoryPath.String(), err)
	}
	kind := EntryKindRegular
	if lstat.Mode()&os.ModeSymlink != 0 {
		kind = EntryKindSymlink
	} else if !lstat.Mode().IsRegular() {
		return FileObservation{}, fmt.Errorf("%w: %s", ErrRepositoryEntryUnsupported, repositoryPath.String())
	}
	opened, err := root.Open(local)
	if err != nil {
		return FileObservation{}, classifyRepositoryEntryError(repositoryPath.String(), err)
	}
	defer opened.Close()
	before, err := opened.Stat()
	if err != nil {
		return FileObservation{}, err
	}
	if !before.Mode().IsRegular() {
		return FileObservation{}, fmt.Errorf("%w: %s target is not regular", ErrRepositoryEntryUnsupported, repositoryPath.String())
	}
	identityKind, identity, changeTime := platformFileIdentity(opened, before)
	observation := newFileObservation(r.repositoryID, repositoryPath, kind, before, identityKind, identity, changeTime)
	if mode == HashQuick || mode == HashQuickAndFull {
		quick, hashErr := hashutil.CalculateQuickHash(opened, before.Size(), hashutil.AlgorithmBLAKE3)
		if hashErr != nil {
			return FileObservation{}, hashErr
		}
		observation.QuickFingerprint = &quick
		version := hashutil.QuickFingerprintVersion
		observation.QuickFingerprintVer = &version
	}
	if mode == HashFull || mode == HashQuickAndFull {
		full, hashErr := hashutil.CalculateReaderHash(io.NewSectionReader(opened, 0, before.Size()), hashutil.AlgorithmBLAKE3)
		if hashErr != nil {
			return FileObservation{}, hashErr
		}
		observation.ContentHash = &full
	}
	if err := ctx.Err(); err != nil {
		return FileObservation{}, err
	}
	after, err := opened.Stat()
	if err != nil {
		return FileObservation{}, err
	}
	afterKind, afterIdentity, afterChange := platformFileIdentity(opened, after)
	afterObservation := newFileObservation(r.repositoryID, repositoryPath, kind, after, afterKind, afterIdentity, afterChange)
	if observation.ObservationToken != afterObservation.ObservationToken {
		return FileObservation{}, fmt.Errorf("%w: %s", ErrRepositoryFileUnstable, repositoryPath.String())
	}
	return observation, nil
}

func (r *RepositoryFS) Revalidate(ctx context.Context, expected FileObservation) error {
	if expected.RepositoryID != r.RepositoryID() {
		return fmt.Errorf("%w: observation belongs to %s", ErrRepositoryIDMismatch, expected.RepositoryID)
	}
	current, err := r.InspectMedia(ctx, expected.Path, HashNone)
	if err != nil {
		return err
	}
	if current.ObservationToken != expected.ObservationToken {
		return fmt.Errorf("%w: %s", ErrRepositoryFileUnstable, expected.Path.String())
	}
	return nil
}

func (r *RepositoryFS) WalkUserMedia(ctx context.Context, options WalkOptions) (WalkSummary, error) {
	summary := WalkSummary{Authoritative: true}
	if options.Now.IsZero() {
		options.Now = time.Now().UTC()
	}
	root, done, err := r.withRoot()
	if err != nil {
		return summary, err
	}
	defer done()
	walkErr := fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			summary.markPartial(name, "walk_error", walkErr)
			return nil
		}
		if name == "." {
			return nil
		}
		if entry.IsDir() {
			if name == ".lumilio" {
				return fs.SkipDir
			}
			if _, markerErr := root.Stat(path.Join(name, ".lumiliorepo")); markerErr == nil {
				topologyErr := fmt.Errorf("%w: %s", ErrNestedRepository, name)
				summary.markPartial(name, "nested_repository", topologyErr)
				return fs.SkipDir
			} else if !errors.Is(markerErr, fs.ErrNotExist) {
				summary.markPartial(name, "nested_repository_check", markerErr)
				return fs.SkipDir
			}
			return nil
		}
		if name == ".lumiliorepo" || name == ".lumilioroot" {
			return nil
		}
		repositoryPath, parseErr := ParseUserMediaPath(name)
		if parseErr != nil {
			summary.markPartial(name, "invalid_path", parseErr)
			return nil
		}
		if !fileutil.IsSupportedExtension(path.Ext(repositoryPath.String())) {
			summary.Skipped++
			return nil
		}
		observation, inspectErr := r.observeMediaWithHeldRoot(ctx, root, repositoryPath)
		if inspectErr != nil {
			if errors.Is(inspectErr, ErrRepositoryEntryUnsupported) || entry.Type()&os.ModeSymlink != 0 {
				summary.Skipped++
				summary.Issues = append(summary.Issues, WalkIssue{Path: name, Reason: "unsupported_entry", Err: inspectErr})
				return nil
			}
			summary.markPartial(name, "inspect_error", inspectErr)
			return nil
		}
		observation.ScanID = options.ScanID
		if options.Settle > 0 && options.Now.Sub(time.Unix(0, observation.ModTimeNS)) < options.Settle {
			summary.Skipped++
			summary.DeferredPaths = append(summary.DeferredPaths, repositoryPath)
			summary.Issues = append(summary.Issues, WalkIssue{Path: name, Reason: "settling"})
			return nil
		}
		summary.Observations = append(summary.Observations, observation)
		return nil
	})
	if walkErr != nil {
		summary.markPartial("", "walk_aborted", walkErr)
		return summary, walkErr
	}
	return summary, nil
}

// ReadUserMediaDirectory enumerates one bounded page without recursively
// walking or reading file contents. A resumed page reopens the directory and
// discards Offset entries in bounded chunks; change capture and the final
// verifier, rather than directory ordering, provide convergence across edits.
func (r *RepositoryFS) ReadUserMediaDirectory(ctx context.Context, options DirectoryReadOptions) (DirectoryReadBatch, error) {
	batch := DirectoryReadBatch{Authoritative: true, NextOffset: options.Offset}
	if err := ctx.Err(); err != nil {
		return batch, err
	}
	if options.Offset < 0 {
		return batch, fmt.Errorf("directory offset must be non-negative")
	}
	if options.Limit <= 0 || options.Limit > 256 {
		return batch, fmt.Errorf("directory limit must be between 1 and 256")
	}
	if options.Now.IsZero() {
		options.Now = time.Now().UTC()
	}
	directory := "."
	if options.Directory != "" {
		parsed, err := ParseUserMediaPath(options.Directory)
		if err != nil {
			return batch, err
		}
		directory, err = parsed.local()
		if err != nil {
			return batch, err
		}
	}

	root, done, err := r.withRoot()
	if err != nil {
		return batch, err
	}
	defer done()
	opened, err := root.Open(directory)
	if err != nil {
		return batch, classifyRepositoryEntryError(options.Directory, err)
	}
	defer opened.Close()
	info, err := opened.Stat()
	if err != nil {
		return batch, err
	}
	if !info.IsDir() {
		return batch, fmt.Errorf("%w: %s is not a directory", ErrRepositoryEntryUnsupported, options.Directory)
	}

	remaining := options.Offset
	for remaining > 0 {
		if err := ctx.Err(); err != nil {
			return batch, err
		}
		pageSize := options.Limit
		if int64(pageSize) > remaining {
			pageSize = int(remaining)
		}
		skipped, readErr := opened.ReadDir(pageSize)
		remaining -= int64(len(skipped))
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return batch, readErr
		}
		if len(skipped) == 0 || errors.Is(readErr, io.EOF) {
			batch.Done = true
			return batch, nil
		}
	}

	entries, readErr := opened.ReadDir(options.Limit)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return batch, readErr
	}
	batch.NextOffset += int64(len(entries))
	batch.Done = errors.Is(readErr, io.EOF) || len(entries) == 0
	batch.Entries = make([]DirectoryReadEntry, 0, len(entries))
	for index, entry := range entries {
		if err := ctx.Err(); err != nil {
			return batch, err
		}
		name := entry.Name()
		child := name
		if options.Directory != "" {
			child = path.Join(options.Directory, name)
		}
		if child == ".lumilio" || child == ".lumiliorepo" || child == ".lumilioroot" {
			continue
		}
		if entry.IsDir() {
			if _, markerErr := root.Stat(path.Join(child, ".lumiliorepo")); markerErr == nil {
				batch.Authoritative = false
				batch.Issues = append(batch.Issues, WalkIssue{Path: child, Reason: "nested_repository", Err: ErrNestedRepository})
				continue
			} else if !errors.Is(markerErr, fs.ErrNotExist) {
				batch.Authoritative = false
				batch.Issues = append(batch.Issues, WalkIssue{Path: child, Reason: "nested_repository_check", Err: markerErr})
				continue
			}
		}
		repositoryPath, parseErr := ParseUserMediaPath(child)
		if parseErr != nil {
			batch.Authoritative = false
			batch.Issues = append(batch.Issues, WalkIssue{Path: child, Reason: "invalid_path", Err: parseErr})
			continue
		}
		if !entry.IsDir() && !fileutil.IsSupportedExtension(path.Ext(repositoryPath.String())) {
			continue
		}
		observation, observeErr := r.observeNodeWithHeldRoot(ctx, root, repositoryPath, entry.IsDir())
		if observeErr != nil {
			if errors.Is(observeErr, ErrRepositoryEntryUnsupported) {
				batch.Issues = append(batch.Issues, WalkIssue{Path: child, Reason: "unsupported_entry", Err: observeErr})
				continue
			}
			batch.Authoritative = false
			batch.Issues = append(batch.Issues, WalkIssue{Path: child, Reason: "inspect_error", Err: observeErr})
			continue
		}
		observation.ScanID = options.ScanID
		if !entry.IsDir() && options.Settle > 0 && options.Now.Sub(time.Unix(0, observation.ModTimeNS)) < options.Settle {
			batch.Authoritative = false
			batch.Issues = append(batch.Issues, WalkIssue{Path: child, Reason: "settling"})
			continue
		}
		batch.Entries = append(batch.Entries, DirectoryReadEntry{
			Observation: observation,
			NextOffset:  options.Offset + int64(index) + 1,
		})
	}
	return batch, nil
}

func (r *RepositoryFS) observeNodeWithHeldRoot(
	ctx context.Context,
	root *os.Root,
	repositoryPath RepositoryPath,
	directory bool,
) (FileObservation, error) {
	if !directory {
		return r.observeMediaWithHeldRoot(ctx, root, repositoryPath)
	}
	local, err := repositoryPath.local()
	if err != nil {
		return FileObservation{}, err
	}
	opened, err := root.Open(local)
	if err != nil {
		return FileObservation{}, err
	}
	defer opened.Close()
	info, err := opened.Stat()
	if err != nil || !info.IsDir() {
		return FileObservation{}, ErrRepositoryEntryUnsupported
	}
	identityKind, identity, changeTime := platformFileIdentity(opened, info)
	return newFileObservation(r.repositoryID, repositoryPath, EntryKindDirectory, info, identityKind, identity, changeTime), nil
}

// observeMediaWithHeldRoot avoids reacquiring the RepositoryFS mutex while a
// walk already holds it. The walk performs no content read between metadata
// snapshots, so one opened-handle snapshot is sufficient; content consumers
// compare the observation token again before committing their work.
func (r *RepositoryFS) observeMediaWithHeldRoot(ctx context.Context, root *os.Root, repositoryPath RepositoryPath) (FileObservation, error) {
	local, err := repositoryPath.local()
	if err != nil {
		return FileObservation{}, err
	}
	if err := ctx.Err(); err != nil {
		return FileObservation{}, err
	}
	lstat, err := root.Lstat(local)
	if err != nil {
		return FileObservation{}, err
	}
	kind := EntryKindRegular
	if lstat.Mode()&os.ModeSymlink != 0 {
		kind = EntryKindSymlink
	} else if !lstat.Mode().IsRegular() {
		return FileObservation{}, ErrRepositoryEntryUnsupported
	}
	opened, err := root.Open(local)
	if err != nil {
		return FileObservation{}, err
	}
	defer opened.Close()
	before, err := opened.Stat()
	if err != nil || !before.Mode().IsRegular() {
		return FileObservation{}, ErrRepositoryEntryUnsupported
	}
	identityKind, identity, changeTime := platformFileIdentity(opened, before)
	return newFileObservation(r.repositoryID, repositoryPath, kind, before, identityKind, identity, changeTime), nil
}

func (s *WalkSummary) markPartial(repositoryPath, reason string, err error) {
	s.Authoritative = false
	s.Skipped++
	s.Issues = append(s.Issues, WalkIssue{Path: repositoryPath, Reason: reason, Err: err})
	if s.PartialReason == "" {
		if err != nil {
			s.PartialReason = err.Error()
		} else {
			s.PartialReason = reason
		}
	}
}

func newFileObservation(repositoryID uuid.UUID, repositoryPath RepositoryPath, kind EntryKind, info fs.FileInfo, identityKind, identity *string, changeTime *int64) FileObservation {
	observation := FileObservation{
		RepositoryID:     repositoryID,
		Path:             repositoryPath,
		EntryKind:        kind,
		Size:             info.Size(),
		ModTimeNS:        info.ModTime().UTC().UnixNano(),
		ChangeTimeNS:     changeTime,
		FileIdentityKind: identityKind,
		FileIdentity:     identity,
	}
	observation.ObservationToken = observationToken(observation)
	return observation
}

func observationToken(observation FileObservation) string {
	parts := []string{
		observation.RepositoryID.String(), observation.Path.String(), string(observation.EntryKind),
		strconv.FormatInt(observation.Size, 10), strconv.FormatInt(observation.ModTimeNS, 10),
	}
	if observation.ChangeTimeNS != nil {
		parts = append(parts, strconv.FormatInt(*observation.ChangeTimeNS, 10))
	}
	if observation.FileIdentityKind != nil {
		parts = append(parts, *observation.FileIdentityKind)
	}
	if observation.FileIdentity != nil {
		parts = append(parts, *observation.FileIdentity)
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("obs-v1:%x", digest[:])
}
