package sync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"

	"server/internal/db/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// FileRecordStore handles database operations for file records using sqlc
type FileRecordStore struct {
	queries *repo.Queries
}

// NewFileRecordStore creates a new file record store
func NewFileRecordStore(queries *repo.Queries) *FileRecordStore {
	return &FileRecordStore{queries: queries}
}

// CreateFileRecord inserts a new file record
func (s *FileRecordStore) CreateFileRecord(ctx context.Context, repoID uuid.UUID, filePath string, fileSize int64, modTime time.Time, contentHash *string, scanGeneration int64) (*repo.FileRecord, error) {
	params := repo.CreateFileRecordParams{
		RepositoryID:   pgtype.UUID{Bytes: repoID, Valid: true},
		FilePath:       filePath,
		FileSize:       fileSize,
		ModTime:        pgtype.Timestamptz{Time: modTime, Valid: true},
		ContentHash:    contentHash,
		LastScanned:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		ScanGeneration: &scanGeneration,
	}

	record, err := s.queries.CreateFileRecord(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create file record: %w", err)
	}

	return &record, nil
}

// UpdateFileRecord updates an existing file record
func (s *FileRecordStore) UpdateFileRecord(ctx context.Context, repoID uuid.UUID, filePath string, fileSize int64, modTime time.Time, contentHash *string, scanGeneration int64) (*repo.FileRecord, error) {
	params := repo.UpdateFileRecordParams{
		RepositoryID:   pgtype.UUID{Bytes: repoID, Valid: true},
		FilePath:       filePath,
		FileSize:       fileSize,
		ModTime:        pgtype.Timestamptz{Time: modTime, Valid: true},
		ContentHash:    contentHash,
		LastScanned:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		ScanGeneration: &scanGeneration,
	}

	record, err := s.queries.UpdateFileRecord(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to update file record: %w", err)
	}

	return &record, nil
}

// GetFileRecord retrieves a file record by repository ID and path
func (s *FileRecordStore) GetFileRecord(ctx context.Context, repoID uuid.UUID, filePath string) (*repo.FileRecord, error) {
	params := repo.GetFileRecordParams{
		RepositoryID: pgtype.UUID{Bytes: repoID, Valid: true},
		FilePath:     filePath,
	}

	record, err := s.queries.GetFileRecord(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("file record not found: %w", err)
	}

	return &record, nil
}

// ListFileRecords returns all file records for a repository
func (s *FileRecordStore) ListFileRecords(ctx context.Context, repoID uuid.UUID) ([]repo.FileRecord, error) {
	records, err := s.queries.ListFileRecords(ctx, pgtype.UUID{Bytes: repoID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list file records: %w", err)
	}

	return records, nil
}

// UpsertFileRecord inserts or updates a file record
func (s *FileRecordStore) UpsertFileRecord(ctx context.Context, repoID uuid.UUID, filePath string, fileSize int64, modTime time.Time, contentHash *string, scanGeneration int64) (*repo.FileRecord, error) {
	params := repo.UpsertFileRecordParams{
		RepositoryID:   pgtype.UUID{Bytes: repoID, Valid: true},
		FilePath:       filePath,
		FileSize:       fileSize,
		ModTime:        pgtype.Timestamptz{Time: modTime, Valid: true},
		ContentHash:    contentHash,
		LastScanned:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		ScanGeneration: &scanGeneration,
	}

	record, err := s.queries.UpsertFileRecord(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert file record: %w", err)
	}

	return &record, nil
}

// DeleteFileRecord deletes a file record
func (s *FileRecordStore) DeleteFileRecord(ctx context.Context, repoID uuid.UUID, filePath string) error {
	params := repo.DeleteFileRecordParams{
		RepositoryID: pgtype.UUID{Bytes: repoID, Valid: true},
		FilePath:     filePath,
	}

	err := s.queries.DeleteFileRecord(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to delete file record: %w", err)
	}

	return nil
}

// DeleteOrphanedRecords deletes file records that haven't been updated in the current scan
func (s *FileRecordStore) DeleteOrphanedRecords(ctx context.Context, repoID uuid.UUID, scanGeneration int64) (int64, error) {
	params := repo.DeleteOrphanedFileRecordsParams{
		RepositoryID:   pgtype.UUID{Bytes: repoID, Valid: true},
		ScanGeneration: &scanGeneration,
	}

	count, err := s.queries.DeleteOrphanedFileRecords(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("failed to delete orphaned records: %w", err)
	}

	return count, nil
}

// GetFileRecordCount returns the number of file records for a repository
func (s *FileRecordStore) GetFileRecordCount(ctx context.Context, repoID uuid.UUID) (int64, error) {
	count, err := s.queries.GetFileRecordCount(ctx, pgtype.UUID{Bytes: repoID, Valid: true})
	if err != nil {
		return 0, fmt.Errorf("failed to get file record count: %w", err)
	}

	return count, nil
}

// BatchUpsertFileRecords performs batch upsert of file records
func (s *FileRecordStore) BatchUpsertFileRecords(ctx context.Context, repoID uuid.UUID, records []FileRecordData) error {
	if len(records) == 0 {
		return nil
	}

	// Process each record individually
	// Note: For better performance, consider using pgx.Batch
	for _, record := range records {
		_, err := s.UpsertFileRecord(ctx, repoID, record.FilePath, record.FileSize, record.ModTime, record.ContentHash, record.ScanGeneration)
		if err != nil {
			return fmt.Errorf("failed to upsert record %s: %w", record.FilePath, err)
		}
	}

	return nil
}

// FileRecordData is a helper struct for batch operations
type FileRecordData struct {
	FilePath       string
	FileSize       int64
	ModTime        time.Time
	ContentHash    *string
	ScanGeneration int64
}

// CalculateFileHash calculates SHA256 hash of a file
func CalculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// GetFileRecordsByHash finds all file records with a specific hash
func (s *FileRecordStore) GetFileRecordsByHash(ctx context.Context, contentHash string) ([]repo.FileRecord, error) {
	records, err := s.queries.GetFileRecordsByHash(ctx, &contentHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get file records by hash: %w", err)
	}

	return records, nil
}
