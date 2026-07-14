// Command devseed materializes the tracked 225-photo demo library into a fresh
// local development instance through the real repository scan and upload APIs.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	defaultAPIURL     = "http://localhost:6680"
	defaultDatasetDir = "../demo/seed/library"
	defaultTimeout    = 20 * time.Minute
)

type options struct {
	datasetPath string
	checkOnly   bool
	timeout     time.Duration
	workers     int
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "dev seed:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("devseed", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var opts options
	flags.StringVar(&opts.datasetPath, "dataset", defaultDatasetDir, "tracked seed dataset root")
	flags.BoolVar(&opts.checkOnly, "check", false, "validate the tracked dataset without using the API")
	flags.DurationVar(&opts.timeout, "timeout", defaultTimeout, "overall runtime timeout")
	flags.IntVar(&opts.workers, "upload-workers", 4, "concurrent upload requests")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if opts.timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	if opts.workers < 1 || opts.workers > 8 {
		return errors.New("upload-workers must be between 1 and 8")
	}

	data, err := validateDataset(opts.datasetPath)
	if err != nil {
		return err
	}
	fmt.Printf("dataset valid: scan=%d upload=%d total=%d root=%s\n", len(data.scanFiles), len(data.uploadFiles), len(data.scanFiles)+len(data.uploadFiles), data.root)
	if opts.checkOnly {
		return nil
	}

	apiURL := strings.TrimRight(strings.TrimSpace(os.Getenv("LUMILIO_API_URL")), "/")
	if apiURL == "" {
		apiURL = defaultAPIURL
	}
	if err := requireLoopbackAPI(apiURL); err != nil {
		return err
	}
	username := strings.TrimSpace(os.Getenv("LUMILIO_ADMIN_USERNAME"))
	if username == "" {
		username = "admin"
	}
	password := os.Getenv("LUMILIO_ADMIN_PASSWORD")
	if strings.TrimSpace(password) == "" {
		return errors.New("LUMILIO_ADMIN_PASSWORD is required; it is used only for the fresh development administrator and is never persisted by this command")
	}

	parent, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(parent, opts.timeout)
	defer cancel()
	client := &apiClient{
		baseURL: apiURL,
		http:    &http.Client{Timeout: 90 * time.Second},
	}

	reachableCtx, reachableCancel := context.WithTimeout(ctx, time.Minute)
	status, err := client.waitUntilReachable(reachableCtx)
	reachableCancel()
	if err != nil {
		return err
	}
	token, primary, err := client.bootstrap(ctx, status, username, password)
	if err != nil {
		return err
	}

	inspection, err := inspectScanTargets(data, primary.Path)
	if err != nil {
		return err
	}
	if len(inspection.Conflicts) > 0 {
		return fmt.Errorf("Primary repository has %d conflicting seed paths (first: %s); refusing to overwrite originals", len(inspection.Conflicts), inspection.Conflicts[0])
	}
	existingAssets, err := client.totalAssets(ctx, token, primary.ID)
	if err != nil {
		return fmt.Errorf("read existing library size: %w", err)
	}
	if existingAssets > expectedScanImages+expectedUploadImages {
		return fmt.Errorf("library already has %d assets; dev seed is limited to a fresh or previously seeded instance", existingAssets)
	}
	if existingAssets > 0 && inspection.Missing > 0 {
		return fmt.Errorf("library already has %d assets but %d tracked scan files are absent; run make dev-reset instead of mixing the demo seed into another library", existingAssets, inspection.Missing)
	}

	copied, retained, err := materializeScanFiles(data, primary.Path)
	if err != nil {
		return err
	}
	fmt.Printf("scan source materialized: copied=%d retained=%d\n", copied, retained)

	queued, queuedAt, err := client.queueManualScan(ctx, token, primary.ID)
	if err != nil {
		return err
	}
	fmt.Printf("manual repository scan queued: job=%d status=%s\n", queued.JobID, queued.Status)
	run, err := client.waitForManualScan(ctx, token, primary.ID, username, queuedAt)
	if err != nil {
		return err
	}
	fmt.Printf("manual repository scan completed: discovered=%d updated=%d skipped=%d\n", run.Discovered, run.Updated, run.Skipped)

	if err := client.uploadDataset(ctx, token, data.uploadFiles, opts.workers); err != nil {
		return err
	}
	if err := client.waitForLibrary(ctx, token, primary.ID); err != nil {
		return err
	}
	fmt.Printf("development library ready: total=%d scan=%d upload=%d\n", expectedScanImages+expectedUploadImages, expectedScanImages, expectedUploadImages)
	return nil
}

func requireLoopbackAPI(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid LUMILIO_API_URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("development seed requires an http(s) API URL, got %q", parsed.Scheme)
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return errors.New("LUMILIO_API_URL must not contain a path")
	}
	if port := parsed.Port(); port != "" {
		if _, err := strconv.ParseUint(port, 10, 16); err != nil {
			return fmt.Errorf("invalid API port %q", port)
		}
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("refusing non-loopback LUMILIO_API_URL %q; this command is development-only", rawURL)
	}
	return nil
}
