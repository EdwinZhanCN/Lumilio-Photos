package raw

/*
#cgo pkg-config: libraw_r

#include <libraw/libraw.h>
#include <stdlib.h>

// Extract embedded preview using libraw
static int extract_embedded_preview(const char* filename, unsigned char** out_data, size_t* out_size) {
    libraw_data_t* iprc = libraw_init(0);

    if (!iprc) {
        return 1;
    }

    int ret = libraw_open_file(iprc, filename);
    if (ret != LIBRAW_SUCCESS) {
        libraw_close(iprc);
        return 2;
    }

    // Extract embedded preview
    ret = libraw_unpack_thumb(iprc);
    if (ret != LIBRAW_SUCCESS) {
        libraw_close(iprc);
        return 3;
    }

    // Get thumbnail image
    libraw_processed_image_t* image = libraw_dcraw_make_mem_thumb(iprc, &ret);
    if (!image) {
        libraw_close(iprc);
        return 4;
    }

    // Copy data
    *out_size = image->data_size;
    *out_data = (unsigned char*)malloc(image->data_size);
    if (!*out_data) {
        libraw_dcraw_clear_mem(image);
        libraw_close(iprc);
        return 5;
    }

    memcpy(*out_data, image->data, image->data_size);

    // Clean up
    libraw_dcraw_clear_mem(image);
    libraw_close(iprc);

    return 0;
}

// Read the camera orientation as libraw's dcraw-style flip code (0,3,5,6). The
// embedded preview JPEG is stored in sensor (landscape) orientation and carries
// no EXIF orientation of its own, so callers need this to rotate it to display
// orientation. Only the header is parsed (no unpack/decode).
static int get_raw_flip(const char* filename, int* out_flip) {
    libraw_data_t* iprc = libraw_init(0);
    if (!iprc) {
        return 1;
    }

    int ret = libraw_open_file(iprc, filename);
    if (ret != LIBRAW_SUCCESS) {
        libraw_close(iprc);
        return 2;
    }

    *out_flip = iprc->sizes.flip;
    libraw_close(iprc);
    return 0;
}

// Full RAW render: demosaic to an 8-bit sRGB image and let libraw write it as a
// baseline TIFF to out_filename. We use TIFF (rather than a headerless bitmap,
// the old bug) because libvips has an in-memory TIFF loader on every platform,
// whereas its RAW loader support varies by build. Decoding is done by libraw, so
// this works anywhere libraw is present.
static int render_raw_to_tiff(const char* in_filename, const char* out_filename) {
    libraw_data_t* iprc = libraw_init(0);
    if (!iprc) {
        return 1; // Init failed
    }

    // 8-bit sRGB output with camera white balance for natural-looking colour.
    iprc->params.output_bps = 8;
    iprc->params.output_color = 1; // sRGB
    iprc->params.use_camera_wb = 1;
    iprc->params.output_tiff = 1;  // write TIFF instead of PPM

    int ret = libraw_open_file(iprc, in_filename);
    if (ret != LIBRAW_SUCCESS) {
        libraw_close(iprc);
        return 2; // Open failed
    }

    ret = libraw_unpack(iprc);
    if (ret != LIBRAW_SUCCESS) {
        libraw_close(iprc);
        return 3; // Unpack failed
    }

    ret = libraw_dcraw_process(iprc);
    if (ret != LIBRAW_SUCCESS) {
        libraw_close(iprc);
        return 4; // Process failed
    }

    ret = libraw_dcraw_ppm_tiff_writer(iprc, out_filename);
    libraw_close(iprc);
    if (ret != LIBRAW_SUCCESS) {
        return 5; // Write failed
    }

    return 0; // Success
}
*/
import "C"
import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"
)

// LibRawProcessor handles RAW processing using libraw library
type LibRawProcessor struct {
	options ProcessingOptions
}

// NewLibRawProcessor creates a new libraw-based processor
func NewLibRawProcessor(opts ProcessingOptions) *LibRawProcessor {
	return &LibRawProcessor{
		options: opts,
	}
}

