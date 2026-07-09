package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// mlQueues are the queues that must be empty/inactive for a valid ML-excluded run.
var mlQueues = []string{"process_semantic", "process_bioclip", "process_ocr", "process_face", "classify_zeroshot"}

// coreTasks are the tasks whose completion defines "photo-ready".
var coreTasks = []string{"metadata_asset", "thumbnail_asset"}

// uploadStatusDuplicate is the status the server returns for an upload it
// satisfied from content it already holds.
const uploadStatusDuplicate = "duplicate"

// instantResult reports the optional second pass over the same dataset.
type instantResult struct {
	Duplicates   int
	Ingested     int
	Failed       int
	BytesSkipped int64
	Duration     time.Duration
}

// runContext bundles everything a single run needs after preflight.
type runContext struct {
	cfg      config
	cli      *client
	db       *dbClient // non-nil when -db is set (exact river_job timing)
	mf       *manifest
	repo     repositoryDTO
	byName   map[string]*fileRec
	t0       time.Time // first upload request start
	lastAcc  time.Time // last HTTP upload accepted
	tEnd     time.Time // last photo-ready completion
	instant  *instantResult
	mlBefore systemSettings
	mlAfter  systemSettings
	qBefore  []queueSummary
	qAfter   []queueSummary
	statsPre jobStats
}

func run(ctx context.Context, cfg config) error {
	if err := os.MkdirAll(cfg.outDir, 0o755); err != nil {
		return fmt.Errorf("create out dir: %w", err)
	}
	log.Printf("run %s | profile=%s | concurrency=%d | out=%s", cfg.runID, cfg.profile, cfg.concurrency, cfg.outDir)

	// 1. Manifest.
	mf, err := buildManifest(cfg)
	if err != nil {
		return err
	}
	log.Printf("manifest: %d files, %.2f GB", mf.FileCount, gb(mf.TotalBytes))
	if err := writeJSON(filepath.Join(cfg.outDir, "manifest.json"), mf); err != nil {
		return err
	}

	rc := &runContext{cfg: cfg, mf: mf, cli: newClient(cfg.baseURL), byName: make(map[string]*fileRec, len(mf.Files))}
	for _, f := range mf.Files {
		rc.byName[f.Name] = f
	}

	// Optional DB connection for exact per-asset timing.
	if cfg.db != "" {
		db, err := newDBClient(ctx, cfg.db)
		if err != nil {
			return fmt.Errorf("connect -db: %w", err)
		}
		defer db.close()
		rc.db = db
		log.Printf("DB timing enabled: exact per-asset completion from river_job.finalized_at")
	}

	// 2-4. Auth, repository, preflight.
	if err := preflight(ctx, rc); err != nil {
		return err
	}

	// 5. Resource sampler (optional).
	stopSampler := startSampler(ctx, cfg)
	defer stopSampler()

	// 6. Upload.
	uploadPhase(ctx, rc)

	// 7. Completion poll.
	if err := pollPhase(ctx, rc); err != nil {
		log.Printf("WARNING: completion polling ended early: %v", err)
	}

	// 7b. Optional instant-upload pass over the same dataset.
	if cfg.instantPass {
		instantPhase(ctx, rc)
	}

	// 8-9. Post-run snapshots + ML evidence.
	postflight(ctx, rc)

	// 10. Artifacts.
	if err := writeEvents(rc); err != nil {
		return err
	}
	sum := summarize(rc)
	if err := writeJSON(filepath.Join(cfg.outDir, "summary.json"), sum); err != nil {
		return err
	}
	if err := writeReport(rc, sum); err != nil {
		return err
	}
	log.Printf("done. artifacts in %s", cfg.outDir)
	printHeadline(sum)
	return nil
}

func preflight(ctx context.Context, rc *runContext) error {
	if err := rc.cli.health(ctx); err != nil {
		return fmt.Errorf("server not healthy at %s: %w", rc.cfg.baseURL, err)
	}
	if err := rc.cli.login(ctx, rc.cfg.username, rc.cfg.password); err != nil {
		return err
	}
	repo, err := rc.cli.primaryRepository(ctx)
	if err != nil {
		return err
	}
	rc.repo = repo
	log.Printf("repository: %s (%s) primary=%v", repo.Name, repo.ID, repo.IsPrimary)

	rc.mlBefore, err = rc.cli.systemSettings(ctx)
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	if rc.cfg.disableML {
		s, err := rc.cli.disableML(ctx)
		if err != nil {
			return err
		}
		rc.mlBefore = s
		log.Printf("ML disabled via settings API: %+v", s.ML)
	} else if rc.mlBefore.ML != (mlSettings{}) {
		log.Printf("WARNING: -disable-ml=false and ML is enabled (%+v); ML queues may contaminate the run", rc.mlBefore.ML)
	}

	// Queue-empty preflight (dataset hygiene / ML-exclusion evidence).
	rc.statsPre, err = rc.cli.jobStats(ctx)
	if err != nil {
		return fmt.Errorf("job stats: %w", err)
	}
	if rc.statsPre.pending() != 0 {
		return fmt.Errorf("preflight: %d jobs still pending; start from a clean queue (reset DB/repository)", rc.statsPre.pending())
	}
	rc.qBefore, _ = rc.cli.queueSummary(ctx)
	log.Printf("preflight queues clean: %d pending", rc.statsPre.pending())
	return nil
}

