package queue

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver"
	"github.com/riverqueue/river/riverdriver/riversqlite"

	"server/internal/queue/jobs"
)

// riverSQLiteSplitDriver keeps every River mutation and transaction on the
// QueueDB's sole writer while moving riversqlite's notification outbox
// polling onto the query-only reader pool. riversqlite otherwise uses the same
// *sql.DB for its executor and its 50ms NotificationGetAfter SELECT loop,
// needlessly admitting read polling through the sole writer connection.
//
// Embedding the pinned upstream driver deliberately inherits its unstable
// interface; the compile-time assertion and held-writer test make a River
// upgrade fail locally if that delegation boundary changes.
type riverSQLiteSplitDriver struct {
	*riversqlite.Driver
	listenerDriver *riversqlite.Driver
}

var _ riverdriver.Driver[*sql.Tx] = (*riverSQLiteSplitDriver)(nil)

func newRiverSQLiteSplitDriver(writerPool, readerPool *sql.DB) *riverSQLiteSplitDriver {
	return &riverSQLiteSplitDriver{
		Driver:         riversqlite.New(writerPool),
		listenerDriver: riversqlite.New(readerPool),
	}
}

func (d *riverSQLiteSplitDriver) GetListener(params *riverdriver.GetListenenerParams) riverdriver.Listener {
	return d.listenerDriver.GetListener(params)
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

func New(writerPool, readerPool *sql.DB, workers *river.Workers, logger *slog.Logger, macroWorkers int) (*river.Client[*sql.Tx], error) {
	if writerPool == nil || readerPool == nil {
		return nil, fmt.Errorf("River SQLite requires writer and query-only listener pools")
	}
	if macroWorkers < 2 {
		return nil, fmt.Errorf("River macro worker count must be at least 2")
	}
	queues := runtimeQueueConfigs(macroWorkers)
	if err := validateRuntimeQueueConfigs(queues); err != nil {
		return nil, err
	}

	client, err := river.NewClient(newRiverSQLiteSplitDriver(writerPool, readerPool), &river.Config{
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
