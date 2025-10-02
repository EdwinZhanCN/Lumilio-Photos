package sync

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ReconciliationScanner performs daily filesystem reconciliation
type ReconciliationScanner struct {
	store          *FileRecordStore
	opStore        *SyncOperationStore
	scanGeneration int64
}

// ReconciliationConfig holds configuration for reconciliation
type ReconciliationConfig struct {
	BatchSize       int
	MaxConcurrency  int
	CalculateHashes bool
}

// DefaultReconciliationConfig returns default configuration
func DefaultReconciliationConfig() ReconciliationConfig {
	return ReconciliationConfig{
		BatchSize:       100,
		MaxConcurrency:  4,
		CalculateHashes: true,
	}
}

// NewReconciliationScanner creates a new reconciliation scanner
func NewReconciliationScanner(store *FileRecordStore, opStore *SyncOperationStore) *ReconciliationScanner {
	return &ReconciliationScanner{
		store:          store,
		opStore:        opStore,
		scanGeneration: time.Now().Unix(),
	}
}

// ReconcileRepository performs full reconciliation for a repository
func (rs *ReconciliationScanner) ReconcileRepository(ctx context.Context, repoID uuid.UUID, repoPath string, config ReconciliationConfig) error {
	log.Printf("[Reconciliation] Starting reconciliation for repository %s", repoID)

	startTime := time.Now()
	stats := &SyncStats{}

	// Create sync operation record
	operation, err := rs.opStore.CreateSyncOperation(ctx, repoID, "reconciliation", startTime)
	if err != nil {
		log.Printf("[Reconciliation] Failed to create sync operation: %v", err)
		return err
	}

	// Perform reconciliation
	reconcileErr := rs.reconcile(ctx, repoID, repoPath, config, stats)

	// Update operation with results
	endTime := time.Now()
	duration := endTime.Sub(startTime).Milliseconds()

	status := "completed"
	var errorMessage *string
	if reconcileErr != nil {
		status = "failed"
		errMsg := reconcileErr.Error()
		errorMessage = &errMsg
	}

	err = rs.opStore.UpdateSyncOperation(ctx, operation.ID, *stats, endTime, duration, status, errorMessage)
	if err != nil {
		log.Printf("[Reconciliation] Failed to update sync operation: %v", err)
	}

	if reconcileErr != nil {
		log.Printf("[Reconciliation] Failed: %v", reconcileErr)
		return reconcileErr
	}

	log.Printf("[Reconciliation] Completed for repository %s - Scanned: %d, Added: %d, Updated: %d, Removed: %d (took %dms)",
		repoID, stats.FilesScanned, stats.FilesAdded, stats.FilesUpdated, stats.FilesRemoved, duration)

	return nil
}

// reconcile performs the actual reconciliation work
func (rs *ReconciliationScanner) reconcile(ctx context.Context, repoID uuid.UUID, repoPath string, config ReconciliationConfig, stats *SyncStats) error {
	userManagedPath := filepath.Join(repoPath, "user")

	// Check if user-managed directory exists
	if _, err := os.Stat(userManagedPath); os.IsNotExist(err) {
		return fmt.Errorf("user-managed directory does not exist: %s", userManagedPath)
	}

	// Increment scan generation
	rs.scanGeneration++

	// Scan filesystem and update database
	err := rs.scanDirectory(ctx, repoID, userManagedPath, config, stats)
	if err != nil {
		return fmt.Errorf("failed to scan directory: %w", err)
	}

	// Remove orphaned records (files that no longer exist)
	removed, err := rs.store.DeleteOrphanedRecords(ctx, repoID, rs.scanGeneration)
	if err != nil {
		return fmt.Errorf("failed to delete orphaned records: %w", err)
	}

	stats.FilesRemoved = int(removed)

	return nil
}

