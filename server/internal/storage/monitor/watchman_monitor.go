package monitor

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"server/config"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/storage"
	"server/internal/storage/watchman"
	"server/internal/utils/file"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

const (
	watchmanClockFile = "watchman.clock"
)

type pendingEntry struct {
	StoragePath string
	FullPath    string
	Filename    string
	LastSize    int64
	LastMTimeMs int64
	ReadyAt     time.Time
	Attempts    int
}

// WatchmanMonitor monitors repository workspace trees and enqueues discovery jobs.
type WatchmanMonitor struct {
	queries    *repo.Queries
	queue      *river.Client[pgx.Tx]
	cfg        config.WatchmanConfig
	extensions []string

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewWatchmanMonitor constructs monitor service.
func NewWatchmanMonitor(
	queries *repo.Queries,
	queue *river.Client[pgx.Tx],
	cfg config.WatchmanConfig,
) *WatchmanMonitor {
	rawExt := file.GetSupportedExtensions()
	suffixes := make([]string, 0, len(rawExt))
	seen := make(map[string]struct{}, len(rawExt))
	for _, ext := range rawExt {
		trimmed := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(ext)), ".")
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		suffixes = append(suffixes, trimmed)
	}
	slices.Sort(suffixes)

	return &WatchmanMonitor{
		queries:    queries,
		queue:      queue,
		cfg:        cfg,
		extensions: suffixes,
	}
}

// Start begins monitoring all active repositories.
func (m *WatchmanMonitor) Start(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if !m.cfg.Enabled {
		log.Println("ℹ️  Watchman monitor disabled (WATCHMAN_ENABLED=false)")
		return nil
	}
	if strings.TrimSpace(m.cfg.SocketPath) == "" {
		return fmt.Errorf("WATCHMAN_ENABLED=true but WATCHMAN_SOCK is empty")
	}
	if m.queue == nil {
		return fmt.Errorf("watchman monitor requires queue client")
	}
	if m.queries == nil {
		return fmt.Errorf("watchman monitor requires repository queries")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return nil
	}

	repos, err := m.queries.ListActiveRepositories(ctx)
	if err != nil {
		return fmt.Errorf("list active repositories: %w", err)
	}
	if len(repos) == 0 {
		log.Println("ℹ️  Watchman monitor: no active repositories found")
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	for _, r := range repos {
		repoItem := r
		if !repoItem.RepoID.Valid {
			continue
		}
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.runRepositoryLoop(runCtx, repoItem)
		}()
	}

	return nil
}

// Stop stops all monitoring goroutines.
func (m *WatchmanMonitor) Stop() {
	if m == nil {
		return
	}

	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	m.wg.Wait()
}

func (m *WatchmanMonitor) runRepositoryLoop(ctx context.Context, repository repo.Repository) {
	repoID := uuid.UUID(repository.RepoID.Bytes).String()
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}

		if err := m.watchRepositorySession(ctx, repository); err != nil {
			log.Printf("⚠️  Watchman monitor (%s): %v", repoID, err)

			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}

			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}

		backoff = time.Second
	}
}

