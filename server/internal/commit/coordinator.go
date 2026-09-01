// Package commit is the sole catalog-write capability for asynchronous work.
package commit

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"server/internal/db/catalogtx"
)

type Outcome uint8

const (
	OutcomeApplied Outcome = iota + 1
	OutcomeStale
	OutcomeDuplicate
)

var (
	ErrNotRunning = errors.New("commit coordinator is not running")
	ErrStopped    = errors.New("commit coordinator stopped")
)

// Acknowledgement is a family-specific value returned only after the catalog
// transaction commits. Implementations must contain immutable result data.
type Acknowledgement interface {
	CommitAcknowledgement()
}

type Result struct {
	Outcome             Outcome
	Acknowledgement     Acknowledgement
	TransactionDuration time.Duration
	CommitDuration      time.Duration
}

// OperationKind is a transport-only batching class. Product identity belongs
// to the typed operation input, not to this queue boundary.
type OperationKind uint8

const (
	OperationKindCatalogAssetStage OperationKind = iota + 1
	OperationKindCatalogAssetMetadata
	OperationKindCatalogAssetDerivatives
	OperationKindCatalogAssetStack
	OperationKindCatalogRepositoryAsset
	OperationKindCatalogRepositoryKnownContent
	OperationKindCatalogRepositoryHash
	OperationKindCatalogVideoFrameEmbeddings
	OperationKindCatalogEnrichment
	OperationKindCatalogIngestReceipt
	OperationKindCatalogOperationReceipt
	OperationKindCatalogProjection
	OperationKindCatalogRepositoryEpoch
	OperationKindRepositoryObservation
	OperationKindRepositoryStaging
)

// Operation is the coordinator's private transport representation. Callers
// normally use one of Coordinator's typed Apply methods. The operation escape
// hatch is only for package-owned commit boundaries (ROE and staging) whose
// typed payloads live outside package commit; it carries no product identity
// or generic payload.
type Operation struct {
	Kind       OperationKind
	BatchLimit int
	Apply      func(context.Context, *sql.Tx) (Result, error)
}

type Config struct {
	Capacity   int
	MaxBatch   int
	OldestWait time.Duration
}

// Snapshot is a stable, bounded view of coordinator load and commit behavior
// for private runtime diagnostics. Latency and size distributions are fixed
// HDR histograms; no raw operation or timing sample is retained.
type Snapshot struct {
	Capacity          int `json:"capacity"`
	Depth             int `json:"depth"`
	PeakDepth         int `json:"peak_depth"`
	BlockedSubmitters int `json:"blocked_submitters"`
	PeakBlocked       int `json:"peak_blocked_submitters"`

	EnqueueCanceled  uint64               `json:"enqueue_canceled"`
	Batches          uint64               `json:"batches"`
	Operations       uint64               `json:"operations"`
	UniqueOperations uint64               `json:"unique_operations"`
	Applied          uint64               `json:"applied"`
	Stale            uint64               `json:"stale"`
	Duplicate        uint64               `json:"duplicate"`
	Failures         uint64               `json:"failures"`
	Acknowledgements uint64               `json:"acknowledgements"`
	EnqueueWait      DurationDistribution `json:"enqueue_wait"`
	OldestWait       DurationDistribution `json:"oldest_wait"`
	BatchSize        SizeDistribution     `json:"batch_size"`
	Transaction      DurationDistribution `json:"transaction"`
	Commit           DurationDistribution `json:"commit"`
}

type metricsState struct {
	peakDepth, blockedSubmitters, peakBlocked             int
	enqueueCanceled                                       uint64
	batches, operations, uniqueOperations                 uint64
	applied, stale, duplicate, failures, acknowledgements uint64
	enqueueWait, oldestWait, transaction, commit          durationHistogram
	batchSize                                             sizeHistogram
}

type submission struct {
	operation Operation
	enqueued  time.Time
	ack       chan submissionResult
}

type submissionResult struct {
	Result Result
	Err    error
}

type Coordinator struct {
	writer       *catalogtx.Writer
	config       Config
	catalog      CatalogDependencies
	input        chan submission
	stop         chan struct{}
	done         chan struct{}
	started      atomic.Bool
	stopOnce     sync.Once
	stateMu      sync.Mutex
	stopped      bool
	submitting   int
	allSubmitted chan struct{}
	metricsMu    sync.Mutex
	metrics      metricsState
}

