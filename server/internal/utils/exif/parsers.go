package exif

import (
	"fmt"
	"server/internal/db/dbtypes"
	"strconv"
	"strings"
)

// Field priority definitions
// Higher priority fields are checked first
var (
	// TakenTime priority fields - from most specific to most generic
	takenTimeFields = []string{
		"DateTimeOriginal",       // Original capture time - highest priority
		"CreateDate",             // File creation time
		"DateTime",               // General datetime
		"ModifyDate",             // Last modification time
		"FileModifyDate",         // File system modification time
		"DateTimeDigitized",      // Digitization time
		"SubSecDateTimeOriginal", // High precision original time
		"GPSDateTime",            // GPS timestamp
	}

	// CameraModel priority fields - from specific to generic
	cameraModelFields = []string{
		"Model",             // Standard camera model field
		"CameraModelName",   // More specific model name
		"UniqueCameraModel", // Unique model identifier
	}

	// LensModel priority fields
	lensModelFields = []string{
		"LensModel", // Standard lens model
		"LensID",    // Lens identifier
		"LensInfo",  // Lens information
		"LensType",  // Lens type
		"Lens",      // Generic lens field
	}

	// ExposureTime priority fields
	exposureTimeFields = []string{
		"ExposureTime",      // Direct exposure time
		"ShutterSpeedValue", // Shutter speed value
		"ShutterSpeed",      // Generic shutter speed
	}

	// FNumber priority fields
	fNumberFields = []string{
		"FNumber",       // Standard f-number
		"Aperture",      // Generic aperture
		"ApertureValue", // Aperture value
	}

	// ISO priority fields
	isoFields = []string{
		"ISO",                      // Standard ISO
		"ISOSpeedRatings",          // ISO speed ratings
		"RecommendedExposureIndex", // Recommended exposure index
	}

	// FocalLength priority fields
	focalLengthFields = []string{
		"FocalLength",             // Standard focal length
		"FocalLengthIn35mmFilm",   // 35mm equivalent
		"FocalLengthIn35mmFormat", // 35mm format equivalent
	}

	// Description priority fields
	descriptionFields = []string{
		"ImageDescription", // Specific image description
		"UserComment",      // User comment
		"XPComment",        // Windows XP comment
		"Caption",          // Caption field
		"Description",      // Generic description
	}

	// Exposure bias priority fields
	exposureBiasFields = []string{
		"ExposureCompensation", // Exposure compensation
	}

	// Codec priority fields for videos
	videoCodecFields = []string{
		"VideoCodec",   // Video-specific codec
		"CompressorID", // Compressor identifier
		"AudioCodec",   // Audio codec (fallback)
		"VideoFormat",  // Video format
	}

	// Bitrate priority fields for videos
	videoBitrateFields = []string{
		"VideoBitrate",   // Video-specific bitrate
		"Bitrate",        // General bitrate
		"AudioBitrate",   // Audio bitrate (fallback)
		"OverallBitrate", // Overall bitrate
	}

	// FrameRate priority fields
	frameRateFields = []string{
		"VideoFrameRate",   // Video-specific frame rate
		"FrameRate",        // General frame rate
		"NominalFrameRate", // Nominal frame rate
	}

	// RecordedTime priority fields for videos
	recordedTimeFields = []string{
		"CreateDate",        // Creation date
		"DateTimeOriginal",  // Original datetime
		"MediaCreateDate",   // Media creation date
		"TrackCreateDate",   // Track creation date
		"ModifyDate",        // Modification date
		"FileModifyDate",    // File system modification date
		"DateTimeDigitized", // Digitization date
	}

	// VideoCameraModel priority fields
	videoCameraModelFields = []string{
		"Model",           // Standard model
		"Make",            // Make of the device
		"CameraModelName", // Camera model name
		"RecorderModel",   // Recorder model (for videos)
	}

	// VideoDescription priority fields
	videoDescriptionFields = []string{
		"Description", // Generic description
		"Comment",     // Comment
		"Title",       // Title
		"Synopsis",    // Synopsis
	}

	// AudioCodec priority fields
	audioCodecFields = []string{
		"AudioCodec",        // Standard audio codec
		"AudioFormat",       // Audio format
		"FileTypeExtension", // File extension
		"AudioEncoding",     // Audio encoding
	}

	// AudioBitrate priority fields
	audioBitrateFields = []string{
		"AudioBitrate",   // Audio-specific bitrate
		"Bitrate",        // General bitrate
		"NominalBitrate", // Nominal bitrate
	}

	// SampleRate priority fields
	sampleRateFields = []string{
		"SampleRate",      // Standard sample rate
		"AudioSampleRate", // Audio-specific sample rate
		"SamplingRate",    // Generic sampling rate
	}

	// Channels priority fields
	channelsFields = []string{
		"AudioChannels", // Audio-specific channels
		"Channels",      // General channels
		"ChannelCount",  // Channel count
	}

	// Artist priority fields
	artistFields = []string{
		"Artist",      // Standard artist field
		"AlbumArtist", // Album artist
		"Performer",   // Performer
		"Author",      // Author
	}

	// Album priority fields
	albumFields = []string{
		"Album",      // Standard album
		"AlbumTitle", // Album title
	}

	// Title priority fields for audio
	audioTitleFields = []string{
		"Title",      // Standard title
		"SongTitle",  // Song title
		"TrackTitle", // Track title
	}

	// Genre priority fields
	genreFields = []string{
		"Genre",       // Standard genre
		"ContentType", // Content type
	}

	// Year priority fields
	yearFields = []string{
		"Year",          // Standard year
		"Date",          // Date field
		"ReleaseDate",   // Release date
		"RecordingDate", // Recording date
	}

	// AudioDescription priority fields
	audioDescriptionFields = []string{
		"Comment",     // Comment
		"Description", // Description
		"Lyrics",      // Lyrics
		"Synopsis",    // Synopsis
	}
)

