package imaging

import (
	"fmt"
	"image"
	"io"

	"github.com/davidbyttow/govips/v2/vips"
)

// RGBImage is a row-major, interleaved RGB uint8 tensor payload.
type RGBImage struct {
	Data       []byte
	Width      int
	Height     int
	Channels   int
	Layout     string
	DType      string
	ColorSpace string
}

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
	// PadWidth and PadHeight embed the resized image into a fixed canvas. When
	// set, the image is centered and padded with PadColor.
	PadWidth  int
	PadHeight int
	PadColor  *vips.Color

	// Format selects the encoder. If unset, defaults to WebP.
	Format vips.ImageType
	// Quality controls lossy encoder quality (1-100). 0 lets the encoder pick.
	Quality int
	// StripMetadata removes EXIF/XMP/IPTC from the encoded output.
	StripMetadata bool
	// NoProfile removes the embedded ICC colour profile.
	NoProfile bool
	// AutoRotate applies EXIF orientation during load. Only supported for
	// JPEG and TIFF sources; should be false for WebP and re-encoded images.
	AutoRotate bool
}

// thumbnailImportParams builds an ImportParams for the thumbnail load path.
// Only sets AutoRotate when requested, because many loaders (PNG, WebP, HEIF)
// don't support the property and will error if it's set at all.
func thumbnailImportParams(autoRotate bool) *vips.ImportParams {
	p := vips.NewImportParams()
	if autoRotate {
		p.AutoRotate.Set(true)
	}
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

// ProcessImageRGBStream reads an image from r, applies opts, and returns raw
// HWC RGB uint8 bytes instead of an encoded image container.
func ProcessImageRGBStream(r io.Reader, opts ProcessOptions) (*RGBImage, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}
	return ProcessImageRGBBytes(buf, opts)
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
	img, err := processImageRefFromBytes(buf, opts)
	if err != nil {
		return nil, err
	}
	defer img.Close()

	return encode(img, opts)
}

// ProcessImageRGBBytes is the buffer-input counterpart of ProcessImageRGBStream.
func ProcessImageRGBBytes(buf []byte, opts ProcessOptions) (*RGBImage, error) {
	img, err := processImageRefFromBytes(buf, opts)
	if err != nil {
		return nil, err
	}
	defer img.Close()

	return exportRGB(img)
}

func processImageRefFromBytes(buf []byte, opts ProcessOptions) (*vips.ImageRef, error) {
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

		img, err = vips.LoadThumbnailFromBuffer(buf, w, h, interest, size, thumbnailImportParams(opts.AutoRotate))
		if err != nil {
			return nil, fmt.Errorf("thumbnail load: %w", err)
		}
	} else {
		img, err = vips.NewImageFromBuffer(buf)
		if err != nil {
			return nil, fmt.Errorf("decode image: %w", err)
		}
		if opts.AutoRotate {
			if err := img.AutoRotate(); err != nil {
				img.Close()
				return nil, fmt.Errorf("autorotate: %w", err)
			}
		}
	}
	if opts.PadWidth > 0 && opts.PadHeight > 0 {
		padColor := opts.PadColor
		if padColor == nil {
			padColor = &vips.Color{R: 0, G: 0, B: 0}
		}
		left := (opts.PadWidth - img.Width()) / 2
		top := (opts.PadHeight - img.Height()) / 2
		if left < 0 || top < 0 {
			img.Close()
			return nil, fmt.Errorf("image %dx%d exceeds pad canvas %dx%d", img.Width(), img.Height(), opts.PadWidth, opts.PadHeight)
		}
		if err := img.EmbedBackground(left, top, opts.PadWidth, opts.PadHeight, padColor); err != nil {
			img.Close()
			return nil, fmt.Errorf("pad image: %w", err)
		}
	}

	return img, nil
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
//
// EXIF orientation is auto-applied only for JPEG and TIFF sources.
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

	params := thumbnailImportParams(shouldAutoRotate(srcBuf))

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

func exportRGB(img *vips.ImageRef) (*RGBImage, error) {
	goImg, err := img.ToGoImage()
	if err != nil {
		return nil, fmt.Errorf("export rgb image: %w", err)
	}

	bounds := goImg.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	data := make([]byte, width*height*3)
	write := 0

	switch src := goImg.(type) {
	case *image.NRGBA:
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			row := (y - bounds.Min.Y) * src.Stride
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				i := row + (x-bounds.Min.X)*4
				data[write] = src.Pix[i]
				data[write+1] = src.Pix[i+1]
				data[write+2] = src.Pix[i+2]
				write += 3
			}
		}
	case *image.Gray:
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			row := (y - bounds.Min.Y) * src.Stride
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				v := src.Pix[row+(x-bounds.Min.X)]
				data[write] = v
				data[write+1] = v
				data[write+2] = v
				write += 3
			}
		}
	default:
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				r, g, b, _ := goImg.At(x, y).RGBA()
				data[write] = uint8(r >> 8)
				data[write+1] = uint8(g >> 8)
				data[write+2] = uint8(b >> 8)
				write += 3
			}
		}
	}

	return &RGBImage{
		Data:       data,
		Width:      width,
		Height:     height,
		Channels:   3,
		Layout:     "HWC",
		DType:      "uint8",
		ColorSpace: "RGB",
	}, nil
}

// shouldAutoRotate returns true for image formats that carry EXIF orientation
// metadata. JPEG and TIFF are the formats with reliable orientation tags;
// WebP, PNG, HEIC/HEIF and others either don't use orientation or handle it
// differently.
func shouldAutoRotate(buf []byte) bool {
	if len(buf) < 4 {
		return false
	}

	// JPEG: 0xFF 0xD8
	if buf[0] == 0xFF && buf[1] == 0xD8 {
		return true
	}

	// TIFF: 0x49 0x49 0x2A 0x00 (little-endian) or 0x4D 0x4D 0x00 0x2A (big-endian)
	if (buf[0] == 0x49 && buf[1] == 0x49 && buf[2] == 0x2A && buf[3] == 0x00) ||
		(buf[0] == 0x4D && buf[1] == 0x4D && buf[2] == 0x00 && buf[3] == 0x2A) {
		return true
	}

	return false
}
