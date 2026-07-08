package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// summary is the machine-readable per-run result written to summary.json.
type summary struct {
	RunID       string    `json:"run_id"`
	Profile     string    `json:"profile"`
	GeneratedAt time.Time `json:"generated_at"`
	BaseURL     string    `json:"base_url"`
	Concurrency int       `json:"concurrency"`
	ClientHash  bool      `json:"client_hash"`
	DBExact     bool      `json:"db_exact_timing"`

	Dataset struct {
		Dir        string         `json:"dir"`
		FileCount  int            `json:"file_count"`
		TotalBytes int64          `json:"total_bytes"`
		TotalGB    float64        `json:"total_gb"`
		ByExt      map[string]int `json:"by_ext"`
	} `json:"dataset"`

	Upload struct {
		Accepted        int     `json:"accepted"`
		Failed          int     `json:"failed"`
		DurationSeconds float64 `json:"duration_seconds"`
		FilesPerSec     float64 `json:"files_per_sec"`
		MBPerSec        float64 `json:"mb_per_sec"`
		ReqLatencyMsP50 float64 `json:"req_latency_ms_p50"`
		ReqLatencyMsP95 float64 `json:"req_latency_ms_p95"`
		ReqLatencyMsP99 float64 `json:"req_latency_ms_p99"`
		HTTPErrors      int     `json:"http_errors"`
	} `json:"upload"`

	PhotoReady struct {
		Completed          int     `json:"completed"`
		FailedCoreTasks    int     `json:"failed_core_tasks"`
		MakespanSeconds    float64 `json:"makespan_seconds"`
		DrainSeconds       float64 `json:"drain_seconds"`
		PhotosPerMin       float64 `json:"photos_per_min"`
		GBPerMin           float64 `json:"gb_per_min"`
		LatencySecP50      float64 `json:"latency_sec_p50"`
		LatencySecP90      float64 `json:"latency_sec_p90"`
		LatencySecP95      float64 `json:"latency_sec_p95"`
		LatencySecP99      float64 `json:"latency_sec_p99"`
		Complete100Percent bool    `json:"complete_100_percent"`
	} `json:"photo_ready"`

	Queues struct {
		Before       []queueSummary `json:"before"`
		After        []queueSummary `json:"after"`
		MLQueuesIdle bool           `json:"ml_queues_idle"`
	} `json:"queues"`

	MLSettingsBefore mlSettings `json:"ml_settings_before"`
	MLSettingsAfter  mlSettings `json:"ml_settings_after"`
}

