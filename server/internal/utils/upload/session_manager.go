package upload

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// UploadSession represents a file upload session
type UploadSession struct {
	SessionID      string    `json:"session_id"`
	Filename       string    `json:"filename"`
	TotalSize      int64     `json:"total_size"`
	TotalChunks    int       `json:"total_chunks"`
	ReceivedChunks []int     `json:"received_chunks"`
	ContentType    string    `json:"content_type"`
	RepositoryID   string    `json:"repository_id"`
	UserID         string    `json:"user_id"`
	Status         string    `json:"status"` // "pending", "uploading", "merging", "completed", "failed"
	CreatedAt      time.Time `json:"created_at"`
	LastActivity   time.Time `json:"last_activity"`
	BytesReceived  int64     `json:"bytes_received"`
	Error          string    `json:"error,omitempty"`
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
	}

	sm.sessions[sessionID] = session
	return session
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*UploadSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	return session, exists
}

// UpdateSessionChunk updates session with a new chunk
func (sm *SessionManager) UpdateSessionChunk(sessionID string, chunkIndex int, chunkSize int64) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return false
	}

	// Check if chunk already received
	for _, received := range session.ReceivedChunks {
		if received == chunkIndex {
			return true // Chunk already processed
		}
	}

	// Add chunk to received list
	session.ReceivedChunks = append(session.ReceivedChunks, chunkIndex)
	session.BytesReceived += chunkSize
	session.LastActivity = time.Now()

	if session.Status == "pending" {
		session.Status = "uploading"
	}

	return true
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
	return true
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
	return true
}

// DeleteSession removes a session
func (sm *SessionManager) DeleteSession(sessionID string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	_, exists := sm.sessions[sessionID]
	if exists {
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
		sessions = append(sessions, session)
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
			userSessions = append(userSessions, session)
		}
	}
	return userSessions
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
