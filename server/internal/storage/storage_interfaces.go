package storage

import (
	"context"
	"io"
	"time"
)

// StorageStrategy defines different file organization strategies
type StorageStrategy string

const (
	// StorageStrategyDate organizes files by date (YYYY/MM)
	StorageStrategyDate StorageStrategy = "date"
	// StorageStrategyCAS organizes files by content hash (hash[0:2]/hash[2:4]/hash[4:6])
	StorageStrategyCAS StorageStrategy = "cas"
	// StorageStrategyFlat stores all files in a single directory
	StorageStrategyFlat StorageStrategy = "flat"
)

// StorageFile represents file metadata
type StorageFile struct {
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type,omitempty"`
	ModTime     time.Time `json:"mod_time"`
}

// Storage defines the interface for file storage services
type Storage interface {
	// Upload saves a file and returns its relative path
	Upload(ctx context.Context, file io.Reader, hash string) (string, error)

	// UploadWithMetadata saves a file with additional metadata and returns its relative path
	UploadWithMetadata(ctx context.Context, file io.Reader, filename string, contentType string) (string, error)

	// Delete removes a file by its path
	Delete(ctx context.Context, path string) error

	// Get returns a file reader for the given path
	Get(ctx context.Context, path string) (io.ReadCloser, error)

	// GetInfo returns metadata about a file
	GetInfo(ctx context.Context, path string) (*StorageFile, error)

	// Exists checks if a file exists
	Exists(ctx context.Context, path string) (bool, error)

	// GetURL returns a URL (or local path) for accessing the file
	GetURL(path string) string
}
