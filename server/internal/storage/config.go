package storage

import (
	"fmt"
	"os"
	"strings"
)

// StorageConfig holds configuration for storage services
type StorageConfig struct {
	BasePath string          `json:"base_path"`
	Strategy StorageStrategy `json:"strategy"`
	Options  StorageOptions  `json:"options"`
}

// StorageOptions holds additional options for storage strategies
type StorageOptions struct {
	// PreserveOriginalFilename whether to preserve original filename in storage
	PreserveOriginalFilename bool `json:"preserve_original_filename"`
	// HandleDuplicateFilenames how to handle files with same name
	// "rename" = add (1), (2) suffix, "uuid" = add UUID, "overwrite" = replace existing
	HandleDuplicateFilenames string `json:"handle_duplicate_filenames"`
	// MaxFileSize maximum file size in bytes (0 = no limit)
	MaxFileSize int64 `json:"max_file_size"`
	// CompressFiles whether to compress files before storage
	CompressFiles bool `json:"compress_files"`
	// CreateBackups whether to create backup copies
	CreateBackups bool `json:"create_backups"`
}

// DefaultStorageConfig returns default storage configuration
func DefaultStorageConfig() StorageConfig {
	// Check if we're in development mode
	isDev := strings.ToLower(os.Getenv("ENV")) == "development" ||
		strings.ToLower(os.Getenv("ENVIRONMENT")) == "development" ||
		os.Getenv("DEV_MODE") == "true"

	basePath := "/app/data/photos" // Default container path
	if isDev {
		basePath = "./data/photos" // Development path
	}

	return StorageConfig{
		BasePath: basePath,
		Strategy: StorageStrategyDate,
		Options: StorageOptions{
			PreserveOriginalFilename: true,
			HandleDuplicateFilenames: "rename", // "rename", "uuid", "overwrite"
			MaxFileSize:              0,        // No limit
			CompressFiles:            false,
			CreateBackups:            false,
		},
	}
}

// LoadStorageConfigFromEnv loads storage configuration from environment variables
func LoadStorageConfigFromEnv() StorageConfig {
	config := DefaultStorageConfig()

	// Load base path from environment
	if basePath := os.Getenv("STORAGE_PATH"); basePath != "" {
		config.BasePath = basePath
	}

	// Load strategy from environment
	if strategy := os.Getenv("STORAGE_STRATEGY"); strategy != "" {
		config.Strategy = StorageStrategy(strategy)
	}

	// Load options from environment
	if preserveFilename := os.Getenv("STORAGE_PRESERVE_FILENAME"); preserveFilename == "false" {
		config.Options.PreserveOriginalFilename = false
	}

	if duplicateHandling := os.Getenv("STORAGE_DUPLICATE_HANDLING"); duplicateHandling != "" {
		config.Options.HandleDuplicateFilenames = duplicateHandling
	}

	return config
}

// Validate validates the storage configuration
func (c *StorageConfig) Validate() error {
	if c.BasePath == "" {
		return fmt.Errorf("storage base path cannot be empty")
	}

	switch c.Strategy {
	case StorageStrategyDate, StorageStrategyCAS, StorageStrategyFlat:
		// Valid strategies
	default:
		return fmt.Errorf("invalid storage strategy: %s", c.Strategy)
	}

	if c.Options.MaxFileSize < 0 {
		return fmt.Errorf("max file size cannot be negative")
	}

	return nil
}

// GetStrategy returns the storage strategy
func (c *StorageConfig) GetStrategy() StorageStrategy {
	return c.Strategy
}

// SetStrategy sets the storage strategy
func (c *StorageConfig) SetStrategy(strategy StorageStrategy) {
	c.Strategy = strategy
}

// GetDescription returns a human-readable description of the strategy
func (s StorageStrategy) GetDescription() string {
	switch s {
	case StorageStrategyDate:
		return "Date-based organization (YYYY/MM) - Easy to browse chronologically"
	case StorageStrategyCAS:
		return "Content-addressable storage - Automatic deduplication, hash-based paths"
	case StorageStrategyFlat:
		return "Flat structure - All files in one directory"
	default:
		return "Unknown storage strategy"
	}
}

// GetExamplePath returns an example path for the strategy
func (s StorageStrategy) GetExamplePath() string {
	switch s {
	case StorageStrategyDate:
		return "2024/01/photo.jpg"
	case StorageStrategyCAS:
		return "ab/cd/ef/abcdef123456789..."
	case StorageStrategyFlat:
		return "photo_uuid.jpg"
	default:
		return "unknown"
	}
}

// NewStorageWithConfig creates a new storage instance with the given configuration
func NewStorageWithConfig(config StorageConfig) (Storage, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid storage config: %w", err)
	}

	return NewLocalStorageWithConfig(config)
}
