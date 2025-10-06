package storage

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"
	"server/internal/sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ValidationResult represents the result of repository validation
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

type RepositoryManager interface {
	// Validation
	ValidateRepository(path string) (*ValidationResult, error)

	// Repository lifecycle CRUD
	InitializeRepository(path string, config repocfg.RepositoryConfig) (*repo.Repository, error)
	AddRepository(path string) (*repo.Repository, error)
	GetRepository(id string) (*repo.Repository, error)
	GetRepositoryByPath(path string) (*repo.Repository, error)
	ListRepositories() ([]*repo.Repository, error)
	RemoveRepository(id string) error
	RemoveRepositories(ids []string) error
	UpdateRepository(id string, config repocfg.RepositoryConfig) (*repo.Repository, error)

	// Repository-Asset relationship (path-based)
	GetRepositoryAssetStats(repoID string, ownerID *int32) (*RepositoryAssetStats, error)

	// Configuration management
	LoadConfig(repoPath string) (*repocfg.RepositoryConfig, error)
	SaveConfig(repoPath string, config *repocfg.RepositoryConfig) error

	// Validation
	IsNestedRepository(path string) (bool, string, error)

	// Helper methods
	GetRepositoryPath(repoID string) (string, error)

	// Staging operations (delegated to staging manager)
	GetStagingManager() StagingManager

	// Sync operations
	GetSyncStatus(ctx context.Context, repoID string) (*sync.SyncStatus, error)
	TriggerSync(ctx context.Context, repoID string) error
	IsSyncEnabled() bool
	StopSync() error
}

// RepositoryAssetStats contains statistics about assets in a repository
type RepositoryAssetStats struct {
	TotalAssets  int64      `json:"total_assets"`
	PhotoCount   int64      `json:"photo_count"`
	VideoCount   int64      `json:"video_count"`
	AudioCount   int64      `json:"audio_count"`
	LikedCount   int64      `json:"liked_count"`
	RatedCount   int64      `json:"rated_count"`
	AvgRating    *float64   `json:"avg_rating"`
	TotalSize    *int64     `json:"total_size"`
	OldestUpload *time.Time `json:"oldest_upload"`
	NewestUpload *time.Time `json:"newest_upload"`
}

// DefaultRepositoryManager implements the RepositoryManager interface
type DefaultRepositoryManager struct {
	queries        *repo.Queries
	dirManager     DirectoryManager
	stagingManager StagingManager
	syncManager    *sync.SyncManager
	pool           *pgxpool.Pool
}

// NewRepositoryManager creates a new repository manager instance
// If pool is nil, sync will be disabled
func NewRepositoryManager(queries *repo.Queries, pool *pgxpool.Pool, enableSync bool) (RepositoryManager, error) {
	rm := &DefaultRepositoryManager{
		queries:        queries,
		dirManager:     NewDirectoryManager(),
		stagingManager: NewStagingManager(),
		pool:           pool,
	}

	// Initialize sync manager if enabled and pool is available
	if enableSync && pool != nil {
		config := sync.DefaultSyncManagerConfig()
		syncManager, err := sync.NewSyncManager(pool, config)
		if err != nil {
			return nil, fmt.Errorf("failed to create sync manager: %w", err)
		}

		err = syncManager.Start()
		if err != nil {
			return nil, fmt.Errorf("failed to start sync manager: %w", err)
		}

		rm.syncManager = syncManager
		log.Println("✅ Sync system enabled and started")
	} else {
		log.Println("⚠️  Sync system disabled")
	}

	return rm, nil
}

