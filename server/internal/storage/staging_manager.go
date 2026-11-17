package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"server/internal/catalog/repo"
	"server/internal/storage/layout"
	"server/internal/storage/repocfg"

	"github.com/google/uuid"
)

// StagingManager is the private-workspace owner for upload and cloud ingest.
// Every staged file belongs to one upload session directory,
// work/uploads/<session>/, which also holds a chunked session's manifest and
// chunks. Ending the session removes that one directory; a failed session
// moves whole to quarantine/uploads/<session>/. Staging handles contain no
// host path and remain valid across repository relocation because every
// operation reopens the current catalog location.
type StagingManager interface {
	// CreateStagingFile stages a file in a fresh session of its own.
	CreateStagingFile(repository repo.Repository, filename string) (*StagingFile, *RepositoryFile, error)
	// CreateSessionFile stages the file of an upload session.
	CreateSessionFile(repository repo.Repository, sessionID uuid.UUID, filename string) (*StagingFile, *RepositoryFile, error)
	// CreateSessionChunk stores one chunk of a chunked upload session.
	CreateSessionChunk(repository repo.Repository, sessionID uuid.UUID, index int) (*StagingFile, *RepositoryFile, error)
	OpenStagingFile(repository repo.Repository, stagingFile *StagingFile) (*RepositoryFile, error)
	RemoveStagingFile(repository repo.Repository, stagingFile *StagingFile) error
	CommitStagingFile(repository repo.Repository, stagingFile *StagingFile, finalPath string) error
	CommitStagingFileToInbox(repository repo.Repository, stagingFile *StagingFile, contentHash string) (string, error)
	ResolveInboxPath(repository repo.Repository, originalFilename, contentHash string) (string, error)
	// MoveStagingToFailed quarantines the file's whole session and points
	// the handle at the quarantined file.
	MoveStagingToFailed(repository repo.Repository, stagingFile *StagingFile) error
	// QuarantineSession moves a whole upload session to quarantine.
	QuarantineSession(repository repo.Repository, sessionID uuid.UUID) error
	// CleanupStaging removes upload sessions idle for longer than maxAge
	// unless keep claims them. Quarantine is never cleaned automatically.
	CleanupStaging(repository repo.Repository, maxAge time.Duration, keep func(sessionID uuid.UUID) bool) error
}

type DefaultStagingManager struct {
	files *RepositoryFSFactory
}

func NewStagingManager(files *RepositoryFSFactory) *DefaultStagingManager {
	if files == nil {
		files = NewRepositoryFSFactory(nil, nil)
	}
	return &DefaultStagingManager{files: files}
}

var _ StagingManager = (*DefaultStagingManager)(nil)

func (sm *DefaultStagingManager) CreateStagingFile(repository repo.Repository, filename string) (*StagingFile, *RepositoryFile, error) {
	return sm.CreateSessionFile(repository, uuid.New(), filename)
}

func (sm *DefaultStagingManager) CreateSessionFile(repository repo.Repository, sessionID uuid.UUID, filename string) (*StagingFile, *RepositoryFile, error) {
	privatePath, err := SessionFilePath(sessionID, filename)
	if err != nil {
		return nil, nil, err
	}
	return sm.createInSession(repository, sessionID, privatePath, path.Base(privatePath.String()))
}

// SessionFilePath is where an upload session stages its file under the
// file's portable name.
func SessionFilePath(sessionID uuid.UUID, filename string) (layout.Path, error) {
	base, err := portableBase(filename)
	if err != nil {
		return layout.Path{}, err
	}
	return layout.UploadFile(sessionID, base)
}

func (sm *DefaultStagingManager) CreateSessionChunk(repository repo.Repository, sessionID uuid.UUID, index int) (*StagingFile, *RepositoryFile, error) {
	privatePath, err := layout.UploadChunk(sessionID, index)
	if err != nil {
		return nil, nil, err
	}
	return sm.createInSession(repository, sessionID, privatePath, path.Base(privatePath.String()))
}

func (sm *DefaultStagingManager) createInSession(repository repo.Repository, sessionID uuid.UUID, privatePath layout.Path, filename string) (*StagingFile, *RepositoryFile, error) {
	repositoryFS, err := sm.files.Open(repository)
	if err != nil {
		return nil, nil, err
	}
	keepOpen := false
	defer func() {
		if !keepOpen {
			_ = repositoryFS.Close()
		}
	}()
	directory, err := layout.UploadSession(sessionID)
	if err != nil {
		return nil, nil, err
	}
	if err := repositoryFS.MkdirAllPrivate(directory, 0o700); err != nil {
		return nil, nil, fmt.Errorf("create upload session directory: %w", err)
	}
	opened, err := repositoryFS.OpenPrivateFile(privatePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("create staging file: %w", err)
	}
	keepOpen = true
	return &StagingFile{
		ID:           sessionID.String(),
		RepositoryID: repository.RepoID,
		PrivatePath:  privatePath.String(),
		Filename:     filename,
		CreatedAt:    time.Now().UTC(),
	}, newRepositoryFile(opened, repositoryFS), nil
}

