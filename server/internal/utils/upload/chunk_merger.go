package upload

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"

	"server/internal/db/repo"
	"server/internal/storage"
)

// ChunkMerger tracks relative chunk handles and streams them into a new
// repository-private staging file. It never receives a host path.
type ChunkMerger struct {
	staging storage.StagingManager
	mu      sync.RWMutex
	chunks  map[string][]ChunkInfo
}

func NewChunkMerger(staging storage.StagingManager) *ChunkMerger {
	return &ChunkMerger{staging: staging, chunks: make(map[string][]ChunkInfo)}
}

type MergeResult struct {
	StagingFile *storage.StagingFile `json:"staging_file"`
	TotalSize   int64                `json:"total_size"`
}

type ChunkInfo struct {
	SessionID   string `json:"session_id"`
	ChunkIndex  int    `json:"chunk_index"`
	PrivatePath string `json:"private_path"`
	Size        int64  `json:"size"`
}

func (cm *ChunkMerger) AddChunks(sessionID string, newChunks []ChunkInfo) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	existing := cm.chunks[sessionID]
	seen := make(map[int]bool, len(existing))
	for _, chunk := range existing {
		seen[chunk.ChunkIndex] = true
	}
	for _, chunk := range newChunks {
		if !seen[chunk.ChunkIndex] {
			existing = append(existing, chunk)
			seen[chunk.ChunkIndex] = true
		}
	}
	cm.chunks[sessionID] = existing
}

func (cm *ChunkMerger) GetChunks(sessionID string) []ChunkInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return append([]ChunkInfo(nil), cm.chunks[sessionID]...)
}

func (cm *ChunkMerger) HasAllChunks(sessionID string, totalChunks int) bool {
	chunks := cm.GetChunks(sessionID)
	if len(chunks) != totalChunks {
		return false
	}
	seen := make(map[int]bool, len(chunks))
	for _, chunk := range chunks {
		seen[chunk.ChunkIndex] = true
	}
	for index := 0; index < totalChunks; index++ {
		if !seen[index] {
			return false
		}
	}
	return true
}

func (cm *ChunkMerger) MergeChunks(repository repo.Repository, sessionID string, totalChunks int, filename string) (*MergeResult, error) {
	chunks := cm.GetChunks(sessionID)
	if !cm.HasAllChunks(sessionID, totalChunks) {
		return nil, fmt.Errorf("incomplete chunk sequence: have %d, need %d", len(chunks), totalChunks)
	}
	sort.Slice(chunks, func(i, j int) bool { return chunks[i].ChunkIndex < chunks[j].ChunkIndex })
	for index, chunk := range chunks {
		if chunk.ChunkIndex != index {
			return nil, fmt.Errorf("missing chunk index %d", index)
		}
	}

	merged, destination, err := cm.staging.CreateStagingFile(repository, filename)
	if err != nil {
		return nil, err
	}
	cleanup := true
	defer func() {
		_ = destination.Close()
		if cleanup {
			_ = cm.staging.RemoveStagingFile(repository, merged)
		}
	}()
	buffer := make([]byte, 1<<20)
	var totalSize int64
	for _, chunk := range chunks {
		handle := &storage.StagingFile{ID: chunk.SessionID, RepositoryID: repository.RepoID, PrivatePath: chunk.PrivatePath}
		source, err := cm.staging.OpenStagingFile(repository, handle)
		if err != nil {
			return nil, fmt.Errorf("open chunk %d: %w", chunk.ChunkIndex, err)
		}
		written, copyErr := io.CopyBuffer(destination, source, buffer)
		closeErr := source.Close()
		if copyErr != nil || closeErr != nil {
			return nil, fmt.Errorf("copy chunk %d: %w", chunk.ChunkIndex, errors.Join(copyErr, closeErr))
		}
		if written != chunk.Size {
			return nil, fmt.Errorf("chunk %d size mismatch: got %d, want %d", chunk.ChunkIndex, written, chunk.Size)
		}
		totalSize += written
	}
	if err := destination.Sync(); err != nil {
		return nil, err
	}
	info, err := destination.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() != totalSize {
		return nil, fmt.Errorf("merged file size mismatch: got %d, want %d", info.Size(), totalSize)
	}
	if err := destination.Close(); err != nil {
		return nil, err
	}
	cleanup = false
	return &MergeResult{StagingFile: merged, TotalSize: totalSize}, nil
}

func (cm *ChunkMerger) CleanupChunks(repository repo.Repository, sessionID string) {
	cm.mu.Lock()
	chunks := cm.chunks[sessionID]
	delete(cm.chunks, sessionID)
	cm.mu.Unlock()
	for _, chunk := range chunks {
		_ = cm.staging.RemoveStagingFile(repository, &storage.StagingFile{
			ID: chunk.SessionID, RepositoryID: repository.RepoID, PrivatePath: chunk.PrivatePath,
		})
	}
}

func (cm *ChunkMerger) ClearSession(sessionID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.chunks, sessionID)
}

func (cm *ChunkMerger) GetChunkCount(sessionID string) int {
	return len(cm.GetChunks(sessionID))
}
