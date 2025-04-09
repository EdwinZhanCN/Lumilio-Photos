package utils

import (
	"context"
	"fmt"
	"github.com/barasher/go-exiftool"
	"os"
	"server/internal/models"
	"strconv"
	"time"
)

// ExtractImageMetadata extracts EXIF metadata from an image file
func (p *ImageProcessor) ExtractImageMetadata(ctx context.Context, photoID string, storagePath string) (*models.PhotoMetadata, error) {
	// 1. Start the ExifTool process
	et, err := exiftool.NewExiftool()
	if err != nil {
		photoUUID, err := models.ParseUUID(photoID)
		if err != nil {
			return nil, fmt.Errorf("invalid photo ID: %w", err)
		}

		metadata := &models.PhotoMetadata{
			PhotoID: photoUUID,
		}
		return metadata, fmt.Errorf("failed to intialize exiftool: %w", err)
	}
	defer et.Close()

	// 2. Extract metadata
	rootStoragePath := os.Getenv("STORAGE_PATH")
	metaData := et.ExtractMetadata(rootStoragePath + "/" + storagePath)

	// 3. Get the important metadata fields
	// taken_time, camera_model, lens_model, exposure_time, f_number, iso_speed, gps_latitude, gps_longitude

	photoUUID, err := models.ParseUUID(photoID)
	if err != nil {
		return nil, fmt.Errorf("invalid photo ID: %w", err)
	}

	metadata := &models.PhotoMetadata{
		PhotoID: photoUUID,
	}

	// Extract date and time
	if datetimeStr, err := metaData[0].GetString("DateTimeOriginal"); err == nil {
		datetime, err := time.Parse("2006:01:02 15:04:05", datetimeStr)
		if err == nil {
			metadata.TakenTime = &datetime
		}
	}

	// Extract camera model
	if cameraModelStr, err := metaData[0].GetString("Model"); err == nil {
		metadata.CameraModel = cameraModelStr
	}

	// Extract lens model
	if lensModelStr, err := metaData[0].GetString("Lens"); err == nil {
		metadata.LensModel = lensModelStr
	}

	// Extract exposure time
	if exposureTimeStr, err := metaData[0].GetString("ExposureTime"); err == nil {
		metadata.ExposureTime = exposureTimeStr
	}

	// Extract f-number
	if fNumberStr, err := metaData[0].GetString("FNumber"); err == nil {
		// Convert f-number to float32
		fNumber, err := strconv.ParseFloat(fNumberStr, 32)
		if err == nil {
			metadata.FNumber = float32(fNumber)
		}
	}

	// Extract ISO speed
	if isoSpeedStr, err := metaData[0].GetString("ISO"); err == nil {
		// Convert ISO speed to int
		isoSpeed, err := strconv.Atoi(isoSpeedStr)
		if err == nil {
			metadata.IsoSpeed = isoSpeed
		}
	}

	// Extract GPS coordinates
	if gpsLatitudeStr, err := metaData[0].GetString("GPSLatitude"); err == nil {
		// Convert GPS latitude to float64
		gpsLatitude, err := strconv.ParseFloat(gpsLatitudeStr, 64)
		if err == nil {
			metadata.GPSLatitude = gpsLatitude
		}
	}

	if gpsLongitudeStr, err := metaData[0].GetString("GPSLongitude"); err == nil {
		// Convert GPS longitude to float64
		gpsLongitude, err := strconv.ParseFloat(gpsLongitudeStr, 64)
		if err == nil {
			metadata.GPSLongitude = gpsLongitude
		}
	}

	return metadata, nil

}
