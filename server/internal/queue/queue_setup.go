package queue

import (
	"database/sql"
	"fmt"
	"log/slog"
	"runtime"
	"sort"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver"
	"github.com/riverqueue/river/riverdriver/riversqlite"

	"server/internal/queue/jobs"
)

// riverSQLiteSplitDriver keeps every River mutation and transaction on the
// catalog's sole writer while moving riversqlite's notification outbox polling
// onto the query-only reader pool. riversqlite otherwise uses the same *sql.DB
// for its executor and its 50ms NotificationGetAfter SELECT loop, needlessly
// admitting read polling through the sole writer connection.
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

func clampWorkers(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func queueWorkerCountsForCPU(cpuCount int) (ingestWorkers int, thumbnailWorkers int, phashWorkers int) {
	if cpuCount < 1 {
		cpuCount = 1
	}

	// Favor user-visible thumbnail generation over lightweight ingest fan-out,
	// and keep pHash from contending with thumbnails during large imports.
	ingestWorkers = clampWorkers((cpuCount+1)/2, 2, 8)
	thumbnailWorkers = clampWorkers(cpuCount, 4, 12)
	phashWorkers = clampWorkers(cpuCount/4, 1, 4)

	if thumbnailWorkers < ingestWorkers {
		thumbnailWorkers = ingestWorkers
	}

	return ingestWorkers, thumbnailWorkers, phashWorkers
}

func queueWorkerCounts() (ingestWorkers int, thumbnailWorkers int, phashWorkers int) {
	return queueWorkerCountsForCPU(runtime.NumCPU())
}

func runtimeQueueConfigsForCPU(cpuCount int) map[string]river.QueueConfig {
	workers := RuntimeQueueWorkerCountsForCPU(cpuCount)
	queues := make(map[string]river.QueueConfig, len(workers))
	for name, count := range workers {
		queues[name] = river.QueueConfig{MaxWorkers: count}
	}
	return queues
}

// RuntimeQueueWorkerCountsForCPU is the closed queue concurrency manifest used
// by both River construction and the host-side qualification run manifest.
// Returning a fresh map prevents diagnostic callers from mutating runtime
// configuration.
func RuntimeQueueWorkerCountsForCPU(cpuCount int) map[string]int {
	ingestWorkers, thumbnailWorkers, phashWorkers := queueWorkerCountsForCPU(cpuCount)
	return map[string]int{
		jobs.QueueIngestAsset:           ingestWorkers,
		jobs.QueueMetadataAsset:         20,
		jobs.QueueThumbnailAsset:        thumbnailWorkers,
		jobs.QueueTranscodeAsset:        1,
		jobs.QueueRetryAsset:            2,
		jobs.QueueReindexAssets:         1,
		jobs.QueueRebuildLocations:      1,
		jobs.QueueRebuildEvents:         4,
		jobs.QueueEventScheduler:        1,
		jobs.QueueObserveRepository:     4,
		jobs.QueueHashRepositoryNode:    4,
		jobs.QueueRepositoryOutbox:      1,
		jobs.QueueDatabaseBackup:        1,
		jobs.QueueDetectStacks:          1,
		jobs.QueueMatchLivePhoto:        2,
		jobs.QueueProcessSemantic:       2,
		jobs.QueueProcessBioClip:        1,
		jobs.QueueProcessOCR:            2,
		jobs.QueueOCRIndex:              1,
		jobs.QueueProcessFace:           1,
		jobs.QueueProcessVideoFrames:    1,
		jobs.QueueClassifyZeroShot:      2,
		jobs.QueueProcessPerceptualHash: phashWorkers,
	}
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

func New(writerPool, readerPool *sql.DB, workers *river.Workers, logger *slog.Logger) (*river.Client[*sql.Tx], error) {
	if writerPool == nil || readerPool == nil {
		return nil, fmt.Errorf("River SQLite requires writer and query-only listener pools")
	}
	queues := runtimeQueueConfigsForCPU(runtime.NumCPU())
	if err := validateRuntimeQueueConfigs(queues); err != nil {
		return nil, err
	}

	client, err := river.NewClient(newRiverSQLiteSplitDriver(writerPool, readerPool), &river.Config{
		Queues:  queues,
		Workers: workers,
		Logger:  logger,
	})
	return client, err
}
