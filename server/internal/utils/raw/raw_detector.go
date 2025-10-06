package raw

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"server/internal/utils/file"
	"strings"
)

// RAWFormat represents different RAW file formats
type RAWFormat string

const (
	FormatCR2     RAWFormat = "CR2" // Canon
	FormatCR3     RAWFormat = "CR3" // Canon
	FormatNEF     RAWFormat = "NEF" // Nikon
	FormatARW     RAWFormat = "ARW" // Sony
	FormatDNG     RAWFormat = "DNG" // Adobe Digital Negative
	FormatORF     RAWFormat = "ORF" // Olympus
	FormatRW2     RAWFormat = "RW2" // Panasonic
	FormatPEF     RAWFormat = "PEF" // Pentax
	FormatRAF     RAWFormat = "RAF" // Fujifilm
	FormatMRW     RAWFormat = "MRW" // Minolta/Konica Minolta
	FormatSRW     RAWFormat = "SRW" // Samsung
	FormatRWL     RAWFormat = "RWL" // Leica
	FormatX3F     RAWFormat = "X3F" // Sigma
	FormatUnknown RAWFormat = "UNKNOWN"
)

// RAWMagicBytes contains magic bytes for different RAW formats
var RAWMagicBytes = map[RAWFormat][]byte{
	FormatCR2: []byte("CR\x02\x00"),
	FormatCR3: []byte("CR\x03\x00"),
	FormatNEF: []byte("FUJIFILMCCD-RAW"),
	FormatDNG: []byte("DNG "),
	FormatORF: []byte("OLYMP\x00"),
	FormatRW2: []byte("RW2"),
	FormatRAF: []byte("FUJIFILMCCD-RAW"),
	FormatX3F: []byte("FOVb"),
}

// RAWExtensions maps file extensions to RAW formats
var RAWExtensions = map[string]RAWFormat{
	".cr2": FormatCR2,
	".cr3": FormatCR3,
	".nef": FormatNEF,
	".arw": FormatARW,
	".dng": FormatDNG,
	".orf": FormatORF,
	".rw2": FormatRW2,
	".pef": FormatPEF,
	".raf": FormatRAF,
	".mrw": FormatMRW,
	".srw": FormatSRW,
	".rwl": FormatRWL,
	".x3f": FormatX3F,
}

// Detector handles RAW file detection and processing
type Detector struct {
	maxReadBytes int
}

// NewDetector creates a new RAW detector
func NewDetector() *Detector {
	return &Detector{
		maxReadBytes: 1024, // Read first 1KB for magic bytes detection
	}
}

// DetectionResult contains information about detected RAW file
type DetectionResult struct {
	IsRAW         bool
	Format        RAWFormat
	HasEmbedded   bool
	EmbeddedData  []byte
	EmbeddedSize  int
	PreviewOffset int64
	PreviewSize   int64
}

// IsRAWFile checks if a file is a RAW file based on filename
// This now uses the centralized file validator for consistency
func IsRAWFile(filename string) bool {
	return file.IsRAWFile(filename)
}

// GetRAWFormat returns the RAW format based on filename
func GetRAWFormat(filename string) RAWFormat {
	ext := strings.ToLower(filepath.Ext(filename))
	if format, exists := RAWExtensions[ext]; exists {
		return format
	}
	return FormatUnknown
}

// DetectRAW analyzes a file stream to determine if it's a RAW file and extract metadata
func (d *Detector) DetectRAW(reader io.Reader, filename string) (*DetectionResult, error) {
	result := &DetectionResult{
		Format: GetRAWFormat(filename),
	}

	// First check by extension
	if result.Format == FormatUnknown {
		result.IsRAW = false
		return result, nil
	}

	// Read initial bytes for magic byte detection
	headerBytes := make([]byte, d.maxReadBytes)
	n, err := io.ReadFull(reader, headerBytes)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return result, fmt.Errorf("failed to read file header: %w", err)
	}
	headerBytes = headerBytes[:n]

	// Verify with magic bytes if available
	if magicBytes, exists := RAWMagicBytes[result.Format]; exists {
		if !bytes.Contains(headerBytes, magicBytes) {
			// Extension suggests RAW but magic bytes don't match
			// Still consider it RAW (some formats have variations)
			result.IsRAW = true
		} else {
			result.IsRAW = true
		}
	} else {
		// No magic bytes defined, trust the extension
		result.IsRAW = true
	}

	if result.IsRAW {
		// Try to detect embedded preview
		d.detectEmbeddedPreview(headerBytes, result)
	}

	return result, nil
}