// AddRepository registers an existing repository with the system
func (rm *DefaultRepositoryManager) AddRepository(path string) (*repo.Repository, error) {
	// Clean and validate path
	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Validate that this is a valid repository
	result, err := rm.ValidateRepository(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to validate repository: %w", err)
	}
	if !result.Valid {
		return nil, fmt.Errorf("invalid repository at %s: %v", cleanPath, result.Errors)
	}

	// Check if repository is already registered
	_, err = rm.GetRepositoryByPath(cleanPath)
	if err == nil {
		return nil, fmt.Errorf("repository at %s is already registered", cleanPath)
	}

	// Load configuration
	config, err := repocfg.LoadConfigFromFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load repository configuration: %w", err)
	}

	// Check if repository ID already exists
	_, err = rm.GetRepository(config.ID)
	if err == nil {
		return nil, fmt.Errorf("repository with ID %s is already registered", config.ID)
	}

	repoUUID, err := uuid.Parse(config.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid repository ID: %w", err)
	}

	now := time.Now()
	dbRepo, err := rm.queries.CreateRepository(context.Background(), repo.CreateRepositoryParams{
		RepoID:    pgtype.UUID{Bytes: repoUUID, Valid: true},
		Name:      config.Name,
		Path:      cleanPath,
		Config:    *config,
		Status:    dbtypes.RepoStatusActive,
		CreatedAt: pgtype.Timestamptz{Time: config.CreatedAt, Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create database record: %w", err)
	}

	// Initialize sync if enabled
	if rm.syncManager != nil {
		err = rm.syncManager.AddRepository(repoUUID, cleanPath)
		if err != nil {
			log.Printf("⚠️  Failed to add repository to sync manager: %v", err)
			// Don't fail the operation, just log warning
		} else {
			log.Printf("✅ Sync enabled for repository %s", config.ID)
		}
	}

	return &dbRepo, nil
}

// ValidateRepository validates a repository at the given path
func (rm *DefaultRepositoryManager) ValidateRepository(path string) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Invalid path: %v", err))
		return result, nil
	}

	// Check if directory exists
	info, err := os.Stat(cleanPath)
	if os.IsNotExist(err) {
		result.Valid = false
		result.Errors = append(result.Errors, "Repository directory does not exist")
		return result, nil
	}

	if !info.IsDir() {
		result.Valid = false
		result.Errors = append(result.Errors, "Path is not a directory")
		return result, nil
	}

	// Check for .lumiliorepo file
	configPath := filepath.Join(cleanPath, ".lumiliorepo")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		result.Valid = false
		result.Errors = append(result.Errors, "Missing .lumiliorepo configuration file")
		return result, nil
	}

	// Validate configuration file
	config, err := repocfg.LoadConfigFromFile(cleanPath)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Invalid configuration: %v", err))
		return result, nil
	}

	// Use directory manager for structure validation
	structureValidation, err := rm.dirManager.ValidateStructure(cleanPath)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Directory structure validation failed: %v", err))
	} else {
		if !structureValidation.Valid {
			result.Valid = false
		}
		result.Errors = append(result.Errors, structureValidation.InvalidPaths...)
		result.Warnings = append(result.Warnings, structureValidation.Warnings...)
	}

	// Check for nested repositories
	isNested, parentRepo, err := rm.IsNestedRepository(cleanPath)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Could not check for nested repositories: %v", err))
	} else if isNested {
		result.Errors = append(result.Errors, fmt.Sprintf("Repository is nested inside another repository at: %s", parentRepo))
		result.Valid = false
	}

	// Validate configuration values
	if config.Version == "" {
		result.Errors = append(result.Errors, "Configuration missing version")
		result.Valid = false
	}

	if config.ID == "" {
		result.Errors = append(result.Errors, "Configuration missing repository ID")
		result.Valid = false
	} else {
		// Validate UUID format
		if _, err := uuid.Parse(config.ID); err != nil {
			result.Errors = append(result.Errors, "Repository ID is not a valid UUID")
			result.Valid = false
		}
	}

	if config.Name == "" {
		result.Errors = append(result.Errors, "Configuration missing repository name")
		result.Valid = false
	}

	// Check permissions
	if err := rm.checkDirectoryPermissions(cleanPath); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Permission issues: %v", err))
	}

	return result, nil
}

// IsNestedRepository checks if a repository path is nested inside another repository
func (rm *DefaultRepositoryManager) IsNestedRepository(path string) (bool, string, error) {
	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false, "", err
	}

	// Start from parent directory and walk up
	currentPath := filepath.Dir(cleanPath)
	for currentPath != "/" && currentPath != "." && len(currentPath) > 1 {
		// Check if current directory has a .lumiliorepo file
		configPath := filepath.Join(currentPath, ".lumiliorepo")
		if _, err := os.Stat(configPath); err == nil {
			return true, currentPath, nil
		}

		// Move up one directory
		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			break // Reached root
		}
		currentPath = parentPath
	}

	return false, "", nil
}

// LoadConfig loads repository configuration from the given path
func (rm *DefaultRepositoryManager) LoadConfig(repoPath string) (*repocfg.RepositoryConfig, error) {
	return repocfg.LoadConfigFromFile(repoPath)
}

// SaveConfig saves repository configuration to the given path
func (rm *DefaultRepositoryManager) SaveConfig(repoPath string, config *repocfg.RepositoryConfig) error {
	return config.SaveConfigToFile(repoPath)
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}