// parsePhotoMetadata parses raw EXIF data into PhotoSpecificMetadata
func parsePhotoMetadata(rawData map[string]string) *dbtypes.PhotoSpecificMetadata {
	metadata := &dbtypes.PhotoSpecificMetadata{}

	// Parse TakenTime using priority-based field list
	for _, field := range takenTimeFields {
		if dateStr, exists := rawData[field]; exists && dateStr != "" {
			if parsedTime, err := parseDateTime(dateStr); err == nil {
				metadata.TakenTime = &parsedTime
				break
			}
		}
	}

	// Parse CameraModel using priority-based field list
	for _, field := range cameraModelFields {
		if model, exists := rawData[field]; exists {
			normalized := normalizeString(model)
			if normalized != "" {
				metadata.CameraModel = normalized
				break
			}
		}
	}

	// Parse LensModel using priority-based field list
	for _, field := range lensModelFields {
		if lens, exists := rawData[field]; exists {
			normalized := normalizeString(lens)
			if normalized != "" {
				metadata.LensModel = normalized
				break
			}
		}
	}

	// Parse ExposureTime using priority-based field list
	for _, field := range exposureTimeFields {
		if exposure, exists := rawData[field]; exists {
			normalized := normalizeString(exposure)
			if normalized != "" {
				metadata.ExposureTime = normalized
				break
			}
		}
	}

	// Parse FNumber using priority-based field list
	for _, field := range fNumberFields {
		if fNum, exists := rawData[field]; exists {
			if val, err := strconv.ParseFloat(fNum, 32); err == nil {
				metadata.FNumber = float32(val)
				break
			}
		}
	}

	// Parse ISO using priority-based field list
	for _, field := range isoFields {
		if iso, exists := rawData[field]; exists {
			if val, err := strconv.Atoi(iso); err == nil {
				metadata.IsoSpeed = val
				break
			}
		}
	}

	// Parse FocalLength using priority-based field list
	for _, field := range focalLengthFields {
		if focalLength, exists := rawData[field]; exists {
			// Remove "mm" suffix and other common units
			cleanFL := normalizeString(focalLength)
			cleanFL = strings.TrimSuffix(cleanFL, " mm")
			cleanFL = strings.TrimSuffix(cleanFL, "mm")
			cleanFL = strings.TrimSpace(cleanFL)

			if val, err := strconv.ParseFloat(cleanFL, 32); err == nil {
				metadata.FocalLength = float32(val)
				break
			}
		}
	}

	// Parse GPS Latitude
	if lat, exists := rawData["GPSLatitude"]; exists {
		if val, err := parseGPSCoordinate(lat); err == nil {
			metadata.GPSLatitude = val
		}
	}

	// Parse GPS Longitude
	if lon, exists := rawData["GPSLongitude"]; exists {
		if val, err := parseGPSCoordinate(lon); err == nil {
			metadata.GPSLongitude = val
		}
	}

	// Parse Description using priority-based field list
	for _, field := range descriptionFields {
		if desc, exists := rawData[field]; exists {
			normalized := normalizeString(desc)
			if normalized != "" {
				metadata.Description = normalized
				break
			}
		}
	}

	// Parse Resolution (MP) from ImageWidth and ImageHeight
	if widthStr, wOk := rawData["ImageWidth"]; wOk {
		if heightStr, hOk := rawData["ImageHeight"]; hOk {
			parseInt := func(s string) (int, bool) {
				s = normalizeString(s)
				if s == "" {
					return 0, false
				}
				// Prefer first token in case of "4032 pixels"
				if fields := strings.Fields(s); len(fields) > 0 {
					s = fields[0]
				}
				if val, err := strconv.Atoi(s); err == nil {
					return val, true
				}
				// Fallback: strip non-digits
				digits := make([]rune, 0, len(s))
				for _, r := range s {
					if r >= '0' && r <= '9' {
						digits = append(digits, r)
					} else if len(digits) > 0 {
						// stop at first non-digit after seeing digits
						break
					}
				}
				if len(digits) == 0 {
					return 0, false
				}
				if val, err := strconv.Atoi(string(digits)); err == nil {
					return val, true
				}
				return 0, false
			}

			if w, ok1 := parseInt(widthStr); ok1 {
				if h, ok2 := parseInt(heightStr); ok2 && w > 0 && h > 0 {
					// Check orientation and correct dimensions if needed
					correctedWidth, correctedHeight := correctDimensionsByOrientation(w, h, rawData["Orientation"])

					// Keep Resolution as an integer number of megapixels (rounded)
					pixels := w * h
					mpInt := (pixels + 500_000) / 1_000_000
					metadata.Resolution = fmt.Sprintf("%dMP", mpInt)
					// Also set Dimensions if not already set from ImageSize
					if metadata.Dimensions == "" {
						metadata.Dimensions = fmt.Sprintf("%dx%d", correctedWidth, correctedHeight)
					}
				}
			}
		}
	}

	// Parse Exposure bias using priority-based field list
	for _, field := range exposureBiasFields {
		if ebStr, exists := rawData[field]; exists {
			if val, err := strconv.ParseFloat(ebStr, 32); err == nil {
				metadata.Exposure = float32(val)
				break
			}
		}
	}

	return metadata
}

