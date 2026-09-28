// Package scan is the repository scan index (#222). The catalog mirrors each
// repository's tree in repository_entries, one row per file or directory;
// the disk is the truth and a full scan is the authority.
//
// A scan is a depth-first walk in sorted name order. Each directory is one
// unit: list it completely, diff the listing against the catalog rows whose
// parent_key is that directory, write only what changed, and mark a
// catalog-only row missing only after a positive absence probe with the
// repository marker re-checked. Hashing is a separate bounded pass over
// pending_hash rows that commits by compare-and-swap on the row's revision.
//
// Nothing here performs filesystem I/O, hashing, or sleeping inside a catalog
// transaction; every write batch is at most MaxBatchRows rows and is sized
// to stay well inside the writer-hold budget.
package scan

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/roe/pathsemantics"
)

// MaxBatchRows bounds every writer transaction. The walk also bounds each
// batch in time: batchSizer keeps it near batchTarget, so a slow disk gets
// smaller batches instead of longer writer holds (the budget is 25 ms).
const (
	MaxBatchRows = 256
	minBatchRows = 16
	batchTarget  = 10 * time.Millisecond
)

// batchSizer adapts the walk's write batch to the measured cost per row:
// it halves after a transaction slower than the target and doubles after
// one well under it.
type batchSizer struct {
	mu   sync.Mutex
	rows int
}

func (b *batchSizer) size() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.rows == 0 {
		b.rows = 64
	}
	return b.rows
}

func (b *batchSizer) observe(rows int, took time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.rows == 0 {
		b.rows = 64
	}
	switch {
	case took > batchTarget:
		b.rows = max(minBatchRows, b.rows/2)
	case took < batchTarget/2 && rows >= b.rows:
		b.rows = min(MaxBatchRows, b.rows*2)
	}
}

// RacyGranularity is the mtime resolution assumed when deciding whether an
// unchanged stat tuple can be trusted (Git's "racily clean" rule). Two
// seconds covers FAT and exFAT.
const RacyGranularity = 2 * time.Second