func (sm *DefaultStagingManager) OpenStagingFile(repository repo.Repository, stagingFile *StagingFile) (*RepositoryFile, error) {
	repositoryPath, err := validateStagingHandle(repository, stagingFile)
	if err != nil {
		return nil, err
	}
	repositoryFS, err := sm.files.Open(repository)
	if err != nil {
		return nil, err
	}
	opened, err := repositoryFS.OpenPrivateFile(repositoryPath, os.O_RDWR, 0)
	if err != nil {
		_ = repositoryFS.Close()
		return nil, err
	}
	return newRepositoryFile(opened, repositoryFS), nil
}

// OpenStagingSibling opens another staging file of the same repository for
// reading through the root this file already holds. Leases are not reentrant:
// opening it with OpenStagingFile would wait behind any lifecycle mutation
// queued since this file was opened, and that mutation waits for this file.
func (f *RepositoryFile) OpenStagingSibling(repository repo.Repository, stagingFile *StagingFile) (*os.File, error) {
	repositoryPath, err := validateStagingHandle(repository, stagingFile)
	if err != nil {
		return nil, err
	}
	if f == nil || f.repositoryFS == nil || f.repositoryFS.RepositoryID() != repository.RepoID {
		return nil, errors.New("staging sibling belongs to a different repository handle")
	}
	return f.repositoryFS.OpenPrivateFile(repositoryPath, os.O_RDONLY, 0)
}

// RemoveStagingFile removes a staged file, and its session directory once
// nothing else is left in it. A quarantined file is kept for a decision.
func (sm *DefaultStagingManager) RemoveStagingFile(repository repo.Repository, stagingFile *StagingFile) error {
	repositoryPath, err := validateStagingHandle(repository, stagingFile)
	if errors.Is(err, errStagingQuarantined) {
		return nil
	}
	if err != nil {
		return err
	}
	repositoryFS, err := sm.files.Open(repository)
	if err != nil {
		return err
	}
	defer repositoryFS.Close()
	if err := repositoryFS.RemovePrivate(repositoryPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove staging file: %w", err)
	}
	removeEmptySession(repositoryFS, repositoryPath)
	return nil
}

func (sm *DefaultStagingManager) CommitStagingFile(repository repo.Repository, stagingFile *StagingFile, finalPath string) error {
	source, err := validateStagingHandle(repository, stagingFile)
	if err != nil {
		return err
	}
	destination, err := layout.ParseMedia(finalPath)
	if err != nil {
		return err
	}
	repositoryFS, err := sm.files.Open(repository)
	if err != nil {
		return err
	}
	defer repositoryFS.Close()

	parent, err := layout.ParseMedia(path.Dir(destination.String()))
	if err != nil {
		return err
	}
	if err := repositoryFS.MkdirAllMedia(parent, 0o755); err != nil {
		return fmt.Errorf("create inbox directory: %w", err)
	}
	if err := repositoryFS.movePrivateNoReplace(source, destination); err != nil {
		return fmt.Errorf("commit staged file: %w", err)
	}
	removeEmptySession(repositoryFS, source)
	return nil
}

func (sm *DefaultStagingManager) CommitStagingFileToInbox(repository repo.Repository, stagingFile *StagingFile, contentHash string) (string, error) {
	if _, err := validateStagingHandle(repository, stagingFile); err != nil {
		return "", err
	}
	target, err := sm.ResolveInboxPath(repository, stagingFile.Filename, contentHash)
	if err != nil {
		return "", err
	}
	if err := sm.CommitStagingFile(repository, stagingFile, target); err != nil {
		return "", err
	}
	return target, nil
}

func (sm *DefaultStagingManager) ResolveInboxPath(repository repo.Repository, originalFilename, contentHash string) (string, error) {
	repositoryFS, err := sm.files.Open(repository)
	if err != nil {
		return "", err
	}
	defer repositoryFS.Close()
	config, err := repositoryConfig(repositoryFS)
	if err != nil {
		return "", err
	}
	return sm.resolveInboxRelativePath(repositoryFS, config, originalFilename, contentHash)
}

func (sm *DefaultStagingManager) MoveStagingToFailed(repository repo.Repository, stagingFile *StagingFile) error {
	source, err := validateStagingHandle(repository, stagingFile)
	if errors.Is(err, errStagingQuarantined) {
		return nil
	}
	if err != nil {
		return err
	}
	sessionID, _ := layout.StagedSession(source)
	if err := sm.QuarantineSession(repository, sessionID); err != nil {
		return err
	}
	quarantined, err := layout.QuarantineUploadFile(sessionID, path.Base(source.String()))
	if err != nil {
		return err
	}
	stagingFile.PrivatePath = quarantined.String()
	return nil
}

