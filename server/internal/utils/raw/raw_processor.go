package raw

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/h2non/bimg"
)

// ProcessingStrategy defines how to handle RAW files
type ProcessingStrategy int

const (
	// StrategyEmbeddedPreview uses embedded preview if available and acceptable
	StrategyEmbeddedPreview ProcessingStrategy = iota
	// StrategyFullRender performs full RAW decoding/rendering
	StrategyFullRender
	// StrategyAuto automatically chooses the best strategy
	StrategyAuto
)

// ProcessingOptions configures RAW processing
type ProcessingOptions struct {
	Strategy          ProcessingStrategy
	MinPreviewWidth   int   // Minimum acceptable preview width
	MinPreviewHeight  int   // Minimum acceptable preview height
	MaxPreviewSize    int64 // Maximum acceptable preview file size
	FullRenderTimeout time.Duration
	PreferEmbedded    bool // Prefer embedded preview over full render
	Quality           int  // JPEG quality for output (1-100)
	OutputFormat      bimg.ImageType
}

// DefaultProcessingOptions returns sensible defaults
func DefaultProcessingOptions() ProcessingOptions {
	return ProcessingOptions{
		Strategy:          StrategyAuto,
		MinPreviewWidth:   800,
		MinPreviewHeight:  600,
		MaxPreviewSize:    5 * 1024 * 1024, // 5MB
		FullRenderTimeout: 30 * time.Second,
		PreferEmbedded:    true,
		Quality:           90,
		OutputFormat:      bimg.JPEG,
	}
}

// Processor handles RAW file processing
type Processor struct {
	detector *Detector
	options  ProcessingOptions
}

// NewProcessor creates a new RAW processor
func NewProcessor(opts ProcessingOptions) *Processor {
	return &Processor{
		detector: NewDetector(),
		options:  opts,
	}
}

// ProcessingResult contains the result of RAW processing
type ProcessingResult struct {
	Strategy       ProcessingStrategy
	PreviewData    []byte
	ThumbnailData  []byte
	IsRAW          bool
	Format         RAWFormat
	UsedEmbedded   bool
	ProcessingTime time.Duration
	Width          int
	Height         int
	Error          error
}

// ProcessRAW processes a RAW file and returns preview/thumbnail data
func (p *Processor) ProcessRAW(ctx context.Context, reader io.ReadSeeker, filename string) (*ProcessingResult, error) {
	start := time.Now()
	result := &ProcessingResult{
		Strategy: p.options.Strategy,
	}

	// First, detect if this is a RAW file
	detection, err := p.detector.DetectRAW(reader, filename)
	if err != nil {
		result.Error = fmt.Errorf("failed to detect RAW: %w", err)
		return result, result.Error
	}

	result.IsRAW = detection.IsRAW
	result.Format = detection.Format

	if !detection.IsRAW {
		result.Error = fmt.Errorf("file is not a RAW format")
		return result, result.Error
	}

	// Reset reader to beginning
	_, err = reader.Seek(0, io.SeekStart)
	if err != nil {
		result.Error = fmt.Errorf("failed to reset reader: %w", err)
		return result, result.Error
	}

	// Choose processing strategy
	strategy := p.chooseStrategy(detection)
	result.Strategy = strategy

	var previewData []byte

	switch strategy {
	case StrategyEmbeddedPreview:
		previewData, err = p.processEmbeddedPreview(reader, detection)
		if err != nil {
			// Fallback to full render if embedded preview fails
			log.Printf("Embedded preview failed, falling back to full render: %v", err)
			previewData, err = p.processFullRender(ctx, reader, filename)
			result.UsedEmbedded = false
		} else {
			result.UsedEmbedded = true
		}

	case StrategyFullRender:
		previewData, err = p.processFullRender(ctx, reader, filename)
		result.UsedEmbedded = false

	default:
		result.Error = fmt.Errorf("unknown processing strategy: %d", strategy)
		return result, result.Error
	}

	if err != nil {
		result.Error = fmt.Errorf("failed to process RAW: %w", err)
		return result, result.Error
	}

	result.PreviewData = previewData
	result.ProcessingTime = time.Since(start)

	// Get image dimensions
	if len(previewData) > 0 {
		if img := bimg.NewImage(previewData); img != nil {
			if size, err := img.Size(); err == nil {
				result.Width = size.Width
				result.Height = size.Height
			}
		}
	}

	return result, nil
}

