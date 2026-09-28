package scan

import (
	"context"
	"errors"
	"math/rand"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/syncthing/notify"
	"go.uber.org/zap"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
)

// Watcher turns filesystem events into scan requests. It is only a hint:
// losing every event delays the catalog but never makes it wrong, because the
// periodic full scan is the authority. Events are aggregated the way
// Syncthing's watch aggregator does: a batch opens with its first event and
// closes WatchDelay later, then requests one scan of the smallest subtree
// covering its directories, or of the whole repository when the batch grew
// past WatchFullScanEvents.
type Watcher struct {
	reader  watchReader
	request func(ctx context.Context, repositoryID uuid.UUID, trigger, scope string) error
	logger  *zap.Logger
	config  WatchConfig

	mu      sync.Mutex
	watches map[uuid.UUID]*repositoryWatch
}

type watchReader interface {
	ListRepositories(context.Context) ([]repo.Repository, error)
}

type WatchConfig struct {
	// Delay is how long a batch collects events before it requests a scan.
	Delay time.Duration
	// FullScanEvents turns a batch into a full-repository scan.
	FullScanEvents int
	// Reconcile is how often the set of watched repositories is refreshed.
	Reconcile time.Duration
	// RetryMin and RetryMax bound the backoff after a watch fails to start.
	RetryMin, RetryMax time.Duration
	// watch starts a recursive watch; tests replace it.
	watch func(path string, events chan<- notify.EventInfo) error
	stop  func(events chan<- notify.EventInfo)
}

func (c WatchConfig) withDefaults() WatchConfig {
	if c.Delay <= 0 {
		c.Delay = 10 * time.Second
	}
	if c.FullScanEvents <= 0 {
		c.FullScanEvents = 512
	}
	if c.Reconcile <= 0 {
		c.Reconcile = time.Minute
	}
	if c.RetryMin <= 0 {
		c.RetryMin = 5 * time.Second
	}
	if c.RetryMax <= 0 {
		c.RetryMax = 10 * time.Minute
	}
	if c.watch == nil {
		c.watch = func(path string, events chan<- notify.EventInfo) error {
			return notify.Watch(filepath.Join(path, "..."), events, notify.All)
		}
	}
	if c.stop == nil {
		c.stop = notify.Stop
	}
	return c
}

type repositoryWatch struct {
	path   string
	cancel context.CancelFunc
	done   chan struct{}
}

func NewWatcher(reader watchReader, scanner *Scanner, logger *zap.Logger, config WatchConfig) *Watcher {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Watcher{
		reader: reader, logger: logger, config: config.withDefaults(),
		watches: make(map[uuid.UUID]*repositoryWatch),
		request: func(ctx context.Context, repositoryID uuid.UUID, trigger, scope string) error {
			_, _, err := scanner.Request(ctx, repositoryID, trigger, scope, "", time.Time{})
			return err
		},
	}
}

