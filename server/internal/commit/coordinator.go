// Package commit is the sole catalog-write capability for asynchronous work.
package commit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

type Key struct {
	Family, Subject, Fence, Stage string
	DesiredVersion                uint64
}

func (k Key) valid() bool {
	return k.Family != "" && k.Subject != "" && k.Fence != "" && k.Stage != "" && k.DesiredVersion > 0
}
func (k Key) String() string {
	return fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d", k.Family, k.Subject, k.Fence, k.Stage, k.DesiredVersion)
}

// Intent contains immutable data only. Producers cannot smuggle transactions,
// SQL callbacks, or mutable catalog services through this boundary.
type Intent struct {
	Key     Key
	Payload any
}

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

type Handler interface {
	Apply(context.Context, *sql.Tx, []Intent) ([]Result, error)
	BatchLimit() int
}

// AcknowledgingHandler preserves one producer turn per catalog transaction so
// its family-specific result and timing describe exactly the acknowledged
// unit. Outcome-only families remain eligible for configured micro-batching.
type AcknowledgingHandler func(context.Context, *sql.Tx, []Intent) ([]Result, error)

func (handler AcknowledgingHandler) Apply(ctx context.Context, tx *sql.Tx, intents []Intent) ([]Result, error) {
	return handler(ctx, tx, intents)
}

func (AcknowledgingHandler) BatchLimit() int { return 1 }

// OutcomeHandler is the canonical implementation for families whose producer
// needs only applied/stale/duplicate acknowledgement.
type OutcomeHandler func(context.Context, *sql.Tx, []Intent) ([]Outcome, error)

func (handler OutcomeHandler) Apply(ctx context.Context, tx *sql.Tx, intents []Intent) ([]Result, error) {
	outcomes, err := handler(ctx, tx, intents)
	if err != nil {
		return nil, err
	}
	results := make([]Result, len(outcomes))
	for index, outcome := range outcomes {
		results[index].Outcome = outcome
	}
	return results, nil
}

func (OutcomeHandler) BatchLimit() int { return 0 }

type Config struct {
	Capacity   int
	MaxBatch   int
	OldestWait time.Duration
}

// Snapshot is a stable, bounded view of coordinator load and commit behavior
// for private runtime diagnostics. Latency and size distributions are fixed
// HDR histograms; no raw intent, key, payload, or timing sample is retained.
type Snapshot struct {
	Capacity          int `json:"capacity"`
	Depth             int `json:"depth"`
	PeakDepth         int `json:"peak_depth"`
	BlockedSubmitters int `json:"blocked_submitters"`
	PeakBlocked       int `json:"peak_blocked_submitters"`

	EnqueueCanceled  uint64               `json:"enqueue_canceled"`
	Batches          uint64               `json:"batches"`
	Intents          uint64               `json:"intents"`
	UniqueIntents    uint64               `json:"unique_intents"`
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
	batches, intents, uniqueIntents                       uint64
	applied, stale, duplicate, failures, acknowledgements uint64
	enqueueWait, oldestWait, transaction, commit          durationHistogram
	batchSize                                             sizeHistogram
}

type submission struct {
	intent   Intent
	enqueued time.Time
	ack      chan submissionResult
}

type submissionResult struct {
	Result Result
	Err    error
}

