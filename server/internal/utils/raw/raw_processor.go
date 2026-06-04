package raw

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/davidbyttow/govips/v2/vips"

	"server/internal/utils/imaging"
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
	OutputFormat      vips.ImageType
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
		OutputFormat:      vips.ImageTypeJPEG,
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

// ProcessRAWFromPath processes a RAW file directly from disk, avoiding the
// extra read-all and temp-file rewrite used by the generic reader-based path.
func (p *Processor) ProcessRAWFromPath(ctx context.Context, fullPath, filename string) (*ProcessingResult, error) {
	start := time.Now()
	result := &ProcessingResult{
		Strategy: p.options.Strategy,
	}

	file, err := os.Open(fullPath)
	if err != nil {
		result.Error = fmt.Errorf("open RAW file: %w", err)
		return result, result.Error
	}
	defer file.Close()

	detection, err := p.detector.DetectRAW(file, filename)
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

	rotate := p.rawDisplayRotation(fullPath)

	if preview, err := p.librawProcessor.ExtractEmbeddedWithLibRawPath(ctx, fullPath); err == nil && len(preview) > 0 {
		valid, err := p.detector.IsPreviewAcceptable(preview, p.options.MinPreviewWidth, p.options.MinPreviewHeight)
		if err == nil && valid {
			preview, err = p.normalizeEmbeddedPreview(preview, rotate)
			if err != nil {
				log.Printf("LibRaw embedded preview normalization failed, falling back to other strategies: %v", err)
			} else {
				result.Strategy = StrategyEmbeddedPreview
				result.UsedEmbedded = true
				result.PreviewData = preview
				result.ProcessingTime = time.Since(start)
				p.populatePreviewDimensions(result, preview)
				return result, nil
			}
		} else {
			log.Printf("LibRaw embedded preview invalid or truncated, falling back to other strategies")
		}
	}

	strategy := p.chooseStrategy(detection)
	result.Strategy = strategy

	var previewData []byte

	switch strategy {
	case StrategyEmbeddedPreview:
		previewData, err = p.processEmbeddedPreviewFromPath(fullPath, detection, rotate)
		if err != nil {
			log.Printf("Embedded preview failed, falling back to full render: %v", err)
			previewData, err = p.processFullRenderFromPath(ctx, fullPath)
			result.UsedEmbedded = false
		} else {
			result.Strategy = StrategyEmbeddedPreview
			result.UsedEmbedded = true
		}
	case StrategyFullRender:
		previewData, err = p.processFullRenderFromPath(ctx, fullPath)
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
	p.populatePreviewDimensions(result, previewData)

	return result, nil
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

	// Try fast embedded preview via LibRaw first (more modern, better camera support).
	// Display rotation is applied only on the path-based ProcessRAWFromPath (the
	// production entry point); this reader-based variant leaves it to AutoRotate.
	if preview, err := p.librawProcessor.ExtractEmbeddedWithLibRaw(ctx, rawData, filename); err == nil && len(preview) > 0 {
		// Validate preview quality and completeness
		valid, err := p.detector.IsPreviewAcceptable(preview, p.options.MinPreviewWidth, p.options.MinPreviewHeight)
		if err == nil && valid {
			preview, err = p.normalizeEmbeddedPreview(preview, vips.Angle0)
			if err != nil {
				log.Printf("LibRaw embedded preview normalization failed, falling back to other strategies: %v", err)
			} else {
				result.Strategy = StrategyEmbeddedPreview
				result.UsedEmbedded = true
				result.PreviewData = preview
				result.ProcessingTime = time.Since(start)
				p.populatePreviewDimensions(result, preview)
				return result, nil
			}
		} else {
			log.Printf("LibRaw embedded preview invalid or truncated, falling back to other strategies")
		}
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
	p.populatePreviewDimensions(result, previewData)

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
	previewData, err := p.detector.ExtractEmbeddedPreview(bytes.NewReader(rawData), detection)
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

	processed, err := p.normalizeEmbeddedPreview(previewData, vips.Angle0)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize embedded preview: %w", err)
	}

	return processed, nil
}

func (p *Processor) processEmbeddedPreviewFromPath(fullPath string, detection *DetectionResult, rotate vips.Angle) ([]byte, error) {
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("open RAW file: %w", err)
	}
	defer file.Close()

	previewData, err := p.detector.ExtractEmbeddedPreview(file, detection)
	if err != nil {
		return nil, fmt.Errorf("failed to extract embedded preview: %w", err)
	}

	acceptable, err := p.detector.IsPreviewAcceptable(previewData, p.options.MinPreviewWidth, p.options.MinPreviewHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to validate preview: %w", err)
	}

	if !acceptable {
		return nil, fmt.Errorf("embedded preview does not meet quality requirements")
	}

	processed, err := p.normalizeEmbeddedPreview(previewData, rotate)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize embedded preview: %w", err)
	}

	return processed, nil
}