func (m *WatchmanMonitor) watchRepositorySession(ctx context.Context, repository repo.Repository) error {
	client, err := watchman.Dial(ctx, m.cfg.SocketPath)
	if err != nil {
		return err
	}
	defer client.Close()

	if _, err := client.Version(ctx); err != nil {
		return fmt.Errorf("version check failed: %w", err)
	}

	wp, err := client.WatchProject(ctx, repository.Path)
	if err != nil {
		return fmt.Errorf("watch-project %s: %w", repository.Path, err)
	}

	relativeRoot := joinWatchRelative(wp.RelativePath)
	expression := m.buildExpression()
	clockToken, _ := m.loadClock(repository.Path)

	if m.cfg.InitialScan {
		queryOpts := map[string]any{
			"expression":    expression,
			"fields":        []string{"name", "exists", "new", "type", "size", "mtime_ms"},
			"relative_root": relativeRoot,
		}
		if clockToken != "" {
			queryOpts["since"] = clockToken
		}

		result, err := client.Query(ctx, wp.Watch, queryOpts)
		if err != nil {
			return fmt.Errorf("initial query failed: %w", err)
		}
		if result.Clock != "" {
			clockToken = result.Clock
			_ = m.saveClock(repository.Path, result.Clock)
		}
		if err := m.enqueueReadyFiles(ctx, repository, result.Files); err != nil {
			log.Printf("⚠️  Watchman initial enqueue (%s): %v", repository.Name, err)
		}
	}

	if clockToken == "" {
		clockToken, err = client.Clock(ctx, wp.Watch)
		if err != nil {
			return fmt.Errorf("clock failed: %w", err)
		}
		_ = m.saveClock(repository.Path, clockToken)
	}

	subscriptionName := fmt.Sprintf("lumilio-%s", uuid.UUID(repository.RepoID.Bytes).String())
	subscribeOpts := map[string]any{
		"expression":    expression,
		"fields":        []string{"name", "exists", "new", "type", "size", "mtime_ms"},
		"relative_root": relativeRoot,
		"since":         clockToken,
	}

	subClock, err := client.Subscribe(ctx, wp.Watch, subscriptionName, subscribeOpts)
	if err != nil {
		return fmt.Errorf("subscribe failed: %w", err)
	}
	if subClock != "" {
		_ = m.saveClock(repository.Path, subClock)
	}

	settle := time.Duration(maxInt(m.cfg.SettleSeconds, 1)) * time.Second
	pending := make(map[string]*pendingEntry)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			m.flushPending(ctx, repository, pending, settle)
		default:
			msg, err := client.ReadMessage(1 * time.Second)
			if err != nil {
				if watchman.IsTimeoutError(err) {
					continue
				}
				return err
			}

			subName, _ := msg["subscription"].(string)
			if subName == "" || subName != subscriptionName {
				continue
			}

			result, err := watchman.ParseQueryResult(msg)
			if err != nil {
				log.Printf("⚠️  Watchman parse payload (%s): %v", repository.Name, err)
				continue
			}
			if result.Clock != "" {
				_ = m.saveClock(repository.Path, result.Clock)
			}

			now := time.Now()
			for _, f := range result.Files {
				cleaned, ok := cleanRelativePath(f.Name)
				if !ok {
					continue
				}
				if !f.Exists {
					delete(pending, cleaned)
					continue
				}
				if f.Type != "" && f.Type != "f" {
					continue
				}
				if !file.IsSupportedExtension(filepath.Ext(cleaned)) {
					continue
				}
				if isExcludedWorkspacePath(cleaned) {
					continue
				}

				storagePath := cleaned
				fullPath := filepath.Join(repository.Path, filepath.FromSlash(storagePath))

				entry, exists := pending[cleaned]
				if !exists {
					entry = &pendingEntry{
						StoragePath: storagePath,
						FullPath:    fullPath,
						Filename:    filepath.Base(cleaned),
					}
					pending[cleaned] = entry
				}
				if f.Size > 0 {
					entry.LastSize = f.Size
				}
				if f.MTimeMs > 0 {
					entry.LastMTimeMs = f.MTimeMs
				}
				entry.ReadyAt = now.Add(settle)
			}
		}
	}
}

func (m *WatchmanMonitor) enqueueReadyFiles(ctx context.Context, repository repo.Repository, files []watchman.FileEvent) error {
	repoID := uuid.UUID(repository.RepoID.Bytes).String()
	for _, f := range files {
		cleaned, ok := cleanRelativePath(f.Name)
		if !ok {
			continue
		}
		if !f.Exists {
			continue
		}
		if f.Type != "" && f.Type != "f" {
			continue
		}
		if !file.IsSupportedExtension(filepath.Ext(cleaned)) {
			continue
		}
		if isExcludedWorkspacePath(cleaned) {
			continue
		}
		storagePath := cleaned
		fullPath := filepath.Join(repository.Path, filepath.FromSlash(storagePath))
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			continue
		}

		if err := m.enqueueDiscover(ctx, repoID, storagePath, filepath.Base(cleaned), info); err != nil {
			log.Printf("⚠️  Watchman initial enqueue (%s:%s): %v", repository.Name, storagePath, err)
		}
	}
	return nil
}

