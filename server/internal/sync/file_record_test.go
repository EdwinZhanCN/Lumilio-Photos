package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/internal/db/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a test database connection
// Note: This requires a running PostgreSQL instance with test database
func setupTestDB(t *testing.T) *pgxpool.Pool {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping database tests")
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)

	return pool
}

// cleanupTestData removes test data from database
func cleanupTestData(t *testing.T, pool *pgxpool.Pool, repoID uuid.UUID) {
	ctx := context.Background()
	_, err := pool.Exec(ctx, "DELETE FROM file_records WHERE repository_id = $1", repoID)
	require.NoError(t, err)
}

func TestFileRecordStore_CreateAndGetFileRecord(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	queries := repo.New(pool)
	store := NewFileRecordStore(queries)
	repoID := uuid.New()
	defer cleanupTestData(t, pool, repoID)

	ctx := context.Background()
	hash := "abc123def456"
	now := time.Now().Truncate(time.Microsecond)

	// Create record
	record, err := store.CreateFileRecord(ctx, repoID, "test/file.jpg", 1024, now, &hash, 1)
	require.NoError(t, err)
	assert.NotZero(t, record.ID)

	// Get record
	retrieved, err := store.GetFileRecord(ctx, repoID, "test/file.jpg")
	require.NoError(t, err)
	assert.Equal(t, repoID, retrieved.RepositoryID.Bytes)
	assert.Equal(t, "test/file.jpg", retrieved.FilePath)
	assert.Equal(t, int64(1024), retrieved.FileSize)
	assert.Equal(t, hash, *retrieved.ContentHash)
}

func TestFileRecordStore_UpdateFileRecord(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	queries := repo.New(pool)
	store := NewFileRecordStore(queries)
	repoID := uuid.New()
	defer cleanupTestData(t, pool, repoID)

	ctx := context.Background()
	hash := "original_hash"
	now := time.Now().Truncate(time.Microsecond)

	// Create record
	_, err := store.CreateFileRecord(ctx, repoID, "test/file.jpg", 1024, now, &hash, 1)
	require.NoError(t, err)

	// Update record
	newHash := "updated_hash"
	_, err = store.UpdateFileRecord(ctx, repoID, "test/file.jpg", 2048, now, &newHash, 2)
	require.NoError(t, err)

	// Verify update
	retrieved, err := store.GetFileRecord(ctx, repoID, "test/file.jpg")
	require.NoError(t, err)
	assert.Equal(t, int64(2048), retrieved.FileSize)
	assert.Equal(t, newHash, *retrieved.ContentHash)
	assert.Equal(t, int64(2), *retrieved.ScanGeneration)
}

func TestFileRecordStore_UpsertFileRecord(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	queries := repo.New(pool)
	store := NewFileRecordStore(queries)
	repoID := uuid.New()
	defer cleanupTestData(t, pool, repoID)

	ctx := context.Background()
	hash := "hash1"
	now := time.Now().Truncate(time.Microsecond)

	// First upsert (insert)
	_, err := store.UpsertFileRecord(ctx, repoID, "test/file.jpg", 1024, now, &hash, 1)
	require.NoError(t, err)

	// Second upsert (update)
	newHash := "hash2"
	_, err = store.UpsertFileRecord(ctx, repoID, "test/file.jpg", 2048, now, &newHash, 1)
	require.NoError(t, err)

	// Verify
	retrieved, err := store.GetFileRecord(ctx, repoID, "test/file.jpg")
	require.NoError(t, err)
	assert.Equal(t, int64(2048), retrieved.FileSize)
	assert.Equal(t, newHash, *retrieved.ContentHash)
}

func TestFileRecordStore_DeleteFileRecord(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	queries := repo.New(pool)
	store := NewFileRecordStore(queries)
	repoID := uuid.New()
	defer cleanupTestData(t, pool, repoID)

	ctx := context.Background()
	hash := "hash"
	now := time.Now().Truncate(time.Microsecond)

	// Create record
	_, err := store.CreateFileRecord(ctx, repoID, "test/file.jpg", 1024, now, &hash, 1)
	require.NoError(t, err)

	// Delete record
	err = store.DeleteFileRecord(ctx, repoID, "test/file.jpg")
	require.NoError(t, err)

	// Verify deletion
	_, err = store.GetFileRecord(ctx, repoID, "test/file.jpg")
	assert.Error(t, err)
}

