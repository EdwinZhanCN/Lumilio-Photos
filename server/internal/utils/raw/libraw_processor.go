package raw

/*
#cgo pkg-config: libraw
#cgo CFLAGS: -I/usr/include/libraw
#cgo LDFLAGS: -lraw -lstdc++

#include <libraw/libraw.h>
#include <stdlib.h>

// Helper function to process RAW file and return JPEG data
static int process_raw_to_jpeg(const char* filename, unsigned char** out_data, size_t* out_size, int quality) {
    libraw_data_t* iprc = libraw_init(0);

    if (!iprc) {
        return 1; // Init failed
    }

    // Open the file
    int ret = libraw_open_file(iprc, filename);
    if (ret != LIBRAW_SUCCESS) {
        libraw_close(iprc);
        return 2; // Open failed
    }

    // Unpack the data
    ret = libraw_unpack(iprc);
    if (ret != LIBRAW_SUCCESS) {
        libraw_close(iprc);
        return 3; // Unpack failed
    }

    // Apply demosaicing and produce a readable image
    ret = libraw_dcraw_process(iprc);
    if (ret != LIBRAW_SUCCESS) {
        libraw_close(iprc);
        return 4; // Process failed
    }

    // Get the processed image
    libraw_processed_image_t* image = libraw_dcraw_make_mem_image(iprc, &ret);
    if (!image) {
        libraw_close(iprc);
        return 5; // Make mem image failed
    }

    // Copy the image data to output
    *out_size = image->data_size;
    *out_data = (unsigned char*)malloc(image->data_size);
    if (!*out_data) {
        libraw_dcraw_clear_mem(image);
        libraw_close(iprc);
        return 6; // Malloc failed
    }

    memcpy(*out_data, image->data, image->data_size);

    // Clean up
    libraw_dcraw_clear_mem(image);
    libraw_close(iprc);

    return 0; // Success
}

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

// ProcessWithLibRaw processes RAW file using libraw
func (p *LibRawProcessor) ProcessWithLibRaw(ctx context.Context, rawData []byte, filename string) ([]byte, error) {
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

	// Call libraw to process
	var outData *C.uchar
	var outSize C.size_t

	ret := C.process_raw_to_jpeg(cFilename, &outData, &outSize, C.int(p.options.Quality))
	if ret != 0 {
		return nil, fmt.Errorf("libraw failed with code %d", ret)
	}
	defer C.free(unsafe.Pointer(outData))

	// Convert C array to Go slice
	if outSize == 0 {
		return nil, fmt.Errorf("libraw produced no output")
	}

	goData := C.GoBytes(unsafe.Pointer(outData), C.int(outSize))

	return goData, nil
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

// getFileExtension returns the file extension including the dot
func getFileExtension(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return ".raw"
	}
	return ext
}
