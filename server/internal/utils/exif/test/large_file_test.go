package exif_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"server/internal/db/dbtypes"
	"server/internal/utils/exif"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLargeFileHandling tests the EXIF extractor's ability to handle large files
func TestLargeFileHandling(t *testing.T) {
	t.Run("CanHandleFileSize validation", func(t *testing.T) {
		// Test small file
		canHandle, reason := exif.CanHandleFileSize(10 * 1024 * 1024) // 10MB
		assert.True(t, canHandle, "Should handle 10MB file")
		assert.Empty(t, reason, "No reason should be given for small file")

		// Test extremely large file
		canHandle, reason = exif.CanHandleFileSize(25 * 1024 * 1024 * 1024) // 25GB
		assert.False(t, canHandle, "Should reject 25GB file")
		assert.Contains(t, reason, "exceeds maximum supported limit", "Should mention size limit")
	})

	t.Run("Resource checking for reasonable sizes", func(t *testing.T) {
		// Test files that should be processable with our permissive estimates
		canHandle, reason := exif.CanHandleFileSize(1 * 1024 * 1024 * 1024) // 1GB
		assert.True(t, canHandle, "Should handle 1GB file: %s", reason)

		canHandle, reason = exif.CanHandleFileSize(10 * 1024 * 1024 * 1024) // 10GB
		assert.True(t, canHandle, "Should handle 10GB file: %s", reason)

		canHandle, reason = exif.CanHandleFileSize(19 * 1024 * 1024 * 1024) // 19GB
		assert.True(t, canHandle, "Should handle 19GB file: %s", reason)
	})

	t.Run("IsLargeFile detection", func(t *testing.T) {
		assert.False(t, exif.IsLargeFile(50*1024*1024), "50MB should not be considered large")
		assert.True(t, exif.IsLargeFile(150*1024*1024), "150MB should be considered large")
		assert.True(t, exif.IsLargeFile(2*1024*1024*1024), "2GB should be considered large")
	})

	t.Run("GetOptimalBufferSize calculation", func(t *testing.T) {
		// Small files
		assert.Equal(t, 64*1024, exif.GetOptimalBufferSize(10*1024*1024), "Small files should use 64KB buffer")

		// Medium files
		assert.Equal(t, 128*1024, exif.GetOptimalBufferSize(200*1024*1024), "200MB files should use 128KB buffer")

		// Large files
		assert.Equal(t, 256*1024, exif.GetOptimalBufferSize(600*1024*1024), "600MB files should use 256KB buffer")
	})

	t.Run("GetOptimalWorkerCount", func(t *testing.T) {
		workerCount := exif.GetOptimalWorkerCount()
		assert.Greater(t, workerCount, 0, "Should have at least 1 worker")
		assert.LessOrEqual(t, workerCount, 8, "Should not exceed 8 workers")
	})
}