// Triggers and statuses stored in repository_scans.
const (
	TriggerManual   = "manual"
	TriggerPeriodic = "periodic"
	TriggerWatcher  = "watcher"
	TriggerStartup  = "startup"
	TriggerSettle   = "settle"

	StatusQueued    = "queued"
	StatusWalking   = "walking"
	StatusSweeping  = "sweeping"
	StatusCompleted = "completed"
	StatusOffline   = "offline"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// Entry states stored in repository_entries.
const (
	StatePendingHash = "pending_hash"
	StatePresent     = "present"
	StateMissing     = "missing"
	StateUnsupported = "unsupported"
	StateTrashed     = "trashed"

	KindFile      = "file"
	KindDirectory = "directory"
)

// Writer runs one named catalog write transaction. *db.DB satisfies it; the
// commit coordinator will in Phase 3.
type Writer interface {
	WithTx(ctx context.Context, operation catalogtx.Operation, fn func(*sql.Tx, *repo.Queries) error) error
}

// ActivateFunc runs inside the hash commit transaction after an entry is
// bound to an Asset, so downstream processing starts atomically with the
// binding.
type ActivateFunc func(ctx context.Context, tx *sql.Tx, queries *repo.Queries, repositoryID, entryID, assetID, contentID uuid.UUID) error

type Config struct {
	// Settle defers files modified more recently than this and schedules a
	// follow-up scan for them (repository_scan.settle_seconds).
	Settle time.Duration
	// Semantics are the volume's path comparison rules.
	Semantics pathsemantics.Semantics
	// TurnFiles and TurnDuration bound one walk turn; a turn always finishes
	// the directory it started.
	TurnFiles    int
	TurnDuration time.Duration
	Activate     ActivateFunc
	Now          func() time.Time
}

type Scanner struct {
	reader *repo.Queries
	writer Writer
	files  *storage.RepositoryFSFactory
	config Config
	batch  batchSizer
}

func New(reader *repo.Queries, writer Writer, files *storage.RepositoryFSFactory, config Config) (*Scanner, error) {
	if reader == nil || writer == nil || files == nil {
		return nil, errors.New("scan requires a reader, a writer, and a repository filesystem factory")
	}
	if err := config.Semantics.Validate(); err != nil {
		return nil, err
	}
	if config.Settle < 0 {
		return nil, errors.New("settle window must not be negative")
	}
	if config.TurnFiles <= 0 {
		config.TurnFiles = 4096
	}
	if config.TurnDuration <= 0 {
		config.TurnDuration = 250 * time.Millisecond
	}
	if config.Now == nil {
		config.Now = func() time.Time { return time.Now().UTC() }
	}
	return &Scanner{reader: reader, writer: writer, files: files, config: config}, nil
}

// pathKey maps a relative path to its comparison key, component by component.
func (s *Scanner) pathKey(relative string) (string, error) {
	if relative == "" {
		return "", nil
	}
	components := strings.Split(relative, "/")
	for index, component := range components {
		key, err := s.config.Semantics.NameKey(component)
		if err != nil {
			return "", err
		}
		components[index] = key
	}
	return strings.Join(components, "/"), nil
}

func parentOf(relative string) string {
	if index := strings.LastIndexByte(relative, '/'); index >= 0 {
		return relative[:index]
	}
	return ""
}

// commonAncestor is the deepest directory containing both scopes; the empty scope is the
// whole repository.
func commonAncestor(left, right string) string {
	if left == "" || right == "" {
		return ""
	}
	a, b := strings.Split(left, "/"), strings.Split(right, "/")
	shared := make([]string, 0, min(len(a), len(b)))
	for index := 0; index < len(a) && index < len(b) && a[index] == b[index]; index++ {
		shared = append(shared, a[index])
	}
	return strings.Join(shared, "/")
}

// Request queues a scan of scope (empty for the whole repository). At most one
// scan per repository is queued: a new request joins it, widening its scope
// and keeping the earlier start time. notBefore delays a settle follow-up.
func (s *Scanner) Request(ctx context.Context, repositoryID uuid.UUID, trigger, scope, requestedBy string, notBefore time.Time) (repo.RepositoryScan, bool, error) {
	var scan repo.RepositoryScan
	var coalesced bool
	err := s.writer.WithTx(ctx, catalogtx.OperationRepositoryScanRequest, func(_ *sql.Tx, queries *repo.Queries) error {
		var err error
		scan, coalesced, err = s.requestTx(ctx, queries, repositoryID, trigger, scope, requestedBy, notBefore)
		return err
	})
	return scan, coalesced, err
}

func (s *Scanner) requestTx(ctx context.Context, queries *repo.Queries, repositoryID uuid.UUID, trigger, scope, requestedBy string, notBefore time.Time) (repo.RepositoryScan, bool, error) {
	switch trigger {
	case TriggerManual, TriggerPeriodic, TriggerWatcher, TriggerStartup, TriggerSettle:
	default:
		return repo.RepositoryScan{}, false, fmt.Errorf("unknown scan trigger %q", trigger)
	}
	scope = strings.Trim(scope, "/")
	if scope != "" {
		if _, err := storage.ParseUserMediaPath(scope); err != nil {
			return repo.RepositoryScan{}, false, err
		}
	}
	now := s.config.Now()
	start := dbtypes.Timestamp{}
	if !notBefore.IsZero() && notBefore.After(now) {
		start = dbtypes.NewTimestamp(notBefore)
	}
	queued, err := queries.GetQueuedRepositoryScan(ctx, repositoryID)
	switch {
	case err == nil:
		// NULL means "now", so it beats any delayed start.
		if queued.NotBefore.Valid && (!start.Valid || start.Time.Before(queued.NotBefore.Time)) {
			queued.NotBefore = start
		}
		joined, joinErr := queries.CoalesceQueuedRepositoryScan(ctx, repo.CoalesceQueuedRepositoryScanParams{
			ScopePath: commonAncestor(queued.ScopePath, scope), NotBefore: queued.NotBefore,
			UpdatedAt: dbtypes.NewTimestamp(now), ScanID: queued.ScanID,
		})
		return joined, true, joinErr
	case !errors.Is(err, sql.ErrNoRows):
		return repo.RepositoryScan{}, false, err
	}
	var by *string
	if requestedBy != "" {
		by = &requestedBy
	}
	created, err := queries.InsertRepositoryScan(ctx, repo.InsertRepositoryScanParams{
		ScanID: uuid.New(), RepositoryID: repositoryID, Trigger: trigger, ScopePath: scope,
		NotBefore: start, RequestedBy: by, CreatedAt: dbtypes.NewTimestamp(now),
	})
	return created, false, err
}
