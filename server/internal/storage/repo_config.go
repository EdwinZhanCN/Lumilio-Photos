package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// DefaultIgnorePatterns provides a comprehensive list of files and patterns to ignore during scanning
var DefaultIgnorePatterns = []string{
	".DS_Store",
	"Thumbs.db",
	"ehthumbs.db",
	"Icon?",
	"Icon\r",
	"*.tmp",
	"*.temp",
	"*.part",
	"*.partial",
	"*.wbk",
	"*.bak",
	"*.orig",
	"*~",
	"~$*",
	"*.swp",
	"*.swo",
	".*.swp",
	".lumilio",
	".Trash",
	".Trashes",
	".fseventsd",
	".Spotlight-V100",
	".TemporaryItems",
	"lost+found",
	"desktop.ini",
	"*._*",
	"npm-debug.log",
	"yarn-error.log",
}

// RepositoryConfig represents the complete .lumiliorepo configuration file structure
type RepositoryConfig struct {
	Version   string    `yaml:"version" json:"version"`
	ID        string    `yaml:"id" json:"id"`
	Name      string    `yaml:"name" json:"name"`
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`

	// Storage configuration
	StorageStrategy string        `yaml:"storage_strategy" json:"storage_strategy"` // "date", "cas", "flat" date -> yyyy/mm/IMG_001.jpg (month based)
	SyncSettings    SyncSettings  `yaml:"sync_settings" json:"sync_settings"`
	LocalSettings   LocalSettings `yaml:"local_settings" json:"local_settings"`
}

// SyncSettings configures how the repository synchronizes with the filesystem
type SyncSettings struct {
	// QuickScanInterval how often to perform quick metadata scans (e.g., "5m", "2m")
	QuickScanInterval string `yaml:"quick_scan_interval" json:"quick_scan_interval"`
	// FullScanInterval how often to perform full content verification (e.g., "30m", "1h")
	FullScanInterval string `yaml:"full_scan_interval" json:"full_scan_interval"`
	// IgnorePatterns list of file patterns to ignore during scanning
	IgnorePatterns []string `yaml:"ignore_patterns" json:"ignore_patterns"`
}

// LocalSettings configures repository-specific behavior
type LocalSettings struct {
	// PreserveOriginalFilename whether to preserve original filename in storage
	PreserveOriginalFilename bool `yaml:"preserve_original_filename" json:"preserve_original_filename"`
	// HandleDuplicateFilenames how to handle files with same name
	// "rename" = add (1), (2) suffix, "uuid" = add UUID, "overwrite" = replace existing
	HandleDuplicateFilenames string `yaml:"handle_duplicate_filenames" json:"handle_duplicate_filenames"`
	// MaxFileSize maximum file size in bytes (0 = no limit)
	MaxFileSize int64 `yaml:"max_file_size" json:"max_file_size"`
	// CompressFiles whether to compress files before storage
	CompressFiles bool `yaml:"compress_files" json:"compress_files"`
	// CreateBackups whether to create backup copies
	CreateBackups bool `yaml:"create_backups" json:"create_backups"`
	// BackupPath where to store backup copies (if CreateBackups is true)
	BackupPath string `yaml:"backup_path,omitempty" json:"backup_path,omitempty"`
}

// DefaultRepositoryConfig returns a sensible default configuration template
// Note: This does not include ID, Name, or CreatedAt as these should be unique per repository
func DefaultRepositoryConfig() *RepositoryConfig {
	return &RepositoryConfig{
		Version:         "1.0",
		StorageStrategy: "date",
		SyncSettings: SyncSettings{
			QuickScanInterval: "5m",
			FullScanInterval:  "30m",
			IgnorePatterns:    DefaultIgnorePatterns,
		},
		LocalSettings: LocalSettings{
			PreserveOriginalFilename: true,
			HandleDuplicateFilenames: "uuid",
			MaxFileSize:              0, // No limit
			CompressFiles:            false,
			CreateBackups:            false,
		},
	}
}

// RepositoryConfigOption defines a function for setting configuration options
type RepositoryConfigOption func(*RepositoryConfig)

// WithStorageStrategy sets the storage strategy for the repository
func WithStorageStrategy(strategy string) RepositoryConfigOption {
	return func(config *RepositoryConfig) {
		config.StorageStrategy = strategy
	}
}

// WithSyncSettings sets the sync settings for the repository
func WithSyncSettings(quickInterval, fullInterval string, ignorePatterns []string) RepositoryConfigOption {
	return func(config *RepositoryConfig) {
		config.SyncSettings.QuickScanInterval = quickInterval
		config.SyncSettings.FullScanInterval = fullInterval
		if ignorePatterns != nil {
			config.SyncSettings.IgnorePatterns = ignorePatterns
		}
	}
}

// WithLocalSettings sets the local settings for the repository
func WithLocalSettings(preserveFilename bool, duplicateHandling string, maxSize int64, compress, backup bool) RepositoryConfigOption {
	return func(config *RepositoryConfig) {
		config.LocalSettings.PreserveOriginalFilename = preserveFilename
		config.LocalSettings.HandleDuplicateFilenames = duplicateHandling
		config.LocalSettings.MaxFileSize = maxSize
		config.LocalSettings.CompressFiles = compress
		config.LocalSettings.CreateBackups = backup
	}
}

// WithBackupPath sets the backup path for file backups
func WithBackupPath(path string) RepositoryConfigOption {
	return func(config *RepositoryConfig) {
		config.LocalSettings.BackupPath = path
	}
}

// NewRepositoryConfig creates a new repository configuration with unique ID and current timestamp
//
// System-managed fields (always auto-generated):
//   - ID: Unique UUID generated automatically
//   - CreatedAt: Current timestamp when config is created
//   - Version: Set to current version ("1.0")
//
// User-configurable fields via options:
//   - StorageStrategy: How files are organized ("date", "cas", "flat")
//   - SyncSettings: Scan intervals and ignore patterns
//   - LocalSettings: File handling preferences
//
// Additional options can be provided to customize the configuration
func NewRepositoryConfig(name string, options ...RepositoryConfigOption) *RepositoryConfig {
	config := DefaultRepositoryConfig()
	config.ID = uuid.New().String()
	config.Name = name
	config.CreatedAt = time.Now()

	// Apply all provided options
	for _, option := range options {
		option(config)
	}

	return config
}

// NewDefaultRepositoryConfig creates a repository with default settings and the given name
// This is a convenience function equivalent to NewRepositoryConfig(name)
func NewDefaultRepositoryConfig(name string) *RepositoryConfig {
	return NewRepositoryConfig(name)
}

// LoadConfigFromFile loads repository configuration from .lumiliorepo file
func LoadConfigFromFile(repoPath string) (*RepositoryConfig, error) {
	configPath := filepath.Join(repoPath, ".lumiliorepo")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("repository configuration not found at %s", configPath)
		}
		return nil, fmt.Errorf("failed to read repository config: %w", err)
	}

	var config RepositoryConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse repository config: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid repository configuration: %w", err)
	}

	return &config, nil
}

// SaveConfigToFile saves repository configuration to .lumiliorepo file, this function is also used for updating the configuration
func (rc *RepositoryConfig) SaveConfigToFile(repoPath string) error {
	configPath := filepath.Join(repoPath, ".lumiliorepo")

	// Validate before saving
	if err := rc.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	data, err := yaml.Marshal(rc)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	// Write with proper permissions (readable by owner/group, not world)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate checks if the repository configuration is valid
func (rc *RepositoryConfig) Validate() error {
	if rc.Version == "" {
		return fmt.Errorf("version is required")
	}

	if rc.ID == "" {
		return fmt.Errorf("repository ID is required")
	}

	if rc.Name == "" {
		return fmt.Errorf("repository name is required")
	}

	// Validate storage strategy
	validStrategies := map[string]bool{
		"date": true,
		"cas":  true,
		"flat": true,
	}
	if !validStrategies[rc.StorageStrategy] {
		return fmt.Errorf("invalid storage strategy '%s', must be one of: date, cas, flat", rc.StorageStrategy)
	}

	// Validate sync intervals
	if _, err := time.ParseDuration(rc.SyncSettings.QuickScanInterval); err != nil {
		return fmt.Errorf("invalid quick_scan_interval: %w", err)
	}

	if _, err := time.ParseDuration(rc.SyncSettings.FullScanInterval); err != nil {
		return fmt.Errorf("invalid full_scan_interval: %w", err)
	}

	// Validate duplicate handling strategy
	validDuplicateStrategies := map[string]bool{
		"rename":    true,
		"uuid":      true,
		"overwrite": true,
	}
	if !validDuplicateStrategies[rc.LocalSettings.HandleDuplicateFilenames] {
		return fmt.Errorf("invalid handle_duplicate_filenames '%s', must be one of: rename, uuid, overwrite", rc.LocalSettings.HandleDuplicateFilenames)
	}

	// Validate max file size
	if rc.LocalSettings.MaxFileSize < 0 {
		return fmt.Errorf("max_file_size cannot be negative")
	}

	return nil
}

// IsRepositoryRoot checks if a directory contains a .lumiliorepo file
func IsRepositoryRoot(path string) bool {
	configPath := filepath.Join(path, ".lumiliorepo")
	_, err := os.Stat(configPath)
	return err == nil
}

// GetQuickScanDuration returns the parsed quick scan interval duration
func (rc *RepositoryConfig) GetQuickScanDuration() (time.Duration, error) {
	return time.ParseDuration(rc.SyncSettings.QuickScanInterval)
}

// GetFullScanDuration returns the parsed full scan interval duration
func (rc *RepositoryConfig) GetFullScanDuration() (time.Duration, error) {
	return time.ParseDuration(rc.SyncSettings.FullScanInterval)
}

// Clone creates a deep copy of the repository configuration
func (rc *RepositoryConfig) Clone() *RepositoryConfig {
	clone := *rc

	// Deep copy slices
	if rc.SyncSettings.IgnorePatterns != nil {
		clone.SyncSettings.IgnorePatterns = make([]string, len(rc.SyncSettings.IgnorePatterns))
		copy(clone.SyncSettings.IgnorePatterns, rc.SyncSettings.IgnorePatterns)
	}

	return &clone
}

// MergeWithDefaults merges this config with default values for any missing fields
func (rc *RepositoryConfig) MergeWithDefaults() {
	defaults := DefaultRepositoryConfig()

	if rc.Version == "" {
		rc.Version = defaults.Version
	}
	if rc.StorageStrategy == "" {
		rc.StorageStrategy = defaults.StorageStrategy
	}
	if rc.SyncSettings.QuickScanInterval == "" {
		rc.SyncSettings.QuickScanInterval = defaults.SyncSettings.QuickScanInterval
	}
	if rc.SyncSettings.FullScanInterval == "" {
		rc.SyncSettings.FullScanInterval = defaults.SyncSettings.FullScanInterval
	}
	if rc.SyncSettings.IgnorePatterns == nil {
		rc.SyncSettings.IgnorePatterns = DefaultIgnorePatterns
	}
	if rc.LocalSettings.HandleDuplicateFilenames == "" {
		rc.LocalSettings.HandleDuplicateFilenames = defaults.LocalSettings.HandleDuplicateFilenames
	}
}