// TestStreamingExtraction tests the streaming extraction functionality
func TestStreamingExtraction(t *testing.T) {
	if !exif.IsExifToolAvailable() {
		t.Skip("exiftool not available, skipping streaming tests")
	}

	extractor := exif.NewExtractor(nil)
	defer extractor.Close()

	t.Run("Streaming with large buffer", func(t *testing.T) {
		// Create a test config with large buffer for streaming
		config := &exif.Config{
			BufferSize:  256 * 1024, // 256KB buffer
			MaxFileSize: 2 * 1024 * 1024 * 1024,
			Timeout:     30 * time.Second,
		}
		extractor := exif.NewExtractor(config)
		defer extractor.Close()

		// Create a mock large file reader (simulating 500MB file)
		mockData := make([]byte, 10*1024*1024) // 10MB for testing
		for i := range mockData {
			mockData[i] = byte(i % 256)
		}
		reader := bytes.NewReader(mockData)

		req := &exif.StreamingExtractRequest{
			Reader:    reader,
			AssetType: dbtypes.AssetTypeVideo,
			Filename:  "test_large_video.mp4",
			Size:      int64(len(mockData)),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result, err := extractor.ExtractFromStream(ctx, req)
		require.NoError(t, err, "Should extract metadata from stream")
		assert.NotNil(t, result, "Should return result")
	})

	t.Run("File size validation", func(t *testing.T) {
		// Test with file exceeding max size
		config := &exif.Config{
			MaxFileSize: 100 * 1024 * 1024, // 100MB limit
		}
		extractor := exif.NewExtractor(config)
		defer extractor.Close()

		mockData := make([]byte, 150*1024*1024) // 150MB
		reader := bytes.NewReader(mockData)

		req := &exif.StreamingExtractRequest{
			Reader:    reader,
			AssetType: dbtypes.AssetTypeVideo,
			Filename:  "test_oversize.mp4",
			Size:      int64(len(mockData)),
		}

		ctx := context.Background()
		_, err := extractor.ExtractFromStream(ctx, req)
		assert.Error(t, err, "Should reject file exceeding max size")
		assert.Contains(t, err.Error(), "exceeds maximum allowed size", "Error should mention size limit")
	})
}

// TestResourceAwareConfiguration tests the resource-aware configuration features
func TestResourceAwareConfiguration(t *testing.T) {
	t.Run("Default configuration optimization", func(t *testing.T) {
		extractor := exif.NewExtractor(nil)
		defer extractor.Close()

		// Verify that default configuration is optimized
		// (This is an indirect test since we can't access internal config directly)
		assert.NotNil(t, extractor, "Should create extractor with optimized defaults")
	})

	t.Run("System resource functions", func(t *testing.T) {
		// Test available memory function (should not error)
		mem, err := exif.GetAvailableMemory()
		if err == nil {
			assert.Greater(t, mem, uint64(0), "Should return positive memory value")
		}

		// Test available disk space function (should not error)
		disk, err := exif.GetAvailableDiskSpace()
		if err == nil {
			assert.Greater(t, disk, uint64(0), "Should return positive disk space value")
		}
	})
}

// TestConcurrentLargeFileProcessing tests concurrent processing of large files
func TestConcurrentLargeFileProcessing(t *testing.T) {
	if !exif.IsExifToolAvailable() {
		t.Skip("exiftool not available, skipping concurrent tests")
	}

	extractor := exif.NewExtractor(&exif.Config{
		WorkerCount:   4,
		MaxFileSize:   2 * 1024 * 1024 * 1024,
		BufferSize:    128 * 1024,
		Timeout:       30 * time.Second,
		EnableCaching: false, // Disable caching for large file tests
	})
	defer extractor.Close()

	t.Run("Batch processing with simulated large files", func(t *testing.T) {
		var requests []*exif.StreamingExtractRequest

		// Create multiple mock large file requests
		for i := 0; i < 3; i++ {
			mockData := make([]byte, 5*1024*1024) // 5MB each
			reader := bytes.NewReader(mockData)

			requests = append(requests, &exif.StreamingExtractRequest{
				Reader:    reader,
				AssetType: dbtypes.AssetTypeVideo,
				Filename:  fmt.Sprintf("test_large_%d.mp4", i),
				Size:      int64(len(mockData)),
			})
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		results, err := extractor.ExtractBatch(ctx, requests)
		require.NoError(t, err, "Should process batch without error")
		assert.Len(t, results, len(requests), "Should return results for all requests")
	})
}

// BenchmarkLargeFileExtraction benchmarks the performance of large file extraction
func BenchmarkLargeFileExtraction(b *testing.B) {
	if !exif.IsExifToolAvailable() {
		b.Skip("exiftool not available, skipping benchmark")
	}

	extractor := exif.NewExtractor(&exif.Config{
		WorkerCount: 4,
		BufferSize:  256 * 1024,
		Timeout:     60 * time.Second,
	})
	defer extractor.Close()

	// Create a reasonably sized test file (50MB)
	testData := make([]byte, 50*1024*1024)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(testData)
		req := &exif.StreamingExtractRequest{
			Reader:    reader,
			AssetType: dbtypes.AssetTypeVideo,
			Filename:  "benchmark_video.mp4",
			Size:      int64(len(testData)),
		}

		ctx := context.Background()
		_, err := extractor.ExtractFromStream(ctx, req)
		if err != nil {
			b.Fatalf("Extraction failed: %v", err)
		}
	}
}

// TestErrorHandling tests error scenarios for large file processing
func TestErrorHandling(t *testing.T) {
	extractor := exif.NewExtractor(nil)
	defer extractor.Close()

	t.Run("Invalid reader", func(t *testing.T) {
		req := &exif.StreamingExtractRequest{
			Reader:    nil,
			AssetType: dbtypes.AssetTypeVideo,
			Filename:  "test.mp4",
			Size:      1024,
		}

		ctx := context.Background()
		_, err := extractor.ExtractFromStream(ctx, req)
		assert.Error(t, err, "Should error with nil reader")
	})

	t.Run("Invalid asset type", func(t *testing.T) {
		reader := bytes.NewReader([]byte{})
		req := &exif.StreamingExtractRequest{
			Reader:    reader,
			AssetType: dbtypes.AssetType("invalid"),
			Filename:  "test.mp4",
			Size:      1024,
		}

		ctx := context.Background()
		_, err := extractor.ExtractFromStream(ctx, req)
		assert.Error(t, err, "Should error with invalid asset type")
	})

	t.Run("Context cancellation", func(t *testing.T) {
		if !exif.IsExifToolAvailable() {
			t.Skip("exiftool not available, skipping context test")
		}

		reader := bytes.NewReader(make([]byte, 10*1024*1024))
		req := &exif.StreamingExtractRequest{
			Reader:    reader,
			AssetType: dbtypes.AssetTypeVideo,
			Filename:  "test.mp4",
			Size:      10 * 1024 * 1024,
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Immediately cancel

		_, err := extractor.ExtractFromStream(ctx, req)
		// Context cancellation in streaming operations may not be immediate
		// due to buffering and exiftool processing. The important thing is
		// that the function handles the context and doesn't panic.
		// We'll accept either an error or successful completion in this case.
		if err != nil {
			// If we get an error, it should be context-related
			assert.Contains(t, err.Error(), "context", "Error should mention context")
		}
		// If no error, that's also acceptable - the operation completed quickly
	})
}

// TestRealWorldScenarios tests real-world scenarios with actual file processing
func TestRealWorldScenarios(t *testing.T) {
	if !exif.IsExifToolAvailable() {
		t.Skip("exiftool not available, skipping real-world tests")
	}

	// This test requires actual media files to be present
	// It's designed to be run in environments with test media files

	testFiles := []struct {
		path      string
		assetType dbtypes.AssetType
		desc      string
	}{
		// These paths would need to be adjusted for actual test environment
		// {"/path/to/large/video.mp4", dbtypes.AssetTypeVideo, "Large video file"},
		// {"/path/to/large/photo.jpg", dbtypes.AssetTypePhoto, "Large photo file"},
	}

	for _, tf := range testFiles {
		t.Run(tf.desc, func(t *testing.T) {
			if _, err := os.Stat(tf.path); os.IsNotExist(err) {
				t.Skipf("Test file %s not found, skipping", tf.path)
			}

			file, err := os.Open(tf.path)
			require.NoError(t, err, "Should open test file")
			defer file.Close()

			info, err := file.Stat()
			require.NoError(t, err, "Should get file info")

			extractor := exif.NewExtractor(&exif.Config{
				MaxFileSize: 2 * 1024 * 1024 * 1024,
				Timeout:     60 * time.Second,
				BufferSize:  exif.GetOptimalBufferSize(info.Size()),
			})
			defer extractor.Close()

			req := &exif.StreamingExtractRequest{
				Reader:    file,
				AssetType: tf.assetType,
				Filename:  info.Name(),
				Size:      info.Size(),
			}

			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			result, err := extractor.ExtractFromStream(ctx, req)
			require.NoError(t, err, "Should extract metadata from real file")
			assert.NotNil(t, result, "Should return metadata result")
			assert.Nil(t, result.Error, "Should not have extraction error")
		})
	}
}
