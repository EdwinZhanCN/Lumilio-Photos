package upload

import (
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"sync"

	"server/internal/storage"
)

// ChunkMerger handles the merging of uploaded file chunks
type ChunkMerger struct {
	directoryManager storage.DirectoryManager
	mu               sync.RWMutex
	chunks           map[string][]ChunkInfo // sessionID -> chunks
}

// NewChunkMerger creates a new chunk merger instance
func NewChunkMerger(directoryManager storage.DirectoryManager) *ChunkMerger {
	return &ChunkMerger{
		directoryManager: directoryManager,
		chunks:           make(map[string][]ChunkInfo),
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

// AddChunks adds chunks to the session's chunk collection
func (cm *ChunkMerger) AddChunks(sessionID string, newChunks []ChunkInfo) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.chunks[sessionID]; !exists {
		cm.chunks[sessionID] = make([]ChunkInfo, 0)
	}

	existingChunks := cm.chunks[sessionID]

	// Create a map of existing chunk indices for quick lookup
	existingIndices := make(map[int]bool)
	for _, chunk := range existingChunks {
		existingIndices[chunk.ChunkIndex] = true
	}

	// Add new chunks that don't already exist
	for _, newChunk := range newChunks {
		if !existingIndices[newChunk.ChunkIndex] {
			existingChunks = append(existingChunks, newChunk)
		}
	}

	cm.chunks[sessionID] = existingChunks
}

// GetChunks returns all chunks for a session
func (cm *ChunkMerger) GetChunks(sessionID string) []ChunkInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.chunks[sessionID]
}

// HasAllChunks checks if all chunks for a session have been received
func (cm *ChunkMerger) HasAllChunks(sessionID string, totalChunks int) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	chunks := cm.chunks[sessionID]
	if len(chunks) != totalChunks {
		return false
	}

	// Check if we have all indices from 0 to totalChunks-1
	indices := make(map[int]bool)
	for _, chunk := range chunks {
		indices[chunk.ChunkIndex] = true
	}

	for i := 0; i < totalChunks; i++ {
		if !indices[i] {
			return false
		}
	}

	return true
}

// MergeChunks merges all chunks for a session into a single file
func (cm *ChunkMerger) MergeChunks(sessionID string, totalChunks int, repoPath string) (*MergeResult, error) {
	chunks := cm.GetChunks(sessionID)

	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks provided for session %s", sessionID)
	}

	// Verify we have all chunks
	if !cm.HasAllChunks(sessionID, totalChunks) {
		return nil, fmt.Errorf("not all chunks received for session %s: have %d, need %d",
			sessionID, len(chunks), totalChunks)
	}

	// Sort chunks by index to ensure correct order
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].ChunkIndex < chunks[j].ChunkIndex
	})

	// Validate chunk sequence
	if err := cm.validateChunkSequence(chunks, totalChunks); err != nil {
		return nil, fmt.Errorf("invalid chunk sequence for session %s: %w", sessionID, err)
	}

	// Create temporary merged file using DirectoryManager
	tempFile, err := cm.directoryManager.CreateTempFile(repoPath, "merged_chunks")
	if err != nil {
		return nil, fmt.Errorf("failed to create merged file: %w", err)
	}

	var totalSize int64

	// Merge chunks sequentially to ensure correct order
	for _, chunk := range chunks {
		err := cm.appendChunkToFile(tempFile.Path, chunk, chunks)
		if err != nil {
			// Clean up the incomplete merged file
			cm.CleanupMergedFile(tempFile.Path)
			return nil, fmt.Errorf("failed to append chunk %d: %w", chunk.ChunkIndex, err)
		}
		totalSize += chunk.Size
	}

	// Verify the final file size matches expected total
	if err := cm.verifyFileSize(tempFile.Path, totalSize); err != nil {
		cm.CleanupMergedFile(tempFile.Path)
		return nil, fmt.Errorf("file size verification failed: %w", err)
	}

	// Clean up the chunk tracking for this session
	cm.ClearSession(sessionID)

	return &MergeResult{
		MergedFilePath: tempFile.Path,
		TotalSize:      totalSize,
	}, nil
}

// appendChunkToFile appends a chunk to the merged file at the correct position
func (cm *ChunkMerger) appendChunkToFile(mergedFilePath string, chunk ChunkInfo, chunks []ChunkInfo) error {
	// Open chunk file
	chunkFile, err := os.Open(chunk.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open chunk file %s: %w", chunk.FilePath, err)
	}
	defer chunkFile.Close()

	// Calculate the position in the merged file
	position, err := cm.calculateChunkPosition(chunk.ChunkIndex, chunks)
	if err != nil {
		return err
	}

	// Open merged file for appending
	mergedFile, err := os.OpenFile(mergedFilePath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open merged file: %w", err)
	}
	defer mergedFile.Close()

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
func (cm *ChunkMerger) calculateChunkPosition(chunkIndex int, chunks []ChunkInfo) (int64, error) {
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
func (cm *ChunkMerger) validateChunkSequence(chunks []ChunkInfo, totalChunks int) error {
	// Check for duplicate chunk indices
	seen := make(map[int]bool)
	for _, chunk := range chunks {
		if seen[chunk.ChunkIndex] {
			return fmt.Errorf("duplicate chunk index %d", chunk.ChunkIndex)
		}
		seen[chunk.ChunkIndex] = true
	}

	// Check if we have a continuous sequence from 0 to totalChunks-1
	for i := 0; i < totalChunks; i++ {
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
func (cm *ChunkMerger) CleanupChunks(sessionID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if chunks, exists := cm.chunks[sessionID]; exists {
		var deletionErrors []string
		successCount := 0

		for _, chunk := range chunks {
			err := os.Remove(chunk.FilePath)
			if err != nil {
				// Log the error but continue with other files
				errMsg := fmt.Sprintf("failed to delete chunk file %s: %v", chunk.FilePath, err)
				log.Print(errMsg)
				deletionErrors = append(deletionErrors, errMsg)
			} else {
				successCount++
			}
		}

		// Log summary of cleanup operation
		if len(deletionErrors) > 0 {
			log.Printf("CleanupChunks: session %s - deleted %d files, %d errors",
				sessionID, successCount, len(deletionErrors))
		} else {
			log.Printf("CleanupChunks: session %s - successfully deleted all %d files",
				sessionID, len(chunks))
		}

		delete(cm.chunks, sessionID)
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

// ClearSession removes all chunks for a session
func (cm *ChunkMerger) ClearSession(sessionID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.chunks, sessionID)
}

// GetChunkCount returns the number of chunks received for a session
func (cm *ChunkMerger) GetChunkCount(sessionID string) int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.chunks[sessionID])
}
