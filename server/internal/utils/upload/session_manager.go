package upload

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// UploadSession represents a file upload session
type UploadSession struct {
	SessionID         string         `json:"session_id"`
	Filename          string         `json:"filename"`
	TotalSize         int64          `json:"total_size"`
	TotalChunks       int            `json:"total_chunks"`
	ReceivedChunks    []int          `json:"received_chunks"`
	ContentType       string         `json:"content_type"`
	ClientFingerprint string         `json:"client_fingerprint"`
	RepositoryID      string         `json:"repository_id"`
	UserID            string         `json:"user_id"`
	Status            string         `json:"status"` // "pending", "uploading", "merging", "completed", "failed"
	CreatedAt         time.Time      `json:"created_at"`
	LastActivity      time.Time      `json:"last_activity"`
	BytesReceived     int64          `json:"bytes_received"`
	Error             string         `json:"error,omitempty"`
	TaskID            *int64         `json:"task_id,omitempty"`
	ChunkFiles        map[int]string `json:"chunk_files,omitempty"`
	ChunkSizes        map[int]int64  `json:"chunk_sizes,omitempty"`
}

// SessionManager manages upload sessions with thread-safe operations
type SessionManager struct {
	sessions map[string]*UploadSession
	mu       sync.RWMutex
	timeout  time.Duration
}

// NewSessionManager creates a new session manager
func NewSessionManager(timeout time.Duration) *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*UploadSession),
		timeout:  timeout,
	}
}

// CreateSession creates a new upload session. If sessionID is empty, a new UUID is generated.
func (sm *SessionManager) CreateSession(sessionID, filename string, totalSize int64, totalChunks int, contentType, repositoryID, userID string) *UploadSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sessionID == "" {
		sessionID = uuid.New().String()
	}
	if restored, err := loadSession(repositoryID, sessionID); err == nil && restored != nil &&
		restored.Filename == filename && restored.TotalSize == totalSize &&
		restored.TotalChunks == totalChunks && restored.UserID == userID {
		restored.RepositoryID = repositoryID
		sm.sessions[sessionID] = restored
		return cloneSession(restored)
	}
	now := time.Now()

	session := &UploadSession{
		SessionID:      sessionID,
		Filename:       filename,
		TotalSize:      totalSize,
		TotalChunks:    totalChunks,
		ReceivedChunks: make([]int, 0),
		ContentType:    contentType,
		RepositoryID:   repositoryID,
		UserID:         userID,
		Status:         "pending",
		CreatedAt:      now,
		LastActivity:   now,
		BytesReceived:  0,
		ChunkFiles:     make(map[int]string),
		ChunkSizes:     make(map[int]int64),
	}

	sm.sessions[sessionID] = session
	_ = persistSession(session)
	return cloneSession(session)
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*UploadSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	return cloneSession(session), exists
}

// UpdateSessionChunk updates session with a new chunk
func (sm *SessionManager) UpdateSessionChunk(sessionID string, chunkIndex int, chunkSize int64, filePath string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return false
	}

	// Check if chunk already received
	for _, received := range session.ReceivedChunks {
		if received == chunkIndex {
			return true
		}
	}

	// Add chunk to received list
	session.ReceivedChunks = append(session.ReceivedChunks, chunkIndex)
	session.BytesReceived += chunkSize
	session.ChunkFiles[chunkIndex] = filePath
	session.ChunkSizes[chunkIndex] = chunkSize
	sort.Ints(session.ReceivedChunks)
	session.LastActivity = time.Now()

	if session.Status == "pending" {
		session.Status = "uploading"
	}

	return persistSession(session) == nil
}

// UpdateSessionStatus updates the status of a session
func (sm *SessionManager) UpdateSessionStatus(sessionID string, status string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return false
	}

	session.Status = status
	session.LastActivity = time.Now()
	return persistSession(session) == nil
}

// SetSessionFingerprint records a non-authoritative client precheck hint.
func (sm *SessionManager) SetSessionFingerprint(sessionID string, fingerprint string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return false
	}

	session.ClientFingerprint = fingerprint
	return persistSession(session) == nil
}

// SetSessionError sets an error message for a session
func (sm *SessionManager) SetSessionError(sessionID string, errorMsg string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return false
	}

	session.Error = errorMsg
	session.Status = "failed"
	session.LastActivity = time.Now()
	return persistSession(session) == nil
}