func uploadPhase(ctx context.Context, rc *runContext) {
	cfg := rc.cfg
	repoField := ""
	if cfg.profile == "default-nonml" {
		repoField = rc.repo.ID // triggers detect_stacks + natural side jobs
	}

	log.Printf("uploading %d files at concurrency %d...", len(rc.mf.Files), cfg.concurrency)
	var (
		firstStart atomic.Int64
		lastAcc    atomic.Int64
		accepted   atomic.Int64
		duplicates atomic.Int64
		failed     atomic.Int64
		wg         sync.WaitGroup
	)
	jobs := make(chan *fileRec)

	for i := 0; i < cfg.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				start := time.Now()
				firstStart.CompareAndSwap(0, start.UnixNano())
				f.UploadStartNs = start.UnixNano()

				resp, err := rc.cli.uploadFile(ctx, f.Path, f.Hash, f.MIME, repoField)
				end := time.Now()
				f.UploadEndNs = end.UnixNano()
				f.HTTPStatus = resp.StatusCode
				if err != nil {
					f.UploadErr = err.Error()
					failed.Add(1)
					continue
				}
				if resp.Status == uploadStatusDuplicate {
					f.Duplicate = true
					duplicates.Add(1)
					continue
				}
				f.TaskID = resp.TaskID
				accepted.Add(1)
				for {
					prev := lastAcc.Load()
					if end.UnixNano() <= prev || lastAcc.CompareAndSwap(prev, end.UnixNano()) {
						break
					}
				}
			}
		}()
	}
	for _, f := range rc.mf.Files {
		if ctx.Err() != nil {
			break
		}
		jobs <- f
	}
	close(jobs)
	wg.Wait()

	rc.t0 = time.Unix(0, firstStart.Load())
	rc.lastAcc = time.Unix(0, lastAcc.Load())
	log.Printf("upload phase done: %d accepted, %d duplicate, %d failed, drain window opens at +%s",
		accepted.Load(), duplicates.Load(), failed.Load(), rc.lastAcc.Sub(rc.t0).Round(time.Millisecond))
}

// instantPhase re-uploads the whole manifest after the first pass has drained.
// Every file should now come back as a duplicate without its bytes being staged
// or ingested, so the wall time measures the instant-upload path end to end.
func instantPhase(ctx context.Context, rc *runContext) {
	cfg := rc.cfg
	repoField := ""
	if cfg.profile == "default-nonml" {
		repoField = rc.repo.ID // same target as the first pass
	}
	log.Printf("instant-upload pass: re-uploading %d files at concurrency %d...", len(rc.mf.Files), cfg.concurrency)

	var (
		duplicates   atomic.Int64
		ingested     atomic.Int64
		failed       atomic.Int64
		bytesSkipped atomic.Int64
		wg           sync.WaitGroup
	)
	jobs := make(chan *fileRec)
	start := time.Now()

	for i := 0; i < cfg.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				resp, err := rc.cli.uploadFile(ctx, f.Path, f.Hash, f.MIME, repoField)
				switch {
				case err != nil:
					failed.Add(1)
				case resp.Status == uploadStatusDuplicate:
					duplicates.Add(1)
					bytesSkipped.Add(f.Size)
				default:
					// A miss here means the fingerprint the server stored does not
					// match the one the client just sent for the same bytes.
					ingested.Add(1)
				}
			}
		}()
	}
	for _, f := range rc.mf.Files {
		if ctx.Err() != nil {
			break
		}
		jobs <- f
	}
	close(jobs)
	wg.Wait()

	rc.instant = &instantResult{
		Duplicates:   int(duplicates.Load()),
		Ingested:     int(ingested.Load()),
		Failed:       int(failed.Load()),
		BytesSkipped: bytesSkipped.Load(),
		Duration:     time.Since(start),
	}
	log.Printf("instant-upload pass done in %s: %d duplicate, %d re-ingested, %d failed",
		rc.instant.Duration.Round(time.Millisecond), rc.instant.Duplicates, rc.instant.Ingested, rc.instant.Failed)
}

