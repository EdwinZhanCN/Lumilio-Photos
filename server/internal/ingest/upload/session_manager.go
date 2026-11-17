package upload

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"server/internal/catalog/repo"
	"server/internal/storage"
	"server/internal/storage/layout"

	"github.com/google/uuid"
)

type UploadSession struct {
	SessionID         string        `json:"session_id"`
	Filename          string        `json:"filename"`
	TotalSize         int64         `json:"total_size"`
	TotalChunks       int           `json:"total_chunks"`
	ReceivedChunks    []int         `json:"received_chunks"`
	ContentType       string        `json:"content_type"`
	ClientFingerprint string        `json:"client_fingerprint"`
	RepositoryID      string        `json:"repository_id"`
	UserID            string        `json:"user_id"`
	Status            string        `json:"status"`
	CreatedAt         time.Time     `json:"created_at"`
	LastActivity      time.Time     `json:"last_activity"`
	BytesReceived     int64         `json:"bytes_received"`
	Error             string        `json:"error,omitempty"`
	ReceiptID         *string       `json:"receipt_id,omitempty"`
	ChunkSizes        map[int]int64 `json:"chunk_sizes,omitempty"`
}

// The session's manifest and chunks live in its upload session directory,
// work/uploads/<session_id>/; layout derives every name, so the manifest
// stores no path.

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
		ChunkSizes: make(map[int]int64),
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

func (sm *SessionManager) UpdateSessionChunk(sessionID string, chunkIndex int, chunkSize int64) bool {
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

func (sm *SessionManager) SetSessionReceiptID(sessionID, receiptID string) bool {
	return sm.mutate(sessionID, func(session *UploadSession) { session.ReceiptID = &receiptID })
}

// EndSession ends a session that completed or failed: it forgets the
// session and removes its manifest, and its directory once nothing else is
// in it. A staged file still awaiting commit keeps the directory until the
// commit moves it out; a failed session was already moved to quarantine.
func (sm *SessionManager) EndSession(sessionID string) bool {
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
		if now.Sub(session.LastActivity) > sm.timeout {
			_ = sm.removeSessionDirectory(session.RepositoryID, id)
			delete(sm.sessions, id)
			removed++
		}
	}
	return removed
}

// SweepStaging removes the repository's upload sessions idle for longer
// than maxAge that nothing will finish: no live session owns them and no
// pending staging commit still needs their file.
func (sm *SessionManager) SweepStaging(ctx context.Context, staging storage.StagingManager, repository repo.Repository, maxAge time.Duration) error {
	if sm.queries == nil {
		return errors.New("upload session persistence unavailable")
	}
	keep := map[uuid.UUID]bool{}
	for _, session := range sm.GetAllSessions() {
		if id, err := uuid.Parse(session.SessionID); err == nil {
			keep[id] = true
		}
	}
	pending, err := sm.queries.ListPendingRepositoryStagingPaths(ctx, repository.RepoID)
	if err != nil {
		return fmt.Errorf("list pending staging commits: %w", err)
	}
	for _, stored := range pending {
		if staged, err := layout.ParseStaged(stored); err == nil {
			if id, ok := layout.StagedSession(staged); ok {
				keep[id] = true
			}
		}
	}
	return staging.CleanupStaging(repository, maxAge, func(id uuid.UUID) bool { return keep[id] })
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
	clone.ChunkSizes = make(map[int]int64, len(session.ChunkSizes))
	for index, value := range session.ChunkSizes {
		clone.ChunkSizes[index] = value
	}
	return &clone
}

func portableFilename(filename string) string {
	return path.Base(strings.ReplaceAll(filename, "\\", "/"))
}

func sessionManifestPath(sessionID string) (layout.Path, error) {
	id, err := uuid.Parse(sessionID)
	if err != nil {
		return layout.Path{}, err
	}
	return layout.UploadManifest(id)
}

// SessionChunks lists the chunks a session received.
func SessionChunks(session *UploadSession) []ChunkInfo {
	chunks := make([]ChunkInfo, 0, len(session.ReceivedChunks))
	for _, index := range session.ReceivedChunks {
		chunks = append(chunks, ChunkInfo{SessionID: session.SessionID, ChunkIndex: index, Size: session.ChunkSizes[index]})
	}
	return chunks
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
	directory, err := manifest.Dir()
	if err != nil {
		return err
	}
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
	if session.ChunkSizes == nil {
		session.ChunkSizes = make(map[int]int64)
	}
	valid := session.ReceivedChunks[:0]
	var bytesReceived int64
	id, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, err
	}
	for _, index := range session.ReceivedChunks {
		privatePath, chunkErr := layout.UploadChunk(id, index)
		if chunkErr != nil {
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
	if directory, err := manifest.Dir(); err == nil {
		_ = repositoryFS.RemovePrivate(directory) // only once it is empty
	}
	return nil
}

// removeSessionDirectory ends an expired session: its whole directory goes.
func (sm *SessionManager) removeSessionDirectory(repositoryID, sessionID string) error {
	if sm.queries == nil || sm.files == nil {
		return nil
	}
	id, err := uuid.Parse(sessionID)
	if err != nil {
		return err
	}
	directory, err := layout.UploadSession(id)
	if err != nil {
		return err
	}
	repositoryFS, err := sm.openRepository(repositoryID)
	if err != nil {
		return err
	}
	defer repositoryFS.Close()
	return repositoryFS.RemoveAllPrivate(directory)
}
