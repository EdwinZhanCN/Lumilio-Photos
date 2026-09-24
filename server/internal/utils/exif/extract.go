package exif

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"server/internal/db/dbtypes"
	"server/internal/utils/sysproc"
	"sync"
	"time"
)

// Extractor handles EXIF metadata extraction using streaming and concurrency
type Extractor struct {
	config     *Config
	tagConfig  *TagConfig
	workerPool chan struct{}
	mu         sync.RWMutex
	cache      map[string]interface{}
}

// NewExtractor creates a new EXIF extractor with streaming capabilities
func NewExtractor(config *Config) *Extractor {
	if config == nil {
		config = DefaultConfig()
	}
	if config.ExifToolPath == "" {
		config.ExifToolPath = "exiftool"
	}

	// Optimize configuration for large file processing
	if config.WorkerCount == 0 {
		config.WorkerCount = GetOptimalWorkerCount()
	}
	if config.BufferSize == 0 {
		config.BufferSize = 64 * 1024 // 64KB default
	}

	return &Extractor{
		config:     config,
		tagConfig:  DefaultTagConfig(),
		workerPool: make(chan struct{}, config.WorkerCount),
		cache:      make(map[string]interface{}),
	}
}

// MetadataResult holds the result of metadata extraction
type MetadataResult struct {
	Metadata interface{}
	Common   dbtypes.CommonMetadata
	Error    error
	Type     dbtypes.AssetType
	Raw      json.RawMessage
}

// StreamingExtractRequest represents a request for streaming metadata extraction
type StreamingExtractRequest struct {
	Reader    io.Reader
	AssetType dbtypes.AssetType
	Filename  string
	Size      int64
}

// ExtractFromStream extracts metadata from an io.Reader stream with true streaming
func (e *Extractor) ExtractFromStream(ctx context.Context, req *StreamingExtractRequest) (*MetadataResult, error) {
	if err := e.validateRequest(req); err != nil {
		return nil, err
	}

	// Check if we can handle this file size
	if canHandle, reason := CanHandleFileSize(req.Size); !canHandle {
		return nil, fmt.Errorf("cannot process file: %s", reason)
	}

	// Optimize buffer size for large files
	if IsLargeFile(req.Size) {
		e.config.BufferSize = GetOptimalBufferSize(req.Size)
	}

	// Acquire worker from pool
	select {
	case e.workerPool <- struct{}{}:
		defer func() { <-e.workerPool }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Extract metadata directly from stream without loading entire file into memory
	result := &MetadataResult{Type: req.AssetType}
	result.Metadata, result.Common, result.Raw, result.Error = e.extractMetadataFromStream(ctx, req.Reader, req.AssetType)
	if result.Error != nil {
		return nil, fmt.Errorf("extract metadata from stream: %w", result.Error)
	}

	return result, nil
}

// ExtractBatch processes multiple extraction requests concurrently
func (e *Extractor) ExtractBatch(ctx context.Context, requests []*StreamingExtractRequest) ([]*MetadataResult, error) {
	if len(requests) == 0 {
		return nil, fmt.Errorf("no requests provided")
	}

	results := make([]*MetadataResult, len(requests))
	var wg sync.WaitGroup
	errChan := make(chan error, len(requests))

	// Process requests concurrently
	for i, req := range requests {
		wg.Add(1)
		go func(index int, request *StreamingExtractRequest) {
			defer wg.Done()

			result, err := e.ExtractFromStream(ctx, request)
			if err != nil {
				errChan <- fmt.Errorf("request %d failed: %w", index, err)
				return
			}

			results[index] = result
		}(i, req)
	}

	// Wait for completion
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Collect errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return results, fmt.Errorf("batch processing completed with %d errors: %v", len(errors), errors[0])
	}

	return results, nil
}

// validateRequest validates an extraction request
func (e *Extractor) validateRequest(req *StreamingExtractRequest) error {
	if req.Reader == nil {
		return fmt.Errorf("reader cannot be nil")
	}

	if req.Size > e.config.MaxFileSize {
		return fmt.Errorf("file size %d exceeds maximum allowed size %d", req.Size, e.config.MaxFileSize)
	}

	if !req.AssetType.Valid() {
		return fmt.Errorf("invalid asset type: %s", req.AssetType)
	}

	// Additional validation for large files
	if IsLargeFile(req.Size) {
		// For large files, ensure we have reasonable timeout
		if e.config.Timeout < 60*time.Second {
			e.config.Timeout = 120 * time.Second // Increase timeout for large files
		}
	}

	return nil
}

// extractMetadataFromStream extracts metadata directly from stream without buffering entire file
func (e *Extractor) extractMetadataFromStream(ctx context.Context, reader io.Reader, assetType dbtypes.AssetType) (interface{}, dbtypes.CommonMetadata, json.RawMessage, error) {
	var tags []string

	switch assetType {
	case dbtypes.AssetTypePhoto:
		tags = e.tagConfig.PhotoTags
	case dbtypes.AssetTypeVideo:
		tags = e.tagConfig.VideoTags
	case dbtypes.AssetTypeAudio:
		tags = e.tagConfig.AudioTags
	default:
		return nil, dbtypes.CommonMetadata{}, nil, fmt.Errorf("unsupported asset type: %s", assetType)
	}

	if e.config.IncludeRaw {
		tags = nil
	}

	rawData, rawJSON, err := e.runExifToolFromStream(ctx, reader, tags)
	if err != nil {
		return nil, dbtypes.CommonMetadata{}, nil, err
	}
	common := parseCommonMetadata(rawData, rawJSON, assetType)

	if !e.config.IncludeRaw {
		rawJSON = nil
	}

	return e.parseMetadata(rawData, rawJSON, assetType), common, rawJSON, nil
}

