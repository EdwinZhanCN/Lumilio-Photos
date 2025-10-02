package sync

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IntegrationExample demonstrates how to integrate the sync system with the repository manager
// This file is for documentation purposes and shows the recommended integration patterns

// Example 1: Initialize sync manager at application startup
func ExampleInitializeSyncManager() {
	// Create database connection pool (usually done in main.go)
	dbURL := "postgresql://user:pass@localhost:5432/lumilio"
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Create sync manager with custom configuration
	config := SyncManagerConfig{
		ReconciliationInterval: 24 * 60 * 60 * 1000000000, // 24 hours in nanoseconds
		FileWatcherConfig: FileWatcherConfig{
			DebounceInterval: 500 * 1000000, // 500ms in nanoseconds
		},
		ReconciliationConfig: ReconciliationConfig{
			BatchSize:       100,
			MaxConcurrency:  4,
			CalculateHashes: true,
		},
	}

	syncManager, err := NewSyncManager(pool, config)
	if err != nil {
		log.Fatalf("Failed to create sync manager: %v", err)
	}

	// Start the sync manager
	err = syncManager.Start()
	if err != nil {
		log.Fatalf("Failed to start sync manager: %v", err)
	}
	defer syncManager.Stop()

	log.Println("Sync manager initialized and started successfully")
}

// Example 2: Integrate with repository manager - AddRepository
func ExampleRepositoryManagerAddRepository(syncManager *SyncManager) {
	// This shows how to modify your repository manager's AddRepository method

	// Assume we have these from the repository manager
	repoID := uuid.New()
	repoPath := "/path/to/repository"

	// After successfully adding the repository to the database,
	// add it to the sync manager
	err := syncManager.AddRepository(repoID, repoPath)
	if err != nil {
		// Don't fail the entire operation if sync fails
		// Just log the error and continue
		log.Printf("Warning: Failed to add repository to sync manager: %v", err)
	} else {
		log.Printf("Repository %s successfully added to sync manager", repoID)
	}
}

// Example 3: Integrate with repository manager - RemoveRepository
func ExampleRepositoryManagerRemoveRepository(syncManager *SyncManager) {
	repoID := uuid.New()

	// After successfully removing the repository from the database,
	// remove it from the sync manager
	err := syncManager.RemoveRepository(repoID)
	if err != nil {
		log.Printf("Warning: Failed to remove repository from sync manager: %v", err)
	} else {
		log.Printf("Repository %s successfully removed from sync manager", repoID)
	}
}

// Example 4: Query sync status for a repository
func ExampleQuerySyncStatus(syncManager *SyncManager) {
	ctx := context.Background()
	repoID := uuid.New()

	// Get comprehensive sync status
	status, err := syncManager.GetSyncStatus(ctx, repoID)
	if err != nil {
		log.Printf("Failed to get sync status: %v", err)
		return
	}

	// Display status information
	log.Printf("Sync Status for Repository %s:", status.RepositoryID)
	log.Printf("  Total Files Tracked: %d", status.TotalFiles)
	log.Printf("  Currently Watching: %v", status.IsWatching)

	if status.LastSyncOperation != nil {
		log.Printf("  Last Sync Operation:")
		log.Printf("    Type: %s", status.LastSyncOperation.OperationType)

		statusStr := "unknown"
		if status.LastSyncOperation.Status != nil {
			statusStr = *status.LastSyncOperation.Status
		}
		log.Printf("    Status: %s", statusStr)

		startTime := "unknown"
		if status.LastSyncOperation.StartTime.Valid {
			startTime = status.LastSyncOperation.StartTime.Time.Format("2006-01-02 15:04:05")
		}
		log.Printf("    Started: %s", startTime)

		if status.LastSyncOperation.FilesScanned != nil {
			log.Printf("    Files Scanned: %d", *status.LastSyncOperation.FilesScanned)
		}
		if status.LastSyncOperation.FilesAdded != nil {
			log.Printf("    Files Added: %d", *status.LastSyncOperation.FilesAdded)
		}
		if status.LastSyncOperation.FilesUpdated != nil {
			log.Printf("    Files Updated: %d", *status.LastSyncOperation.FilesUpdated)
		}
		if status.LastSyncOperation.FilesRemoved != nil {
			log.Printf("    Files Removed: %d", *status.LastSyncOperation.FilesRemoved)
		}

		if status.LastSyncOperation.DurationMs != nil {
			log.Printf("    Duration: %dms", *status.LastSyncOperation.DurationMs)
		}

		if status.LastSyncOperation.ErrorMessage != nil {
			log.Printf("    Error: %s", *status.LastSyncOperation.ErrorMessage)
		}
	}
}

