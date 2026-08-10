package upload

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"server/internal/db/repo"
	"server/internal/storage"

	"github.com/google/uuid"
)

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
	Status            string         `json:"status"`
	CreatedAt         time.Time      `json:"created_at"`
	LastActivity      time.Time      `json:"last_activity"`
	BytesReceived     int64          `json:"bytes_received"`
	Error             string         `json:"error,omitempty"`
	TaskID            *int64         `json:"task_id,omitempty"`
	ChunkFiles        map[int]string `json:"chunk_files,omitempty"` // private repository-relative paths
	ChunkSizes        map[int]int64  `json:"chunk_sizes,omitempty"`
}

type SessionManager struct {
	sessions map[string]*UploadSession
	mu       sync.RWMutex
	timeout  time.Duration
	queries  *repo.Queries
	files    *storage.RepositoryFSFactory
}

func NewSessionManager(timeout time.Duration, queries *repo.Queries, files *storage.RepositoryFSFactory) *SessionManager {
	return &SessionManager{sessions: make(map[string]*UploadSession), timeout: timeout, queries: queries, files: files}
}

func (sm *SessionManager) CreateSession(sessionID, filename string, totalSize int64, totalChunks int, contentType, repositoryID, userID string) *UploadSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sessionID == "" {
		sessionID = uuid.NewString()
	}
	if restored, err := sm.loadSession(repositoryID, sessionID); err == nil && restored != nil &&
		restored.Filename == portableFilename(filename) && restored.TotalSize == totalSize &&
		restored.TotalChunks == totalChunks && restored.UserID == userID {
		sm.sessions[sessionID] = restored
		return cloneSession(restored)
	}
	now := time.Now().UTC()
	session := &UploadSession{
		SessionID: sessionID, Filename: portableFilename(filename), TotalSize: totalSize, TotalChunks: totalChunks,
		ReceivedChunks: []int{}, ContentType: contentType, RepositoryID: repositoryID, UserID: userID,
		Status: "pending", CreatedAt: now, LastActivity: now,
		ChunkFiles: make(map[int]string), ChunkSizes: make(map[int]int64),
	}
	sm.sessions[sessionID] = session
	_ = sm.persistSession(session)
	return cloneSession(session)
}

func (sm *SessionManager) GetSession(sessionID string) (*UploadSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	session, ok := sm.sessions[sessionID]
	return cloneSession(session), ok
}

func (sm *SessionManager) UpdateSessionChunk(sessionID string, chunkIndex int, chunkSize int64, privatePath string) bool {
	if _, err := storage.ParsePrivateRepositoryPath(privatePath); err != nil {
		return false
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	session, ok := sm.sessions[sessionID]
	if !ok {
		return false
	}
	for _, received := range session.ReceivedChunks {
		if received == chunkIndex {
			return true
		}
	}
	session.ReceivedChunks = append(session.ReceivedChunks, chunkIndex)
	sort.Ints(session.ReceivedChunks)
	session.BytesReceived += chunkSize
	session.ChunkFiles[chunkIndex] = privatePath
	session.ChunkSizes[chunkIndex] = chunkSize
	session.LastActivity = time.Now().UTC()
	if session.Status == "pending" {
		session.Status = "uploading"
	}
	return sm.persistSession(session) == nil
}

func (sm *SessionManager) mutate(sessionID string, apply func(*UploadSession)) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	session, ok := sm.sessions[sessionID]
	if !ok {
		return false
	}
	apply(session)
	session.LastActivity = time.Now().UTC()
	return sm.persistSession(session) == nil
}

func (sm *SessionManager) UpdateSessionStatus(sessionID, status string) bool {
	return sm.mutate(sessionID, func(session *UploadSession) { session.Status = status })
}

func (sm *SessionManager) SetSessionFingerprint(sessionID, fingerprint string) bool {
	return sm.mutate(sessionID, func(session *UploadSession) { session.ClientFingerprint = fingerprint })
}

func (sm *SessionManager) SetSessionError(sessionID, message string) bool {
	return sm.mutate(sessionID, func(session *UploadSession) { session.Error, session.Status = message, "failed" })
}

func (sm *SessionManager) SetSessionTaskID(sessionID string, taskID int64) bool {
	return sm.mutate(sessionID, func(session *UploadSession) { session.TaskID = &taskID })
}

func (sm *SessionManager) DeleteSession(sessionID string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	session, ok := sm.sessions[sessionID]
	if !ok {
		return false
	}
	_ = sm.removeManifest(session.RepositoryID, sessionID)
	delete(sm.sessions, sessionID)
	return true
}

func (sm *SessionManager) GetAllSessions() []*UploadSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make([]*UploadSession, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		result = append(result, cloneSession(session))
	}
	return result
}

func (sm *SessionManager) GetSessionsByUser(userID string) []*UploadSession {
	all := sm.GetAllSessions()
	result := all[:0]
	for _, session := range all {
		if session.UserID == userID {
			result = append(result, session)
		}
	}
	return result
}

