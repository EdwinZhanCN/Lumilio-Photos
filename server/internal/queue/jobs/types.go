package jobs

import (
	"fmt"
	"sort"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// activeUniqueStates is the convergence boundary for repeatable work. River
// deduplicates only active control-plane jobs; catalog desired/applied state
// remains the product correctness authority.
func activeUniqueStates() []rivertype.JobState {
	return []rivertype.JobState{
		rivertype.JobStateAvailable,
		rivertype.JobStateDiscarded,
		rivertype.JobStatePending,
		rivertype.JobStateRetryable,
		rivertype.JobStateRunning,
		rivertype.JobStateScheduled,
	}
}

// RuntimeJob is the minimum contract shared by every registered River macro.
// The closed catalog prevents unknown payloads from entering an implicit
// default queue after a QueueDB rebuild.
type RuntimeJob interface {
	Kind() string
	InsertOpts() river.InsertOpts
}

// RuntimeJobCatalog is the only River job catalog. Fine-grained asset,
// repository-node, retry, scheduler, and projection jobs intentionally do not
// have representations here.
func RuntimeJobCatalog() []RuntimeJob {
	return []RuntimeJob{
		IngestAssetArgs{}, AnalyzeAssetArgs{}, GenerateAssetDerivativesArgs{},
		TranscodeMediaArgs{}, EnrichAssetArgs{}, ScanRepositoryBatchArgs{},
		RebuildProjectionBatchArgs{}, BackupCatalogArgs{},
	}
}

// NewArgs constructs a concrete macro payload for a domain command kind.
func NewArgs(kind string) (river.JobArgs, error) {
	for _, representative := range RuntimeJobCatalog() {
		if representative.Kind() != kind {
			continue
		}
		switch representative.(type) {
		case IngestAssetArgs:
			return &IngestAssetArgs{}, nil
		case AnalyzeAssetArgs:
			return &AnalyzeAssetArgs{}, nil
		case GenerateAssetDerivativesArgs:
			return &GenerateAssetDerivativesArgs{}, nil
		case TranscodeMediaArgs:
			return &TranscodeMediaArgs{}, nil
		case EnrichAssetArgs:
			return &EnrichAssetArgs{}, nil
		case ScanRepositoryBatchArgs:
			return &ScanRepositoryBatchArgs{}, nil
		case RebuildProjectionBatchArgs:
			return &RebuildProjectionBatchArgs{}, nil
		case BackupCatalogArgs:
			return &BackupCatalogArgs{}, nil
		default:
			return nil, fmt.Errorf("runtime job kind %q has no constructor", kind)
		}
	}
	return nil, fmt.Errorf("unknown runtime job kind %q", kind)
}

// RuntimeQueueNames returns the canonical queue set used by setup and
// diagnostics. There is deliberately one macro admission lane.
func RuntimeQueueNames() []string {
	seen := map[string]struct{}{}
	for _, job := range RuntimeJobCatalog() {
		seen[job.InsertOpts().Queue] = struct{}{}
	}
	queues := make([]string, 0, len(seen))
	for queue := range seen {
		queues = append(queues, queue)
	}
	sort.Strings(queues)
	return queues
}
