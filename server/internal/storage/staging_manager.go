package storage

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"

	"server/internal/db/repo"
	"server/internal/storage/repocfg"

	"github.com/google/uuid"
)

// StagingManager is the private-workspace owner for upload and cloud ingest.
// Staging handles contain no host path and remain valid across repository
// relocation because every operation reopens the current catalog location.
type StagingManager interface {
	CreateStagingFile(repository repo.Repository, filename string) (*StagingFile, *RepositoryFile, error)
	OpenStagingFile(repository repo.Repository, stagingFile *StagingFile) (*RepositoryFile, error)
	RemoveStagingFile(repository repo.Repository, stagingFile *StagingFile) error
	CommitStagingFile(repository repo.Repository, stagingFile *StagingFile, finalPath string) error
	CommitStagingFileToInbox(repository repo.Repository, stagingFile *StagingFile, contentHash string) (string, error)
	ResolveInboxPath(repository repo.Repository, originalFilename, contentHash string) (string, error)
	MoveStagingToFailed(repository repo.Repository, stagingFile *StagingFile) error
	CleanupStaging(repository repo.Repository, maxAge time.Duration) error
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
	base, err := portableBase(filename)
	if err != nil {
		return nil, nil, err
	}
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

	directory, _ := ParsePrivateRepositoryPath(DefaultStructure.IncomingDir)
	if err := repositoryFS.MkdirAllPrivate(directory, 0o700); err != nil {
		return nil, nil, fmt.Errorf("create incoming staging directory: %w", err)
	}
	id := uuid.NewString()
	privatePath, err := ParsePrivateRepositoryPath(path.Join(DefaultStructure.IncomingDir, id+"_"+base))
	if err != nil {
		return nil, nil, err
	}
	opened, err := repositoryFS.OpenPrivateFile(privatePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("create staging file: %w", err)
	}
	keepOpen = true
	return &StagingFile{
		ID:           id,
		RepositoryID: repository.RepoID,
		PrivatePath:  privatePath.String(),
		Filename:     base,
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

func (sm *DefaultStagingManager) RemoveStagingFile(repository repo.Repository, stagingFile *StagingFile) error {
	repositoryPath, err := validateStagingHandle(repository, stagingFile)
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
	return nil
}

func (sm *DefaultStagingManager) CommitStagingFile(repository repo.Repository, stagingFile *StagingFile, finalPath string) error {
	source, err := validateStagingHandle(repository, stagingFile)
	if err != nil {
		return err
	}
	destination, err := ParseUserMediaPath(finalPath)
	if err != nil {
		return err
	}
	repositoryFS, err := sm.files.Open(repository)
	if err != nil {
		return err
	}
	defer repositoryFS.Close()

	parent, err := ParseUserMediaPath(path.Dir(destination.String()))
	if err != nil {
		return err
	}
	if err := repositoryFS.MkdirAllMedia(parent, 0o755); err != nil {
		return fmt.Errorf("create inbox directory: %w", err)
	}
	if err := repositoryFS.movePrivateNoReplace(source, destination); err != nil {
		return fmt.Errorf("commit staged file: %w", err)
	}
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
	if err != nil {
		return err
	}
	repositoryFS, err := sm.files.Open(repository)
	if err != nil {
		return err
	}
	defer repositoryFS.Close()
	failedDir, _ := ParsePrivateRepositoryPath(DefaultStructure.FailedDir)
	if err := repositoryFS.MkdirAllPrivate(failedDir, 0o700); err != nil {
		return err
	}
	ext := path.Ext(stagingFile.Filename)
	name := strings.TrimSuffix(stagingFile.Filename, ext)
	shortID := stagingFile.ID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	failed, err := ParsePrivateRepositoryPath(path.Join(DefaultStructure.FailedDir,
		fmt.Sprintf("%s_%s_%s%s", name, time.Now().UTC().Format("20060102_150405"), shortID, ext)))
	if err != nil {
		return err
	}
	if err := repositoryFS.movePrivateNoReplace(source, failed); err != nil {
		return fmt.Errorf("quarantine staging file: %w", err)
	}
	stagingFile.PrivatePath = failed.String()
	return nil
}

func (sm *DefaultStagingManager) CleanupStaging(repository repo.Repository, maxAge time.Duration) error {
	repositoryFS, err := sm.files.Open(repository)
	if err != nil {
		return err
	}
	defer repositoryFS.Close()
	cutoff := time.Now().Add(-maxAge)
	for _, name := range []string{DefaultStructure.IncomingDir, DefaultStructure.FailedDir} {
		directory, _ := ParsePrivateRepositoryPath(name)
		entries, readErr := fs.ReadDir(repositoryFS.root.FS(), name)
		if errors.Is(readErr, fs.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return fmt.Errorf("read staging directory %s: %w", name, readErr)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, infoErr := entry.Info()
			if infoErr != nil || !info.ModTime().Before(cutoff) {
				continue
			}
			candidate, parseErr := ParsePrivateRepositoryPath(path.Join(directory.String(), entry.Name()))
			if parseErr == nil {
				_ = repositoryFS.RemovePrivate(candidate)
			}
		}
	}
	return nil
}

func validateStagingHandle(repository repo.Repository, stagingFile *StagingFile) (RepositoryPath, error) {
	if stagingFile == nil {
		return RepositoryPath{}, errors.New("staging file is nil")
	}
	if stagingFile.RepositoryID != repository.RepoID {
		return RepositoryPath{}, fmt.Errorf("%w: staging=%s repository=%s", ErrRepositoryIDMismatch, stagingFile.RepositoryID, repository.RepoID)
	}
	return ParsePrivateRepositoryPath(stagingFile.PrivatePath)
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
		directory := path.Join(DefaultStructure.InboxDir, contentHash[0:2], contentHash[2:4], contentHash[4:6])
		return path.Join(directory, contentHash+path.Ext(filename)), nil
	}
	if strategy == "flat" {
		unique, err := uniqueInboxFilename(repositoryFS, DefaultStructure.InboxDir, filename, config.LocalSettings.HandleDuplicateFilenames)
		return path.Join(DefaultStructure.InboxDir, unique), err
	}
	now := time.Now().UTC()
	directory := path.Join(DefaultStructure.InboxDir, fmt.Sprintf("%d", now.Year()), fmt.Sprintf("%02d", now.Month()))
	unique, err := uniqueInboxFilename(repositoryFS, directory, filename, config.LocalSettings.HandleDuplicateFilenames)
	return path.Join(directory, unique), err
}

func uniqueInboxFilename(repositoryFS *RepositoryFS, directory, filename, mode string) (string, error) {
	available := func(candidate string) (bool, error) {
		repositoryPath, err := ParseUserMediaPath(path.Join(directory, candidate))
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
		return "", ErrRepositoryPathInvalid
	}
	if _, err := ParseUserMediaPath(path.Join(DefaultStructure.InboxDir, base)); err != nil {
		return "", err
	}
	return base, nil
}

func (r *RepositoryFS) movePrivateNoReplace(source, destination RepositoryPath) error {
	if !source.isPrivate() || (!destination.isPrivate() && !destination.isUserMedia()) {
		return ErrRepositoryPathNamespace
	}
	sourceLocal, err := source.local()
	if err != nil {
		return err
	}
	destinationLocal, err := destination.local()
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

func syncRootDirectory(root *os.Root, directory string) error {
	local, err := ParsePrivateRepositoryPath(directory)
	if err != nil {
		local, err = ParseUserMediaPath(directory)
		if err != nil {
			return err
		}
	}
	name, err := local.local()
	if err != nil {
		return err
	}
	opened, err := root.Open(name)
	if err != nil {
		return err
	}
	defer opened.Close()
	return syncRepositoryDirectory(opened)
}
