package processors

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"runtime"
	"sync"
	"testing"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/utils/exif"
	"server/internal/utils/imaging"
	"server/internal/utils/phash"
)

type mediaMatrixStage struct {
	name string
	run  func(context.Context, []byte) error
}

func TestLocalMediaConcurrencyMatrix(t *testing.T) {
	imaging.StartVips()
	source := testJPEG(t)
	stages := []mediaMatrixStage{
		{
			name: "metadata",
			run: func(ctx context.Context, source []byte) error {
				if !exif.IsExifToolAvailable() {
					return nil
				}
				extractor := exif.NewExtractor(&exif.Config{
					ExifToolPath: "exiftool", Timeout: 60 * time.Second,
					BufferSize: 128 * 1024, MaxFileSize: 2 * 1024 * 1024 * 1024,
					FastMode: false, IncludeRaw: true,
				})
				result, err := extractor.ExtractFromStream(ctx, &exif.StreamingExtractRequest{
					Reader: bytes.NewReader(source), AssetType: dbtypes.AssetTypePhoto,
					Filename: "matrix.jpg", Size: int64(len(source)),
				})
				if err != nil {
					return err
				}
				if result == nil {
					return fmt.Errorf("metadata result is nil")
				}
				return nil
			},
		},
	}
	for _, size := range []string{"small", "medium", "large"} {
		size := size
		stages = append(stages, mediaMatrixStage{
			name: "thumbnail_" + size,
			run: func(_ context.Context, source []byte) error {
				var output bytes.Buffer
				if err := imaging.StreamThumbnails(
					bytes.NewReader(source),
					map[string][2]int{size: thumbnailSizes[size]},
					map[string]io.Writer{size: &output},
				); err != nil {
					return err
				}
				if output.Len() == 0 {
					return fmt.Errorf("%s thumbnail is empty", size)
				}
				return nil
			},
		})
	}
	stages = append(stages,
		mediaMatrixStage{
			name: "phash",
			run: func(_ context.Context, source []byte) error {
				hash, err := phash.ComputeFromReader(bytes.NewReader(source))
				if err != nil {
					return err
				}
				if hash == nil {
					return fmt.Errorf("pHash result is nil")
				}
				return nil
			},
		},
		mediaMatrixStage{
			name: "normal_pipeline",
			run: func(_ context.Context, source []byte) error {
				outputs := make(map[string]io.Writer, len(thumbnailSizes))
				buffers := make(map[string]*bytes.Buffer, len(thumbnailSizes))
				for size := range thumbnailSizes {
					buffer := &bytes.Buffer{}
					buffers[size] = buffer
					outputs[size] = buffer
				}
				if err := imaging.StreamThumbnails(bytes.NewReader(source), thumbnailSizes, outputs); err != nil {
					return err
				}
				for size, buffer := range buffers {
					if buffer.Len() == 0 {
						return fmt.Errorf("normal pipeline %s thumbnail is empty", size)
					}
				}
				return nil
			},
		},
	)

	workerCounts := []int{1, 2, 4}
	for _, stage := range stages {
		for _, workers := range workerCounts {
			if stage.name == "metadata" && !exif.IsExifToolAvailable() {
				t.Log("metadata matrix skipped: exiftool is unavailable")
				break
			}
			metrics, err := runMediaMatrixStage(t, stage, source, workers)
			if err != nil {
				t.Fatalf("stage=%s workers=%d: %v", stage.name, workers, err)
			}
			t.Logf(
				"media_matrix stage=%s workers=%d tasks=%d elapsed=%s max_active=%d alloc_delta=%d cpu_count=%d",
				stage.name, workers, metrics.tasks, metrics.elapsed, metrics.maxActive,
				metrics.allocDelta, runtime.NumCPU(),
			)
		}
	}
}

type mediaMatrixMetrics struct {
	tasks      int
	elapsed    time.Duration
	maxActive  int
	allocDelta uint64
}

func runMediaMatrixStage(
	t *testing.T,
	stage mediaMatrixStage,
	source []byte,
	workers int,
) (mediaMatrixMetrics, error) {
	t.Helper()
	tasks := workers * 2
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	started := time.Now()
	var mu sync.Mutex
	active, maxActive := 0, 0
	jobs := make(chan struct{})
	errors := make(chan error, tasks)
	var group sync.WaitGroup
	for index := 0; index < workers; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for range jobs {
				mu.Lock()
				active++
				if active > maxActive {
					maxActive = active
				}
				mu.Unlock()
				err := stage.run(context.Background(), source)
				mu.Lock()
				active--
				mu.Unlock()
				if err != nil {
					errors <- fmt.Errorf("task %d: %w", index, err)
				}
			}
		}()
	}
	for index := 0; index < tasks; index++ {
		jobs <- struct{}{}
	}
	close(jobs)
	group.Wait()
	close(errors)
	for err := range errors {
		return mediaMatrixMetrics{}, err
	}
	runtime.ReadMemStats(&after)
	var allocDelta uint64
	if after.TotalAlloc >= before.TotalAlloc {
		allocDelta = after.TotalAlloc - before.TotalAlloc
	}
	return mediaMatrixMetrics{
		tasks: tasks, elapsed: time.Since(started), maxActive: maxActive, allocDelta: allocDelta,
	}, nil
}