func (m *WatchmanMonitor) flushPending(
	ctx context.Context,
	repository repo.Repository,
	pending map[string]*pendingEntry,
	settle time.Duration,
) {
	if len(pending) == 0 {
		return
	}
	now := time.Now()
	repoID := uuid.UUID(repository.RepoID.Bytes).String()

	for key, entry := range pending {
		if now.Before(entry.ReadyAt) {
			continue
		}

		info, err := os.Stat(entry.FullPath)
		if err != nil || info.IsDir() {
			delete(pending, key)
			continue
		}

		size := info.Size()
		mtime := info.ModTime().UnixMilli()
		if size != entry.LastSize || (entry.LastMTimeMs > 0 && mtime != entry.LastMTimeMs) {
			entry.LastSize = size
			entry.LastMTimeMs = mtime
			entry.ReadyAt = now.Add(settle)
			continue
		}

		if err := m.enqueueDiscover(ctx, repoID, entry.StoragePath, entry.Filename, info); err != nil {
			entry.Attempts++
			if entry.Attempts >= 3 {
				log.Printf("⚠️  Watchman enqueue failed after retries (%s:%s): %v", repository.Name, entry.StoragePath, err)
				delete(pending, key)
				continue
			}
			entry.ReadyAt = now.Add(2 * time.Second)
			continue
		}

		delete(pending, key)
	}
}

func (m *WatchmanMonitor) enqueueDiscover(
	ctx context.Context,
	repoID string,
	storagePath string,
	filename string,
	info os.FileInfo,
) error {
	args := jobs.DiscoverAssetArgs{
		RepositoryID: repoID,
		RelativePath: filepath.ToSlash(storagePath),
		FileName:     filename,
		ContentType:  file.NewValidator().GetMimeTypeFromExtension(filepath.Ext(filename)),
		FileSize:     info.Size(),
		DetectedAt:   time.Now().UTC(),
	}

	_, err := m.queue.Insert(ctx, args, &river.InsertOpts{Queue: "discover_asset"})
	return err
}

func (m *WatchmanMonitor) buildExpression() []any {
	suffixAny := make([]any, 0, len(m.extensions))
	for _, ext := range m.extensions {
		suffixAny = append(suffixAny, []any{"suffix", ext})
	}
	return []any{
		"allof",
		[]any{"type", "f"},
		[]any{"not", []any{"match", ".lumilio/**", "wholename"}},
		[]any{"not", []any{"match", "inbox/**", "wholename"}},
		append([]any{"anyof"}, suffixAny...),
	}
}

func (m *WatchmanMonitor) loadClock(repoPath string) (string, error) {
	b, err := os.ReadFile(m.clockFilePath(repoPath))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func (m *WatchmanMonitor) saveClock(repoPath, clock string) error {
	if strings.TrimSpace(clock) == "" {
		return nil
	}

	path := m.clockFilePath(repoPath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(clock), 0644)
}

func (m *WatchmanMonitor) clockFilePath(repoPath string) string {
	return filepath.Join(repoPath, storage.DefaultStructure.SystemDir, watchmanClockFile)
}

func joinWatchRelative(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" || t == "." {
			continue
		}
		cleaned = append(cleaned, t)
	}
	if len(cleaned) == 0 {
		return ""
	}
	return filepath.ToSlash(filepath.Join(cleaned...))
}

func cleanRelativePath(path string) (string, bool) {
	if strings.TrimSpace(path) == "" {
		return "", false
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "." || filepath.IsAbs(clean) {
		return "", false
	}
	if strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", false
	}
	return filepath.ToSlash(clean), true
}

func isExcludedWorkspacePath(path string) bool {
	normalized := filepath.ToSlash(strings.TrimSpace(path))
	if normalized == "" {
		return true
	}
	if normalized == storage.DefaultStructure.SystemDir || strings.HasPrefix(normalized, storage.DefaultStructure.SystemDir+"/") {
		return true
	}
	if normalized == storage.DefaultStructure.InboxDir || strings.HasPrefix(normalized, storage.DefaultStructure.InboxDir+"/") {
		return true
	}
	return false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
