package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"server/internal/models"

	"github.com/rwcarlsen/goexif/exif"
)

func init() {
	// Register known manufacturers' makernote parsers
	exif.RegisterParsers()
}

// ExtractImageMetadata extracts EXIF metadata from an image file
func (p *ImageProcessor) ExtractImageMetadata(ctx context.Context, photoID string, storagePath string) (*models.PhotoMetadata, error) {
	// 1. Get the original image
	file, err := p.storage.Get(ctx, storagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get original image: %w", err)
	}
	defer file.Close()

	// 2. Read the image data
	imgData, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	// 3. Parse EXIF data
	x, err := exif.Decode(bytes.NewReader(imgData))
	if err != nil {
		// Not all images have EXIF data, so we'll create a basic metadata object
		photoUUID, parseErr := models.ParseUUID(photoID)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid photo ID: %w", parseErr)
		}
		return &models.PhotoMetadata{
			PhotoID: photoUUID,
		}, nil
	}

	// 4. Extract metadata fields
	photoUUID, err := models.ParseUUID(photoID)
	if err != nil {
		return nil, fmt.Errorf("invalid photo ID: %w", err)
	}

	metadata := &models.PhotoMetadata{
		PhotoID: photoUUID,
	}

	// Extract date and time
	if datetime, err := x.DateTime(); err == nil {
		metadata.TakenTime = &datetime
	}

	// Extract camera model
	if model, err := x.Get(exif.Model); err == nil {
		if str, err := model.StringVal(); err == nil {
			metadata.CameraModel = str
		}
	}

	// Extract lens model
	if lens, err := x.Get(exif.LensModel); err == nil {
		if str, err := lens.StringVal(); err == nil {
			metadata.LensModel = str
		}
	}

	// Extract exposure time
	if exposure, err := x.Get(exif.ExposureTime); err == nil {
		if str, err := exposure.StringVal(); err == nil {
			metadata.ExposureTime = str
		}
	}

	// Extract f-number
	if fnumber, err := x.Get(exif.FNumber); err == nil {
		if num, err := fnumber.Float(32); err == nil {
			metadata.FNumber = float32(num)
		}
	}

	// Extract ISO speed
	if iso, err := x.Get(exif.ISOSpeedRatings); err == nil {
		if val, err := iso.Int(0); err == nil {
			metadata.IsoSpeed = val
		}
	}

	// Extract GPS coordinates
	if lat, long, err := x.LatLong(); err == nil {
		metadata.GPSLatitude = lat
		metadata.GPSLongitude = long
	}

	return metadata, nil
}