func (sm *DefaultStagingManager) QuarantineSession(repository repo.Repository, sessionID uuid.UUID) error {
	session, err := layout.UploadSession(sessionID)
	if err != nil {
		return err
	}
	destination, err := layout.QuarantineUpload(sessionID)
	if err != nil {
		return err
	}
	repositoryFS, err := sm.files.Open(repository)
	if err != nil {
		return err
	}
	defer repositoryFS.Close()
	files, err := repositoryFS.ListPrivateFiles(context.Background(), session)
	if err != nil {
		return fmt.Errorf("inspect upload session: %w", err)
	}
	if len(files) == 0 {
		return nil
	}
	if err := repositoryFS.MkdirAllPrivate(destination, 0o700); err != nil {
		return err
	}
	for _, file := range files {
		target, err := layout.QuarantineUploadFile(sessionID, path.Base(file.Path.String()))
		if err != nil {
			return err
		}
		if err := repositoryFS.movePrivateNoReplace(file.Path, target); err != nil {
			return fmt.Errorf("quarantine upload session: %w", err)
		}
	}
	return repositoryFS.RemoveAllPrivate(session)
}

func (sm *DefaultStagingManager) CleanupStaging(repository repo.Repository, maxAge time.Duration, keep func(sessionID uuid.UUID) bool) error {
	repositoryFS, err := sm.files.Open(repository)
	if err != nil {
		return err
	}
	defer repositoryFS.Close()
	files, err := repositoryFS.ListPrivateFiles(context.Background(), layout.UploadsDir())
	if err != nil {
		return fmt.Errorf("read upload sessions: %w", err)
	}
	// A session's age is the age of its newest file.
	newest := map[uuid.UUID]time.Time{}
	for _, file := range files {
		sessionID, ok := layout.StagedSession(file.Path)
		if !ok {
			continue
		}
		if file.ModTime.After(newest[sessionID]) {
			newest[sessionID] = file.ModTime
		}
	}
	cutoff := time.Now().Add(-maxAge)
	for sessionID, modified := range newest {
		if !modified.Before(cutoff) || keep(sessionID) {
			continue
		}
		if session, err := layout.UploadSession(sessionID); err == nil {
			_ = repositoryFS.RemoveAllPrivate(session)
		}
	}
	return nil
}

// removeEmptySession removes the session directory of a staged file once
// nothing is left in it. A directory that still holds files stays.
func removeEmptySession(repositoryFS *RepositoryFS, staged layout.Path) {
	if sessionID, ok := layout.StagedSession(staged); ok {
		if session, err := layout.UploadSession(sessionID); err == nil {
			_ = repositoryFS.RemovePrivate(session)
		}
	}
}

var errStagingQuarantined = errors.New("staging file is quarantined")

func validateStagingHandle(repository repo.Repository, stagingFile *StagingFile) (layout.Path, error) {
	if stagingFile == nil {
		return layout.Path{}, errors.New("staging file is nil")
	}
	if stagingFile.RepositoryID != repository.RepoID {
		return layout.Path{}, fmt.Errorf("%w: staging=%s repository=%s", ErrRepositoryIDMismatch, stagingFile.RepositoryID, repository.RepoID)
	}
	staged, err := layout.ParseStaged(stagingFile.PrivatePath)
	if err != nil {
		return layout.Path{}, err
	}
	if layout.Classify(staged) == layout.Quarantine {
		return staged, errStagingQuarantined
	}
	return staged, nil
}

func repositoryConfig(repositoryFS *RepositoryFS) (*repocfg.RepositoryConfig, error) {
	root, done, err := repositoryFS.withRoot()
	if err != nil {
		return nil, err
	}
	defer done()
	data, err := root.ReadFile(".lumiliorepo")
	if err != nil {
		return nil, err
	}
	config, err := repocfg.ParseConfig(data)
	if err != nil {
		return nil, fmt.Errorf("load repository config: %w", err)
	}
	return config, nil
}

func (sm *DefaultStagingManager) resolveInboxRelativePath(repositoryFS *RepositoryFS, config *repocfg.RepositoryConfig, originalFilename, contentHash string) (string, error) {
	filename, err := portableBase(originalFilename)
	if err != nil {
		return "", err
	}
	strategy := strings.ToLower(config.StorageStrategy)
	if strategy == "cas" && len(contentHash) >= 6 {
		directory := path.Join(layout.Inbox, contentHash[0:2], contentHash[2:4], contentHash[4:6])
		return path.Join(directory, contentHash+path.Ext(filename)), nil
	}
	if strategy == "flat" {
		unique, err := uniqueInboxFilename(repositoryFS, layout.Inbox, filename, config.LocalSettings.HandleDuplicateFilenames)
		return path.Join(layout.Inbox, unique), err
	}
	now := time.Now().UTC()
	directory := path.Join(layout.Inbox, fmt.Sprintf("%d", now.Year()), fmt.Sprintf("%02d", now.Month()))
	unique, err := uniqueInboxFilename(repositoryFS, directory, filename, config.LocalSettings.HandleDuplicateFilenames)
	return path.Join(directory, unique), err
}

