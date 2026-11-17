package rootcfg

import (
	"fmt"
	"os"
	"path/filepath"
	"server/internal/storage/marker"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const (
	FileName = ".lumilioroot"
	// CurrentVersion is the marker format since the v26.1.0-rc.1
	// compatibility baseline; pre-release markers have the same shape and are
	// read as-is. The marker lives inside user media folders, so a later format
	// must keep reading every earlier version rather than rejecting it.
	CurrentVersion = "1.0"
)

// RootConfig is the complete portable identity stored at a Storage Location.
// Host authorization and reachability remain machine-local database state.
type RootConfig struct {
	Version   string    `yaml:"version" json:"version"`
	ID        string    `yaml:"id" json:"id"`
	Name      string    `yaml:"name" json:"name"`
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
}

func New(name string) *RootConfig {
	return &RootConfig{
		Version:   CurrentVersion,
		ID:        uuid.NewString(),
		Name:      strings.TrimSpace(name),
		CreatedAt: time.Now(),
	}
}

func Load(path string) (*RootConfig, error) {
	marker := filepath.Join(path, FileName)
	data, err := os.ReadFile(marker)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("storage location marker not found at %s", marker)
		}
		return nil, fmt.Errorf("read storage location marker: %w", err)
	}

	var config RootConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse storage location marker: %w", err)
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid storage location marker: %w", err)
	}
	return &config, nil
}

func (c *RootConfig) Save(path string) error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("invalid storage location marker: %w", err)
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal storage location marker: %w", err)
	}
	if err := os.WriteFile(filepath.Join(path, FileName), data, 0o644); err != nil {
		return fmt.Errorf("write storage location marker: %w", err)
	}
	return nil
}

func (c *RootConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("configuration is required")
	}
	if version := strings.TrimSpace(c.Version); version != CurrentVersion {
		return fmt.Errorf("version must be %s, found %q: a newer Lumilio Photos may have written this marker", CurrentVersion, version)
	}
	if _, err := uuid.Parse(strings.TrimSpace(c.ID)); err != nil {
		return fmt.Errorf("id must be a UUID: %w", err)
	}
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if c.CreatedAt.IsZero() {
		return fmt.Errorf("created_at is required")
	}
	return nil
}

func Exists(path string) bool {
	info, err := os.Stat(filepath.Join(path, FileName))
	return err == nil && !info.IsDir()
}

// ReadMarker preserves access errors separately from marker compatibility.
func ReadMarker(reader marker.Reader, path string) marker.Reading[RootConfig] {
	return marker.Read(reader, filepath.Join(path, ".lumilioroot"), DecodeMarker)
}

func DecodeMarker(data []byte) marker.Reading[RootConfig] {
	var config RootConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return marker.Reading[RootConfig]{State: marker.Corrupt, Err: err}
	}
	reading := marker.Reading[RootConfig]{Version: config.Version, UUID: config.ID}
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
func (config *RootConfig) SaveGuarded(files marker.AtomicFS, path, expectedID string) error {
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
	return marker.WriteAtomic(files, filepath.Join(path, ".lumilioroot"), data, func(current []byte) error {
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