// rawFlipToAngle maps libraw's dcraw-style flip code to the clockwise rotation
// needed to bring a sensor-orientation preview to display orientation.
func rawFlipToAngle(flip int) vips.Angle {
	switch flip {
	case 3:
		return vips.Angle180
	case 5:
		return vips.Angle270 // EXIF 8 / "Rotate 270 CW"
	case 6:
		return vips.Angle90 // EXIF 6 / "Rotate 90 CW"
	default:
		return vips.Angle0
	}
}

// rawDisplayRotation reads the camera orientation from libraw and returns the
// rotation to bake into the embedded preview. Best-effort: if libraw can't read
// the flip, no rotation is applied rather than failing the whole thumbnail.
func (p *Processor) rawDisplayRotation(fullPath string) vips.Angle {
	flip, err := p.librawProcessor.RawFlipPath(fullPath)
	if err != nil {
		log.Printf("RAW flip unavailable for %s; leaving preview unrotated: %v", fullPath, err)
		return vips.Angle0
	}
	return rawFlipToAngle(flip)
}

// normalizeEmbeddedPreview re-encodes the embedded preview into the configured
// output format and bakes in the display rotation. The embedded JPEG is stored
// in sensor orientation with no EXIF orientation of its own, so the rotation
// (derived from the RAW container via libraw) must be applied explicitly here.
func (p *Processor) normalizeEmbeddedPreview(previewData []byte, rotate vips.Angle) ([]byte, error) {
	if rotate == vips.Angle0 && p.options.Quality >= 100 && p.options.OutputFormat == 0 {
		return previewData, nil
	}

	processed, err := imaging.ProcessImageBytes(previewData, imaging.ProcessOptions{
		Format:        p.options.OutputFormat,
		Quality:       p.options.Quality,
		StripMetadata: true,
		NoProfile:     true,
		Rotate:        rotate,
	})
	if err != nil {
		return nil, err
	}
	if len(processed) == 0 {
		return nil, fmt.Errorf("normalized embedded preview is empty")
	}

	return processed, nil
}

// encodeFullRender normalizes the libraw TIFF render into the configured output
// format (JPEG by default) so the result is compact and consistent with the
// embedded-preview path. libvips only decodes the TIFF here (a loader present on
// every platform); the RAW decode itself is done by libraw, so this works
// regardless of the libvips build's RAW loader support.
func (p *Processor) encodeFullRender(tiff []byte) ([]byte, error) {
	if len(tiff) == 0 {
		return nil, fmt.Errorf("libraw full render produced no output")
	}
	encoded, err := imaging.ProcessImageBytes(tiff, imaging.ProcessOptions{
		Format:        p.options.OutputFormat,
		Quality:       p.options.Quality,
		StripMetadata: true,
		NoProfile:     true,
	})
	if err != nil {
		return nil, fmt.Errorf("encode full render: %w", err)
	}
	if len(encoded) == 0 {
		return nil, fmt.Errorf("full render encode produced no output")
	}
	return encoded, nil
}

// processFullRender fully decodes RAW bytes via libraw (returning a TIFF) and
// re-encodes it. Used only when no acceptable embedded preview is available.
func (p *Processor) processFullRender(_ context.Context, rawData []byte, filename string) ([]byte, error) {
	tiff, err := p.librawProcessor.RenderToTIFF(rawData, filename)
	if err != nil {
		return nil, fmt.Errorf("libraw full render failed: %w", err)
	}
	return p.encodeFullRender(tiff)
}

// processFullRenderFromPath is the on-disk counterpart of processFullRender.
func (p *Processor) processFullRenderFromPath(_ context.Context, fullPath string) ([]byte, error) {
	tiff, err := p.librawProcessor.RenderToTIFFPath(fullPath)
	if err != nil {
		return nil, fmt.Errorf("libraw full render failed: %w", err)
	}
	return p.encodeFullRender(tiff)
}

func (p *Processor) populatePreviewDimensions(result *ProcessingResult, previewData []byte) {
	if len(previewData) == 0 {
		return
	}
	img, err := vips.NewImageFromBuffer(previewData)
	if err != nil {
		return
	}
	defer img.Close()
	result.Width = img.Width()
	result.Height = img.Height()
}

// GenerateThumbnails creates various sized thumbnails from the processed preview
func (p *Processor) GenerateThumbnails(previewData []byte, sizes map[string][2]int) (map[string][]byte, error) {
	if len(previewData) == 0 {
		return nil, fmt.Errorf("no preview data provided")
	}

	thumbnails := make(map[string][]byte)

	for sizeName, dimensions := range sizes {
		width, height := dimensions[0], dimensions[1]

		thumbnail, err := imaging.ProcessImageBytes(previewData, imaging.ProcessOptions{
			Width:         width,
			Height:        height,
			Crop:          true,
			Smart:         true,
			Format:        p.options.OutputFormat,
			Quality:       p.options.Quality,
			StripMetadata: true,
		})
		if err != nil {
			log.Printf("Failed to generate %s thumbnail: %v", sizeName, err)
			continue
		}

		thumbnails[sizeName] = thumbnail
	}

	return thumbnails, nil
}
