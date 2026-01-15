package raw

import (
	"context"
	"fmt"
	"io"
	"log"
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

// Processor handles RAW file processing using LibRaw
type Processor struct {
	detector        *Detector
	librawProcessor *LibRawProcessor
	options         ProcessingOptions
}

// NewProcessor creates a new RAW processor
func NewProcessor(opts ProcessingOptions) *Processor {
	return &Processor{
		detector:        NewDetector(),
		librawProcessor: NewLibRawProcessor(opts),
		options:         opts,
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

	// Reset reader to beginning and load RAW data once
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		result.Error = fmt.Errorf("failed to reset reader: %w", err)
		return result, result.Error
	}
	rawData, err := io.ReadAll(reader)
	if err != nil {
		result.Error = fmt.Errorf("failed to read RAW: %w", err)
		return result, result.Error
	}

	// Try fast embedded preview via LibRaw first (more modern, better camera support)
	if preview, err := p.librawProcessor.ExtractEmbeddedWithLibRaw(ctx, rawData, filename); err == nil && len(preview) > 0 {
		result.Strategy = StrategyEmbeddedPreview
		result.UsedEmbedded = true
		result.PreviewData = preview
		result.ProcessingTime = time.Since(start)
		if img := bimg.NewImage(preview); img != nil {
			if size, err := img.Size(); err == nil {
				result.Width = size.Width
				result.Height = size.Height
			}
		}
		return result, nil
	}

	// Fallback to full LibRaw rendering
	strategy := p.chooseStrategy(detection)
	result.Strategy = strategy

	var previewData []byte

	switch strategy {
	case StrategyEmbeddedPreview:
		previewData, err = p.processEmbeddedPreview(rawData, detection)
		if err != nil {
			// Fallback to full render if embedded preview fails
			log.Printf("Embedded preview failed, falling back to full render: %v", err)
			previewData, err = p.processFullRender(ctx, rawData, filename)
			result.UsedEmbedded = false
		} else {
			result.UsedEmbedded = true
		}

	case StrategyFullRender:
		previewData, err = p.processFullRender(ctx, rawData, filename)
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
func (p *Processor) processEmbeddedPreview(rawData []byte, detection *DetectionResult) ([]byte, error) {
	// This is a fallback - we already tried LibRaw embedded preview in ProcessRAW
	// If we get here, it means the fast path failed, so try with full context
	previewData, err := p.detector.ExtractEmbeddedPreview(nil, detection)
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

// processFullRender performs full RAW decoding using LibRaw
func (p *Processor) processFullRender(ctx context.Context, rawData []byte, filename string) ([]byte, error) {
	// Create a timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, p.options.FullRenderTimeout)
	defer cancel()

	// Use LibRaw for full RAW rendering
	previewData, err := p.librawProcessor.ProcessWithLibRaw(timeoutCtx, rawData, filename)
	if err != nil {
		return nil, fmt.Errorf("LibRaw processing failed: %w", err)
	}

	if len(previewData) == 0 {
		return nil, fmt.Errorf("LibRaw produced no output")
	}

	return previewData, nil
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
