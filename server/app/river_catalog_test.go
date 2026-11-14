package app

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"server/internal/commit"
	"server/internal/execution"
	"server/internal/pipeline"
	"server/internal/processors"
	"server/internal/queue"
	"server/internal/queue/jobs"
	"server/internal/storage"
	"server/internal/storage/scan"

	"github.com/riverqueue/river"
)

// The production worker set is the closed macro catalog in queue/jobs: every
// kind registered once, and nothing else. A new job kind is a catalog change
// in queue/jobs, never a worker added in server/app.
func TestProductionWorkersAreTheMacroCatalog(t *testing.T) {
	runtime := &pipelineRuntime{
		engine:             &execution.Engine{},
		commits:            &commit.Coordinator{},
		processor:          &processors.AssetProcessor{},
		scanner:            &scan.Scanner{},
		files:              &storage.RepositoryFSFactory{},
		pipelineReader:     struct{ pipeline.Queryer }{},
		eventProjection:    struct{ eventProjectionPreparer }{},
		locationProjection: struct{ locationProjectionPreparer }{},
		ocrProjection:      struct{ ocrProjectionPreparer }{},
		reindexProjection:  struct{ reindexProjectionPreparer }{},
		enrichmentReader:   struct{ queue.EnrichmentReader }{},
	}
	workers := river.NewWorkers()
	if err := runtime.register(workers); err != nil {
		t.Fatal(err)
	}
	if err := runtime.registerBackup(workers, func(context.Context, bool) error { return nil }); err != nil {
		t.Fatal(err)
	}

	// river.Workers keeps its kinds unexported; reading the map keys is
	// enough to compare the set.
	var registered []string
	for _, key := range reflect.ValueOf(workers).Elem().FieldByName("workersMap").MapKeys() {
		registered = append(registered, key.String())
	}
	catalog := []string{
		jobs.IngestAssetArgs{}.Kind(),
		jobs.AnalyzeAssetArgs{}.Kind(),
		jobs.GenerateAssetDerivativesArgs{}.Kind(),
		jobs.TranscodeMediaArgs{}.Kind(),
		jobs.EnrichAssetArgs{}.Kind(),
		jobs.ScanRepositoryBatchArgs{}.Kind(),
		jobs.RebuildProjectionBatchArgs{}.Kind(),
		jobs.BackupCatalogArgs{}.Kind(),
	}
	slices.Sort(registered)
	slices.Sort(catalog)
	if !slices.Equal(registered, catalog) {
		t.Fatalf("production River workers = %v, want the queue/jobs macro catalog %v", registered, catalog)
	}
}