func New(writer *catalogtx.Writer, config Config, catalog CatalogDependencies) (*Coordinator, error) {
	if writer == nil || config.Capacity < 1 || config.MaxBatch < 1 || config.OldestWait <= 0 {
		return nil, errors.New("commit coordinator requires writer and positive bounds")
	}
	allSubmitted := make(chan struct{})
	close(allSubmitted)
	return &Coordinator{
		writer: writer, config: config, catalog: catalog,
		input: make(chan submission, config.Capacity), stop: make(chan struct{}), done: make(chan struct{}), allSubmitted: allSubmitted,
		metrics: metricsState{
			enqueueWait: newDurationHistogram(), oldestWait: newDurationHistogram(),
			transaction: newDurationHistogram(), commit: newDurationHistogram(),
			batchSize: newSizeHistogram(config.MaxBatch),
		},
	}, nil
}

func (c *Coordinator) Start() {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.stopped {
		return
	}
	if c.started.CompareAndSwap(false, true) {
		go c.run()
	}
}

func (c *Coordinator) SubmitOperation(ctx context.Context, operation Operation) (Result, error) {
	if ctx == nil {
		return Result{}, errors.New("commit submission context is nil")
	}
	if operation.Kind == 0 || operation.Apply == nil {
		return Result{}, errors.New("invalid commit operation")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if !c.started.Load() {
		return Result{}, ErrNotRunning
	}
	s := submission{operation: operation, enqueued: time.Now(), ack: make(chan submissionResult, 1)}
	started := time.Now()
	if err := c.beginSubmission(); err != nil {
		return Result{}, err
	}
	select {
	case c.input <- s:
		c.observeEnqueue(time.Since(started))
	default:
		c.beginBlockedSubmitter()
		select {
		case c.input <- s:
			c.observeEnqueue(time.Since(started))
		case <-c.stop:
			c.endBlockedSubmitter()
			c.endSubmission()
			return Result{}, ErrStopped
		case <-ctx.Done():
			c.endBlockedSubmitter()
			c.observeEnqueueCanceled()
			c.endSubmission()
			return Result{}, ctx.Err()
		}
		c.endBlockedSubmitter()
	}
	c.endSubmission()
	select {
	case result := <-s.ack:
		return result.Result, result.Err
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
}

func (c *Coordinator) Stop(ctx context.Context) error {
	c.stopOnce.Do(func() {
		c.stateMu.Lock()
		c.stopped = true
		close(c.stop)
		c.stateMu.Unlock()
	})
	if !c.started.Load() {
		return nil
	}
	c.stateMu.Lock()
	allSubmitted := c.allSubmitted
	c.stateMu.Unlock()
	select {
	case <-allSubmitted:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-c.done:
		// A sender can win a select against the already-closed stop channel when
		// the input buffer has room. The run loop may have drained before that
		// send became visible, so perform one final acknowledgement drain after
		// the sender barrier and run-loop completion.
		c.rejectBuffered()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Coordinator) beginSubmission() error {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.stopped {
		return ErrStopped
	}
	if c.submitting == 0 {
		c.allSubmitted = make(chan struct{})
	}
	c.submitting++
	return nil
}

func (c *Coordinator) endSubmission() {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.submitting <= 0 {
		return
	}
	c.submitting--
	if c.submitting == 0 {
		close(c.allSubmitted)
	}
}

func (c *Coordinator) run() {
	defer close(c.done)
	var carry *submission
	for {
		var pending []submission
		if carry != nil {
			pending = append(pending, *carry)
			carry = nil
		} else {
			select {
			case item := <-c.input:
				pending = append(pending, item)
			case <-c.stop:
				c.rejectBuffered()
				return
			}
		}
		kind := pending[0].operation.Kind
		batchLimit := c.config.MaxBatch
		if operationLimit := pending[0].operation.BatchLimit; operationLimit > 0 && operationLimit < batchLimit {
			batchLimit = operationLimit
		}
		deadline := time.NewTimer(c.config.OldestWait)
	collect:
		for len(pending) < batchLimit {
			select {
			case item := <-c.input:
				if item.operation.Kind == kind && (item.operation.BatchLimit <= 0 || len(pending) < item.operation.BatchLimit) {
					pending = append(pending, item)
				} else {
					carry = &item
					break collect
				}
			case <-deadline.C:
				break collect
			case <-c.stop:
				deadline.Stop()
				c.process(pending)
				c.rejectBuffered()
				return
			}
		}
		if !deadline.Stop() {
			select {
			case <-deadline.C:
			default:
			}
		}
		c.process(pending)
		pending = nil
	}
}

func (c *Coordinator) process(items []submission) {
	oldestWait := time.Since(items[0].enqueued)
	results := c.apply(items)
	for index, result := range results {
		items[index].ack <- result
		c.incrementAck(result)
	}
	c.metricsMu.Lock()
	c.metrics.operations += uint64(len(items))
	c.metrics.uniqueOperations += uint64(len(items))
	c.metrics.oldestWait.record(oldestWait)
	c.metricsMu.Unlock()
}

func (c *Coordinator) apply(items []submission) []submissionResult {
	results := make([]Result, len(items))
	started := time.Now()
	err := c.writer.Transact(context.Background(), catalogtx.OperationBackgroundCommitBatch, nil, func(tx *sql.Tx) error {
		for index, item := range items {
			itemStarted := time.Now()
			result, applyErr := item.operation.Apply(context.Background(), tx)
			result.TransactionDuration = time.Since(itemStarted)
			results[index] = result
			if applyErr != nil {
				return applyErr
			}
		}
		return nil
	})
	totalDuration := time.Since(started)
	var statementDuration time.Duration
	for _, result := range results {
		statementDuration += result.TransactionDuration
	}
	commitDuration := totalDuration - statementDuration
	if commitDuration < 0 {
		commitDuration = 0
	}
	for index := range results {
		results[index].CommitDuration = commitDuration
	}
	c.metricsMu.Lock()
	c.metrics.batches++
	c.metrics.batchSize.record(len(items))
	c.metrics.transaction.record(totalDuration)
	c.metrics.commit.record(commitDuration)
	c.metricsMu.Unlock()
	if err != nil && len(items) > 1 {
		mid := len(items) / 2
		left := c.apply(items[:mid])
		right := c.apply(items[mid:])
		return append(left, right...)
	}
	acknowledgements := make([]submissionResult, len(items))
	for i := range acknowledgements {
		if err != nil {
			acknowledgements[i].Err = err
		} else {
			acknowledgements[i].Result = results[i]
		}
	}
	return acknowledgements
}

func (c *Coordinator) rejectBuffered() {
	for {
		select {
		case item := <-c.input:
			item.ack <- submissionResult{Err: ErrStopped}
		default:
			return
		}
	}
}
func (c *Coordinator) observeEnqueue(wait time.Duration) {
	c.metricsMu.Lock()
	c.metrics.enqueueWait.record(wait)
	if depth := len(c.input); depth > c.metrics.peakDepth {
		c.metrics.peakDepth = depth
	}
	c.metricsMu.Unlock()
}
func (c *Coordinator) observeEnqueueCanceled() {
	c.metricsMu.Lock()
	c.metrics.enqueueCanceled++
	c.metricsMu.Unlock()
}
func (c *Coordinator) beginBlockedSubmitter() {
	c.metricsMu.Lock()
	c.metrics.blockedSubmitters++
	if c.metrics.blockedSubmitters > c.metrics.peakBlocked {
		c.metrics.peakBlocked = c.metrics.blockedSubmitters
	}
	c.metricsMu.Unlock()
}
func (c *Coordinator) endBlockedSubmitter() {
	c.metricsMu.Lock()
	if c.metrics.blockedSubmitters > 0 {
		c.metrics.blockedSubmitters--
	}
	c.metricsMu.Unlock()
}
func (c *Coordinator) incrementAck(result submissionResult) {
	c.metricsMu.Lock()
	defer c.metricsMu.Unlock()
	if result.Err != nil {
		c.metrics.failures++
		return
	}
	c.metrics.acknowledgements++
	switch result.Result.Outcome {
	case OutcomeApplied:
		c.metrics.applied++
	case OutcomeStale:
		c.metrics.stale++
	case OutcomeDuplicate:
		c.metrics.duplicate++
	}
}
func (c *Coordinator) Snapshot() Snapshot {
	c.metricsMu.Lock()
	defer c.metricsMu.Unlock()
	return Snapshot{
		Capacity: c.config.Capacity, Depth: len(c.input), PeakDepth: c.metrics.peakDepth,
		BlockedSubmitters: c.metrics.blockedSubmitters, PeakBlocked: c.metrics.peakBlocked,
		EnqueueCanceled: c.metrics.enqueueCanceled,
		Batches:         c.metrics.batches, Operations: c.metrics.operations, UniqueOperations: c.metrics.uniqueOperations,
		Applied: c.metrics.applied, Stale: c.metrics.stale, Duplicate: c.metrics.duplicate,
		Failures: c.metrics.failures, Acknowledgements: c.metrics.acknowledgements,
		EnqueueWait: c.metrics.enqueueWait.snapshot(), OldestWait: c.metrics.oldestWait.snapshot(),
		BatchSize: c.metrics.batchSize.snapshot(), Transaction: c.metrics.transaction.snapshot(),
		Commit: c.metrics.commit.snapshot(),
	}
}