// correctDimensionsByOrientation corrects width and height based on EXIF Orientation
// Returns corrected width and height that match the actual display orientation
func correctDimensionsByOrientation(width, height int, orientation string) (int, int) {
	if orientation == "" {
		return width, height
	}

	// Check for orientation values that require swapping dimensions
	// Orientation values that require 90° or 270° rotation
	orientationLower := strings.ToLower(orientation)

	// These orientations indicate the photo was taken in portrait mode
	// and needs to be rotated 90° or 270° for correct display
	// In these cases, the sensor width/height are swapped
	rotateOrientations := []string{
		"rotate 90 cw",                        // Orientation 6
		"rotate 90",                           // Orientation 6 (short form)
		"rotate 270 cw",                       // Orientation 8
		"rotate 270",                          // Orientation 8 (short form)
		"rotate 90 ccw",                       // Orientation 8 (alternative description)
		"rotate 270 ccw",                      // Orientation 6 (alternative description)
		"mirror horizontal and rotate 270 cw", // Orientation 5
		"mirror horizontal and rotate 90 cw",  // Orientation 7
		"mirror horizontal and rotate 270",    // Orientation 5 (short form)
		"mirror horizontal and rotate 90",     // Orientation 7 (short form)
	}

	// Also check for numeric orientation codes (1-8)
	if len(orientationLower) == 1 && orientationLower >= "1" && orientationLower <= "8" {
		// Orientation codes 5, 6, 7, 8 require swapping dimensions
		if orientationLower == "5" || orientationLower == "6" || orientationLower == "7" || orientationLower == "8" {
			return height, width
		}
		return width, height
	}

	// Check for text descriptions
	for _, rot := range rotateOrientations {
		if strings.Contains(orientationLower, rot) {
			// Swap width and height for rotated orientations
			return height, width
		}
	}

	// Check for common orientation descriptions that don't require swapping
	noSwapOrientations := []string{
		"horizontal (normal)", // Orientation 1
		"mirror horizontal",   // Orientation 2
		"rotate 180",          // Orientation 3
		"mirror vertical",     // Orientation 4
		"normal",              // Orientation 1 (short form)
		"horizontal",          // Orientation 1 (short form)
	}

	for _, noSwap := range noSwapOrientations {
		if strings.Contains(orientationLower, noSwap) {
			return width, height
		}
	}

	// Default: no swap needed
	return width, height
}

