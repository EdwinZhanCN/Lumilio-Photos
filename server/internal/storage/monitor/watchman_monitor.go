package monitor

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"server/config"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/storage"
	"server/internal/storage/repocfg"
	"server/internal/storage/watchman"
	"server/internal/utils/file"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"go.uber.org/zap"
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

type fileSnapshot struct {
	Size    int64
	MTimeMs int64
}

// WatchmanMonitor monitors repository workspace trees and enqueues discovery jobs.
type WatchmanMonitor struct {
	queries    *repo.Queries
	queue      *river.Client[pgx.Tx]
	cfg        config.WatchmanConfig
	extensions []string
	logger     *zap.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewWatchmanMonitor constructs monitor service.
func NewWatchmanMonitor(
	queries *repo.Queries,
	queue *river.Client[pgx.Tx],
	cfg config.WatchmanConfig,
	logger *zap.Logger,
) *WatchmanMonitor {
	if logger == nil {
		logger = zap.NewNop()
	}
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
		logger:     logger.With(zap.String("component", "watchman")),
	}
}

// Start begins monitoring all active repositories.
func (m *WatchmanMonitor) Start(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if !m.cfg.Enabled {
		m.logger.Info("watchman monitor disabled", zap.String("operation", "watchman.start"))
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
		m.logger.Info("watchman monitor found no active repositories", zap.String("operation", "watchman.start"))
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	started := 0
	for _, r := range repos {
		repoItem := r
		if !repoItem.RepoID.Valid {
			continue
		}
		if !isWatchableRepositoryRoot(repoItem.Path) {
			m.logger.Warn("watchman monitor skipped non-watchable repository",
				zap.String("operation", "watchman.start"),
				zap.String("repository_path", repoItem.Path),
				zap.String("repository_id", uuid.UUID(repoItem.RepoID.Bytes).String()),
			)
			continue
		}
		m.wg.Add(1)
		started++
		go func() {
			defer m.wg.Done()
			m.runRepositoryLoop(runCtx, repoItem)
		}()
	}

	if started == 0 {
		cancel()
		m.cancel = nil
		return fmt.Errorf("watchman monitor has no valid repository roots to watch")
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
			m.logger.Warn("watchman repository session failed",
				zap.String("operation", "watchman.session"),
				zap.String("repository_id", repoID),
				zap.Error(err),
			)

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
	repoID := uuid.UUID(repository.RepoID.Bytes).String()

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
			"expression": expression,
			"fields":     []string{"name", "exists", "new", "type", "size", "mtime_ms"},
		}
		if relativeRoot != "" {
			queryOpts["relative_root"] = relativeRoot
		}
		if clockToken != "" {
			queryOpts["since"] = clockToken
		}

		result, queryErr := client.Query(ctx, wp.Watch, queryOpts)
		if queryErr != nil && clockToken != "" {
			// Clock tokens can become invalid after watchman restarts/rebuilds.
			// Fallback to a full scan instead of stalling the monitor loop forever.
			m.logger.Warn("watchman initial query with saved clock failed; retrying full scan",
				zap.String("operation", "watchman.initial_query"),
				zap.String("repository_name", repository.Name),
				zap.String("clock", clockToken),
				zap.Error(queryErr),
			)
			clockToken = ""
			_ = m.clearClock(repository.Path)
			delete(queryOpts, "since")
			result, queryErr = client.Query(ctx, wp.Watch, queryOpts)
		}
		if queryErr != nil {
			return fmt.Errorf("initial query failed: %w", queryErr)
		}

		if result.Clock != "" {
			clockToken = result.Clock
			_ = m.saveClock(repository.Path, result.Clock)
		}
		if err := m.enqueueReadyFiles(ctx, repository, result.Files); err != nil {
			m.logger.Warn("watchman initial enqueue failed",
				zap.String("operation", "watchman.initial_enqueue"),
				zap.String("repository_name", repository.Name),
				zap.Error(err),
			)
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
		"expression": expression,
		"fields":     []string{"name", "exists", "new", "type", "size", "mtime_ms"},
		"since":      clockToken,
	}
	if relativeRoot != "" {
		subscribeOpts["relative_root"] = relativeRoot
	}

	subClock, err := client.Subscribe(ctx, wp.Watch, subscriptionName, subscribeOpts)
	if err != nil {
		if clockToken != "" {
			m.logger.Warn("watchman subscribe with saved clock failed; retrying with fresh clock",
				zap.String("operation", "watchman.subscribe"),
				zap.String("repository_name", repository.Name),
				zap.String("clock", clockToken),
				zap.Error(err),
			)
			freshClock, clockErr := client.Clock(ctx, wp.Watch)
			if clockErr != nil {
				return fmt.Errorf("subscribe failed and fresh clock failed: %w / %v", err, clockErr)
			}
			subscribeOpts["since"] = freshClock
			subClock, err = client.Subscribe(ctx, wp.Watch, subscriptionName, subscribeOpts)
		}
		if err != nil {
			return fmt.Errorf("subscribe failed: %w", err)
		}
	}
	if subClock != "" {
		_ = m.saveClock(repository.Path, subClock)
	}
	settle := time.Duration(maxInt(m.cfg.SettleSeconds, 1)) * time.Second
	m.logger.Info("watchman monitor subscribed",
		zap.String("operation", "watchman.subscribe"),
		zap.String("repository_name", repository.Name),
		zap.String("repository_id", repoID),
		zap.String("watch", wp.Watch),
		zap.String("relative_root", relativeRoot),
		zap.String("clock", subClock),
	)
	pending := make(map[string]*pendingEntry)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var (
		snapshot   map[string]fileSnapshot
		pollTicker *time.Ticker
		pollCh     <-chan time.Time
	)
	msgCh := make(chan watchman.Response, 1)
	readErrCh := make(chan error, 1)

	go func() {
		for {
			msg, err := client.ReadMessage(0)
			if err != nil {
				select {
				case readErrCh <- err:
				case <-ctx.Done():
				}
				return
			}

			select {
			case msgCh <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()

	if m.cfg.PollFallbackSeconds > 0 {
		snapshot, err = snapshotRepositoryFiles(repository.Path)
		if err != nil {
			m.logger.Warn("watchman poll fallback snapshot init failed",
				zap.String("operation", "watchman.poll"),
				zap.String("repository_name", repository.Name),
				zap.Error(err),
			)
		}
		pollTicker = time.NewTicker(time.Duration(m.cfg.PollFallbackSeconds) * time.Second)
		pollCh = pollTicker.C
		defer pollTicker.Stop()
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-readErrCh:
			return err
		case <-ticker.C:
			m.flushPending(ctx, repository, pending, settle)
		case <-pollCh:
			current, scanErr := snapshotRepositoryFiles(repository.Path)
			if scanErr != nil {
				m.logger.Warn("watchman poll fallback snapshot failed",
					zap.String("operation", "watchman.poll"),
					zap.String("repository_name", repository.Name),
					zap.Error(scanErr),
				)
				continue
			}
			if snapshot == nil {
				snapshot = current
				continue
			}
			changes := diffRepositorySnapshots(snapshot, current)
			snapshot = current
			if len(changes) == 0 {
				continue
			}
			m.logger.Info("watchman poll fallback detected changes",
				zap.String("operation", "watchman.poll"),
				zap.String("repository_name", repository.Name),
				zap.Int("change_count", len(changes)),
			)
			m.handleFileEvents(ctx, repository, pending, settle, changes)
		case msg := <-msgCh:
			subName, _ := msg["subscription"].(string)
			if subName == "" || subName != subscriptionName {
				continue
			}

			result, err := watchman.ParseQueryResult(msg)
			if err != nil {
				m.logger.Warn("watchman parse payload failed",
					zap.String("operation", "watchman.payload"),
					zap.String("repository_name", repository.Name),
					zap.Error(err),
				)
				continue
			}
			if result.Clock != "" {
				_ = m.saveClock(repository.Path, result.Clock)
			}
			if snapshot != nil {
				applyFileEventsToSnapshot(repository.Path, snapshot, result.Files)
			}

			m.handleFileEvents(ctx, repository, pending, settle, result.Files)
		}
	}
}

func (m *WatchmanMonitor) handleFileEvents(
	ctx context.Context,
	repository repo.Repository,
	pending map[string]*pendingEntry,
	settle time.Duration,
	files []watchman.FileEvent,
) {
	if len(files) == 0 {
		return
	}

	now := time.Now()
	repoID := uuid.UUID(repository.RepoID.Bytes).String()
	for _, f := range files {
		cleaned, ok := shouldQueueDiscoveredPath(f.Name)
		if !ok {
			continue
		}
		if !f.Exists {
			delete(pending, cleaned)
			if err := m.enqueueDiscover(ctx, repoID, cleaned, filepath.Base(cleaned), nil, jobs.DiscoverOperationDelete); err != nil {
				m.logger.Warn("watchman delete enqueue failed",
					zap.String("operation", "watchman.enqueue_delete"),
					zap.String("repository_name", repository.Name),
					zap.String("storage_path", cleaned),
					zap.Error(err),
				)
			}
			continue
		}
		if f.Type != "" && f.Type != "f" {
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
		if f.Size >= 0 {
			entry.LastSize = f.Size
		}
		if f.MTimeMs > 0 {
			entry.LastMTimeMs = f.MTimeMs
		}
		entry.ReadyAt = now.Add(settle)
	}
}

func (m *WatchmanMonitor) enqueueReadyFiles(ctx context.Context, repository repo.Repository, files []watchman.FileEvent) error {
	repoID := uuid.UUID(repository.RepoID.Bytes).String()
	for _, f := range files {
		cleaned, ok := shouldQueueDiscoveredPath(f.Name)
		if !ok {
			continue
		}
		if !f.Exists {
			if err := m.enqueueDiscover(ctx, repoID, cleaned, filepath.Base(cleaned), nil, jobs.DiscoverOperationDelete); err != nil {
				m.logger.Warn("watchman initial delete enqueue failed",
					zap.String("operation", "watchman.initial_delete_enqueue"),
					zap.String("repository_name", repository.Name),
					zap.String("storage_path", cleaned),
					zap.Error(err),
				)
			}
			continue
		}
		if f.Type != "" && f.Type != "f" {
			continue
		}
		storagePath := cleaned
		fullPath := filepath.Join(repository.Path, filepath.FromSlash(storagePath))
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			continue
		}

		if err := m.enqueueDiscover(ctx, repoID, storagePath, filepath.Base(cleaned), info, jobs.DiscoverOperationUpsert); err != nil {
			m.logger.Warn("watchman initial enqueue failed",
				zap.String("operation", "watchman.initial_enqueue"),
				zap.String("repository_name", repository.Name),
				zap.String("storage_path", storagePath),
				zap.Error(err),
			)
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
		if err != nil {
			if os.IsNotExist(err) {
				if enqueueErr := m.enqueueDiscover(ctx, repoID, entry.StoragePath, entry.Filename, nil, jobs.DiscoverOperationDelete); enqueueErr != nil {
					m.logger.Warn("watchman delete enqueue failed",
						zap.String("operation", "watchman.enqueue_delete"),
						zap.String("repository_name", repository.Name),
						zap.String("storage_path", entry.StoragePath),
						zap.Error(enqueueErr),
					)
				}
			}
			delete(pending, key)
			continue
		}
		if info.IsDir() {
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

		if err := m.enqueueDiscover(ctx, repoID, entry.StoragePath, entry.Filename, info, jobs.DiscoverOperationUpsert); err != nil {
			entry.Attempts++
			if entry.Attempts >= 3 {
				m.logger.Warn("watchman enqueue failed after retries",
					zap.String("operation", "watchman.enqueue"),
					zap.String("repository_name", repository.Name),
					zap.String("storage_path", entry.StoragePath),
					zap.Error(err),
				)
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
	operation string,
) error {
	args := jobs.DiscoverAssetArgs{
		RepositoryID: repoID,
		RelativePath: filepath.ToSlash(storagePath),
		Operation:    operation,
		FileName:     filename,
		DetectedAt:   time.Now().UTC(),
	}
	if args.Operation == "" {
		args.Operation = jobs.DiscoverOperationUpsert
	}
	if args.Operation == jobs.DiscoverOperationUpsert && info != nil {
		args.ContentType = file.NewValidator().GetMimeTypeFromExtension(filepath.Ext(filename))
		args.FileSize = info.Size()
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

func (m *WatchmanMonitor) clearClock(repoPath string) error {
	path := m.clockFilePath(repoPath)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
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

func shouldQueueDiscoveredPath(path string) (string, bool) {
	cleaned, ok := cleanRelativePath(path)
	if !ok {
		return "", false
	}
	if !file.IsSupportedExtension(filepath.Ext(cleaned)) {
		return "", false
	}
	if isExcludedWorkspacePath(cleaned) {
		return "", false
	}
	return cleaned, true
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

func snapshotRepositoryFiles(repoPath string) (map[string]fileSnapshot, error) {
	snapshot := make(map[string]fileSnapshot)
	walkErr := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == repoPath {
			return nil
		}

		rel, err := filepath.Rel(repoPath, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		if d.IsDir() {
			if isExcludedWorkspacePath(rel) {
				return filepath.SkipDir
			}
			return nil
		}

		cleaned, ok := shouldQueueDiscoveredPath(rel)
		if !ok {
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil || info.IsDir() {
			return nil
		}

		snapshot[cleaned] = fileSnapshot{
			Size:    info.Size(),
			MTimeMs: info.ModTime().UnixMilli(),
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	return snapshot, nil
}

func diffRepositorySnapshots(previous, current map[string]fileSnapshot) []watchman.FileEvent {
	events := make([]watchman.FileEvent, 0)
	if previous == nil {
		return events
	}

	for path, cur := range current {
		prev, ok := previous[path]
		if !ok {
			events = append(events, watchman.FileEvent{
				Name:    path,
				Exists:  true,
				New:     true,
				Type:    "f",
				Size:    cur.Size,
				MTimeMs: cur.MTimeMs,
			})
			continue
		}

		if prev.Size != cur.Size || prev.MTimeMs != cur.MTimeMs {
			events = append(events, watchman.FileEvent{
				Name:    path,
				Exists:  true,
				Type:    "f",
				Size:    cur.Size,
				MTimeMs: cur.MTimeMs,
			})
		}
	}

	for path := range previous {
		if _, ok := current[path]; ok {
			continue
		}
		events = append(events, watchman.FileEvent{
			Name:   path,
			Exists: false,
			Type:   "f",
		})
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Name < events[j].Name
	})
	return events
}

func applyFileEventsToSnapshot(repoPath string, snapshot map[string]fileSnapshot, files []watchman.FileEvent) {
	if snapshot == nil {
		return
	}

	for _, f := range files {
		cleaned, ok := shouldQueueDiscoveredPath(f.Name)
		if !ok {
			continue
		}

		if !f.Exists {
			delete(snapshot, cleaned)
			continue
		}
		if f.Type != "" && f.Type != "f" {
			continue
		}

		size := f.Size
		mtimeMs := f.MTimeMs
		if mtimeMs <= 0 {
			fullPath := filepath.Join(repoPath, filepath.FromSlash(cleaned))
			info, err := os.Stat(fullPath)
			if err != nil || info.IsDir() {
				continue
			}
			size = info.Size()
			mtimeMs = info.ModTime().UnixMilli()
		}

		snapshot[cleaned] = fileSnapshot{
			Size:    size,
			MTimeMs: mtimeMs,
		}
	}
}

func isWatchableRepositoryRoot(repoPath string) bool {
	cleaned := strings.TrimSpace(repoPath)
	if cleaned == "" {
		return false
	}

	info, err := os.Stat(cleaned)
	if err != nil || !info.IsDir() {
		return false
	}

	// Ensure we only watch actual repository roots. Watching a parent folder can
	// produce incorrect relative paths (e.g. prefixed with "primary/").
	return repocfg.IsRepositoryRoot(cleaned)
}