func summarize(rc *runContext) *summary {
	s := &summary{
		RunID:       rc.cfg.runID,
		Profile:     rc.cfg.profile,
		GeneratedAt: time.Now(),
		BaseURL:     rc.cfg.baseURL,
		Concurrency: rc.cfg.concurrency,
		ClientHash:  rc.cfg.clientHash,
		DBExact:     rc.db != nil,
	}
	s.Dataset.Dir = rc.mf.Dataset
	s.Dataset.FileCount = rc.mf.FileCount
	s.Dataset.TotalBytes = rc.mf.TotalBytes
	s.Dataset.TotalGB = gb(rc.mf.TotalBytes)
	s.Dataset.ByExt = map[string]int{}
	for _, f := range rc.mf.Files {
		s.Dataset.ByExt[f.Ext]++
	}

	// Upload stats.
	var reqLatMs []float64
	var acceptedBytes int64
	for _, f := range rc.mf.Files {
		ok := f.UploadErr == "" && f.HTTPStatus >= 200 && f.HTTPStatus < 300
		if ok {
			s.Upload.Accepted++
			acceptedBytes += f.Size
			reqLatMs = append(reqLatMs, float64(f.UploadEndNs-f.UploadStartNs)/1e6)
		} else {
			s.Upload.Failed++
			if f.HTTPStatus >= 400 || f.UploadErr != "" {
				s.Upload.HTTPErrors++
			}
		}
	}
	uploadDur := rc.lastAcc.Sub(rc.t0).Seconds()
	s.Upload.DurationSeconds = uploadDur
	if uploadDur > 0 {
		s.Upload.FilesPerSec = float64(s.Upload.Accepted) / uploadDur
		s.Upload.MBPerSec = (float64(acceptedBytes) / (1 << 20)) / uploadDur
	}
	s.Upload.ReqLatencyMsP50 = percentile(reqLatMs, 50)
	s.Upload.ReqLatencyMsP95 = percentile(reqLatMs, 95)
	s.Upload.ReqLatencyMsP99 = percentile(reqLatMs, 99)

	// Photo-ready stats.
	var completionSec []float64
	var completedBytes int64
	for _, f := range rc.mf.Files {
		if f.CompleteNs != 0 {
			s.PhotoReady.Completed++
			completedBytes += f.Size
			// Completion latency = ready - HTTP acceptance (upload end).
			completionSec = append(completionSec, float64(f.CompleteNs-f.UploadEndNs)/1e9)
		}
		if f.Failed {
			s.PhotoReady.FailedCoreTasks++
		}
	}
	if !rc.tEnd.IsZero() {
		makespan := rc.tEnd.Sub(rc.t0).Seconds()
		s.PhotoReady.MakespanSeconds = makespan
		s.PhotoReady.DrainSeconds = rc.tEnd.Sub(rc.lastAcc).Seconds()
		if makespan > 0 {
			s.PhotoReady.PhotosPerMin = float64(s.PhotoReady.Completed) / makespan * 60
			s.PhotoReady.GBPerMin = gb(completedBytes) / makespan * 60
		}
	}
	s.PhotoReady.LatencySecP50 = percentile(completionSec, 50)
	s.PhotoReady.LatencySecP90 = percentile(completionSec, 90)
	s.PhotoReady.LatencySecP95 = percentile(completionSec, 95)
	s.PhotoReady.LatencySecP99 = percentile(completionSec, 99)
	s.PhotoReady.Complete100Percent = s.Upload.Accepted > 0 && s.PhotoReady.Completed == s.Upload.Accepted

	// Queues + ML evidence.
	s.Queues.Before = rc.qBefore
	s.Queues.After = rc.qAfter
	s.Queues.MLQueuesIdle = mlQueuesIdle(rc.qAfter)
	s.MLSettingsBefore = rc.mlBefore.ML
	s.MLSettingsAfter = rc.mlAfter.ML
	return s
}

// mlQueuesIdle reports whether every ML queue processed zero jobs and has none
// remaining — the evidence the report needs to claim ML was excluded.
func mlQueuesIdle(qs []queueSummary) bool {
	idx := map[string]queueSummary{}
	for _, q := range qs {
		idx[q.Name] = q
	}
	for _, name := range mlQueues {
		q, ok := idx[name]
		if !ok {
			continue
		}
		if q.TotalJobs != 0 || q.RemainingJobs != 0 {
			return false
		}
	}
	return true
}