// pollPhase polls /assets/list until every accepted upload is photo-ready, a
// core task fails, or the timeout elapses.
func pollPhase(ctx context.Context, rc *runContext) error {
	deadline := time.Now().Add(rc.cfg.timeout)
	expected := 0
	for _, f := range rc.mf.Files {
		if f.Duplicate {
			continue // no ingest job was enqueued, so nothing will complete
		}
		if f.UploadErr == "" && f.HTTPStatus >= 200 && f.HTTPStatus < 300 {
			expected++
		}
	}
	log.Printf("polling for %d photo-ready assets (timeout %s)...", expected, rc.cfg.timeout)

	scan := func() (int, int) { return scanCompletions(ctx, rc) }
	if rc.db != nil {
		scan = func() (int, int) { return scanCompletionsDB(ctx, rc) }
	}

	ticker := time.NewTicker(rc.cfg.pollEvery)
	defer ticker.Stop()

	for {
		done, failedCount := scan()
		if done >= expected {
			log.Printf("all %d assets photo-ready", expected)
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout after %s: %d/%d ready, %d failed", rc.cfg.timeout, done, expected, failedCount)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// scanCompletions lists assets and records the first-observed completion time
// for each matched file. Returns (completed, failed) counts.
func scanCompletions(ctx context.Context, rc *runContext) (int, int) {
	assets, err := rc.cli.listAssets(ctx)
	if err != nil {
		log.Printf("poll: list assets failed: %v", err)
		return countCompleted(rc)
	}
	now := time.Now().UnixNano()
	for _, a := range assets {
		f := rc.byName[a.OriginalFilename]
		if f == nil {
			continue
		}
		if f.AssetID == "" {
			f.AssetID = a.AssetID
		}
		if f.CompleteNs != 0 || f.Failed {
			continue
		}
		state, ok := coreTaskState(a.Status)
		switch {
		case state == taskAllComplete:
			f.CompleteNs = now
		case state == taskAnyFailed:
			f.Failed = true
		}
		_ = ok
	}
	done, failed := countCompleted(rc)
	log.Printf("poll: %d/%d ready, %d failed", done, len(rc.mf.Files), failed)
	return done, failed
}

// scanCompletionsDB records exact per-asset photo-ready time from
// river_job.finalized_at (photo-ready = later of the two completed core jobs).
// This removes the poll-cadence bound that scanCompletions has.
func scanCompletionsDB(ctx context.Context, rc *runContext) (int, int) {
	timings, err := rc.db.coreTimings(ctx)
	if err != nil {
		log.Printf("poll: db timings failed: %v", err)
		return countCompleted(rc)
	}
	for _, t := range timings {
		f := rc.byName[t.filename]
		if f == nil || f.CompleteNs != 0 || f.Failed {
			continue
		}
		if t.failed {
			f.Failed = true
			continue
		}
		if t.metaDone != nil && t.thumbDone != nil {
			done := *t.metaDone
			if t.thumbDone.After(done) {
				done = *t.thumbDone
			}
			f.CompleteNs = done.UnixNano()
		}
	}
	done, failed := countCompleted(rc)
	log.Printf("poll(db): %d/%d ready, %d failed", done, len(rc.mf.Files), failed)
	return done, failed
}

func countCompleted(rc *runContext) (int, int) {
	var done, failed int
	var last int64
	for _, f := range rc.mf.Files {
		if f.CompleteNs != 0 {
			done++
			if f.CompleteNs > last {
				last = f.CompleteNs
			}
		}
		if f.Failed {
			failed++
		}
	}
	if last != 0 {
		rc.tEnd = time.Unix(0, last)
	}
	return done, failed
}

type coreState int

const (
	taskIncomplete coreState = iota
	taskAllComplete
	taskAnyFailed
)

// coreTaskState parses the asset status JSONB and reports whether both core
// tasks are complete, any core task failed, or neither yet.
func coreTaskState(raw []byte) (coreState, bool) {
	if len(raw) == 0 {
		return taskIncomplete, false
	}
	var st struct {
		Tasks map[string]struct {
			State string `json:"state"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return taskIncomplete, false
	}
	all := true
	for _, name := range coreTasks {
		t, ok := st.Tasks[name]
		if !ok {
			all = false
			continue
		}
		if t.State == "failed" {
			return taskAnyFailed, true
		}
		if t.State != "complete" {
			all = false
		}
	}
	if all {
		return taskAllComplete, true
	}
	return taskIncomplete, true
}

func postflight(ctx context.Context, rc *runContext) {
	rc.mlAfter, _ = rc.cli.systemSettings(ctx)
	rc.qAfter, _ = rc.cli.queueSummary(ctx)
}

// startSampler optionally spawns sample.sh in the background and returns a stop
// function. If cfg.sampler is empty the sampler is skipped.
func startSampler(ctx context.Context, cfg config) func() {
	if cfg.sampler == "" {
		return func() {}
	}
	args := []string{cfg.sampler, cfg.outDir, fmt.Sprintf("%d", int(cfg.pollEvery.Seconds()))}
	if cfg.pgContainer != "" {
		args = append(args, cfg.pgContainer)
	}
	cmd := exec.Command("bash", args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Printf("WARNING: could not start sampler %s: %v", cfg.sampler, err)
		return func() {}
	}
	log.Printf("resource sampler started (pid %d)", cmd.Process.Pid)
	return func() {
		_ = cmd.Process.Signal(os.Interrupt)
		_ = cmd.Wait()
	}
}

func writeEvents(rc *runContext) error {
	files := append([]*fileRec(nil), rc.mf.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].UploadStartNs < files[j].UploadStartNs })
	f, err := os.Create(filepath.Join(rc.cfg.outDir, "events.jsonl"))
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, rec := range files {
		if err := enc.Encode(rec); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(path string, v any) error {
	buf, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0o644)
}

func gb(bytes int64) float64 { return float64(bytes) / (1 << 30) }
