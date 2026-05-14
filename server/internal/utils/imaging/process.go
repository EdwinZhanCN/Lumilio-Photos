package imaging

import (
	"fmt"
	"io"

	"github.com/davidbyttow/govips/v2/vips"
)

// ProcessOptions describes a single image-processing pass: resize/crop target
// dimensions, encode format, and quality knobs. It is the package-local option
// type so callers don't have to depend on govips/vips directly.
type ProcessOptions struct {
	// Width and Height define the maximum bounding box. When Crop is false the
	// aspect ratio is preserved (libvips thumbnail "fit"). When Crop is true a
	// center/smart crop is performed to fill the box.
	Width  int
	Height int
	// Crop enables a Smartcrop-style fill behavior (Attention if Smart, otherwise
	// centre).
	Crop bool
	// Smart selects libvips' attention-based focus point for the crop. Ignored
	// when Crop is false.
	Smart bool
	// Enlarge allows upscaling. Default false (libvips SizeDown).
	Enlarge bool

	// Format selects the encoder. If unset, defaults to WebP.
	Format vips.ImageType
	// Quality controls lossy encoder quality (1-100). 0 lets the encoder pick.
	Quality int
	// StripMetadata removes EXIF/XMP/IPTC from the encoded output.
	StripMetadata bool
	// NoProfile removes the embedded ICC colour profile.
	NoProfile bool
}

// thumbnailImportParams builds an ImportParams that enables EXIF autorotation
// during load. We always want this so downstream encoders see upright pixels
// and orientation metadata is normalized away.
func thumbnailImportParams() *vips.ImportParams {
	p := vips.NewImportParams()
	p.AutoRotate.Set(true)
	return p
}

// ProcessImageStream reads an image from r, applies opts, and returns the
// encoded bytes. EXIF orientation is baked in during the thumbnail load so
// callers don't need a separate autorotate step.
func ProcessImageStream(r io.Reader, opts ProcessOptions) ([]byte, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}
	return ProcessImageBytes(buf, opts)
}

// ProcessImageBytes is the buffer-input counterpart of ProcessImageStream.
//
// The hot path uses libvips' "thumbnail_buffer" op (via LoadThumbnailFromBuffer)
// which performs shrink-on-load: JPEG / HEIC decoders skip high-frequency DCT
// coefficients to produce a buffer close to the target resolution before any
// further work, which is dramatically cheaper than full-resolution decode +
// pixel-space resample.
//
// When no resize is requested (Width=Height=0) we fall back to a plain
// NewImageFromBuffer + AutoRotate so encode-only callers still work.
func ProcessImageBytes(buf []byte, opts ProcessOptions) ([]byte, error) {
	if len(buf) == 0 {
		return nil, fmt.Errorf("empty image buffer")
	}

	w := opts.Width
	h := opts.Height
	if w == 0 {
		w = h
	}
	if h == 0 {
		h = w
	}

	var img *vips.ImageRef
	var err error

	if w > 0 && h > 0 {
		interest := vips.InterestingNone
		if opts.Crop {
			if opts.Smart {
				interest = vips.InterestingAttention
			} else {
				interest = vips.InterestingCentre
			}
		}

		size := vips.SizeDown
		if opts.Enlarge {
			size = vips.SizeBoth
		}

		img, err = vips.LoadThumbnailFromBuffer(buf, w, h, interest, size, thumbnailImportParams())
		if err != nil {
			return nil, fmt.Errorf("thumbnail load: %w", err)
		}
	} else {
		img, err = vips.NewImageFromBuffer(buf)
		if err != nil {
			return nil, fmt.Errorf("decode image: %w", err)
		}
		if err := img.AutoRotate(); err != nil {
			img.Close()
			return nil, fmt.Errorf("autorotate: %w", err)
		}
	}
	defer img.Close()

	return encode(img, opts)
}

// StreamThumbnails reads a single source image from r and encodes one
// thumbnail per entry in sizes. Each entry is the maximum (width, height)
// bounding box. Each size goes through the libvips thumbnail_buffer op
// independently so we get shrink-on-load for every output.
//
// We deliberately do NOT pre-decode the source into a shared ImageRef: that
// path would force a full-resolution pixel buffer and a Copy() per size, which
// is much more expensive than letting libvips decode straight to the target
// scale.
func StreamThumbnails(
	r io.Reader,
	sizes map[string][2]int,
	outputs map[string]io.Writer,
) error {
	srcBuf, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read source image: %w", err)
	}
	if len(srcBuf) == 0 {
		return fmt.Errorf("empty source image")
	}

	params := thumbnailImportParams()

	for name, dim := range sizes {
		out, ok := outputs[name]
		if !ok {
			return fmt.Errorf("missing writer for size %q", name)
		}

		thumb, err := vips.LoadThumbnailFromBuffer(
			srcBuf,
			dim[0], dim[1],
			vips.InterestingNone,
			vips.SizeDown,
			params,
		)
		if err != nil {
			return fmt.Errorf("[%s] thumbnail load: %w", name, err)
		}

		encoded, encErr := encode(thumb, ProcessOptions{
			Format:        vips.ImageTypeWEBP,
			Quality:       80,
			StripMetadata: true,
			NoProfile:     true,
		})
		thumb.Close()
		if encErr != nil {
			return fmt.Errorf("[%s] encode: %w", name, encErr)
		}
		if _, err := out.Write(encoded); err != nil {
			return fmt.Errorf("[%s] write: %w", name, err)
		}
	}
	return nil
}

// encode writes the in-memory ImageRef to bytes in the requested format. Metadata
// and ICC profiles are stripped according to opts to keep thumbnail output
// browser-friendly and small.
func encode(img *vips.ImageRef, opts ProcessOptions) ([]byte, error) {
	if opts.NoProfile {
		_ = img.RemoveICCProfile()
	}

	format := opts.Format
	if format == 0 {
		format = vips.ImageTypeWEBP
	}

	switch format {
	case vips.ImageTypeWEBP:
		params := vips.NewWebpExportParams()
		if opts.Quality > 0 {
			params.Quality = opts.Quality
		}
		params.StripMetadata = opts.StripMetadata
		out, _, err := img.ExportWebp(params)
		if err != nil {
			return nil, fmt.Errorf("export webp: %w", err)
		}
		return out, nil

	case vips.ImageTypeJPEG:
		params := vips.NewJpegExportParams()
		if opts.Quality > 0 {
			params.Quality = opts.Quality
		}
		params.StripMetadata = opts.StripMetadata
		out, _, err := img.ExportJpeg(params)
		if err != nil {
			return nil, fmt.Errorf("export jpeg: %w", err)
		}
		return out, nil

	case vips.ImageTypePNG:
		params := vips.NewPngExportParams()
		params.StripMetadata = opts.StripMetadata
		out, _, err := img.ExportPng(params)
		if err != nil {
			return nil, fmt.Errorf("export png: %w", err)
		}
		return out, nil

	default:
		return nil, fmt.Errorf("unsupported output format: %v", format)
	}
}
