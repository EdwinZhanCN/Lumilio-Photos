package sync

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
)

// FileWatcher monitors filesystem changes in real-time
type FileWatcher struct {
	watcher      *fsnotify.Watcher
	store        *FileRecordStore
	repositories map[uuid.UUID]*WatchedRepository
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup

	// Debouncing
	debounceInterval time.Duration
	pendingChanges   map[string]*pendingChange
	changesMu        sync.Mutex
}

// WatchedRepository represents a repository being watched
type WatchedRepository struct {
	ID              uuid.UUID
	Path            string
	UserManagedPath string
	ScanGeneration  int64
}

// pendingChange holds information about a pending file change
type pendingChange struct {
	repoID    uuid.UUID
	filePath  string
	eventType fsnotify.Op
	timer     *time.Timer
}

// FileWatcherConfig holds configuration for the file watcher
type FileWatcherConfig struct {
	DebounceInterval time.Duration
}

// DefaultFileWatcherConfig returns the default configuration
func DefaultFileWatcherConfig() FileWatcherConfig {
	return FileWatcherConfig{
		DebounceInterval: 500 * time.Millisecond,
	}
}

// NewFileWatcher creates a new file watcher instance
func NewFileWatcher(store *FileRecordStore, config FileWatcherConfig) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	fw := &FileWatcher{
		watcher:          watcher,
		store:            store,
		repositories:     make(map[uuid.UUID]*WatchedRepository),
		ctx:              ctx,
		cancel:           cancel,
		debounceInterval: config.DebounceInterval,
		pendingChanges:   make(map[string]*pendingChange),
	}

	return fw, nil
}

// AddRepository adds a repository to watch
func (fw *FileWatcher) AddRepository(repoID uuid.UUID, repoPath string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	// Check if already watching
	if _, exists := fw.repositories[repoID]; exists {
		return fmt.Errorf("repository %s is already being watched", repoID)
	}

	// Construct user-managed path (use repository root)
	userManagedPath := repoPath

	// Check if user-managed directory exists
	if _, err := os.Stat(userManagedPath); os.IsNotExist(err) {
		return fmt.Errorf("repository directory does not exist: %s", userManagedPath)
	}

	// Add directory and all subdirectories to watcher
	err := fw.addDirectoryRecursive(userManagedPath)
	if err != nil {
		return fmt.Errorf("failed to add directory to watcher: %w", err)
	}

	// Store repository info
	fw.repositories[repoID] = &WatchedRepository{
		ID:              repoID,
		Path:            repoPath,
		UserManagedPath: userManagedPath,
		ScanGeneration:  time.Now().Unix(),
	}

	log.Printf("[FileWatcher] Now watching repository %s at %s", repoID, userManagedPath)

	return nil
}

// RemoveRepository stops watching a repository
func (fw *FileWatcher) RemoveRepository(repoID uuid.UUID) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	repo, exists := fw.repositories[repoID]
	if !exists {
		return fmt.Errorf("repository %s is not being watched", repoID)
	}

	// Remove directory from watcher
	err := fw.removeDirectoryRecursive(repo.UserManagedPath)
	if err != nil {
		log.Printf("[FileWatcher] Error removing directory from watcher: %v", err)
	}

	delete(fw.repositories, repoID)

	log.Printf("[FileWatcher] Stopped watching repository %s", repoID)

	return nil
}

// Start begins watching for file changes
func (fw *FileWatcher) Start() error {
	fw.wg.Add(1)
	go fw.watchLoop()

	log.Println("[FileWatcher] Started watching for file changes")

	return nil
}

// Stop stops the file watcher
func (fw *FileWatcher) Stop() error {
	log.Println("[FileWatcher] Stopping file watcher...")

	fw.cancel()
	fw.wg.Wait()

	err := fw.watcher.Close()
	if err != nil {
		return fmt.Errorf("failed to close watcher: %w", err)
	}

	log.Println("[FileWatcher] File watcher stopped")

	return nil
}

// watchLoop is the main event loop for processing file system events
func (fw *FileWatcher) watchLoop() {
	defer fw.wg.Done()

	for {
		select {
		case <-fw.ctx.Done():
			return

		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			fw.handleEvent(event)

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}

			log.Printf("[FileWatcher] Error: %v", err)
		}
	}
}

// handleEvent processes a file system event
func (fw *FileWatcher) handleEvent(event fsnotify.Event) {
	// Skip events for hidden files or temporary files
	if fw.shouldIgnoreFile(event.Name) {
		return
	}

	// Find which repository this file belongs to
	repoID, relPath, err := fw.findRepositoryForPath(event.Name)
	if err != nil {
		return // File not in any watched repository
	}

	// Handle directory creation - need to watch new directories
	if event.Op&fsnotify.Create == fsnotify.Create {
		info, err := os.Stat(event.Name)
		if err == nil && info.IsDir() {
			fw.addDirectoryRecursive(event.Name)
		}
	}

	// Debounce the change
	fw.debounceChange(repoID, relPath, event.Op)
}

