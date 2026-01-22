package raw

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"server/internal/utils/file"
	"strings"

	"github.com/h2non/bimg"
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
		// We use a stateful scanner to skip embedded thumbnails (e.g. in Exif)
		buffer := make([]byte, 4096)
		previewData = make([]byte, 0, 1024*1024)
		pos := 0

		for {
			n, err := reader.Read(buffer)
			if err != nil && err != io.EOF {
				return nil, fmt.Errorf("failed to read preview data: %w", err)
			}

			if n > 0 {
				previewData = append(previewData, buffer[:n]...)

				// Scan for EOI
				foundEOI := false
				for pos < len(previewData) {
					// Check SOI at start
					if pos == 0 {
						if len(previewData) < 2 {
							break // Need more data
						}
						if previewData[0] != 0xFF || previewData[1] != 0xD8 {
							return nil, fmt.Errorf("invalid JPEG SOI")
						}
						pos = 2
						continue
					}

					// Find next FF
					if previewData[pos] != 0xFF {
						idx := bytes.IndexByte(previewData[pos:], 0xFF)
						if idx == -1 {
							pos = len(previewData)
							break
						}
						pos += idx
					}

					// Check byte after FF
					if pos+1 >= len(previewData) {
						break // Need more data
					}

					marker := previewData[pos+1]

					if marker == 0xFF { // Padding
						pos++
						continue
					}
					if marker == 0x00 { // Stuffed
						pos += 2
						continue
					}

					if marker == 0xD9 { // EOI
						previewData = previewData[:pos+2]
						foundEOI = true
						break
					}

					if marker == 0xDA { // SOS
						// Skip header
						if pos+4 > len(previewData) {
							break
						}
						length := int(previewData[pos+2])<<8 | int(previewData[pos+3])
						if pos+2+length > len(previewData) {
							break // Need more data
						}
						pos += 2 + length

						// Scan entropy data
						for pos < len(previewData) {
							if previewData[pos] != 0xFF {
								idx := bytes.IndexByte(previewData[pos:], 0xFF)
								if idx == -1 {
									pos = len(previewData)
									break
								}
								pos += idx
							}

							if pos+1 >= len(previewData) {
								break // Need more data
							}

							next := previewData[pos+1]
							if next == 0x00 || (next >= 0xD0 && next <= 0xD7) || next == 0xFF {
								if next == 0xFF {
									pos++
								} else {
									pos += 2
								}
								continue
							}

							// Found a marker!
							break
						}
						continue
					}

					// Markers with no parameters
					if (marker >= 0xD0 && marker <= 0xD7) || marker == 0x01 {
						pos += 2
						continue
					}

					// Markers with parameters
					if pos+4 > len(previewData) {
						break
					}
					length := int(previewData[pos+2])<<8 | int(previewData[pos+3])
					if pos+2+length > len(previewData) {
						break // Need more data
					}
					pos += 2 + length
				}

				if foundEOI {
					break
				}
			}

			if err == io.EOF {
				return nil, fmt.Errorf("EOF reached without finding JPEG EOI")
			}

			// Prevent infinite reading (max 20MB preview)
			if len(previewData) > 20*1024*1024 {
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

	// Check for EOI marker (FF D9) to detect truncation
	// Use LastIndex to find the end of the image.
	lastEOI := bytes.LastIndex(previewData, []byte{0xFF, 0xD9})
	if lastEOI == -1 {
		return false, nil
	}

	// If the EOI is too far from the end, it might be an embedded thumbnail's EOI
	// while the main image is truncated.
	// We allow some padding (e.g. 4KB), but if it's huge, it's suspicious.
	if len(previewData)-lastEOI > 4096 {
		return false, nil
	}

	// Check dimensions using bimg
	img := bimg.NewImage(previewData)
	size, err := img.Size()
	if err != nil {
		return false, nil
	}

	if size.Width < minWidth || size.Height < minHeight {
		return false, nil
	}

	return true, nil
}