// chooseStrategy determines the best processing strategy
func (p *Processor) chooseStrategy(detection *DetectionResult) ProcessingStrategy {
	if p.options.Strategy != StrategyAuto {
		return p.options.Strategy
	}

	// Auto strategy logic
	if detection.HasEmbedded && p.options.PreferEmbedded {
		// Check if embedded preview meets requirements
		if detection.EmbeddedSize > 0 && int64(detection.EmbeddedSize) <= p.options.MaxPreviewSize {
			return StrategyEmbeddedPreview
		}
		// If we don't know the size yet, try embedded first
		if detection.EmbeddedSize == 0 {
			return StrategyEmbeddedPreview
		}
	}

	return StrategyFullRender
}

// processEmbeddedPreview extracts and validates embedded preview
func (p *Processor) processEmbeddedPreview(reader io.ReadSeeker, detection *DetectionResult) ([]byte, error) {
	previewData, err := p.detector.ExtractEmbeddedPreview(reader, detection)
	if err != nil {
		return nil, fmt.Errorf("failed to extract embedded preview: %w", err)
	}

	// Validate preview quality
	acceptable, err := p.detector.IsPreviewAcceptable(previewData, p.options.MinPreviewWidth, p.options.MinPreviewHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to validate preview: %w", err)
	}

	if !acceptable {
		return nil, fmt.Errorf("embedded preview does not meet quality requirements")
	}

	// Optionally re-compress to standardize quality
	if p.options.Quality < 100 {
		img := bimg.NewImage(previewData)
		processed, err := img.Process(bimg.Options{
			Quality: p.options.Quality,
			Type:    p.options.OutputFormat,
		})
		if err != nil {
			log.Printf("Failed to reprocess embedded preview, using original: %v", err)
			return previewData, nil
		}
		return processed, nil
	}

	return previewData, nil
}

// processFullRender performs full RAW decoding using external tools
func (p *Processor) processFullRender(ctx context.Context, reader io.ReadSeeker, filename string) ([]byte, error) {
	// Create a timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, p.options.FullRenderTimeout)
	defer cancel()

	// Reset reader to beginning
	_, err := reader.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to reset reader: %w", err)
	}

	// Read all data (we need it for external processing)
	rawData, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read RAW data: %w", err)
	}

	// Try different RAW processing tools in order of preference
	processors := []func(context.Context, []byte, string) ([]byte, error){
		p.processWithDcraw,
		p.processWithLibRaw,
		p.processWithImageMagick,
	}

	var lastErr error
	for _, processor := range processors {
		result, err := processor(timeoutCtx, rawData, filename)
		if err == nil {
			return result, nil
		}
		lastErr = err
		log.Printf("RAW processor failed, trying next: %v", err)
	}

	return nil, fmt.Errorf("all RAW processors failed, last error: %w", lastErr)
}

// processWithDcraw uses dcraw for RAW processing
func (p *Processor) processWithDcraw(ctx context.Context, rawData []byte, filename string) ([]byte, error) {
	// Check if dcraw is available
	if _, err := exec.LookPath("dcraw"); err != nil {
		return nil, fmt.Errorf("dcraw not found: %w", err)
	}

	// Write RAW data to a temp file because some dcraw builds don't accept '-' stdin
	tmpFile, err := os.CreateTemp("", "dcraw-*.raw")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file for dcraw: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(rawData); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to write RAW data to temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temp RAW file: %w", err)
	}

	// dcraw command: -c (stdout), -q 3 (high quality), -w (auto white balance)
	cmd := exec.CommandContext(ctx, "dcraw", "-c", "-q", "3", "-w", tmpFile.Name())

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("dcraw failed: %w, stderr: %s", err, stderr.String())
	}

	ppmData := stdout.Bytes()
	if len(ppmData) == 0 {
		return nil, fmt.Errorf("dcraw produced no output")
	}

	// Convert PPM to JPEG using bimg
	img := bimg.NewImage(ppmData)
	jpegData, err := img.Process(bimg.Options{
		Quality: p.options.Quality,
		Type:    bimg.JPEG,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to convert PPM to JPEG: %w", err)
	}

	return jpegData, nil
}