func uniqueInboxFilename(repositoryFS *RepositoryFS, directory, filename, mode string) (string, error) {
	available := func(candidate string) (bool, error) {
		repositoryPath, err := layout.ParseMedia(path.Join(directory, candidate))
		if err != nil {
			return false, err
		}
		_, err = repositoryFS.StatMedia(repositoryPath)
		if errors.Is(err, fs.ErrNotExist) {
			return true, nil
		}
		return false, err
	}
	if ok, err := available(filename); ok || err != nil {
		return filename, err
	}
	ext := path.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	if strings.EqualFold(mode, "uuid") {
		return fmt.Sprintf("%s_%s%s", base, uuid.NewString()[:8], ext), nil
	}
	for i := 1; i <= 999; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if ok, err := available(candidate); ok || err != nil {
			return candidate, err
		}
	}
	return fmt.Sprintf("%s_%s%s", base, uuid.NewString()[:8], ext), nil
}

func portableBase(filename string) (string, error) {
	base := path.Base(strings.ReplaceAll(strings.TrimSpace(filename), "\\", "/"))
	if base == "" || base == "." || base == "/" {
		return "", layout.ErrPathInvalid
	}
	if _, err := layout.ParseMedia(path.Join(layout.Inbox, base)); err != nil {
		return "", err
	}
	return base, nil
}

// MovePrivateFile moves one file inside the private tree without replacing
// an existing destination. The recovery service uses it to give a Studio
// record its reassociated Asset name.
func (r *RepositoryFS) MovePrivateFile(source, destination layout.Path) error {
	if !source.IsPrivate() || !destination.IsPrivate() {
		return layout.ErrPathNamespace
	}
	return r.movePrivateNoReplace(source, destination)
}

func (r *RepositoryFS) movePrivateNoReplace(source, destination layout.Path) error {
	if !source.IsPrivate() || (!destination.IsPrivate() && !destination.IsMedia()) {
		return layout.ErrPathNamespace
	}
	sourceLocal, err := source.Local()
	if err != nil {
		return err
	}
	destinationLocal, err := destination.Local()
	if err != nil {
		return err
	}
	root, done, err := r.withRoot()
	if err != nil {
		return err
	}
	defer done()

	if err := root.Link(sourceLocal, destinationLocal); err == nil {
		committed := false
		defer func() {
			if !committed {
				_ = root.Remove(destinationLocal)
			}
		}()
		sourceFile, openErr := root.OpenFile(sourceLocal, os.O_RDWR, 0)
		if openErr != nil {
			return openErr
		}
		syncErr := sourceFile.Sync()
		_ = sourceFile.Close()
		if syncErr != nil {
			return syncErr
		}
		if err := syncRootDirectory(root, path.Dir(destination.String())); err != nil {
			return err
		}
		if err := root.Remove(sourceLocal); err != nil {
			return err
		}
		committed = true
		return syncRootDirectory(root, path.Dir(source.String()))
	} else if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("destination already exists: %w", err)
	}

	sourceFile, err := root.Open(sourceLocal)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	info, err := sourceFile.Stat()
	if err != nil {
		return err
	}
	destinationFile, err := root.OpenFile(destinationLocal, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		_ = destinationFile.Close()
		if cleanup {
			_ = root.Remove(destinationLocal)
		}
	}()
	if _, err := io.Copy(destinationFile, sourceFile); err != nil {
		return err
	}
	if err := destinationFile.Sync(); err != nil {
		return err
	}
	if err := destinationFile.Close(); err != nil {
		return err
	}
	if err := syncRootDirectory(root, path.Dir(destination.String())); err != nil {
		return err
	}
	cleanup = false
	if err := root.Remove(sourceLocal); err != nil {
		return err
	}
	return syncRootDirectory(root, path.Dir(source.String()))
}

// syncRootDirectory syncs a root-relative directory, private or media. The
// root confines the name; Localize rejects a rooted or escaping one.
func syncRootDirectory(root *os.Root, directory string) error {
	name, err := filepath.Localize(directory)
	if err != nil {
		return fmt.Errorf("%w: %q", layout.ErrPathInvalid, directory)
	}
	opened, err := root.Open(name)
	if err != nil {
		return err
	}
	defer opened.Close()
	return syncRepositoryDirectory(opened)
}