// Example 5: Manually trigger reconciliation
func ExampleManualReconciliation(syncManager *SyncManager) {
	repoID := uuid.New()
	repoPath := "/path/to/repository"

	log.Printf("Triggering manual reconciliation for repository %s", repoID)

	err := syncManager.TriggerReconciliation(repoID, repoPath)
	if err != nil {
		log.Printf("Manual reconciliation failed: %v", err)
		return
	}

	log.Println("Manual reconciliation completed successfully")
}

// Example 6: List file records for a repository
func ExampleListFileRecords(syncManager *SyncManager) {
	ctx := context.Background()
	repoID := uuid.New()

	records, err := syncManager.ListFileRecords(ctx, repoID)
	if err != nil {
		log.Printf("Failed to list file records: %v", err)
		return
	}

	log.Printf("Found %d file records for repository %s:", len(records), repoID)

	for i, record := range records {
		hashPreview := "none"
		if record.ContentHash != nil && len(*record.ContentHash) >= 8 {
			hashPreview = (*record.ContentHash)[:8] + "..."
		}

		scannedTime := "unknown"
		if record.LastScanned.Valid {
			scannedTime = record.LastScanned.Time.Format("2006-01-02 15:04:05")
		}
		log.Printf("  [%d] %s (size: %d bytes, hash: %s, scanned: %s)",
			i+1,
			record.FilePath,
			record.FileSize,
			hashPreview,
			scannedTime,
		)

		// Only show first 10 to avoid spam
		if i >= 9 {
			log.Printf("  ... and %d more files", len(records)-10)
			break
		}
	}
}

// Example 7: Get sync operation history
func ExampleGetSyncHistory(syncManager *SyncManager) {
	ctx := context.Background()
	repoID := uuid.New()

	// Get last 10 sync operations
	operations, err := syncManager.GetSyncOperations(ctx, repoID, 10)
	if err != nil {
		log.Printf("Failed to get sync operations: %v", err)
		return
	}

	log.Printf("Recent sync operations for repository %s:", repoID)

	for i, op := range operations {
		statusIcon := "✓"
		if op.Status != nil && *op.Status == "failed" {
			statusIcon = "✗"
		} else if op.Status != nil && *op.Status == "running" {
			statusIcon = "→"
		}

		duration := "N/A"
		if op.DurationMs != nil {
			duration = fmt.Sprintf("%dms", *op.DurationMs)
		}

		startTime := "unknown"
		if op.StartTime.Valid {
			startTime = op.StartTime.Time.Format("2006-01-02 15:04:05")
		}

		log.Printf("  %s [%d] %s sync at %s (%s)",
			statusIcon,
			i+1,
			op.OperationType,
			startTime,
			duration,
		)

		filesScanned := int32(0)
		if op.FilesScanned != nil {
			filesScanned = *op.FilesScanned
		}
		filesAdded := int32(0)
		if op.FilesAdded != nil {
			filesAdded = *op.FilesAdded
		}
		filesUpdated := int32(0)
		if op.FilesUpdated != nil {
			filesUpdated = *op.FilesUpdated
		}
		filesRemoved := int32(0)
		if op.FilesRemoved != nil {
			filesRemoved = *op.FilesRemoved
		}

		log.Printf("      Changes: +%d ~%d -%d files (scanned: %d)",
			filesAdded,
			filesUpdated,
			filesRemoved,
			filesScanned,
		)

		if op.ErrorMessage != nil {
			log.Printf("      Error: %s", *op.ErrorMessage)
		}
	}
}