// percentile returns the p-th percentile (0-100) using nearest-rank on a copy.
func percentile(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	if p <= 0 {
		return s[0]
	}
	if p >= 100 {
		return s[len(s)-1]
	}
	rank := int(math.Ceil(p/100*float64(len(s)))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(s) {
		rank = len(s) - 1
	}
	return s[rank]
}

func writeReport(rc *runContext, s *summary) error {
	var b strings.Builder
	f := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }

	f("# Upload benchmark — run %s\n\n", s.RunID)
	f("- Profile: **%s**  ", s.Profile)
	f("Concurrency: **%d**  ", s.Concurrency)
	f("Client-hash mode: **%v**  ", s.ClientHash)
	f("API: `%s`\n", s.BaseURL)
	f("- Generated: %s\n\n", s.GeneratedAt.Format(time.RFC3339))

	f("## Dataset\n\n")
	f("- %d files, %.2f GB\n", s.Dataset.FileCount, s.Dataset.TotalGB)
	f("- By extension: %s\n\n", extBreakdown(s.Dataset.ByExt))

	f("## Photo-ready (headline)\n\n")
	f("| Metric | Value |\n|---|---|\n")
	f("| Completed / accepted | %d / %d |\n", s.PhotoReady.Completed, s.Upload.Accepted)
	f("| 100%% complete | %v |\n", s.PhotoReady.Complete100Percent)
	f("| Failed core tasks | %d |\n", s.PhotoReady.FailedCoreTasks)
	f("| Throughput | %.1f photos/min |\n", s.PhotoReady.PhotosPerMin)
	f("| Data throughput | %.2f GB/min |\n", s.PhotoReady.GBPerMin)
	f("| Makespan (first req → last ready) | %.1f s |\n", s.PhotoReady.MakespanSeconds)
	f("| Drain (last accept → last ready) | %.1f s |\n", s.PhotoReady.DrainSeconds)
	f("| Completion latency p50/p90/p95/p99 | %.1f / %.1f / %.1f / %.1f s |\n\n",
		s.PhotoReady.LatencySecP50, s.PhotoReady.LatencySecP90, s.PhotoReady.LatencySecP95, s.PhotoReady.LatencySecP99)

	f("## Upload acceptance\n\n")
	f("| Metric | Value |\n|---|---|\n")
	f("| Accepted / failed | %d / %d |\n", s.Upload.Accepted, s.Upload.Failed)
	f("| HTTP errors | %d |\n", s.Upload.HTTPErrors)
	f("| Accept rate | %.1f files/s, %.1f MB/s |\n", s.Upload.FilesPerSec, s.Upload.MBPerSec)
	f("| Request latency p50/p95/p99 | %.0f / %.0f / %.0f ms |\n\n",
		s.Upload.ReqLatencyMsP50, s.Upload.ReqLatencyMsP95, s.Upload.ReqLatencyMsP99)

	f("## ML exclusion evidence\n\n")
	f("- ML settings before: `%+v`\n", s.MLSettingsBefore)
	f("- ML settings after: `%+v`\n", s.MLSettingsAfter)
	f("- ML queues idle (zero jobs, zero remaining): **%v**\n\n", s.Queues.MLQueuesIdle)

	f("## Queue activity (after run)\n\n")
	f("| Queue | total | processed | remaining | attention | avg latency ms | avg runtime ms |\n")
	f("|---|---|---|---|---|---|---|\n")
	for _, q := range s.Queues.After {
		f("| %s | %d | %d | %d | %d | %s | %s |\n",
			q.Name, q.TotalJobs, q.ProcessedJobs, q.RemainingJobs, q.AttentionJobs,
			ms(q.AverageLatencyMs), ms(q.AverageRuntimeMs))
	}
	f("\n")

	f("## Publishability checklist\n\n")
	f("- [%s] 100%% of accepted photos reached photo-ready\n", check(s.PhotoReady.Complete100Percent))
	f("- [%s] Zero HTTP errors\n", check(s.Upload.HTTPErrors == 0))
	f("- [%s] Zero failed core tasks\n", check(s.PhotoReady.FailedCoreTasks == 0))
	f("- [%s] ML queues idle / excluded\n", check(s.Queues.MLQueuesIdle))
	f("- [ ] At least 5 repetitions collected for the headline config (aggregate across runs)\n\n")

	timingNote := "completion times are poll-cadence-bounded (see -poll-interval)"
	if s.DBExact {
		timingNote = "completion times are exact (river_job.finalized_at)"
	}
	f("> Client-hash mode = %v; %s. HTTP acceptance is asynchronous and is NOT photo-ready. ", s.ClientHash, timingNote)
	f("This is a single run; publish the median of >=5 runs with hardware/dataset disclosure.\n")

	path := filepath.Join(rc.cfg.outDir, "report.md")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return nil
}

func extBreakdown(byExt map[string]int) string {
	keys := make([]string, 0, len(byExt))
	for k := range byExt {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, byExt[k]))
	}
	return strings.Join(parts, ", ")
}

func ms(v *int64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *v)
}

func check(ok bool) string {
	if ok {
		return "x"
	}
	return " "
}

func printHeadline(s *summary) {
	log.Printf("HEADLINE: %d/%d photo-ready | %.1f photos/min | %.2f GB/min | p95 latency %.1fs | ML idle=%v",
		s.PhotoReady.Completed, s.Upload.Accepted, s.PhotoReady.PhotosPerMin, s.PhotoReady.GBPerMin,
		s.PhotoReady.LatencySecP95, s.Queues.MLQueuesIdle)
}
