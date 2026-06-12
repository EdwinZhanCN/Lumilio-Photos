package imaging

import (
	"fmt"

	"github.com/davidbyttow/govips/v2/vips"
)

// ResizeKernel selects the resampling kernel for ML decode helpers without
// leaking govips types to callers.
type ResizeKernel int

const (
	// KernelBilinear matches HF processors configured with `resample: bilinear`
	// (e.g. SigLIP).
	KernelBilinear ResizeKernel = iota
	// KernelBicubic matches HF processors configured with bicubic resampling
	// (e.g. CLIP/BioCLIP).
	KernelBicubic
)

func (k ResizeKernel) vipsKernel() vips.Kernel {
	if k == KernelBicubic {
		return vips.KernelCubic
	}
	return vips.KernelLinear
}

// DecodeRGBResizeExact decodes buf and resizes it to exactly width x height,
// ignoring the source aspect ratio (a single squash pass). This mirrors HF
// image processors with `do_center_crop=false` such as SigLIP, where the
// model's training-time preprocessing is one direct resize.
func DecodeRGBResizeExact(buf []byte, width, height int, kernel ResizeKernel) (*RGBImage, error) {
	img, err := decodeForML(buf)
	if err != nil {
		return nil, err
	}
	defer img.Close()

	if img.Width() != width || img.Height() != height {
		hScale := float64(width) / float64(img.Width())
		vScale := float64(height) / float64(img.Height())
		if err := img.ResizeWithVScale(hScale, vScale, kernel.vipsKernel()); err != nil {
			return nil, fmt.Errorf("ml exact resize: %w", err)
		}
	}
	return exportMLRGB(img, width, height)
}

// DecodeRGBShortestEdgeCenterCrop decodes buf, resizes the shortest edge to
// the crop target, then center-crops to width x height. This mirrors HF
// CLIP-style processors with `do_center_crop=true` such as BioCLIP.
func DecodeRGBShortestEdgeCenterCrop(buf []byte, width, height int, kernel ResizeKernel) (*RGBImage, error) {
	img, err := decodeForML(buf)
	if err != nil {
		return nil, err
	}
	defer img.Close()

	shortest := img.Width()
	if img.Height() < shortest {
		shortest = img.Height()
	}
	if shortest <= 0 {
		return nil, fmt.Errorf("ml crop: invalid source size %dx%d", img.Width(), img.Height())
	}
	target := width
	if height < target {
		target = height
	}
	if shortest != target {
		scale := float64(target) / float64(shortest)
		if err := img.Resize(scale, kernel.vipsKernel()); err != nil {
			return nil, fmt.Errorf("ml shortest-edge resize: %w", err)
		}
	}
	left := (img.Width() - width) / 2
	top := (img.Height() - height) / 2
	if left < 0 || top < 0 {
		return nil, fmt.Errorf("ml crop: resized image %dx%d smaller than crop %dx%d", img.Width(), img.Height(), width, height)
	}
	if err := img.ExtractArea(left, top, width, height); err != nil {
		return nil, fmt.Errorf("ml center crop: %w", err)
	}
	return exportMLRGB(img, width, height)
}

func decodeForML(buf []byte) (*vips.ImageRef, error) {
	if len(buf) == 0 {
		return nil, fmt.Errorf("empty image buffer")
	}
	img, err := vips.NewImageFromBuffer(buf)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	if img.Interpretation() != vips.InterpretationSRGB {
		if err := img.ToColorSpace(vips.InterpretationSRGB); err != nil {
			img.Close()
			return nil, fmt.Errorf("convert to srgb: %w", err)
		}
	}
	return img, nil
}

func exportMLRGB(img *vips.ImageRef, width, height int) (*RGBImage, error) {
	rgb, err := exportRGB(img)
	if err != nil {
		return nil, err
	}
	if rgb.Width != width || rgb.Height != height {
		return nil, fmt.Errorf("ml decode produced %dx%d, want %dx%d", rgb.Width, rgb.Height, width, height)
	}
	return rgb, nil
}