func (sm *SessionManager) SetSessionTaskID(sessionID string, taskID int64) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	session, exists := sm.sessions[sessionID]
	if !exists {
		return false
	}
	session.TaskID = &taskID
	session.LastActivity = time.Now()
	return persistSession(session) == nil
}

// DeleteSession removes a session
func (sm *SessionManager) DeleteSession(sessionID string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	_, exists := sm.sessions[sessionID]
	if exists {
		_ = os.Remove(sessionManifestPath(sm.sessions[sessionID].RepositoryID, sessionID))
		delete(sm.sessions, sessionID)
		return true
	}
	return false
}

// GetAllSessions returns all active sessions
func (sm *SessionManager) GetAllSessions() []*UploadSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*UploadSession, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, cloneSession(session))
	}
	return sessions
}

// GetSessionsByUser returns sessions for a specific user
func (sm *SessionManager) GetSessionsByUser(userID string) []*UploadSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var userSessions []*UploadSession
	for _, session := range sm.sessions {
		if session.UserID == userID {
			userSessions = append(userSessions, cloneSession(session))
		}
	}
	return userSessions
}

func cloneSession(session *UploadSession) *UploadSession {
	if session == nil {
		return nil
	}
	clone := *session
	clone.ReceivedChunks = append([]int(nil), session.ReceivedChunks...)
	clone.ChunkFiles = make(map[int]string, len(session.ChunkFiles))
	clone.ChunkSizes = make(map[int]int64, len(session.ChunkSizes))
	for k, v := range session.ChunkFiles {
		clone.ChunkFiles[k] = v
	}
	for k, v := range session.ChunkSizes {
		clone.ChunkSizes[k] = v
	}
	return &clone
}

func sessionManifestPath(repoPath, sessionID string) string {
	return filepath.Join(repoPath, ".lumilio", "staging", "incoming", "upload_sessions", sessionID+".json")
}

func persistSession(session *UploadSession) error {
	if _, err := uuid.Parse(session.SessionID); err != nil {
		return fmt.Errorf("invalid session id: %w", err)
	}
	path := sessionManifestPath(session.RepositoryID, session.SessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func loadSession(repoPath, sessionID string) (*UploadSession, error) {
	if _, err := uuid.Parse(sessionID); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(sessionManifestPath(repoPath, sessionID))
	if err != nil {
		return nil, err
	}
	var session UploadSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	if session.ChunkFiles == nil {
		session.ChunkFiles = make(map[int]string)
	}
	if session.ChunkSizes == nil {
		session.ChunkSizes = make(map[int]int64)
	}
	valid := session.ReceivedChunks[:0]
	var bytes int64
	incomingRoot := filepath.Join(repoPath, ".lumilio", "staging", "incoming")
	for _, index := range session.ReceivedChunks {
		path := session.ChunkFiles[index]
		rel, relErr := filepath.Rel(incomingRoot, path)
		if relErr != nil || rel == ".." || filepath.IsAbs(rel) || len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator) {
			continue
		}
		info, err := os.Stat(path)
		if err == nil && info.Size() == session.ChunkSizes[index] {
			valid = append(valid, index)
			bytes += info.Size()
		}
	}
	session.ReceivedChunks, session.BytesReceived = valid, bytes
	return &session, nil
}

// CleanupExpiredSessions removes sessions that have timed out
func (sm *SessionManager) CleanupExpiredSessions() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	expiredCount := 0

	for sessionID, session := range sm.sessions {
		// Skip completed sessions
		if session.Status == "completed" {
			continue
		}

		// Check if session has timed out
		if now.Sub(session.LastActivity) > sm.timeout {
			delete(sm.sessions, sessionID)
			expiredCount++
		}
	}

	return expiredCount
}

// GetSessionProgress calculates the progress of a session
func (sm *SessionManager) GetSessionProgress(sessionID string) (float64, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return 0, false
	}

	if session.TotalChunks == 0 {
		return 0, true
	}

	progress := float64(len(session.ReceivedChunks)) / float64(session.TotalChunks)
	return progress, true
}

// IsSessionComplete checks if all chunks have been received
func (sm *SessionManager) IsSessionComplete(sessionID string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return false
	}

	return len(session.ReceivedChunks) == session.TotalChunks
}

// GetActiveSessionCount returns the number of active sessions
func (sm *SessionManager) GetActiveSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	count := 0
	for _, session := range sm.sessions {
		if session.Status != "completed" && session.Status != "failed" {
			count++
		}
	}
	return count
}