func (sm *SessionManager) CleanupExpiredSessions() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	now := time.Now().UTC()
	removed := 0
	for id, session := range sm.sessions {
		if session.Status != "completed" && now.Sub(session.LastActivity) > sm.timeout {
			_ = sm.removeManifest(session.RepositoryID, id)
			delete(sm.sessions, id)
			removed++
		}
	}
	return removed
}

func (sm *SessionManager) GetSessionProgress(sessionID string) (float64, bool) {
	session, ok := sm.GetSession(sessionID)
	if !ok || session.TotalChunks == 0 {
		return 0, ok
	}
	return float64(len(session.ReceivedChunks)) / float64(session.TotalChunks), true
}

func (sm *SessionManager) IsSessionComplete(sessionID string) bool {
	session, ok := sm.GetSession(sessionID)
	return ok && len(session.ReceivedChunks) == session.TotalChunks
}

func (sm *SessionManager) GetActiveSessionCount() int {
	count := 0
	for _, session := range sm.GetAllSessions() {
		if session.Status != "completed" && session.Status != "failed" {
			count++
		}
	}
	return count
}

func cloneSession(session *UploadSession) *UploadSession {
	if session == nil {
		return nil
	}
	clone := *session
	clone.ReceivedChunks = append([]int(nil), session.ReceivedChunks...)
	clone.ChunkFiles = make(map[int]string, len(session.ChunkFiles))
	clone.ChunkSizes = make(map[int]int64, len(session.ChunkSizes))
	for index, value := range session.ChunkFiles {
		clone.ChunkFiles[index] = value
	}
	for index, value := range session.ChunkSizes {
		clone.ChunkSizes[index] = value
	}
	return &clone
}

func portableFilename(filename string) string {
	return path.Base(strings.ReplaceAll(filename, "\\", "/"))
}

func sessionManifestPath(sessionID string) (storage.RepositoryPath, error) {
	if _, err := uuid.Parse(sessionID); err != nil {
		return storage.RepositoryPath{}, err
	}
	return storage.ParsePrivateRepositoryPath(path.Join(storage.DefaultStructure.IncomingDir, "upload_sessions", sessionID+".json"))
}

func (sm *SessionManager) openRepository(repositoryID string) (*storage.RepositoryFS, error) {
	if sm.queries == nil || sm.files == nil {
		return nil, errors.New("upload session persistence unavailable")
	}
	id, err := uuid.Parse(repositoryID)
	if err != nil {
		return nil, err
	}
	repository, err := sm.queries.GetRepository(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return sm.files.Open(repository)
}

func (sm *SessionManager) persistSession(session *UploadSession) error {
	if sm.queries == nil || sm.files == nil {
		return nil
	}
	manifest, err := sessionManifestPath(session.SessionID)
	if err != nil {
		return err
	}
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	repositoryFS, err := sm.openRepository(session.RepositoryID)
	if err != nil {
		return err
	}
	defer repositoryFS.Close()
	directory, _ := storage.ParsePrivateRepositoryPath(path.Dir(manifest.String()))
	if err := repositoryFS.MkdirAllPrivate(directory, 0o700); err != nil {
		return err
	}
	_, err = repositoryFS.WritePrivateFileAtomic(manifest, bytes.NewReader(data), 0o600)
	return err
}

func (sm *SessionManager) loadSession(repositoryID, sessionID string) (*UploadSession, error) {
	if sm.queries == nil || sm.files == nil {
		return nil, fs.ErrNotExist
	}
	manifest, err := sessionManifestPath(sessionID)
	if err != nil {
		return nil, err
	}
	repositoryFS, err := sm.openRepository(repositoryID)
	if err != nil {
		return nil, err
	}
	defer repositoryFS.Close()
	data, err := repositoryFS.ReadPrivateFile(manifest)
	if err != nil {
		return nil, err
	}
	var session UploadSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	if session.RepositoryID != repositoryID {
		return nil, errors.New("upload session repository mismatch")
	}
	if session.ChunkFiles == nil {
		session.ChunkFiles = make(map[int]string)
	}
	if session.ChunkSizes == nil {
		session.ChunkSizes = make(map[int]int64)
	}
	valid := session.ReceivedChunks[:0]
	var bytesReceived int64
	for _, index := range session.ReceivedChunks {
		privatePath, parseErr := storage.ParsePrivateRepositoryPath(session.ChunkFiles[index])
		if parseErr != nil {
			continue
		}
		info, statErr := repositoryFS.StatPrivate(privatePath)
		if statErr == nil && info.Size() == session.ChunkSizes[index] {
			valid = append(valid, index)
			bytesReceived += info.Size()
		}
	}
	session.ReceivedChunks, session.BytesReceived = valid, bytesReceived
	return &session, nil
}

func (sm *SessionManager) removeManifest(repositoryID, sessionID string) error {
	if sm.queries == nil || sm.files == nil {
		return nil
	}
	manifest, err := sessionManifestPath(sessionID)
	if err != nil {
		return err
	}
	repositoryFS, err := sm.openRepository(repositoryID)
	if err != nil {
		return err
	}
	defer repositoryFS.Close()
	if err := repositoryFS.RemovePrivate(manifest); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