// RenderToTIFFPath fully decodes a RAW file from disk with libraw and returns a
// baseline TIFF image. Used as the full-render fallback when no acceptable
// embedded preview is available; the TIFF is decodable by libvips on any platform.
func (p *LibRawProcessor) RenderToTIFFPath(fullPath string) ([]byte, error) {
	outFile, err := os.CreateTemp("", "libraw-render-*.tiff")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp render file: %w", err)
	}
	outPath := outFile.Name()
	outFile.Close()
	defer os.Remove(outPath)

	cIn := C.CString(fullPath)
	defer C.free(unsafe.Pointer(cIn))
	cOut := C.CString(outPath)
	defer C.free(unsafe.Pointer(cOut))

	ret := C.render_raw_to_tiff(cIn, cOut)
	if ret != 0 {
		return nil, fmt.Errorf("libraw full render failed with code %d", ret)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("read libraw render output: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("libraw full render produced no output")
	}

	return data, nil
}

// RenderToTIFF is the in-memory counterpart of RenderToTIFFPath. libraw needs a
// file path, so the bytes are staged to a temp file first.
func (p *LibRawProcessor) RenderToTIFF(rawData []byte, filename string) ([]byte, error) {
	ext := ".raw"
	if len(filename) > 0 {
		ext = getFileExtension(filename)
	}

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("libraw-*%s", ext))
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

	return p.RenderToTIFFPath(tmpFile.Name())
}

// RawFlipPath returns the camera orientation as libraw's dcraw-style flip code
// (0 = none, 3 = 180°, 5 = 270° CW, 6 = 90° CW). Used to rotate the embedded
// preview, which is stored unrotated and carries no EXIF orientation.
func (p *LibRawProcessor) RawFlipPath(fullPath string) (int, error) {
	cFilename := C.CString(fullPath)
	defer C.free(unsafe.Pointer(cFilename))

	var flip C.int
	ret := C.get_raw_flip(cFilename, &flip)
	if ret != 0 {
		return 0, fmt.Errorf("libraw read flip failed with code %d", ret)
	}
	return int(flip), nil
}

// ExtractEmbeddedWithLibRawPath extracts an embedded preview directly from the source file.
func (p *LibRawProcessor) ExtractEmbeddedWithLibRawPath(ctx context.Context, fullPath string) ([]byte, error) {
	return p.extractEmbeddedFile(ctx, fullPath)
}

// ExtractEmbeddedWithLibRaw extracts embedded preview using libraw
func (p *LibRawProcessor) ExtractEmbeddedWithLibRaw(ctx context.Context, rawData []byte, filename string) ([]byte, error) {
	ext := ""
	if len(filename) > 0 {
		ext = getFileExtension(filename)
	} else {
		ext = ".raw"
	}

	// Create temp file
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("libraw-*%s", ext))
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file for libraw: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpPath := tmpFile.Name()

	// Write RAW data to temp file
	if _, err := tmpFile.Write(rawData); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to write RAW data to temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temp RAW file: %w", err)
	}

	// Convert filename to C string
	cFilename := C.CString(tmpPath)
	defer C.free(unsafe.Pointer(cFilename))

	// Call libraw to extract embedded preview
	var outData *C.uchar
	var outSize C.size_t

	ret := C.extract_embedded_preview(cFilename, &outData, &outSize)
	if ret != 0 {
		return nil, fmt.Errorf("libraw extract preview failed with code %d", ret)
	}
	defer C.free(unsafe.Pointer(outData))

	// Convert C array to Go slice
	if outSize == 0 {
		return nil, fmt.Errorf("libraw produced no preview")
	}

	goData := C.GoBytes(unsafe.Pointer(outData), C.int(outSize))

	return goData, nil
}

func (p *LibRawProcessor) extractEmbeddedFile(_ context.Context, fullPath string) ([]byte, error) {
	cFilename := C.CString(fullPath)
	defer C.free(unsafe.Pointer(cFilename))

	var outData *C.uchar
	var outSize C.size_t

	ret := C.extract_embedded_preview(cFilename, &outData, &outSize)
	if ret != 0 {
		return nil, fmt.Errorf("libraw extract preview failed with code %d", ret)
	}
	defer C.free(unsafe.Pointer(outData))

	if outSize == 0 {
		return nil, fmt.Errorf("libraw produced no preview")
	}

	return C.GoBytes(unsafe.Pointer(outData), C.int(outSize)), nil
}

// getFileExtension returns the file extension including the dot
func getFileExtension(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return ".raw"
	}
	return ext
}