// processWithLibRaw uses libraw for RAW processing
func (p *Processor) processWithLibRaw(ctx context.Context, rawData []byte, filename string) ([]byte, error) {
	// Check if libraw tools are available
	if _, err := exec.LookPath("unprocessed_raw"); err != nil {
		return nil, fmt.Errorf("libraw tools not found: %w", err)
	}

	// This is a simplified implementation - in practice you might use libraw directly via cgo
	// Write RAW data to a temp file because some libraw tools don't accept '-' stdin
	tmpFile, err := os.CreateTemp("", "libraw-*.raw")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file for libraw: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(rawData); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to write RAW data to temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temp RAW file: %w", err)
	}

	// This is a simplified implementation - in practice you might use libraw directly via cgo
	cmd := exec.CommandContext(ctx, "unprocessed_raw", "-T", tmpFile.Name())

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("libraw failed: %w, stderr: %s", err, stderr.String())
	}

	tiffData := stdout.Bytes()
	if len(tiffData) == 0 {
		return nil, fmt.Errorf("libraw produced no output")
	}

	// Convert TIFF to JPEG using bimg
	img := bimg.NewImage(tiffData)
	jpegData, err := img.Process(bimg.Options{
		Quality: p.options.Quality,
		Type:    bimg.JPEG,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to convert TIFF to JPEG: %w", err)
	}

	return jpegData, nil
}

// processWithImageMagick uses ImageMagick for RAW processing
func (p *Processor) processWithImageMagick(ctx context.Context, rawData []byte, filename string) ([]byte, error) {
	// Check if convert is available
	if _, err := exec.LookPath("convert"); err != nil {
		return nil, fmt.Errorf("ImageMagick convert not found: %w", err)
	}

	// ImageMagick command to convert RAW to JPEG
	// Write RAW data to a temp file to avoid stdin '-' issues on some builds
	tmpFile, err := os.CreateTemp("", "imagemagick-*.raw")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file for ImageMagick: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(rawData); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to write RAW data to temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temp RAW file: %w", err)
	}
	cmd := exec.CommandContext(ctx, "convert", tmpFile.Name(), "-quality", fmt.Sprintf("%d", p.options.Quality), "jpeg:-")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ImageMagick failed: %w, stderr: %s", err, stderr.String())
	}

	jpegData := stdout.Bytes()
	if len(jpegData) == 0 {
		return nil, fmt.Errorf("ImageMagick produced no output")
	}

	return jpegData, nil
}

// GenerateThumbnails creates various sized thumbnails from the processed preview
func (p *Processor) GenerateThumbnails(previewData []byte, sizes map[string][2]int) (map[string][]byte, error) {
	if len(previewData) == 0 {
		return nil, fmt.Errorf("no preview data provided")
	}

	thumbnails := make(map[string][]byte)
	img := bimg.NewImage(previewData)

	for sizeName, dimensions := range sizes {
		width, height := dimensions[0], dimensions[1]

		thumbnail, err := img.Process(bimg.Options{
			Width:   width,
			Height:  height,
			Crop:    true,
			Gravity: bimg.GravitySmart,
			Quality: p.options.Quality,
			Type:    p.options.OutputFormat,
		})
		if err != nil {
			log.Printf("Failed to generate %s thumbnail: %v", sizeName, err)
			continue
		}

		thumbnails[sizeName] = thumbnail
	}

	return thumbnails, nil
}