// Example 8: API endpoint for sync status
func ExampleAPIEndpoint(syncManager *SyncManager) {
	// Example of how to create an API endpoint in Gin
	// In your actual API router setup:

	/*
		router.GET("/api/repositories/:id/sync/status", func(c *gin.Context) {
			repoIDStr := c.Param("id")
			repoID, err := uuid.Parse(repoIDStr)
			if err != nil {
				c.JSON(400, gin.H{"error": "invalid repository ID"})
				return
			}

			status, err := syncManager.GetSyncStatus(c.Request.Context(), repoID)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}

			c.JSON(200, status)
		})

		router.POST("/api/repositories/:id/sync/reconcile", func(c *gin.Context) {
			repoIDStr := c.Param("id")
			repoID, err := uuid.Parse(repoIDStr)
			if err != nil {
				c.JSON(400, gin.H{"error": "invalid repository ID"})
				return
			}

			// Get repository path from database
			var repoPath string
			err = pool.QueryRow(c.Request.Context(),
				"SELECT path FROM repositories WHERE repo_id = $1",
				repoID).Scan(&repoPath)
			if err != nil {
				c.JSON(404, gin.H{"error": "repository not found"})
				return
			}

			// Trigger reconciliation in background
			go func() {
				err := syncManager.TriggerReconciliation(repoID, repoPath)
				if err != nil {
					log.Printf("Background reconciliation failed: %v", err)
				}
			}()

			c.JSON(202, gin.H{"message": "reconciliation started"})
		})

		router.GET("/api/repositories/:id/sync/files", func(c *gin.Context) {
			repoIDStr := c.Param("id")
			repoID, err := uuid.Parse(repoIDStr)
			if err != nil {
				c.JSON(400, gin.H{"error": "invalid repository ID"})
				return
			}

			records, err := syncManager.ListFileRecords(c.Request.Context(), repoID)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}

			c.JSON(200, gin.H{
				"repository_id": repoID,
				"file_count":    len(records),
				"files":         records,
			})
		})

		router.GET("/api/repositories/:id/sync/history", func(c *gin.Context) {
			repoIDStr := c.Param("id")
			repoID, err := uuid.Parse(repoIDStr)
			if err != nil {
				c.JSON(400, gin.H{"error": "invalid repository ID"})
				return
			}

			limit := 20
			if limitStr := c.Query("limit"); limitStr != "" {
				fmt.Sscanf(limitStr, "%d", &limit)
				if limit > 100 {
					limit = 100
				}
			}

			operations, err := syncManager.GetSyncOperations(c.Request.Context(), repoID, limit)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}

			c.JSON(200, gin.H{
				"repository_id": repoID,
				"operations":    operations,
			})
		})
	*/

	log.Println("See code comments for API endpoint examples")
}

// Example 9: Graceful shutdown
func ExampleGracefulShutdown(syncManager *SyncManager) {
	// In your main.go, handle graceful shutdown

	/*
		// Set up signal handling
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		// Wait for shutdown signal
		<-sigChan
		log.Println("Received shutdown signal, cleaning up...")

		// Stop sync manager (this will stop file watcher and wait for operations to complete)
		err := syncManager.Stop()
		if err != nil {
			log.Printf("Error during sync manager shutdown: %v", err)
		}

		log.Println("Sync manager stopped cleanly")
	*/
}

// Example 10: Complete integration with repository manager
type RepositoryManagerWithSync struct {
	// Your existing repository manager fields
	syncManager *SyncManager
}

func NewRepositoryManagerWithSync(pool *pgxpool.Pool) (*RepositoryManagerWithSync, error) {
	// Create sync manager
	config := DefaultSyncManagerConfig()
	syncManager, err := NewSyncManager(pool, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync manager: %w", err)
	}

	// Start sync manager
	err = syncManager.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start sync manager: %w", err)
	}

	return &RepositoryManagerWithSync{
		syncManager: syncManager,
	}, nil
}

func (rm *RepositoryManagerWithSync) AddRepository(repoID uuid.UUID, repoPath string) error {
	// 1. Add repository to database (your existing logic)
	// ... database operations ...

	// 2. Add to sync manager
	err := rm.syncManager.AddRepository(repoID, repoPath)
	if err != nil {
		log.Printf("Warning: Failed to add repository to sync manager: %v", err)
		// Don't fail the operation, sync can be added later
	}

	return nil
}

func (rm *RepositoryManagerWithSync) RemoveRepository(repoID uuid.UUID) error {
	// 1. Remove from sync manager first
	_ = rm.syncManager.RemoveRepository(repoID)

	// 2. Remove from database (your existing logic)
	// ... database operations ...

	return nil
}

func (rm *RepositoryManagerWithSync) GetSyncStatus(ctx context.Context, repoID uuid.UUID) (*SyncStatus, error) {
	return rm.syncManager.GetSyncStatus(ctx, repoID)
}

func (rm *RepositoryManagerWithSync) Shutdown() error {
	return rm.syncManager.Stop()
}
