package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/zeebo/blake3"
)

type LocalStorage struct {
	basePath string
	strategy StorageStrategy
	options  StorageOptions
}

func NewLocalStorage(basePath string) (Storage, error) {
	return NewLocalStorageWithStrategy(basePath, StorageStrategyDate)
}

func NewLocalStorageWithStrategy(basePath string, strategy StorageStrategy) (Storage, error) {
	return NewLocalStorageWithConfig(StorageConfig{
		BasePath: basePath,
		Strategy: strategy,
		Options:  DefaultStorageConfig().Options,
	})
}

func NewLocalStorageWithConfig(config StorageConfig) (Storage, error) {
	// Ensure the base path exists
	if err := os.MkdirAll(config.BasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &LocalStorage{
		basePath: config.BasePath,
		strategy: config.Strategy,
		options:  config.Options,
	}, nil
}

func (s *LocalStorage) Upload(ctx context.Context, file io.Reader) (string, error) {
	// Generate a unique filename using UUID and current timestamp
	filename := fmt.Sprintf("%s_%d", uuid.New().String(), time.Now().Unix())
	return s.saveFile(ctx, file, filename, "")
}

func (s *LocalStorage) UploadWithMetadata(ctx context.Context, file io.Reader, filename string, contentType string) (string, error) {
	var finalFilename string

	if s.options.PreserveOriginalFilename {
		// Preserve original filename for all strategies except CAS (which uses hash)
		finalFilename = filename
	} else {
		// Generate unique filename
		ext := filepath.Ext(filename)
		base := strings.TrimSuffix(filename, ext)
		finalFilename = fmt.Sprintf("%s_%s%s", base, uuid.New().String()[:8], ext)
	}

	return s.saveFile(ctx, file, finalFilename, contentType)
}

// saveFile is a helper method used by both Upload methods
func (s *LocalStorage) saveFile(ctx context.Context, file io.Reader, filename string, contentType string) (string, error) {
	var relativePath string
	var finalFilename string
	var fileData []byte
	var err error

	// For CAS strategy, we need to read the file first to calculate hash
	if s.strategy == StorageStrategyCAS {
		fileData, err = io.ReadAll(file)
		if err != nil {
			return "", fmt.Errorf("failed to read file for CAS: %w", err)
		}

		relativePath, finalFilename, err = s.getCASBasedPathAndFilename(fileData, filename)
		if err != nil {
			return "", err
		}
	} else {
		// For other strategies, use original filename with duplicate handling
		switch s.strategy {
		case StorageStrategyDate:
			relativePath, err = s.getDateBasedPath()
			if err != nil {
				return "", err
			}
			finalFilename = s.getUniqueFilename(relativePath, filename)
		case StorageStrategyFlat:
			relativePath = ""
			finalFilename = s.getUniqueFilename(relativePath, filename)
		default:
			relativePath, err = s.getDateBasedPath()
			if err != nil {
				return "", err
			}
			finalFilename = s.getUniqueFilename(relativePath, filename)
		}
	}

	dirPath := filepath.Join(s.basePath, relativePath)

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Create the file
	filePath := filepath.Join(dirPath, finalFilename)
	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Copy the file content
	if s.strategy == StorageStrategyCAS {
		// Write the data we already read
		if _, err := dst.Write(fileData); err != nil {
			os.Remove(filePath)
			return "", fmt.Errorf("failed to write file: %w", err)
		}
	} else {
		// Copy from the reader
		if _, err := io.Copy(dst, file); err != nil {
			os.Remove(filePath)
			return "", fmt.Errorf("failed to write file: %w", err)
		}
	}

	// Return the relative path that will be stored in the database
	return filepath.Join(relativePath, finalFilename), nil
}

// getDateBasedPath returns a date-based path (YYYY/MM)
func (s *LocalStorage) getDateBasedPath() (string, error) {
	now := time.Now()
	return filepath.Join(fmt.Sprintf("%d", now.Year()), fmt.Sprintf("%02d", now.Month())), nil
}

// getCASBasedPathAndFilename returns both directory path and filename for CAS storage
func (s *LocalStorage) getCASBasedPathAndFilename(data []byte, originalFilename string) (string, string, error) {
	// Calculate BLAKE3 hash
	hasher := blake3.New()
	hasher.Write(data)
	hashBytes := hasher.Sum(nil)
	hash := fmt.Sprintf("%x", hashBytes)

	if len(hash) < 6 {
		return "", "", fmt.Errorf("hash too short")
	}

	// Create hash-based directory structure: hash[0:2]/hash[2:4]/hash[4:6]
	dirPath := filepath.Join(hash[0:2], hash[2:4], hash[4:6])

	// Use hash as filename with original extension
	ext := filepath.Ext(originalFilename)
	filename := hash + ext

	return dirPath, filename, nil
}

// getUniqueFilename generates a unique filename to avoid conflicts
func (s *LocalStorage) getUniqueFilename(relativePath, filename string) string {
	dirPath := filepath.Join(s.basePath, relativePath)
	originalPath := filepath.Join(dirPath, filename)

	// If file doesn't exist, use original name
	if _, err := os.Stat(originalPath); os.IsNotExist(err) {
		return filename
	}

	// Handle duplicates based on configuration
	switch s.options.HandleDuplicateFilenames {
	case "overwrite":
		return filename // Keep original name, will overwrite existing file
	case "uuid":
		ext := filepath.Ext(filename)
		base := strings.TrimSuffix(filename, ext)
		return fmt.Sprintf("%s_%s%s", base, uuid.New().String()[:8], ext)
	case "rename":
		fallthrough
	default:
		// File exists, generate unique name with (1), (2), etc.
		ext := filepath.Ext(filename)
		base := strings.TrimSuffix(filename, ext)

		for i := 1; i <= 999; i++ {
			newFilename := fmt.Sprintf("%s (%d)%s", base, i, ext)
			newPath := filepath.Join(dirPath, newFilename)
			if _, err := os.Stat(newPath); os.IsNotExist(err) {
				return newFilename
			}
		}

		// Fallback: use timestamp if too many duplicates
		timestamp := time.Now().Format("20060102_150405")
		return fmt.Sprintf("%s_%s%s", base, timestamp, ext)
	}
}

// getCASBasedPath returns a content-addressable path based on file hash (deprecated)
func (s *LocalStorage) getCASBasedPath(file io.Reader, filename string) (string, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file for hashing: %w", err)
	}

	dirPath, _, err := s.getCASBasedPathAndFilename(data, filename)
	return dirPath, err
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