// debounceChange debounces file changes to avoid processing rapid successive events
func (fw *FileWatcher) debounceChange(repoID uuid.UUID, filePath string, eventType fsnotify.Op) {
	fw.changesMu.Lock()
	defer fw.changesMu.Unlock()

	key := fmt.Sprintf("%s:%s", repoID, filePath)

	// Cancel existing timer if any
	if existing, exists := fw.pendingChanges[key]; exists {
		existing.timer.Stop()
	}

	// Create new timer
	timer := time.AfterFunc(fw.debounceInterval, func() {
		fw.processChange(repoID, filePath, eventType)

		// Remove from pending changes
		fw.changesMu.Lock()
		delete(fw.pendingChanges, key)
		fw.changesMu.Unlock()
	})

	fw.pendingChanges[key] = &pendingChange{
		repoID:    repoID,
		filePath:  filePath,
		eventType: eventType,
		timer:     timer,
	}
}

// processChange processes a debounced file change
func (fw *FileWatcher) processChange(repoID uuid.UUID, filePath string, eventType fsnotify.Op) {
	fw.mu.RLock()
	repo, exists := fw.repositories[repoID]
	fw.mu.RUnlock()

	if !exists {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullPath := filepath.Join(repo.UserManagedPath, filePath)

	switch {
	case eventType&fsnotify.Remove == fsnotify.Remove || eventType&fsnotify.Rename == fsnotify.Rename:
		// File deleted or renamed
		err := fw.store.DeleteFileRecord(ctx, repoID, filePath)
		if err != nil {
			log.Printf("[FileWatcher] Error deleting file record for %s: %v", filePath, err)
		} else {
			log.Printf("[FileWatcher] Removed file record: %s", filePath)
		}

	case eventType&fsnotify.Create == fsnotify.Create || eventType&fsnotify.Write == fsnotify.Write:
		// File created or modified
		info, err := os.Stat(fullPath)
		if err != nil {
			log.Printf("[FileWatcher] Error getting file info for %s: %v", fullPath, err)
			return
		}

		// Skip directories
		if info.IsDir() {
			return
		}

		// Calculate hash for the file
		hash, err := CalculateFileHash(fullPath)
		if err != nil {
			log.Printf("[FileWatcher] Error calculating hash for %s: %v", fullPath, err)
			return
		}

		// Create or update file record
		_, err = fw.store.UpsertFileRecord(ctx, repoID, filePath, info.Size(), info.ModTime(), &hash, repo.ScanGeneration)
		if err != nil {
			log.Printf("[FileWatcher] Error upserting file record for %s: %v", filePath, err)
		} else {
			log.Printf("[FileWatcher] Updated file record: %s (size: %d, hash: %s)", filePath, info.Size(), hash[:8])
		}
	}
}

// findRepositoryForPath finds which repository a file path belongs to
func (fw *FileWatcher) findRepositoryForPath(absPath string) (uuid.UUID, string, error) {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	for _, repo := range fw.repositories {
		if strings.HasPrefix(absPath, repo.UserManagedPath) {
			relPath, err := filepath.Rel(repo.UserManagedPath, absPath)
			if err != nil {
				continue
			}
			return repo.ID, relPath, nil
		}
	}

	return uuid.UUID{}, "", fmt.Errorf("path not in any watched repository")
}

// addDirectoryRecursive adds a directory and all its subdirectories to the watcher
func (fw *FileWatcher) addDirectoryRecursive(path string) error {
	return filepath.Walk(path, func(walkPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// Skip hidden directories and inbox
			base := filepath.Base(walkPath)
			if (strings.HasPrefix(base, ".") || base == "inbox") && walkPath != path {
				return filepath.SkipDir
			}

			err = fw.watcher.Add(walkPath)
			if err != nil {
				return fmt.Errorf("failed to watch directory %s: %w", walkPath, err)
			}
		}

		return nil
	})
}

// removeDirectoryRecursive removes a directory and all its subdirectories from the watcher
func (fw *FileWatcher) removeDirectoryRecursive(path string) error {
	return filepath.Walk(path, func(walkPath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue even if there's an error
		}

		if info.IsDir() {
			_ = fw.watcher.Remove(walkPath)
		}

		return nil
	})
}

// shouldIgnoreFile checks if a file should be ignored
func (fw *FileWatcher) shouldIgnoreFile(path string) bool {
	base := filepath.Base(path)

	// Ignore hidden files
	if strings.HasPrefix(base, ".") {
		return true
	}

	// Ignore temporary files
	if strings.HasSuffix(base, "~") || strings.HasSuffix(base, ".tmp") {
		return true
	}

	// Ignore system files
	if base == ".DS_Store" || base == "Thumbs.db" {
		return true
	}

	return false
}

// GetWatchedRepositories returns a list of currently watched repositories
func (fw *FileWatcher) GetWatchedRepositories() []uuid.UUID {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	repos := make([]uuid.UUID, 0, len(fw.repositories))
	for id := range fw.repositories {
		repos = append(repos, id)
	}

	return repos
}