// Run watches every active repository until ctx ends, reconciling the set of
// watches as repositories come, go, or change reachability.
func (w *Watcher) Run(ctx context.Context) {
	ticker := time.NewTicker(w.config.Reconcile)
	defer ticker.Stop()
	defer w.stopAll()
	for {
		w.reconcile(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Watcher) reconcile(ctx context.Context) {
	repositories, err := w.reader.ListRepositories(ctx)
	if err != nil {
		if ctx.Err() == nil {
			w.logger.Warn("list repositories to watch", zap.Error(err))
		}
		return
	}
	wanted := make(map[uuid.UUID]string, len(repositories))
	for _, repository := range repositories {
		if repository.Reachability == dbtypes.RepositoryReachabilityActive {
			wanted[repository.RepoID] = repository.Path
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for id, watch := range w.watches {
		if path, ok := wanted[id]; !ok || path != watch.path {
			watch.cancel()
			<-watch.done
			delete(w.watches, id)
		}
	}
	for id, path := range wanted {
		if _, ok := w.watches[id]; ok {
			continue
		}
		watchCtx, cancel := context.WithCancel(ctx)
		watch := &repositoryWatch{path: path, cancel: cancel, done: make(chan struct{})}
		w.watches[id] = watch
		go func() {
			defer close(watch.done)
			w.watchRepository(watchCtx, id, path)
		}()
	}
}

func (w *Watcher) stopAll() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for id, watch := range w.watches {
		watch.cancel()
		<-watch.done
		delete(w.watches, id)
	}
}

// watchRepository keeps one recursive watch alive. A failure to start is a
// lost-events signal: it schedules a full scan and retries with backoff.
func (w *Watcher) watchRepository(ctx context.Context, repositoryID uuid.UUID, root string) {
	retry := w.config.RetryMin
	for ctx.Err() == nil {
		events := make(chan notify.EventInfo, 1024)
		if err := w.config.watch(root, events); err != nil {
			w.logger.Warn("repository watch failed; scheduling a full scan",
				zap.String("repository_id", repositoryID.String()), zap.Error(err))
			w.requestScan(ctx, repositoryID, "")
			select {
			case <-ctx.Done():
				return
			case <-time.After(retry):
			}
			retry = min(retry*2, w.config.RetryMax)
			continue
		}
		w.aggregate(ctx, repositoryID, root, events)
		w.config.stop(events)
		return
	}
}

// aggregate batches events until ctx ends. A full event buffer means events
// were dropped, which is handled like a watch error: a full scan.
func (w *Watcher) aggregate(ctx context.Context, repositoryID uuid.UUID, root string, events <-chan notify.EventInfo) {
	var (
		timer       *time.Timer
		fire        <-chan time.Time
		directories = map[string]struct{}{}
		count       int
		full        bool
	)
	flush := func() {
		scope := ""
		if !full {
			first := true
			for directory := range directories {
				if first {
					scope, first = directory, false
					continue
				}
				scope = commonAncestor(scope, directory)
			}
		}
		w.requestScan(ctx, repositoryID, scope)
		directories, count, full, fire = map[string]struct{}{}, 0, false, nil
	}
	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case event := <-events:
			relative, ok := watchedRelative(root, event.Path())
			if !ok {
				continue
			}
			count++
			if len(events) == cap(events) || count > w.config.FullScanEvents {
				full = true
			} else {
				directories[parentOf(relative)] = struct{}{}
			}
			if fire == nil {
				timer = time.NewTimer(w.config.Delay)
				fire = timer.C
			}
		case <-fire:
			flush()
		}
	}
}

func (w *Watcher) requestScan(ctx context.Context, repositoryID uuid.UUID, scope string) {
	if err := w.request(ctx, repositoryID, TriggerWatcher, scope); err != nil && !errors.Is(err, context.Canceled) {
		w.logger.Warn("request watcher scan", zap.String("repository_id", repositoryID.String()), zap.Error(err))
	}
}

// watchedRelative maps an event path to a repository-relative path, dropping
// Lumilio's own private tree and marker files so its writes cannot feed back.
func watchedRelative(root, eventPath string) (string, bool) {
	relative, err := filepath.Rel(root, eventPath)
	if err != nil || relative == "." || strings.HasPrefix(relative, "..") {
		return "", false
	}
	relative = filepath.ToSlash(relative)
	if relative == ".lumilio" || strings.HasPrefix(relative, ".lumilio/") ||
		relative == ".lumiliorepo" || relative == ".lumilioroot" {
		return "", false
	}
	return relative, true
}

// PeriodicDelay returns the next periodic full-scan delay: the interval with
// Syncthing's 3/4 to 5/4 jitter, so repositories do not scan in lockstep.
func PeriodicDelay(interval time.Duration, random *rand.Rand) time.Duration {
	if interval <= 0 {
		return interval
	}
	return interval*3/4 + time.Duration(random.Int63n(int64(interval)/2+1))
}