func TestFileRecordStore_ListFileRecords(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	queries := repo.New(pool)
	store := NewFileRecordStore(queries)
	repoID := uuid.New()
	defer cleanupTestData(t, pool, repoID)

	ctx := context.Background()
	now := time.Now().Truncate(time.Microsecond)

	// Create multiple records
	files := []string{"file1.jpg", "file2.jpg", "file3.jpg"}
	for _, file := range files {
		hash := "hash_" + file
		_, err := store.CreateFileRecord(ctx, repoID, file, 1024, now, &hash, 1)
		require.NoError(t, err)
	}

	// List records
	records, err := store.ListFileRecords(ctx, repoID)
	require.NoError(t, err)
	assert.Len(t, records, 3)

	// Verify order (should be alphabetical by file_path)
	assert.Equal(t, "file1.jpg", records[0].FilePath)
	assert.Equal(t, "file2.jpg", records[1].FilePath)
	assert.Equal(t, "file3.jpg", records[2].FilePath)
}

func TestFileRecordStore_DeleteOrphanedRecords(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	queries := repo.New(pool)
	store := NewFileRecordStore(queries)
	repoID := uuid.New()
	defer cleanupTestData(t, pool, repoID)

	ctx := context.Background()
	now := time.Now().Truncate(time.Microsecond)

	// Create records with different scan generations
	for i := 1; i <= 5; i++ {
		hash := "hash"
		_, err := store.CreateFileRecord(ctx, repoID, filepath.Join("test", "file"+string(rune('0'+i))+".jpg"), 1024, now, &hash, int64(i))
		require.NoError(t, err)
	}

	// Delete records older than generation 3
	deleted, err := store.DeleteOrphanedRecords(ctx, repoID, 3)
	require.NoError(t, err)
	assert.Equal(t, int64(2), deleted) // Generations 1 and 2 should be deleted

	// Verify remaining records
	records, err := store.ListFileRecords(ctx, repoID)
	require.NoError(t, err)
	assert.Len(t, records, 3)
}

func TestFileRecordStore_GetFileRecordCount(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	queries := repo.New(pool)
	store := NewFileRecordStore(queries)
	repoID := uuid.New()
	defer cleanupTestData(t, pool, repoID)

	ctx := context.Background()
	now := time.Now().Truncate(time.Microsecond)

	// Initially should be 0
	count, err := store.GetFileRecordCount(ctx, repoID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Create 3 records
	for i := 1; i <= 3; i++ {
		hash := "hash"
		_, err := store.CreateFileRecord(ctx, repoID, filepath.Join("test", "file"+string(rune('0'+i))+".jpg"), 1024, now, &hash, 1)
		require.NoError(t, err)
	}

	// Should be 3
	count, err = store.GetFileRecordCount(ctx, repoID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestFileRecordStore_BatchUpsertFileRecords(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	queries := repo.New(pool)
	store := NewFileRecordStore(queries)
	repoID := uuid.New()
	defer cleanupTestData(t, pool, repoID)

	ctx := context.Background()
	now := time.Now().Truncate(time.Microsecond)

	// Create batch of records
	records := make([]FileRecordData, 10)
	for i := 0; i < 10; i++ {
		hash := "hash"
		records[i] = FileRecordData{
			FilePath:       filepath.Join("test", "file"+string(rune('0'+i))+".jpg"),
			FileSize:       1024,
			ModTime:        now,
			ContentHash:    &hash,
			ScanGeneration: 1,
		}
	}

	// Batch upsert
	err := store.BatchUpsertFileRecords(ctx, repoID, records)
	require.NoError(t, err)

	// Verify all records were inserted
	count, err := store.GetFileRecordCount(ctx, repoID)
	require.NoError(t, err)
	assert.Equal(t, int64(10), count)
}

func TestCalculateFileHash(t *testing.T) {
	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := []byte("Hello, World!")
	err := os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)

	// Calculate hash
	hash, err := CalculateFileHash(testFile)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 64) // SHA256 produces 64 character hex string

	// Verify hash is consistent
	hash2, err := CalculateFileHash(testFile)
	require.NoError(t, err)
	assert.Equal(t, hash, hash2)

	// Modify file and verify hash changes
	err = os.WriteFile(testFile, []byte("Different content"), 0644)
	require.NoError(t, err)

	hash3, err := CalculateFileHash(testFile)
	require.NoError(t, err)
	assert.NotEqual(t, hash, hash3)
}

func TestCalculateFileHash_NonExistentFile(t *testing.T) {
	_, err := CalculateFileHash("/nonexistent/file.txt")
	assert.Error(t, err)
}