// parseMetadata parses raw metadata based on asset type
func (e *Extractor) parseMetadata(rawData map[string]string, rawJSON json.RawMessage, assetType dbtypes.AssetType) interface{} {
	switch assetType {
	case dbtypes.AssetTypePhoto:
		return parsePhotoMetadata(rawData)
	case dbtypes.AssetTypeVideo:
		return parseVideoMetadata(rawData)
	case dbtypes.AssetTypeAudio:
		return parseAudioMetadataWithRaw(rawData, rawJSON)
	default:
		return nil
	}
}

// runExifToolFromStream executes exiftool with true streaming input (no memory buffering)
func (e *Extractor) runExifToolFromStream(ctx context.Context, reader io.Reader, tags []string) (map[string]string, json.RawMessage, error) {
	// Create context with timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, e.config.Timeout)
	defer cancel()

	// Build command arguments
	args := e.buildExifToolArgs(tags)

	// Create and configure command
	cmd := exec.CommandContext(ctxWithTimeout, e.config.ExifToolPath, args...)
	sysproc.HideConsole(cmd)

	// Let os/exec own the copy goroutines and process lifetime. Its stdin
	// copier tolerates a child closing its input early (ExifTool does this for
	// JXL codestreams); Run still waits for exit and drains both output streams.
	// Track source failures separately so a reader's EPIPE cannot be mistaken
	// for the child's successful early close.
	source := &metadataSourceReader{reader: reader, cancel: cancel}
	var output, diagnostics bytes.Buffer
	cmd.Stdin = bufio.NewReaderSize(source, e.config.BufferSize)
	cmd.Stdout = &output
	cmd.Stderr = &diagnostics
	runErr := cmd.Run()
	if source.err != nil {
		return nil, nil, fmt.Errorf("reading metadata source: %w", source.err)
	}
	if err := ctxWithTimeout.Err(); err != nil {
		return nil, nil, fmt.Errorf("exiftool execution canceled: %w", err)
	}
	if runErr != nil {
		return nil, nil, fmt.Errorf("exiftool command failed: %w", runErr)
	}
	if containsCriticalError(diagnostics.String()) {
		return nil, nil, fmt.Errorf("exiftool reported error: %s", diagnostics.String())
	}
	return e.parseExifToolOutput(output.Bytes())
}

// buildExifToolArgs builds command line arguments for exiftool
func (e *Extractor) buildExifToolArgs(tags []string) []string {
	args := []string{"-j", "-charset", "utf8", "-ignoreMinorErrors"}

	// Add fast mode if configured
	if e.config.FastMode {
		args = append(args, "-fast")
	}

	// Add specific tags
	for _, tag := range tags {
		args = append(args, "-"+tag)
	}

	// Read from stdin
	args = append(args, "-")

	return args
}

// metadataSourceReader records errors from the media source, independently of
// errors writing to the subprocess pipe. Run joins its reader before inspection.
type metadataSourceReader struct {
	reader io.Reader
	cancel context.CancelFunc
	err    error
}

func (r *metadataSourceReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if err != nil && err != io.EOF {
		r.err = err
		r.cancel()
	}
	return n, err
}

// containsCriticalError checks if stderr contains critical errors (not warnings)
func containsCriticalError(stderr string) bool {
	// Common exiftool warnings that can be ignored
	ignorableWarnings := []string{
		"Warning",
		"Unknown file type",
		"End of directory",
		"Minor errors",
	}

	stderrLower := stderr
	for _, warning := range ignorableWarnings {
		if contains(stderrLower, warning) {
			return false
		}
	}

	// If stderr contains actual error messages, return true
	return len(stderr) > 0 && !contains(stderrLower, "Warning")
}

// contains checks if string contains substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			contains(s[1:], substr))))
}

// parseExifToolOutput parses JSON output from exiftool
func (e *Extractor) parseExifToolOutput(output []byte) (map[string]string, json.RawMessage, error) {
	if len(output) == 0 {
		return make(map[string]string), nil, nil
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, nil, fmt.Errorf("failed to parse exiftool JSON output: %w", err)
	}

	if len(result) == 0 {
		return make(map[string]string), nil, nil
	}

	rawJSON, err := json.Marshal(result[0])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal exiftool JSON object: %w", err)
	}

	// Convert to string map
	stringMap := make(map[string]string)
	for key, value := range result[0] {
		if value != nil {
			stringMap[key] = fmt.Sprintf("%v", value)
		}
	}

	return stringMap, rawJSON, nil
}

// Close cleans up resources
func (e *Extractor) Close() error {
	close(e.workerPool)
	return nil
}
