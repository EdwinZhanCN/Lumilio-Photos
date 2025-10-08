package upload

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/google/uuid"
)

// ChunkMerger handles the merging of uploaded file chunks
type ChunkMerger struct {
	tempDir string
}

// NewChunkMerger creates a new chunk merger instance
func NewChunkMerger(tempDir string) *ChunkMerger {
	return &ChunkMerger{
		tempDir: tempDir,
	}
}

// MergeResult represents the result of a chunk merging operation
type MergeResult struct {
	MergedFilePath string `json:"merged_file_path"`
	TotalSize      int64  `json:"total_size"`
	Error          string `json:"error,omitempty"`
}

// ChunkInfo represents information about a file chunk
type ChunkInfo struct {
	SessionID  string `json:"session_id"`
	ChunkIndex int    `json:"chunk_index"`
	FilePath   string `json:"file_path"`
	Size       int64  `json:"size"`
}

// MergeChunks merges all chunks for a session into a single file
func (cm *ChunkMerger) MergeChunks(sessionID string, chunks []ChunkInfo) (*MergeResult, error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks provided for session %s", sessionID)
	}

	// Sort chunks by index to ensure correct order
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].ChunkIndex < chunks[j].ChunkIndex
	})

	// Validate chunk sequence
	if err := cm.validateChunkSequence(chunks); err != nil {
		return nil, fmt.Errorf("invalid chunk sequence for session %s: %w", sessionID, err)
	}

	// Create temporary merged file
	mergedFileName := fmt.Sprintf("merged_%s_%s", sessionID, uuid.New().String())
	mergedFilePath := filepath.Join(cm.tempDir, mergedFileName)

	mergedFile, err := os.Create(mergedFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create merged file: %w", err)
	}
	defer mergedFile.Close()

	var totalSize int64
	var mergeError error
	var mu sync.Mutex

	// Use a worker pool to merge chunks concurrently
	const maxWorkers = 3
	chunkChan := make(chan ChunkInfo, len(chunks))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for chunk := range chunkChan {
				if mergeError != nil {
					continue // Skip if there's already an error
				}

				err := cm.appendChunkToFile(mergedFile, chunk, chunks)
				if err != nil {
					mu.Lock()
					if mergeError == nil {
						mergeError = fmt.Errorf("failed to append chunk %d: %w", chunk.ChunkIndex, err)
					}
					mu.Unlock()
					continue
				}

				mu.Lock()
				totalSize += chunk.Size
				mu.Unlock()
			}
		}()
	}

	// Send chunks to workers
	for _, chunk := range chunks {
		chunkChan <- chunk
	}
	close(chunkChan)

	// Wait for all workers to complete
	wg.Wait()

	if mergeError != nil {
		// Clean up the incomplete merged file
		os.Remove(mergedFilePath)
		return nil, mergeError
	}

	// Verify the final file size matches expected total
	if err := cm.verifyFileSize(mergedFilePath, totalSize); err != nil {
		os.Remove(mergedFilePath)
		return nil, fmt.Errorf("file size verification failed: %w", err)
	}

	return &MergeResult{
		MergedFilePath: mergedFilePath,
		TotalSize:      totalSize,
	}, nil
}

// appendChunkToFile appends a chunk to the merged file at the correct position
func (cm *ChunkMerger) appendChunkToFile(mergedFile *os.File, chunk ChunkInfo, chunks []ChunkInfo) error {
	// Open chunk file
	chunkFile, err := os.Open(chunk.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open chunk file %s: %w", chunk.FilePath, err)
	}
	defer chunkFile.Close()

	// Calculate the position in the merged file
	position, err := cm.calculateChunkPosition(mergedFile, chunk.ChunkIndex, chunks)
	if err != nil {
		return err
	}

	// Seek to the correct position in the merged file
	if _, err := mergedFile.Seek(position, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek in merged file: %w", err)
	}

	// Copy chunk data to merged file
	bytesWritten, err := io.Copy(mergedFile, chunkFile)
	if err != nil {
		return fmt.Errorf("failed to copy chunk data: %w", err)
	}

	if bytesWritten != chunk.Size {
		return fmt.Errorf("chunk size mismatch: expected %d, wrote %d", chunk.Size, bytesWritten)
	}

	return nil
}

// calculateChunkPosition calculates the file position for a chunk based on previous chunks
func (cm *ChunkMerger) calculateChunkPosition(mergedFile *os.File, chunkIndex int, chunks []ChunkInfo) (int64, error) {
	if chunkIndex == 0 {
		return 0, nil
	}

	var position int64
	for i := 0; i < chunkIndex; i++ {
		position += chunks[i].Size
	}

	return position, nil
}

// validateChunkSequence validates that chunks form a complete sequence
func (cm *ChunkMerger) validateChunkSequence(chunks []ChunkInfo) error {
	// Check for duplicate chunk indices
	seen := make(map[int]bool)
	for _, chunk := range chunks {
		if seen[chunk.ChunkIndex] {
			return fmt.Errorf("duplicate chunk index %d", chunk.ChunkIndex)
		}
		seen[chunk.ChunkIndex] = true
	}

	// Check if we have a continuous sequence from 0 to max index
	maxIndex := -1
	for _, chunk := range chunks {
		if chunk.ChunkIndex > maxIndex {
			maxIndex = chunk.ChunkIndex
		}
	}

	for i := 0; i <= maxIndex; i++ {
		if !seen[i] {
			return fmt.Errorf("missing chunk index %d", i)
		}
	}

	return nil
}

// verifyFileSize verifies that the merged file has the expected size
func (cm *ChunkMerger) verifyFileSize(filePath string, expectedSize int64) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat merged file: %w", err)
	}

	if info.Size() != expectedSize {
		return fmt.Errorf("file size mismatch: expected %d, got %d", expectedSize, info.Size())
	}

	return nil
}

// CleanupChunks removes temporary chunk files after successful merge
func (cm *ChunkMerger) CleanupChunks(chunks []ChunkInfo) {
	for _, chunk := range chunks {
		os.Remove(chunk.FilePath)
	}
}

// CleanupMergedFile removes the temporary merged file
func (cm *ChunkMerger) CleanupMergedFile(filePath string) error {
	return os.Remove(filePath)
}

// GetChunkFileSize gets the size of a chunk file
func (cm *ChunkMerger) GetChunkFileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