// detectEmbeddedPreview attempts to find embedded JPEG preview in RAW files
func (d *Detector) detectEmbeddedPreview(headerBytes []byte, result *DetectionResult) {
	// Look for JPEG SOI (Start of Image) marker
	jpegSOI := []byte{0xFF, 0xD8}
	jpegEOI := []byte{0xFF, 0xD9}

	// Find JPEG start
	soiIndex := bytes.Index(headerBytes, jpegSOI)
	if soiIndex == -1 {
		result.HasEmbedded = false
		return
	}

	// Find JPEG end (simple heuristic)
	eoiIndex := bytes.Index(headerBytes[soiIndex:], jpegEOI)
	if eoiIndex == -1 {
		// JPEG might extend beyond our header read
		result.HasEmbedded = true
		result.PreviewOffset = int64(soiIndex)
		// We'll need to read more to find the actual size
		return
	}

	// Found complete JPEG in header
	result.HasEmbedded = true
	result.PreviewOffset = int64(soiIndex)
	result.PreviewSize = int64(eoiIndex + 2) // +2 for EOI marker
	result.EmbeddedData = headerBytes[soiIndex : soiIndex+int(result.PreviewSize)]
	result.EmbeddedSize = len(result.EmbeddedData)
}

// ExtractEmbeddedPreview extracts embedded preview from RAW file
func (d *Detector) ExtractEmbeddedPreview(reader io.ReadSeeker, result *DetectionResult) ([]byte, error) {
	if !result.HasEmbedded {
		return nil, fmt.Errorf("no embedded preview found")
	}

	if len(result.EmbeddedData) > 0 {
		// Already extracted in detection phase
		return result.EmbeddedData, nil
	}

	// Seek to preview location
	_, err := reader.Seek(result.PreviewOffset, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek to preview offset: %w", err)
	}

	// Read preview data
	var previewData []byte
	if result.PreviewSize > 0 {
		// Known size
		previewData = make([]byte, result.PreviewSize)
		_, err = io.ReadFull(reader, previewData)
		if err != nil {
			return nil, fmt.Errorf("failed to read preview data: %w", err)
		}
	} else {
		// Unknown size - read until we find JPEG EOI
		buffer := make([]byte, 4096)
		jpegEOI := []byte{0xFF, 0xD9}

		for {
			n, err := reader.Read(buffer)
			if err != nil && err != io.EOF {
				return nil, fmt.Errorf("failed to read preview data: %w", err)
			}
			if n == 0 {
				break
			}

			previewData = append(previewData, buffer[:n]...)

			// Check for JPEG end marker
			if eoiIndex := bytes.Index(previewData, jpegEOI); eoiIndex != -1 {
				previewData = previewData[:eoiIndex+2]
				break
			}

			// Prevent infinite reading (max 10MB preview)
			if len(previewData) > 10*1024*1024 {
				return nil, fmt.Errorf("preview too large, aborting")
			}
		}
	}

	// Validate JPEG data
	if len(previewData) < 2 || previewData[0] != 0xFF || previewData[1] != 0xD8 {
		return nil, fmt.Errorf("invalid JPEG preview data")
	}

	return previewData, nil
}

// IsPreviewAcceptable checks if embedded preview meets quality/size requirements
func (d *Detector) IsPreviewAcceptable(previewData []byte, minWidth, minHeight int) (bool, error) {
	if len(previewData) < 10 {
		return false, nil
	}

	// Basic JPEG validation
	if previewData[0] != 0xFF || previewData[1] != 0xD8 {
		return false, fmt.Errorf("invalid JPEG data")
	}

	// For now, accept any valid JPEG preview
	// TODO: Add actual dimension checking using image decoder
	if len(previewData) > 50*1024 { // At least 50KB suggests reasonable quality
		return true, nil
	}

	return false, nil
}
