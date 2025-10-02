package sync

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"server/internal/db/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SyncManager orchestrates file watching and reconciliation
type SyncManager struct {
	pool               *pgxpool.Pool
	queries            *repo.Queries
	fileWatcher        *FileWatcher
	reconciliation     *ReconciliationScanner
	fileRecordStore    *FileRecordStore
	syncOperationStore *SyncOperationStore

	// Scheduling
	reconciliationTicker *time.Ticker
	ctx                  context.Context
	cancel               context.CancelFunc
	wg                   sync.WaitGroup

	// Configuration
	reconciliationInterval time.Duration
}

// SyncManagerConfig holds configuration for the sync manager
type SyncManagerConfig struct {
	ReconciliationInterval time.Duration
	FileWatcherConfig      FileWatcherConfig
	ReconciliationConfig   ReconciliationConfig
}

// DefaultSyncManagerConfig returns the default configuration
func DefaultSyncManagerConfig() SyncManagerConfig {
	return SyncManagerConfig{
		ReconciliationInterval: 24 * time.Hour, // Daily reconciliation
		FileWatcherConfig:      DefaultFileWatcherConfig(),
		ReconciliationConfig:   DefaultReconciliationConfig(),
	}
}

// NewSyncManager creates a new sync manager instance
func NewSyncManager(pool *pgxpool.Pool, config SyncManagerConfig) (*SyncManager, error) {
	queries := repo.New(pool)

	fileRecordStore := NewFileRecordStore(queries)
	syncOperationStore := NewSyncOperationStore(queries)

	fileWatcher, err := NewFileWatcher(fileRecordStore, config.FileWatcherConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	reconciliation := NewReconciliationScanner(fileRecordStore, syncOperationStore)

	ctx, cancel := context.WithCancel(context.Background())

	sm := &SyncManager{
		pool:                   pool,
		queries:                queries,
		fileWatcher:            fileWatcher,
		reconciliation:         reconciliation,
		fileRecordStore:        fileRecordStore,
		syncOperationStore:     syncOperationStore,
		ctx:                    ctx,
		cancel:                 cancel,
		reconciliationInterval: config.ReconciliationInterval,
	}

	return sm, nil
}

// Start starts the sync manager
func (sm *SyncManager) Start() error {
	log.Println("[SyncManager] Starting sync manager...")

	// Start file watcher
	err := sm.fileWatcher.Start()
	if err != nil {
		return fmt.Errorf("failed to start file watcher: %w", err)
	}

	// Start reconciliation scheduler
	sm.startReconciliationScheduler()

	log.Println("[SyncManager] Sync manager started successfully")

	return nil
}

// Stop stops the sync manager
func (sm *SyncManager) Stop() error {
	log.Println("[SyncManager] Stopping sync manager...")

	sm.cancel()

	// Stop reconciliation scheduler
	if sm.reconciliationTicker != nil {
		sm.reconciliationTicker.Stop()
	}

	// Wait for background tasks
	sm.wg.Wait()

	// Stop file watcher
	err := sm.fileWatcher.Stop()
	if err != nil {
		log.Printf("[SyncManager] Error stopping file watcher: %v", err)
	}

	log.Println("[SyncManager] Sync manager stopped")

	return nil
}

// AddRepository adds a repository to be monitored
func (sm *SyncManager) AddRepository(repoID uuid.UUID, repoPath string) error {
	log.Printf("[SyncManager] Adding repository %s at %s", repoID, repoPath)

	// Perform startup sync
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	err := sm.reconciliation.PerformStartupSync(ctx, repoID, repoPath)
	if err != nil {
		return fmt.Errorf("failed to perform startup sync: %w", err)
	}

	// Add to file watcher
	err = sm.fileWatcher.AddRepository(repoID, repoPath)
	if err != nil {
		return fmt.Errorf("failed to add repository to file watcher: %w", err)
	}

	log.Printf("[SyncManager] Successfully added repository %s", repoID)

	return nil
}

// RemoveRepository stops monitoring a repository
func (sm *SyncManager) RemoveRepository(repoID uuid.UUID) error {
	log.Printf("[SyncManager] Removing repository %s", repoID)

	err := sm.fileWatcher.RemoveRepository(repoID)
	if err != nil {
		return fmt.Errorf("failed to remove repository from file watcher: %w", err)
	}

	log.Printf("[SyncManager] Successfully removed repository %s", repoID)

	return nil
}

// TriggerReconciliation manually triggers reconciliation for a repository
func (sm *SyncManager) TriggerReconciliation(repoID uuid.UUID, repoPath string) error {
	log.Printf("[SyncManager] Manually triggering reconciliation for repository %s", repoID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	config := DefaultReconciliationConfig()
	err := sm.reconciliation.ReconcileRepository(ctx, repoID, repoPath, config)
	if err != nil {
		return fmt.Errorf("reconciliation failed: %w", err)
	}

	return nil
}

// startReconciliationScheduler starts the daily reconciliation scheduler
func (sm *SyncManager) startReconciliationScheduler() {
	sm.reconciliationTicker = time.NewTicker(sm.reconciliationInterval)

	sm.wg.Add(1)
	go func() {
		defer sm.wg.Done()

		log.Printf("[SyncManager] Reconciliation scheduler started (interval: %v)", sm.reconciliationInterval)

		for {
			select {
			case <-sm.ctx.Done():
				return

			case <-sm.reconciliationTicker.C:
				sm.performScheduledReconciliation()
			}
		}
	}()
}

// performScheduledReconciliation performs reconciliation for all watched repositories
func (sm *SyncManager) performScheduledReconciliation() {
	log.Println("[SyncManager] Starting scheduled reconciliation for all repositories")

	watchedRepos := sm.fileWatcher.GetWatchedRepositories()

	for _, repoID := range watchedRepos {
		// Get repository path from database
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

		// Query repository path
		var repoPath string
		query := `SELECT path FROM repositories WHERE repo_id = $1`
		err := sm.pool.QueryRow(ctx, query, repoID).Scan(&repoPath)

		if err != nil {
			log.Printf("[SyncManager] Failed to get repository path for %s: %v", repoID, err)
			cancel()
			continue
		}

		// Perform reconciliation
		config := DefaultReconciliationConfig()
		err = sm.reconciliation.ReconcileRepository(ctx, repoID, repoPath, config)
		if err != nil {
			log.Printf("[SyncManager] Reconciliation failed for repository %s: %v", repoID, err)
		}

		cancel()
	}

	log.Println("[SyncManager] Scheduled reconciliation completed for all repositories")
}

// GetFileRecord retrieves a file record
func (sm *SyncManager) GetFileRecord(ctx context.Context, repoID uuid.UUID, filePath string) (*repo.FileRecord, error) {
	return sm.fileRecordStore.GetFileRecord(ctx, repoID, filePath)
}

// ListFileRecords lists all file records for a repository
func (sm *SyncManager) ListFileRecords(ctx context.Context, repoID uuid.UUID) ([]repo.FileRecord, error) {
	return sm.fileRecordStore.ListFileRecords(ctx, repoID)
}

// GetFileRecordCount returns the count of file records for a repository
func (sm *SyncManager) GetFileRecordCount(ctx context.Context, repoID uuid.UUID) (int64, error) {
	return sm.fileRecordStore.GetFileRecordCount(ctx, repoID)
}

// GetSyncOperations returns recent sync operations for a repository
func (sm *SyncManager) GetSyncOperations(ctx context.Context, repoID uuid.UUID, limit int) ([]repo.SyncOperation, error) {
	return sm.syncOperationStore.ListSyncOperations(ctx, repoID, limit)
}

// GetLatestSyncOperation returns the most recent sync operation for a repository
func (sm *SyncManager) GetLatestSyncOperation(ctx context.Context, repoID uuid.UUID) (*repo.SyncOperation, error) {
	return sm.syncOperationStore.GetLatestSyncOperation(ctx, repoID)
}

// GetSyncStatus returns the current sync status for a repository
func (sm *SyncManager) GetSyncStatus(ctx context.Context, repoID uuid.UUID) (*SyncStatus, error) {
	fileCount, err := sm.fileRecordStore.GetFileRecordCount(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file record count: %w", err)
	}

	latestOp, err := sm.syncOperationStore.GetLatestSyncOperation(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest sync operation: %w", err)
	}

	status := &SyncStatus{
		RepositoryID:      repoID,
		TotalFiles:        fileCount,
		IsWatching:        sm.isWatching(repoID),
		LastSyncOperation: latestOp,
	}

	return status, nil
}

// isWatching checks if a repository is currently being watched
func (sm *SyncManager) isWatching(repoID uuid.UUID) bool {
	watchedRepos := sm.fileWatcher.GetWatchedRepositories()
	for _, id := range watchedRepos {
		if id == repoID {
			return true
		}
	}
	return false
}

// SyncStatus represents the current sync status of a repository
type SyncStatus struct {
	RepositoryID      uuid.UUID
	TotalFiles        int64
	IsWatching        bool
	LastSyncOperation *repo.SyncOperation
}
