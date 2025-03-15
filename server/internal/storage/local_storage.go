package storage

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"io"
	"os"
	"path/filepath"
	"server/internal/service"
	"time"
)

type LocalStorage struct {
	basePath string
}

func NewLocalStorage(basePath string) (service.CloudStorage, error) {
	// Ensure the base path exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &LocalStorage{basePath: basePath}, nil
}

func (s *LocalStorage) Upload(ctx context.Context, file io.Reader) (string, error) {
	// Generate a unique filename using UUID and current timestamp
	filename := fmt.Sprintf("%s_%d", uuid.New().String(), time.Now().Unix())

	// Create year/month based directory structure
	now := time.Now()
	relativePath := filepath.Join(fmt.Sprintf("%d", now.Year()), fmt.Sprintf("%02d", now.Month()))
	dirPath := filepath.Join(s.basePath, relativePath)

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Create the file
	filePath := filepath.Join(dirPath, filename)
	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Copy the file content
	if _, err := io.Copy(dst, file); err != nil {
		// Clean up the file if copy fails
		os.Remove(filePath)
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Return the relative path that will be stored in the database
	return filepath.Join(relativePath, filename), nil
}

func (s *LocalStorage) Delete(ctx context.Context, path string) error {
	fullPath := filepath.Join(s.basePath, path)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

func (s *LocalStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	fullPath := filepath.Join(s.basePath, path)
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	return file, nil
}
