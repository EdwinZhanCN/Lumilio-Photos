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
	"sync"
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
	Error    error
	Type     dbtypes.AssetType
}

// StreamingExtractRequest represents a request for streaming metadata extraction
type StreamingExtractRequest struct {
	Reader    io.Reader
	AssetType dbtypes.AssetType
	Filename  string
	Size      int64
}

// ExtractFromStream extracts metadata from an io.Reader stream
func (e *Extractor) ExtractFromStream(ctx context.Context, req *StreamingExtractRequest) (*MetadataResult, error) {
	if err := e.validateRequest(req); err != nil {
		return nil, err
	}

	// Acquire worker from pool
	select {
	case e.workerPool <- struct{}{}:
		defer func() { <-e.workerPool }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Stream data to buffer
	buffer, err := e.streamToBuffer(req.Reader, req.Size)
	if err != nil {
		return nil, fmt.Errorf("failed to read stream: %w", err)
	}

	// Extract metadata based on asset type
	result := &MetadataResult{Type: req.AssetType}
	result.Metadata, result.Error = e.extractMetadataFromBuffer(ctx, buffer, req.AssetType)

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

	return nil
}

// streamToBuffer efficiently streams data from reader to buffer
func (e *Extractor) streamToBuffer(reader io.Reader, size int64) (*bytes.Buffer, error) {
	bufferedReader := bufio.NewReaderSize(reader, e.config.BufferSize)

	var buffer bytes.Buffer
	buffer.Grow(int(size))

	copyBuffer := make([]byte, e.config.BufferSize)
	_, err := io.CopyBuffer(&buffer, bufferedReader, copyBuffer)

	return &buffer, err
}

// extractMetadataFromBuffer extracts metadata from buffer based on asset type
func (e *Extractor) extractMetadataFromBuffer(ctx context.Context, buffer *bytes.Buffer, assetType dbtypes.AssetType) (interface{}, error) {
	var tags []string

	switch assetType {
	case dbtypes.AssetTypePhoto:
		tags = e.tagConfig.PhotoTags
	case dbtypes.AssetTypeVideo:
		tags = e.tagConfig.VideoTags
	case dbtypes.AssetTypeAudio:
		tags = e.tagConfig.AudioTags
	default:
		return nil, fmt.Errorf("unsupported asset type: %s", assetType)
	}

	rawData, err := e.runExifToolFromBuffer(ctx, buffer, tags)
	if err != nil {
		return nil, err
	}

	return e.parseMetadata(rawData, assetType), nil
}

// parseMetadata parses raw metadata based on asset type
func (e *Extractor) parseMetadata(rawData map[string]string, assetType dbtypes.AssetType) interface{} {
	switch assetType {
	case dbtypes.AssetTypePhoto:
		return parsePhotoMetadata(rawData)
	case dbtypes.AssetTypeVideo:
		return parseVideoMetadata(rawData)
	case dbtypes.AssetTypeAudio:
		return parseAudioMetadata(rawData)
	default:
		return nil
	}
}

// runExifToolFromBuffer executes exiftool with streaming input
func (e *Extractor) runExifToolFromBuffer(ctx context.Context, buffer *bytes.Buffer, tags []string) (map[string]string, error) {
	// Create context with timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, e.config.Timeout)
	defer cancel()

	// Build command arguments
	args := e.buildExifToolArgs(tags)

	// Create and configure command
	cmd := exec.CommandContext(ctxWithTimeout, "exiftool", args...)

	// Set up pipes
	stdin, stdout, stderr, err := e.setupPipes(cmd)
	if err != nil {
		return nil, err
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start exiftool: %w", err)
	}

	// Handle I/O concurrently
	outputBuffer, err := e.handleIOConcurrent(stdin, stdout, stderr, buffer)
	if err != nil {
		cmd.Process.Kill()
		return nil, err
	}

	// Wait for command completion
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("exiftool command failed: %w", err)
	}

	return e.parseExifToolOutput(outputBuffer.Bytes())
}

// buildExifToolArgs builds command line arguments for exiftool
func (e *Extractor) buildExifToolArgs(tags []string) []string {
	args := []string{"-j", "-charset", "utf8", "-fast", "-ignoreMinorErrors"}

	// Add specific tags
	for _, tag := range tags {
		args = append(args, "-"+tag)
	}

	// Read from stdin
	args = append(args, "-")

	return args
}

// setupPipes sets up stdin, stdout, and stderr pipes for the command
func (e *Extractor) setupPipes(cmd *exec.Cmd) (io.WriteCloser, io.ReadCloser, io.ReadCloser, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, nil, nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	return stdin, stdout, stderr, nil
}

// handleIOConcurrent handles I/O operations concurrently using goroutines
func (e *Extractor) handleIOConcurrent(stdin io.WriteCloser, stdout, stderr io.ReadCloser, buffer *bytes.Buffer) (*bytes.Buffer, error) {
	var outputBuffer, errorBuffer bytes.Buffer
	done := make(chan error, 3)

	// Write to stdin
	go func() {
		defer stdin.Close()
		_, err := io.Copy(stdin, buffer)
		done <- err
	}()

	// Read from stdout
	go func() {
		defer stdout.Close()
		_, err := io.Copy(&outputBuffer, stdout)
		done <- err
	}()

	// Read from stderr
	go func() {
		defer stderr.Close()
		_, err := io.Copy(&errorBuffer, stderr)
		done <- err
	}()

	// Wait for all I/O operations
	for i := 0; i < 3; i++ {
		if err := <-done; err != nil && err != io.EOF {
			return nil, fmt.Errorf("I/O error during exiftool execution: %w", err)
		}
	}

	// Ignore exiftool stderr; warnings are common even with exit code 0

	return &outputBuffer, nil
}

// parseExifToolOutput parses JSON output from exiftool
func (e *Extractor) parseExifToolOutput(output []byte) (map[string]string, error) {
	if len(output) == 0 {
		return make(map[string]string), nil
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse exiftool JSON output: %w", err)
	}

	if len(result) == 0 {
		return make(map[string]string), nil
	}

	// Convert to string map
	stringMap := make(map[string]string)
	for key, value := range result[0] {
		if value != nil {
			stringMap[key] = fmt.Sprintf("%v", value)
		}
	}

	return stringMap, nil
}

// Close cleans up resources
func (e *Extractor) Close() error {
	close(e.workerPool)
	return nil
}
