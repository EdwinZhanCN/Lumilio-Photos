package queue

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver"

	"server/internal/work/jobs"
)

// continuationDriver wraps the catalog's River driver so a worker that
// finishes with a continuation wakes the producer without waiting for the
// fetch poll; see continuationWake.
type continuationDriver struct {
	riverdriver.Driver[*sql.Tx]
	continuation *continuationWake
}

func newContinuationDriver(driver riverdriver.Driver[*sql.Tx]) *continuationDriver {
	return &continuationDriver{Driver: driver, continuation: newContinuationWake()}
}

func (d *continuationDriver) GetListener(params *riverdriver.GetListenenerParams) riverdriver.Listener {
	return &continuationListener{Listener: d.Driver.GetListener(params), wake: d.continuation}
}

func (d *continuationDriver) GetExecutor() riverdriver.Executor {
	return &continuationExecutor{Executor: d.Driver.GetExecutor(), wake: d.continuation}
}

func runtimeQueueConfigs(macroWorkers int) map[string]river.QueueConfig {
	workers := RuntimeMacroWorkerCounts(macroWorkers)
	queues := make(map[string]river.QueueConfig, len(workers))
	for name, count := range workers {
		queues[name] = river.QueueConfig{MaxWorkers: count}
	}
	return queues
}

// RuntimeMacroWorkerCounts returns the River admission lane width. Fine-grained
// work is governed by execution.Governor, so host CPU count never multiplies the
// number of competing macro workers. Returning a fresh map prevents diagnostic
// callers from mutating runtime configuration.
func RuntimeMacroWorkerCounts(macroWorkers int) map[string]int {
	return map[string]int{jobs.QueueMacro: macroWorkers}
}

func validateRuntimeQueueConfigs(queues map[string]river.QueueConfig) error {
	required := make(map[string]struct{})
	for _, job := range jobs.RuntimeJobCatalog() {
		queue := job.InsertOpts().Queue
		if queue == "" || queue == river.QueueDefault {
			return fmt.Errorf("job kind %q routes to forbidden implicit queue %q", job.Kind(), queue)
		}
		required[queue] = struct{}{}
	}
	missing := make([]string, 0)
	for queue := range required {
		if _, ok := queues[queue]; !ok {
			missing = append(missing, queue)
		}
	}
	extra := make([]string, 0)
	for queue := range queues {
		if _, ok := required[queue]; !ok {
			extra = append(extra, queue)
		}
	}
	if len(missing) == 0 && len(extra) == 0 {
		return nil
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return fmt.Errorf("runtime queue catalog drift: missing=%v extra=%v", missing, extra)
}

// New builds the River client over driver, the QueueDB's River driver
// (catalog.QueueDB.RiverDriver), which keeps the executor on the queue writer
// and the listener on the queue readers.
func New(driver riverdriver.Driver[*sql.Tx], workers *river.Workers, logger *slog.Logger, macroWorkers int) (*river.Client[*sql.Tx], error) {
	if driver == nil {
		return nil, fmt.Errorf("River requires the QueueDB driver")
	}
	if macroWorkers < 2 {
		return nil, fmt.Errorf("River macro worker count must be at least 2")
	}
	queues := runtimeQueueConfigs(macroWorkers)
	if err := validateRuntimeQueueConfigs(queues); err != nil {
		return nil, err
	}

	client, err := river.NewClient(newContinuationDriver(driver), &river.Config{
		Queues:  queues,
		Workers: workers,
		Logger:  logger,
		// SQLite notification rows wake the producer immediately. Thirty
		// seconds is only the crash/recovery fallback; the existing cooldown
		// remains the throughput guard until measured evidence changes it.
		FetchPollInterval: 30 * time.Second,
	})
	return client, err
}