// scanDirectory recursively scans a directory and updates file records
func (rs *ReconciliationScanner) scanDirectory(ctx context.Context, repoID uuid.UUID, basePath string, config ReconciliationConfig, stats *SyncStats) error {
	batch := make([]FileRecordData, 0, config.BatchSize)

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("[Reconciliation] Error accessing path %s: %v", path, err)
			return nil // Continue walking
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip directories
		if info.IsDir() {
			// Skip hidden directories
			if strings.HasPrefix(filepath.Base(path), ".") && path != basePath {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip files we should ignore
		if rs.shouldIgnoreFile(path) {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			log.Printf("[Reconciliation] Error getting relative path for %s: %v", path, err)
			return nil
		}

		stats.FilesScanned++

		// Check if file record exists
		existing, err := rs.store.GetFileRecord(ctx, repoID, relPath)
		if err != nil && !strings.Contains(err.Error(), "no rows") {
			log.Printf("[Reconciliation] Error getting file record for %s: %v", relPath, err)
			return nil
		}

		// Determine if we need to update
		needsUpdate := false
		var contentHash *string

		if existing == nil {
			// New file
			needsUpdate = true
			stats.FilesAdded++
		} else if existing.FileSize != info.Size() || !existing.ModTime.Time.Equal(info.ModTime()) {
			// File changed
			needsUpdate = true
			stats.FilesUpdated++
		}

		// Calculate hash if needed
		if needsUpdate && config.CalculateHashes {
			hash, err := CalculateFileHash(path)
			if err != nil {
				log.Printf("[Reconciliation] Error calculating hash for %s: %v", relPath, err)
			} else {
				contentHash = &hash
			}
		} else if existing != nil {
			// Keep existing hash
			contentHash = existing.ContentHash
		}

		// Create file record data
		recordData := FileRecordData{
			FilePath:       relPath,
			FileSize:       info.Size(),
			ModTime:        info.ModTime(),
			ContentHash:    contentHash,
			ScanGeneration: rs.scanGeneration,
		}

		batch = append(batch, recordData)

		// Flush batch if full
		if len(batch) >= config.BatchSize {
			if err := rs.store.BatchUpsertFileRecords(ctx, repoID, batch); err != nil {
				log.Printf("[Reconciliation] Error upserting batch: %v", err)
			}
			batch = batch[:0] // Clear batch
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Flush remaining records
	if len(batch) > 0 {
		if err := rs.store.BatchUpsertFileRecords(ctx, repoID, batch); err != nil {
			return fmt.Errorf("failed to upsert final batch: %w", err)
		}
	}

	return nil
}

// shouldIgnoreFile checks if a file should be ignored during scanning
func (rs *ReconciliationScanner) shouldIgnoreFile(path string) bool {
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

	// Ignore common backup extensions
	if strings.HasSuffix(base, ".bak") || strings.HasSuffix(base, ".swp") {
		return true
	}

	return false
}

// PerformStartupSync performs an initial sync when the system starts
func (rs *ReconciliationScanner) PerformStartupSync(ctx context.Context, repoID uuid.UUID, repoPath string) error {
	log.Printf("[Reconciliation] Starting startup sync for repository %s", repoID)

	startTime := time.Now()
	stats := &SyncStats{}

	// Create sync operation record
	operation, err := rs.opStore.CreateSyncOperation(ctx, repoID, "startup", startTime)
	if err != nil {
		log.Printf("[Reconciliation] Failed to create sync operation: %v", err)
		return err
	}

	// Use default config for startup
	config := DefaultReconciliationConfig()

	// Perform reconciliation
	reconcileErr := rs.reconcile(ctx, repoID, repoPath, config, stats)

	// Update operation with results
	endTime := time.Now()
	duration := endTime.Sub(startTime).Milliseconds()

	status := "completed"
	var errorMessage *string
	if reconcileErr != nil {
		status = "failed"
		errMsg := reconcileErr.Error()
		errorMessage = &errMsg
	}

	err = rs.opStore.UpdateSyncOperation(ctx, operation.ID, *stats, endTime, duration, status, errorMessage)
	if err != nil {
		log.Printf("[Reconciliation] Failed to update sync operation: %v", err)
	}

	if reconcileErr != nil {
		log.Printf("[Reconciliation] Startup sync failed: %v", reconcileErr)
		return reconcileErr
	}

	log.Printf("[Reconciliation] Startup sync completed for repository %s - Scanned: %d, Added: %d, Updated: %d, Removed: %d (took %dms)",
		repoID, stats.FilesScanned, stats.FilesAdded, stats.FilesUpdated, stats.FilesRemoved, duration)

	return nil
}
