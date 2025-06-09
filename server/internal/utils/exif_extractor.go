package utils

import (
	"context"
	"fmt"
	"os"
	"server/internal/models"
	"strconv"
	"time"

	"github.com/barasher/go-exiftool"
)

// ExtractAssetMetadata extracts EXIF metadata from an image asset and returns asset-compatible metadata
func (p *ImageProcessor) ExtractAssetMetadata(ctx context.Context, assetID string, storagePath string) (models.PhotoSpecificMetadata, error) {
	// 1. Start the ExifTool process
	et, err := exiftool.NewExiftool()
	if err != nil {
		metadata := models.PhotoSpecificMetadata{
			CameraModel:  "",
			LensModel:    "",
			ExposureTime: "",
			FNumber:      0,
			IsoSpeed:     0,
			GPSLatitude:  0,
			GPSLongitude: 0,
		}
		return metadata, fmt.Errorf("failed to intialize exiftool: %w", err)
	}
	defer et.Close()

	// 2. Extract metadata
	rootStoragePath := os.Getenv("STORAGE_PATH")
	metaData := et.ExtractMetadata(rootStoragePath + "/" + storagePath)

	// 3. Initialize metadata structure
	metadata := models.PhotoSpecificMetadata{
		CameraModel:  "",
		LensModel:    "",
		ExposureTime: "",
		FNumber:      0,
		IsoSpeed:     0,
		GPSLatitude:  0,
		GPSLongitude: 0,
	}

	if len(metaData) == 0 {
		return metadata, nil
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

	// Extract GPS latitude
	if gpsLatStr, err := metaData[0].GetString("GPSLatitude"); err == nil {
		gpsLat, err := strconv.ParseFloat(gpsLatStr, 64)
		if err == nil {
			metadata.GPSLatitude = gpsLat
		}
	}

	// Extract GPS longitude
	if gpsLonStr, err := metaData[0].GetString("GPSLongitude"); err == nil {
		gpsLon, err := strconv.ParseFloat(gpsLonStr, 64)
		if err == nil {
			metadata.GPSLongitude = gpsLon
		}
	}

	return metadata, nil
}

// ExtractImageMetadata - Deprecated: Use ExtractAssetMetadata instead
func (p *ImageProcessor) ExtractImageMetadata(ctx context.Context, photoID string, storagePath string) (*models.PhotoSpecificMetadata, error) {
	metadata, err := p.ExtractAssetMetadata(ctx, photoID, storagePath)
	return &metadata, err
}
