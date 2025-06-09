package service

import (
	"context"
	"io"
)

// CloudStorage defines the interface for storage services
type CloudStorage interface {
	Upload(ctx context.Context, file io.Reader) (string, error)
	Delete(ctx context.Context, path string) error
	Get(ctx context.Context, path string) (io.ReadCloser, error)
}