// checkDirectoryPermissions checks if we have proper read/write permissions
func (rm *DefaultRepositoryManager) checkDirectoryPermissions(path string) error {
	// Test read permission
	if _, err := os.Open(path); err != nil {
		return fmt.Errorf("cannot read directory: %w", err)
	}

	// Test write permission by trying to create a temporary file
	tempFile := filepath.Join(path, ".lumilio_permission_test")
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("cannot write to directory: %w", err)
	}
	file.Close()
	os.Remove(tempFile) // Clean up

	return nil
}

// InitializeRepository creates a new repository with full directory structure
func (rm *DefaultRepositoryManager) InitializeRepository(path string, config repocfg.RepositoryConfig) (*repo.Repository, error) {
	// Clean and validate path
	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Check if repository already exists
	if repocfg.IsRepositoryRoot(cleanPath) {
		return nil, fmt.Errorf("repository already exists at %s", cleanPath)
	}

	// Check for nested repositories
	isNested, parentRepo, err := rm.IsNestedRepository(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check for nested repositories: %w", err)
	}
	if isNested {
		return nil, fmt.Errorf("cannot create repository inside existing repository at %s", parentRepo)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create directory structure using directory manager
	if err := rm.dirManager.CreateStructure(cleanPath); err != nil {
		return nil, fmt.Errorf("failed to create repository structure: %w", err)
	}

	// Save configuration file
	if err := config.SaveConfigToFile(cleanPath); err != nil {
		// Clean up on failure
		os.RemoveAll(cleanPath)
		return nil, fmt.Errorf("failed to save configuration: %w", err)
	}

	repoUUID, err := uuid.Parse(config.ID)
	if err != nil {
		// Clean up on failure
		os.RemoveAll(cleanPath)
		return nil, fmt.Errorf("invalid repository ID: %w", err)
	}

	now := time.Now()
	dbRepo, err := rm.queries.CreateRepository(context.Background(), repo.CreateRepositoryParams{
		RepoID:    pgtype.UUID{Bytes: repoUUID, Valid: true},
		Name:      config.Name,
		Path:      cleanPath,
		Config:    config,
		Status:    dbtypes.RepoStatusActive,
		CreatedAt: pgtype.Timestamptz{Time: config.CreatedAt, Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		// Clean up on failure
		os.RemoveAll(cleanPath)
		return nil, fmt.Errorf("failed to create database record: %w", err)
	}

	// Initialize sync if enabled
	if rm.syncManager != nil {
		err = rm.syncManager.AddRepository(repoUUID, cleanPath)
		if err != nil {
			log.Printf("⚠️  Failed to add repository to sync manager: %v", err)
			// Don't fail the operation, just log warning
		} else {
			log.Printf("✅ Sync enabled for repository %s", config.ID)
		}
	}

	return &dbRepo, nil
}

func (rm *DefaultRepositoryManager) GetRepository(id string) (*repo.Repository, error) {
	repoUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid repository ID: %w", err)
	}

	dbRepo, err := rm.queries.GetRepository(context.Background(), pgtype.UUID{Bytes: repoUUID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("repository not found: %w", err)
	}

	return &dbRepo, nil
}

func (rm *DefaultRepositoryManager) GetRepositoryByPath(path string) (*repo.Repository, error) {
	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	dbRepo, err := rm.queries.GetRepositoryByPath(context.Background(), cleanPath)
	if err != nil {
		return nil, fmt.Errorf("repository not found at path %s: %w", cleanPath, err)
	}

	return &dbRepo, nil
}

func (rm *DefaultRepositoryManager) ListRepositories() ([]*repo.Repository, error) {
	repos, err := rm.queries.ListRepositories(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	result := make([]*repo.Repository, len(repos))
	for i := range repos {
		result[i] = &repos[i]
	}

	return result, nil
}

func (rm *DefaultRepositoryManager) RemoveRepository(id string) error {
	repoUUID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid repository ID: %w", err)
	}

	// Remove from sync manager first
	if rm.syncManager != nil {
		err = rm.syncManager.RemoveRepository(repoUUID)
		if err != nil {
			log.Printf("⚠️  Failed to remove repository from sync manager: %v", err)
		}
	}

	err = rm.queries.DeleteRepository(context.Background(), pgtype.UUID{Bytes: repoUUID, Valid: true})
	if err != nil {
		return fmt.Errorf("failed to remove repository: %w", err)
	}

	return nil
}

// GetStagingManager returns the staging manager instance
func (rm *DefaultRepositoryManager) GetStagingManager() StagingManager {
	return rm.stagingManager
}

// GetSyncStatus returns the sync status for a repository
func (rm *DefaultRepositoryManager) GetSyncStatus(ctx context.Context, repoID string) (*sync.SyncStatus, error) {
	if rm.syncManager == nil {
		return nil, fmt.Errorf("sync is not enabled")
	}

	repoUUID, err := uuid.Parse(repoID)
	if err != nil {
		return nil, fmt.Errorf("invalid repository ID: %w", err)
	}

	return rm.syncManager.GetSyncStatus(ctx, repoUUID)
}

// TriggerSync manually triggers reconciliation for a repository
func (rm *DefaultRepositoryManager) TriggerSync(ctx context.Context, repoID string) error {
	if rm.syncManager == nil {
		return fmt.Errorf("sync is not enabled")
	}

	repoUUID, err := uuid.Parse(repoID)
	if err != nil {
		return fmt.Errorf("invalid repository ID: %w", err)
	}

	repository, err := rm.GetRepository(repoID)
	if err != nil {
		return err
	}

	return rm.syncManager.TriggerReconciliation(repoUUID, repository.Path)
}

// IsSyncEnabled checks if the sync system is enabled
func (rm *DefaultRepositoryManager) IsSyncEnabled() bool {
	return rm.syncManager != nil
}

// StopSync stops the sync manager
func (rm *DefaultRepositoryManager) StopSync() error {
	if rm.syncManager == nil {
		return nil
	}

	return rm.syncManager.Stop()
}

func (rm *DefaultRepositoryManager) RemoveRepositories(ids []string) error {
	uuids := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		repoUUID, err := uuid.Parse(id)
		if err != nil {
			return fmt.Errorf("invalid repository ID %s: %w", id, err)
		}
		uuids[i] = pgtype.UUID{Bytes: repoUUID, Valid: true}
	}

	err := rm.queries.DeleteRepositories(context.Background(), uuids)
	if err != nil {
		return fmt.Errorf("failed to remove repositories: %w", err)
	}

	return nil
}

func (rm *DefaultRepositoryManager) UpdateRepository(id string, config repocfg.RepositoryConfig) (*repo.Repository, error) {
	repoUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid repository ID: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Update database record
	now := time.Now()
	dbRepo, err := rm.queries.UpdateRepository(context.Background(), repo.UpdateRepositoryParams{
		RepoID:    pgtype.UUID{Bytes: repoUUID, Valid: true},
		Name:      config.Name,
		Config:    config,
		Status:    dbtypes.RepoStatusActive,
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update repository: %w", err)
	}

	// Update configuration file
	if err := config.SaveConfigToFile(dbRepo.Path); err != nil {
		return nil, fmt.Errorf("failed to update configuration file: %w", err)
	}

	return &dbRepo, nil
}

// GetRepositoryAssetStats returns comprehensive statistics about assets in a repository
func (rm *DefaultRepositoryManager) GetRepositoryAssetStats(repoID string, ownerID *int32) (*RepositoryAssetStats, error) {
	// Get repository to find its path
	repository, err := rm.GetRepository(repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	// Get repository asset statistics
	stats, err := rm.queries.GetRepositoryAssetStats(context.Background(), repo.GetRepositoryAssetStatsParams{
		RepoPath: &repository.Path,
		OwnerID:  ownerID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get asset stats for repository: %w", err)
	}

	result := &RepositoryAssetStats{
		TotalAssets: stats.TotalAssets,
		PhotoCount:  stats.PhotoCount,
		VideoCount:  stats.VideoCount,
		AudioCount:  stats.AudioCount,
		LikedCount:  stats.LikedCount,
		RatedCount:  stats.RatedCount,
	}

	// Handle non-zero values (SQLite aggregates return 0 for empty sets)
	if stats.AvgRating != 0 {
		result.AvgRating = &stats.AvgRating
	}

	if stats.TotalSize != 0 {
		result.TotalSize = &stats.TotalSize
	}

	// Handle timestamp interfaces - need to convert from interface{}
	if stats.OldestUpload != nil {
		if oldest, ok := stats.OldestUpload.(time.Time); ok {
			result.OldestUpload = &oldest
		}
	}

	if stats.NewestUpload != nil {
		if newest, ok := stats.NewestUpload.(time.Time); ok {
			result.NewestUpload = &newest
		}
	}

	return result, nil
}

// GetRepositoryPath returns the repository path for use in asset filtering
func (rm *DefaultRepositoryManager) GetRepositoryPath(repoID string) (string, error) {
	repository, err := rm.GetRepository(repoID)
	if err != nil {
		return "", fmt.Errorf("failed to get repository: %w", err)
	}
	return repository.Path, nil
}
