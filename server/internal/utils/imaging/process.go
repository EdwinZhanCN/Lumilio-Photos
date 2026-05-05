package imaging

import (
	"fmt"
	"io"
	"math"
	"sync"

	"github.com/h2non/bimg"
)

// libvips/libexif can deadlock under concurrent EXIF metadata parsing on macOS.
// Keep image transforms serialized in-process; thumbnail queues provide the
// outer throughput control.
var bimgMu sync.Mutex

// ProcessImageStream reads an image from the provided io.Reader, processes it using bimg
func ProcessImageStream(r io.Reader, opts bimg.Options) ([]byte, error) {
	// Read entire input into buffer
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	bimgMu.Lock()
	defer bimgMu.Unlock()

	// Initialize bimg image
	img := bimg.NewImage(buf)

	// Perform processing with given options
	newBuf, err := img.Process(opts)
	if err != nil {
		return nil, err
	}

	return newBuf, nil
}

func StreamThumbnails(
	r io.Reader,
	sizes map[string][2]int,
	outputs map[string]io.Writer,
) error {
	srcBuf, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read source image: %w", err)
	}

	bimgMu.Lock()
	defer bimgMu.Unlock()

	size, err := bimg.NewImage(srcBuf).Size()
	if err != nil {
		return fmt.Errorf("get source image size: %w", err)
	}
	if size.Width == 0 || size.Height == 0 {
		return fmt.Errorf("invalid source image size")
	}

	for name, dim := range sizes {
		w, ok := outputs[name]
		if !ok {
			return fmt.Errorf("missing writer for size %q", name)
		}

		// Calculate scale to fit within max dimensions while preserving aspect ratio
		scaleW := float64(dim[0]) / float64(size.Width)
		scaleH := float64(dim[1]) / float64(size.Height)
		scale := math.Min(scaleW, scaleH)
		if scale > 1 {
			scale = 1
		}
		targetWidth := int(float64(size.Width) * scale)
		if targetWidth < 1 {
			targetWidth = 1
		}

		opts := bimg.Options{
			Width:         targetWidth,
			Crop:          false,
			Quality:       80,
			Type:          bimg.WEBP,
			Enlarge:       false,
			NoProfile:     true,
			StripMetadata: true,
		}
		thumb, err := bimg.NewImage(srcBuf).Process(opts)
		if err != nil {
			return fmt.Errorf("[%s] process: %w", name, err)
		}
		if _, err := w.Write(thumb); err != nil {
			return fmt.Errorf("[%s] write: %w", name, err)
		}
	}
	return nil
}

// ProcessImageBytes processes raw image bytes with bimg options and returns the result.
func ProcessImageBytes(buf []byte, opts bimg.Options) ([]byte, error) {
	bimgMu.Lock()
	defer bimgMu.Unlock()

	img := bimg.NewImage(buf)
	return img.Process(opts)
}
