package storage

import (
	"context"
	"io"
	"time"
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
	Upload(ctx context.Context, file io.Reader) (string, error)
	
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
