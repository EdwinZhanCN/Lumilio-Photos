// Command uploadbench is a reproducible benchmark for the Lumilio Photos
// upload-to-photo-ready pipeline with ML/AI processing excluded.
//
// It measures Profile A (core photo-ready: metadata_asset + thumbnail_asset)
// and, optionally, Profile B (default non-ML, which lets natural side jobs such
// as detect_stacks run). A photo counts as "photo-ready" only once its
// metadata_asset and thumbnail_asset task states are both "complete" — HTTP
// upload acceptance is never treated as completion.
//
// The tool is a host-side client: it drives a running server (native or the
// docker-compose.release.yml stack) over HTTP only, so it needs no direct
// database access. See README.md for the full runbook and the push/pull flow
// for the Docker release environment.
//
// Usage:
//
//	go run ./tools/uploadbench \
//	  -base http://localhost:6680 \
//	  -dataset "/Volumes/CodeBase/Photography/Sep 28 2025" \
//	  -user admin -pass '***' \
//	  -concurrency 8 -profile core
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// config holds the resolved CLI configuration for one benchmark run.
type config struct {
	baseURL     string
	dataset     string
	exts        []string
	username    string
	password    string
	concurrency int
	profile     string // "core" | "default-nonml"
	runID       string
	outDir      string
	disableML   bool
	clientHash  bool
	instantPass bool // re-upload the dataset after drain to measure instant upload
	pollEvery   time.Duration
	timeout     time.Duration
	limit       int    // cap number of files (0 = all)
	sampler     string // path to sample.sh; empty = do not spawn
	pgContainer string // docker container name for PostgreSQL sampling
	db          string // optional Postgres DSN for exact river_job timing
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("[uploadbench] ")

	cfg := parseFlags()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg); err != nil {
		log.Fatalf("benchmark failed: %v", err)
	}
}

func parseFlags() config {
	var (
		cfg     config
		extsRaw string
		pollRaw string
		toRaw   string
	)

	flag.StringVar(&cfg.baseURL, "base", "http://localhost:6680", "server API base URL")
	flag.StringVar(&cfg.dataset, "dataset", "", "dataset directory to walk (required)")
	flag.StringVar(&extsRaw, "exts", "jpg,jpeg,nef", "comma-separated file extensions to include (case-insensitive)")
	flag.StringVar(&cfg.username, "user", "", "benchmark user for login (required)")
	flag.StringVar(&cfg.password, "pass", "", "benchmark user password (required)")
	flag.IntVar(&cfg.concurrency, "concurrency", 8, "number of concurrent uploads")
	flag.StringVar(&cfg.profile, "profile", "core", "profile: core | default-nonml")
	flag.StringVar(&cfg.runID, "run-id", "", "run identifier (default: timestamp)")
	flag.StringVar(&cfg.outDir, "out", "", "output directory (default: ./benchruns/<run-id>)")
	flag.BoolVar(&cfg.disableML, "disable-ml", true, "disable ML settings via the settings API before the run")
	flag.BoolVar(&cfg.clientHash, "client-hash", true, "send a client-computed precheck fingerprint hint (server still verifies full BLAKE3)")
	flag.BoolVar(&cfg.instantPass, "instant-pass", false, "after drain, re-upload the dataset to measure the instant-upload (duplicate-skip) path")
	flag.StringVar(&pollRaw, "poll-interval", "1s", "completion poll interval")
	flag.StringVar(&toRaw, "timeout", "60m", "overall completion timeout")
	flag.IntVar(&cfg.limit, "limit", 0, "cap number of files uploaded (0 = all; useful for smoke tests)")
	flag.StringVar(&cfg.sampler, "sampler", "", "path to sample.sh to spawn for resource sampling (empty = skip)")
	flag.StringVar(&cfg.pgContainer, "pg-container", "", "docker container name for PostgreSQL (passed to sampler)")
	flag.StringVar(&cfg.db, "db", "", "optional Postgres DSN for exact river_job timing (removes the poll-cadence bound on completion latency)")
	flag.Parse()

	fail := func(msg string) {
		fmt.Fprintln(os.Stderr, "error: "+msg)
		flag.Usage()
		os.Exit(2)
	}

	if cfg.dataset == "" {
		fail("-dataset is required")
	}
	if cfg.username == "" || cfg.password == "" {
		fail("-user and -pass are required")
	}
	if cfg.profile != "core" && cfg.profile != "default-nonml" {
		fail("-profile must be 'core' or 'default-nonml'")
	}
	if cfg.concurrency < 1 {
		fail("-concurrency must be >= 1")
	}

	for _, e := range strings.Split(extsRaw, ",") {
		e = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(e, ".")))
		if e != "" {
			cfg.exts = append(cfg.exts, e)
		}
	}
	if len(cfg.exts) == 0 {
		fail("-exts resolved to an empty set")
	}

	var err error
	if cfg.pollEvery, err = time.ParseDuration(pollRaw); err != nil {
		fail("invalid -poll-interval: " + err.Error())
	}
	if cfg.timeout, err = time.ParseDuration(toRaw); err != nil {
		fail("invalid -timeout: " + err.Error())
	}

	if cfg.runID == "" {
		cfg.runID = time.Now().Format("20060102-150405")
	}
	if cfg.outDir == "" {
		cfg.outDir = filepath.Join("benchruns", cfg.runID)
	}
	return cfg
}
