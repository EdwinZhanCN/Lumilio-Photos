package repocfg

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"server/internal/storage/marker"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// CurrentVersion is the .lumiliorepo format since the v26.1.0-rc.1
// compatibility baseline; pre-release markers have the same shape and are read
// as-is. The marker lives inside user media folders, so a later format must
// keep reading every earlier version rather than rejecting it.
const CurrentVersion = "1.0"

// RepositoryConfig represents the complete .lumiliorepo configuration file structure
type RepositoryConfig struct {
	Version   string    `yaml:"version" json:"version"`
	ID        string    `yaml:"id" json:"id"`
	Name      string    `yaml:"name" json:"name"`
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`

	// Storage configuration
	StorageStrategy string        `yaml:"storage_strategy" json:"storage_strategy"` // "date", "cas", "flat" date -> yyyy/mm/IMG_001.jpg (month based)
	LocalSettings   LocalSettings `yaml:"local_settings" json:"local_settings"`
}

func (rc *RepositoryConfig) Scan(src any) error {
	if rc == nil {
		return fmt.Errorf("repocfg.RepositoryConfig: nil receiver")
	}
	var value []byte
	switch source := src.(type) {
	case string:
		value = []byte(source)
	case []byte:
		value = source
	case nil:
		*rc = RepositoryConfig{}
		return nil
	default:
		return fmt.Errorf("repocfg.RepositoryConfig: unsupported source %T", src)
	}
	if err := json.Unmarshal(value, rc); err != nil {
		return fmt.Errorf("repocfg.RepositoryConfig: decode: %w", err)
	}
	return nil
}

func (rc RepositoryConfig) Value() (driver.Value, error) {
	value, err := json.Marshal(rc)
	if err != nil {
		return nil, fmt.Errorf("repocfg.RepositoryConfig: encode: %w", err)
	}
	return string(value), nil
}

// LocalSettings configures repository-specific behavior
type LocalSettings struct {
	// HandleDuplicateFilenames how to handle files with same name
	// "rename" = add (1), (2) suffix; "uuid" = add a unique suffix. Existing originals are never replaced.
	HandleDuplicateFilenames string `yaml:"handle_duplicate_filenames" json:"handle_duplicate_filenames"`
}

// DefaultRepositoryConfig returns a sensible default configuration template
// Note: This does not include ID, Name, or CreatedAt as these should be unique per repository
func DefaultRepositoryConfig() *RepositoryConfig {
	return &RepositoryConfig{
		Version:         CurrentVersion,
		StorageStrategy: "date",
		LocalSettings: LocalSettings{
			HandleDuplicateFilenames: "uuid",
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

// WithLocalSettings sets the local settings for the repository
func WithLocalSettings(duplicateHandling string) RepositoryConfigOption {
	return func(config *RepositoryConfig) {
		config.LocalSettings.HandleDuplicateFilenames = duplicateHandling
	}
}

// NewRepositoryConfig creates a new repository configuration with unique ID and current timestamp
//
// System-managed fields (always auto-generated):
//   - ID: Unique UUID generated automatically
//   - CreatedAt: Current timestamp when config is created
//   - Version: Set to CurrentVersion
//
// User-configurable fields via options:
//   - StorageStrategy: How files are organized ("date", "cas", "flat")
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

	return ParseConfig(data)
}

// ParseConfig parses and validates .lumiliorepo contents read through a
// repository-scoped filesystem capability.
func ParseConfig(data []byte) (*RepositoryConfig, error) {
	var config RepositoryConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse repository config: %w", err)
	}

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
	if version := strings.TrimSpace(rc.Version); version != CurrentVersion {
		return fmt.Errorf("version must be %s, found %q: a newer Lumilio Photos may have written this marker", CurrentVersion, version)
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

	// Validate duplicate handling strategy
	validDuplicateStrategies := map[string]bool{
		"rename": true,
		"uuid":   true,
	}
	if !validDuplicateStrategies[rc.LocalSettings.HandleDuplicateFilenames] {
		return fmt.Errorf("invalid handle_duplicate_filenames '%s', must be one of: rename, uuid", rc.LocalSettings.HandleDuplicateFilenames)
	}

	return nil
}

// IsStorageLocation checks if a directory contains a .lumiliorepo file
func IsStorageLocation(path string) bool {
	configPath := filepath.Join(path, ".lumiliorepo")
	_, err := os.Stat(configPath)
	return err == nil
}

// ReadMarker preserves access errors separately from marker compatibility.
func ReadMarker(reader marker.Reader, path string) marker.Reading[RepositoryConfig] {
	return marker.Read(reader, filepath.Join(path, ".lumiliorepo"), DecodeMarker)
}

func DecodeMarker(data []byte) marker.Reading[RepositoryConfig] {
	var config RepositoryConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return marker.Reading[RepositoryConfig]{State: marker.Corrupt, Err: err}
	}
	reading := marker.Reading[RepositoryConfig]{Version: config.Version, UUID: config.ID}
	if strings.TrimSpace(config.Version) == "" {
		reading.State = marker.Corrupt
		reading.Err = fmt.Errorf("marker version is required")
		return reading
	}
	if strings.TrimSpace(config.Version) != CurrentVersion {
		reading.State = marker.UnsupportedVersion
		return reading
	}
	if _, err := uuid.Parse(config.ID); err != nil {
		reading.State = marker.Corrupt
		reading.Err = err
		return reading
	}
	if config.CreatedAt.IsZero() {
		reading.State = marker.Corrupt
		reading.Err = fmt.Errorf("created_at is required")
		return reading
	}
	if err := config.Validate(); err != nil {
		reading.State = marker.Corrupt
		reading.Err = err
		return reading
	}
	reading.State = marker.Valid
	reading.Config = &config
	return reading
}

// SaveGuarded is the P1 atomic primitive; legacy Save callers retain their
// current transition semantics until they refresh identity under P3 barriers.
// expectedID == "" means the marker must be positively absent.
func (config *RepositoryConfig) SaveGuarded(files marker.AtomicFS, path, expectedID string) error {
	if err := config.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	if reading := DecodeMarker(data); reading.State != marker.Valid {
		return fmt.Errorf("invalid complete marker: %v", reading.Err)
	}
	return marker.WriteAtomic(files, filepath.Join(path, ".lumiliorepo"), data, func(current []byte) error {
		if current == nil && expectedID == "" {
			return nil
		}
		reading := DecodeMarker(current)
		if expectedID == "" || reading.State != marker.Valid || reading.UUID != expectedID {
			return marker.ErrIdentityChanged
		}
		return nil
	})
}
