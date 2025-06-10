package storage

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"io"
	"mime"
	"os"
	"path/filepath"
	"time"
)

type LocalStorage struct {
	basePath string
}

func NewLocalStorage(basePath string) (Storage, error) {
	// Ensure the base path exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &LocalStorage{basePath: basePath}, nil
}

func (s *LocalStorage) Upload(ctx context.Context, file io.Reader) (string, error) {
	// Generate a unique filename using UUID and current timestamp
	filename := fmt.Sprintf("%s_%d", uuid.New().String(), time.Now().Unix())
	return s.saveFile(ctx, file, filename, "")
}

func (s *LocalStorage) UploadWithMetadata(ctx context.Context, file io.Reader, filename string, contentType string) (string, error) {
	// Create a unique filename while preserving original extension
	ext := filepath.Ext(filename)
	baseFilename := fmt.Sprintf("%s_%d%s", uuid.New().String(), time.Now().Unix(), ext)
	return s.saveFile(ctx, file, baseFilename, contentType)
}

// saveFile is a helper method used by both Upload methods
func (s *LocalStorage) saveFile(ctx context.Context, file io.Reader, filename string, contentType string) (string, error) {
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

func (s *LocalStorage) GetInfo(ctx context.Context, path string) (*StorageFile, error) {
	fullPath := filepath.Join(s.basePath, path)

	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Try to determine content type from file extension
	ext := filepath.Ext(fullPath)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return &StorageFile{
		Path:        path,
		Size:        fileInfo.Size(),
		ContentType: contentType,
		ModTime:     fileInfo.ModTime(),
	}, nil
}

func (s *LocalStorage) Exists(ctx context.Context, path string) (bool, error) {
	fullPath := filepath.Join(s.basePath, path)
	_, err := os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check if file exists: %w", err)
}

func (s *LocalStorage) GetURL(path string) string {
	// For local storage, we just return the relative path
	// TODO: This could be expanded to include a base URL if needed
	return filepath.ToSlash(path)
}
