// Package rootcfg owns the portable .lumilioroot marker used to identify an
// authorized repository container independently from its current mount path.
package rootcfg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const (
	FileName       = ".lumilioroot"
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
	if strings.TrimSpace(c.Version) != CurrentVersion {
		return fmt.Errorf("version must be %s", CurrentVersion)
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
