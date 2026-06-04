package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"server/config"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

const primaryRepositoryFolderName = "primary"

type primaryStorageRepositoryManager interface {
	GetRepositoryByPath(path string) (*repo.Repository, error)
	AddRepository(path string, defaultOwnerID *int32) (*repo.Repository, error)
	InitializeRepository(path string, config repocfg.RepositoryConfig, defaultOwnerID *int32) (*repo.Repository, error)
}

func initPrimaryStorage(repoManager primaryStorageRepositoryManager, logger *zap.Logger, storageConfig config.StorageConfig) error {
	if logger == nil {
		logger = zap.NewNop()
	}
	storagePath := storageConfig.Path
	storageRootPath, primaryRepoPath, err := resolvePrimaryStoragePaths(storagePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(storageRootPath, 0755); err != nil {
		return fmt.Errorf("failed to create storage root path %s: %w", storageRootPath, err)
	}

	storageStrategy := strings.TrimSpace(storageConfig.Strategy)
	if storageStrategy == "" {
		storageStrategy = "date" // Default to date strategy
	}

	preserve := storageConfig.PreserveFilename

	duplicateHandling := strings.TrimSpace(storageConfig.DuplicateHandling)
	if duplicateHandling == "" {
		duplicateHandling = "rename" // Default to rename
	}

	// Strict mode: repository root must not be STORAGE_PATH itself.
	// Primary repository is always STORAGE_PATH/primary.
	if repocfg.IsRepositoryRoot(storageRootPath) {
		return fmt.Errorf("legacy repository detected at STORAGE_PATH root (%s); move repository to %s", storageRootPath, primaryRepoPath)
	}

	// If a repository already exists at the primary path, register it if needed.
	if repocfg.IsRepositoryRoot(primaryRepoPath) {
		// If it's already registered in DB, we're done
		if existing, err := repoManager.GetRepositoryByPath(primaryRepoPath); err == nil {
			logger.Info("primary storage already initialized",
				zap.String("operation", "repository.primary"),
				zap.String("repository_path", primaryRepoPath),
				zap.String("repository_id", repoUUIDString(existing.RepoID)),
			)
			return nil
		}

		// Otherwise, register the existing repository
		existingRepo, err := repoManager.AddRepository(primaryRepoPath, nil)
		if err != nil {
			return fmt.Errorf("failed to register existing primary storage repository: %w", err)
		}

		logger.Info("primary storage registered",
			zap.String("operation", "repository.primary"),
			zap.String("repository_path", primaryRepoPath),
			zap.String("repository_id", repoUUIDString(existingRepo.RepoID)),
		)
		return nil
	}

	// Create repository configuration
	cfg := repocfg.NewRepositoryConfig(
		"Primary Storage",
		repocfg.WithStorageStrategy(storageStrategy),
		repocfg.WithLocalSettings(preserve, duplicateHandling),
	)

	// Initialize a new repository with the configuration
	repository, err := repoManager.InitializeRepository(primaryRepoPath, *cfg, nil)
	if err != nil {
		return fmt.Errorf("failed to initialize primary storage repository: %w", err)
	}

	logger.Info("primary storage initialized",
		zap.String("operation", "repository.primary"),
		zap.String("repository_path", primaryRepoPath),
		zap.String("repository_id", repoUUIDString(repository.RepoID)),
		zap.String("storage_strategy", storageStrategy),
		zap.String("duplicate_handling", duplicateHandling),
		zap.Bool("preserve_filename", preserve),
	)

	return nil
}

func resolvePrimaryStoragePaths(storagePath string) (string, string, error) {
	trimmed := strings.TrimSpace(storagePath)
	if trimmed == "" {
		return "", "", fmt.Errorf("storage.path is required")
	}

	storageRootPath, err := filepath.Abs(filepath.Clean(trimmed))
	if err != nil {
		return "", "", fmt.Errorf("invalid STORAGE_PATH %q: %w", storagePath, err)
	}

	primaryRepoPath := filepath.Join(storageRootPath, primaryRepositoryFolderName)
	return storageRootPath, primaryRepoPath, nil
}

func repoUUIDString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return uuid.UUID(id.Bytes).String()
}