type Coordinator struct {
	writer       *catalogtx.Writer
	config       Config
	handlers     map[string]Handler
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

func New(writer *catalogtx.Writer, config Config, handlers map[string]Handler) (*Coordinator, error) {
	if writer == nil || config.Capacity < 1 || config.MaxBatch < 1 || config.OldestWait <= 0 {
		return nil, errors.New("commit coordinator requires writer and positive bounds")
	}
	if len(handlers) == 0 {
		return nil, errors.New("commit coordinator requires typed handlers")
	}
	owned := make(map[string]Handler, len(handlers))
	for family, handler := range handlers {
		if family == "" || handler == nil {
			return nil, errors.New("invalid commit handler")
		}
		owned[family] = handler
	}
	allSubmitted := make(chan struct{})
	close(allSubmitted)
	return &Coordinator{
		writer: writer, config: config, handlers: owned,
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

func (c *Coordinator) Submit(ctx context.Context, intent Intent) (Result, error) {
	if ctx == nil {
		return Result{}, errors.New("commit submission context is nil")
	}
	if !intent.Key.valid() {
		return Result{}, errors.New("invalid commit intent key")
	}
	if _, ok := c.handlers[intent.Key.Family]; !ok {
		return Result{}, fmt.Errorf("unknown commit family %q", intent.Key.Family)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if !c.started.Load() {
		return Result{}, ErrNotRunning
	}
	s := submission{intent: intent, enqueued: time.Now(), ack: make(chan submissionResult, 1)}
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
		family := pending[0].intent.Key.Family
		batchLimit := c.config.MaxBatch
		if handlerLimit := c.handlers[family].BatchLimit(); handlerLimit > 0 && handlerLimit < batchLimit {
			batchLimit = handlerLimit
		}
		deadline := time.NewTimer(c.config.OldestWait)
	collect:
		for len(pending) < batchLimit {
			select {
			case item := <-c.input:
				if item.intent.Key.Family == family {
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
	// Coalesce exact fenced duplicates while retaining an acknowledgement for
	// every producer.
	unique := make([]Intent, 0, len(items))
	indexes := make(map[string]int)
	followers := make(map[int][]chan submissionResult)
	for _, item := range items {
		key := coalescingKey(item.intent)
		if index, ok := indexes[key]; ok {
			followers[index] = append(followers[index], item.ack)
			continue
		}
		indexes[key] = len(unique)
		followers[len(unique)] = []chan submissionResult{item.ack}
		unique = append(unique, item.intent)
	}
	oldestWait := time.Since(items[0].enqueued)
	results := c.apply(unique)
	for index, result := range results {
		for _, ack := range followers[index] {
			ack <- result
			c.incrementAck(result)
		}
	}
	c.metricsMu.Lock()
	c.metrics.intents += uint64(len(items))
	c.metrics.uniqueIntents += uint64(len(unique))
	c.metrics.oldestWait.record(oldestWait)
	c.metricsMu.Unlock()
}

// coalescingKey includes a stable payload representation. A producer may
// legitimately reuse a fenced key while carrying a different immutable
// result; collapsing those values would acknowledge work that was never
// committed. JSON is preferred for the catalog DTO payloads and a stable
// formatted representation is the fallback for internal values that are not
// JSON encodable.
func coalescingKey(intent Intent) string {
	key := intent.Key.String()
	if encoded, err := json.Marshal(intent.Payload); err == nil {
		return key + "\x00" + string(encoded)
	}
	return key + "\x00" + fmt.Sprintf("%#v", intent.Payload)
}

func (c *Coordinator) apply(intents []Intent) []submissionResult {
	results := make([]Result, len(intents))
	started := time.Now()
	err := c.writer.Transact(context.Background(), catalogtx.OperationBackgroundCommitBatch, nil, func(tx *sql.Tx) error {
		var err error
		results, err = c.handlers[intents[0].Key.Family].Apply(context.Background(), tx, intents)
		if err == nil && len(results) != len(intents) {
			return errors.New("commit handler returned wrong outcome count")
		}
		return err
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
	c.metrics.batchSize.record(len(intents))
	c.metrics.transaction.record(totalDuration)
	c.metrics.commit.record(commitDuration)
	c.metricsMu.Unlock()
	if err != nil && len(intents) > 1 {
		mid := len(intents) / 2
		left := c.apply(intents[:mid])
		right := c.apply(intents[mid:])
		return append(left, right...)
	}
	acknowledgements := make([]submissionResult, len(intents))
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
		Batches:         c.metrics.batches, Intents: c.metrics.intents, UniqueIntents: c.metrics.uniqueIntents,
		Applied: c.metrics.applied, Stale: c.metrics.stale, Duplicate: c.metrics.duplicate,
		Failures: c.metrics.failures, Acknowledgements: c.metrics.acknowledgements,
		EnqueueWait: c.metrics.enqueueWait.snapshot(), OldestWait: c.metrics.oldestWait.snapshot(),
		BatchSize: c.metrics.batchSize.snapshot(), Transaction: c.metrics.transaction.snapshot(),
		Commit: c.metrics.commit.snapshot(),
	}
}