// parseVideoMetadata parses raw EXIF data into VideoSpecificMetadata
func parseVideoMetadata(rawData map[string]string) *dbtypes.VideoSpecificMetadata {
	metadata := &dbtypes.VideoSpecificMetadata{}

	// Parse Codec using priority-based field list
	for _, field := range videoCodecFields {
		if codec, exists := rawData[field]; exists {
			normalized := normalizeString(codec)
			if normalized != "" {
				metadata.Codec = normalized
				break
			}
		}
	}

	// Parse Bitrate using priority-based field list
	for _, field := range videoBitrateFields {
		if bitrate, exists := rawData[field]; exists {
			if val, err := parseBitrate(bitrate); err == nil {
				metadata.Bitrate = val
				break
			}
		}
	}

	// Parse FrameRate using priority-based field list
	for _, field := range frameRateFields {
		if frameRate, exists := rawData[field]; exists {
			if val, err := parseFrameRate(frameRate); err == nil {
				metadata.FrameRate = val
				break
			}
		}
	}

	// Parse RecordedTime using priority-based field list
	for _, field := range recordedTimeFields {
		if dateStr, exists := rawData[field]; exists && dateStr != "" {
			if parsedTime, err := parseDateTime(dateStr); err == nil {
				metadata.RecordedTime = &parsedTime
				break
			}
		}
	}

	// Parse CameraModel using priority-based field list
	for _, field := range videoCameraModelFields {
		if model, exists := rawData[field]; exists {
			normalized := normalizeString(model)
			if normalized != "" {
				metadata.CameraModel = normalized
				break
			}
		}
	}

	// Parse GPS Latitude
	if lat, exists := rawData["GPSLatitude"]; exists {
		if val, err := parseGPSCoordinate(lat); err == nil {
			metadata.GPSLatitude = val
		}
	}

	// Parse GPS Longitude
	if lon, exists := rawData["GPSLongitude"]; exists {
		if val, err := parseGPSCoordinate(lon); err == nil {
			metadata.GPSLongitude = val
		}
	}

	// Parse Description using priority-based field list
	for _, field := range videoDescriptionFields {
		if desc, exists := rawData[field]; exists {
			normalized := normalizeString(desc)
			if normalized != "" {
				metadata.Description = normalized
				break
			}
		}
	}

	return metadata
}

// parseAudioMetadata parses raw EXIF data into AudioSpecificMetadata
func parseAudioMetadata(rawData map[string]string) *dbtypes.AudioSpecificMetadata {
	metadata := &dbtypes.AudioSpecificMetadata{}

	// Parse Codec using priority-based field list
	for _, field := range audioCodecFields {
		if codec, exists := rawData[field]; exists {
			normalized := normalizeString(codec)
			if normalized != "" {
				// Convert file extension to uppercase codec name
				if field == "FileTypeExtension" {
					normalized = normalizeString(codec)
				}
				metadata.Codec = normalized
				break
			}
		}
	}

	// Parse Bitrate using priority-based field list
	for _, field := range audioBitrateFields {
		if bitrate, exists := rawData[field]; exists {
			if val, err := parseBitrate(bitrate); err == nil {
				metadata.Bitrate = val
				break
			}
		}
	}

	// Parse SampleRate using priority-based field list
	for _, field := range sampleRateFields {
		if sampleRate, exists := rawData[field]; exists {
			if val, err := parseSampleRate(sampleRate); err == nil {
				metadata.SampleRate = val
				break
			}
		}
	}

	// Parse Channels using priority-based field list
	for _, field := range channelsFields {
		if channels, exists := rawData[field]; exists {
			if val, err := strconv.Atoi(channels); err == nil {
				metadata.Channels = val
				break
			}
		}
	}

	// Parse Artist using priority-based field list
	for _, field := range artistFields {
		if artist, exists := rawData[field]; exists {
			normalized := normalizeString(artist)
			if normalized != "" {
				metadata.Artist = normalized
				break
			}
		}
	}

	// Parse Album using priority-based field list
	for _, field := range albumFields {
		if album, exists := rawData[field]; exists {
			normalized := normalizeString(album)
			if normalized != "" {
				metadata.Album = normalized
				break
			}
		}
	}

	// Parse Title using priority-based field list
	for _, field := range audioTitleFields {
		if title, exists := rawData[field]; exists {
			normalized := normalizeString(title)
			if normalized != "" {
				metadata.Title = normalized
				break
			}
		}
	}

	// Parse Genre using priority-based field list
	for _, field := range genreFields {
		if genre, exists := rawData[field]; exists {
			normalized := normalizeString(genre)
			if normalized != "" {
				metadata.Genre = normalized
				break
			}
		}
	}

	// Parse Year using priority-based field list
	for _, field := range yearFields {
		if yearStr, exists := rawData[field]; exists {
			if year, err := parseYear(yearStr); err == nil {
				metadata.Year = year
				break
			}
		}
	}

	// Parse Description using priority-based field list
	for _, field := range audioDescriptionFields {
		if desc, exists := rawData[field]; exists {
			normalized := normalizeString(desc)
			if normalized != "" {
				metadata.Description = normalized
				break
			}
		}
	}

	return metadata
}
