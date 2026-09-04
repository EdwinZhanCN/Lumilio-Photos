package exif

import (
	"encoding/json"
	"server/internal/db/dbtypes"
	"strconv"
	"strings"
	"time"
)

// Field priority definitions
// Higher priority fields are checked first
var (
	// TakenTime priority fields - from most specific to most generic
	takenTimeFields = []string{
		"SubSecDateTimeOriginal", // High precision original time
		"DateTimeOriginal",       // Original capture time
		"CreateDate",             // File creation time
		"DateTime",               // General datetime
		"ModifyDate",             // Last modification time
		"FileModifyDate",         // File system modification time
		"DateTimeDigitized",      // Digitization time
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
		"FocalLength", // Physical focal length; do not mix in 35mm equivalents.
	}

	// Description priority fields
	descriptionFields = []string{
		"Description",      // XMP dc:description
		"Caption-Abstract", // IPTC caption/abstract
		"ImageDescription", // EXIF image description
		"UserComment",      // EXIF user comment
		"XPComment",        // Windows XP comment
		"Caption",          // Generic caption fallback
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
		"CreationDate",     // QuickTime creation date with timezone when available
		"CreateDate",       // Creation date
		"DateTimeOriginal", // Original datetime
		"SubSecDateTimeOriginal",
		"MediaCreateDate",   // Media creation date
		"TrackCreateDate",   // Track creation date
		"ModifyDate",        // Modification date
		"FileModifyDate",    // File system modification date
		"DateTimeDigitized", // Digitization date
	}

	// VideoCameraModel priority fields
	videoCameraModelFields = []string{
		"Model",           // Standard model
		"CameraModelName", // Camera model name
		"RecorderModel",   // Recorder model (for videos)
	}

	// VideoDescription priority fields
	videoDescriptionFields = []string{
		"Description",      // XMP/general description
		"Caption-Abstract", // IPTC caption/abstract
		"Comment",          // Comment
		"Title",            // Title
		"Synopsis",         // Synopsis
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
		"Description", // Description
		"Comment",     // Comment
		"Lyrics",      // Lyrics
		"Synopsis",    // Synopsis
	}
)

// parsePhotoMetadata parses raw EXIF data into PhotoSpecificMetadata
func parsePhotoMetadata(rawData map[string]string) *dbtypes.PhotoSpecificMetadata {
	metadata := &dbtypes.PhotoSpecificMetadata{}

	metadata.CameraMake = firstNormalizedString(rawData, []string{"Make", "Manufacturer"})

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

	// Parse Exposure bias using priority-based field list
	for _, field := range exposureBiasFields {
		if ebStr, exists := rawData[field]; exists {
			if val, err := parseRationalFloat32(ebStr); err == nil {
				metadata.ExposureCompensation = &val
				break
			}
		}
	}

	metadata.ContentIdentifier = extractContentIdentifier(rawData)

	return metadata
}

func parseCommonMetadata(rawData map[string]string, rawJSON json.RawMessage, assetType dbtypes.AssetType) dbtypes.CommonMetadata {
	common := dbtypes.CommonMetadata{}

	switch assetType {
	case dbtypes.AssetTypePhoto:
		common.TakenTime, common.CaptureOffsetMinutes = parseCaptureTimestamp(rawData, []captureTimePair{
			{TimeField: "SubSecDateTimeOriginal", OffsetFields: []string{"OffsetTimeOriginal", "OffsetTime", "TimeZoneOffset"}},
			{TimeField: "DateTimeOriginal", OffsetFields: []string{"OffsetTimeOriginal", "OffsetTime", "TimeZoneOffset"}},
			{TimeField: "CreateDate", OffsetFields: []string{"OffsetTimeDigitized", "OffsetTime", "TimeZoneOffset"}},
			{TimeField: "DateTime", OffsetFields: []string{"OffsetTime", "TimeZoneOffset"}},
		}, takenTimeFields)
		common.Width, common.Height = parsePhotoDimensions(rawData)
	case dbtypes.AssetTypeVideo:
		common.TakenTime, common.CaptureOffsetMinutes = parseCaptureTimestamp(rawData, []captureTimePair{
			{TimeField: "CreationDate", OffsetFields: []string{"TimeZone", "TimeZoneOffset"}},
			{TimeField: "SubSecDateTimeOriginal", OffsetFields: []string{"OffsetTimeOriginal", "OffsetTime", "TimeZoneOffset"}},
			{TimeField: "DateTimeOriginal", OffsetFields: []string{"OffsetTimeOriginal", "OffsetTime", "TimeZoneOffset"}},
			{TimeField: "CreateDate", OffsetFields: []string{"OffsetTimeDigitized", "OffsetTime", "TimeZoneOffset"}},
			{TimeField: "MediaCreateDate", OffsetFields: []string{"OffsetTime", "TimeZone", "TimeZoneOffset"}},
			{TimeField: "TrackCreateDate", OffsetFields: []string{"OffsetTime", "TimeZone", "TimeZoneOffset"}},
		}, recordedTimeFields)
	}

	if latitude, err := parseGPSCoordinate(rawData["GPSLatitude"]); err == nil {
		common.GPSLatitude = &latitude
	}
	if longitude, err := parseGPSCoordinate(rawData["GPSLongitude"]); err == nil {
		common.GPSLongitude = &longitude
	}
	if rating, ok := parseEmbeddedRating(rawData["Rating"]); ok {
		common.Rating = &rating
	}
	common.Keywords = parseEmbeddedKeywords(rawJSON)

	return common
}

func parsePhotoDimensions(rawData map[string]string) (*int32, *int32) {
	width, widthOK := parsePositiveDimension(rawData["ImageWidth"])
	height, heightOK := parsePositiveDimension(rawData["ImageHeight"])
	if !widthOK || !heightOK {
		return nil, nil
	}

	width, height = correctDimensionsByOrientation(width, height, rawData["Orientation"])
	w, h := int32(width), int32(height)
	return &w, &h
}

func parsePositiveDimension(value string) (int, bool) {
	value = normalizeString(value)
	if fields := strings.Fields(value); len(fields) > 0 {
		value = fields[0]
	}
	if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
		return parsed, true
	}

	digits := make([]rune, 0, len(value))
	for _, char := range value {
		if char >= '0' && char <= '9' {
			digits = append(digits, char)
		} else if len(digits) > 0 {
			break
		}
	}
	parsed, err := strconv.Atoi(string(digits))
	return parsed, err == nil && parsed > 0
}

func parseEmbeddedRating(value string) (int32, bool) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || parsed < 1 || parsed > 5 || parsed != float64(int32(parsed)) {
		return 0, false
	}
	return int32(parsed), true
}

func parseEmbeddedKeywords(rawJSON json.RawMessage) []string {
	if len(rawJSON) == 0 {
		return nil
	}

	var raw map[string]any
	if err := json.Unmarshal(rawJSON, &raw); err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	keywords := make([]string, 0)
	for _, field := range []string{"Keywords", "Subject", "HierarchicalSubject"} {
		for _, keyword := range metadataStringValues(raw[field]) {
			keyword = normalizeString(keyword)
			key := strings.ToLower(keyword)
			if keyword == "" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			keywords = append(keywords, keyword)
		}
	}
	return keywords
}

func metadataStringValues(value any) []string {
	switch typed := value.(type) {
	case string:
		return []string{typed}
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				values = append(values, text)
			}
		}
		return values
	default:
		return nil
	}
}

func firstNormalizedString(rawData map[string]string, fields []string) string {
	for _, field := range fields {
		if value := normalizeString(rawData[field]); value != "" {
			return value
		}
	}
	return ""
}

func parseRationalFloat32(value string) (float32, error) {
	value = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(value), "EV"))
	parts := strings.Fields(value)
	if len(parts) == 2 {
		whole, wholeErr := strconv.ParseFloat(parts[0], 32)
		fraction, fractionErr := parseRationalFloat32(parts[1])
		if wholeErr == nil && fractionErr == nil {
			if whole < 0 {
				return float32(whole) - fraction, nil
			}
			return float32(whole) + fraction, nil
		}
	}
	if numerator, denominator, ok := strings.Cut(value, "/"); ok {
		n, numeratorErr := strconv.ParseFloat(strings.TrimSpace(numerator), 32)
		d, denominatorErr := strconv.ParseFloat(strings.TrimSpace(denominator), 32)
		if numeratorErr != nil {
			return 0, numeratorErr
		}
		if denominatorErr != nil {
			return 0, denominatorErr
		}
		if d == 0 {
			return 0, strconv.ErrSyntax
		}
		return float32(n / d), nil
	}
	parsed, err := strconv.ParseFloat(value, 32)
	return float32(parsed), err
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

	metadata.CameraMake = firstNormalizedString(rawData, []string{"Make", "Manufacturer"})

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

	metadata.ContentIdentifier = extractContentIdentifier(rawData)

	return metadata
}

func extractContentIdentifier(rawData map[string]string) string {
	if value, exists := rawData["ContentIdentifier"]; exists {
		trimmed := strings.TrimRight(value, "\x00")
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
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

type captureTimePair struct {
	TimeField    string
	OffsetFields []string
}

func parseCaptureTimestamp(
	rawData map[string]string,
	pairs []captureTimePair,
	fallbackFields []string,
) (*time.Time, *int16) {
	for _, pair := range pairs {
		dateStr := strings.TrimSpace(rawData[pair.TimeField])
		if dateStr == "" {
			continue
		}

		for _, offsetField := range pair.OffsetFields {
			offsetStr := strings.TrimSpace(rawData[offsetField])
			if offsetStr == "" {
				continue
			}

			parsedTime, offsetMinutes, err := parseDateTimeWithCaptureOffset(dateStr, offsetStr)
			if err == nil {
				return &parsedTime, offsetMinutes
			}
		}

		parsedTime, offsetMinutes, err := parseDateTimeWithCaptureOffset(dateStr, "")
		if err == nil {
			return &parsedTime, offsetMinutes
		}
	}

	for _, field := range fallbackFields {
		dateStr := strings.TrimSpace(rawData[field])
		if dateStr == "" {
			continue
		}

		parsedTime, offsetMinutes, err := parseDateTimeWithCaptureOffset(dateStr, "")
		if err == nil {
			return &parsedTime, offsetMinutes
		}
	}

	return nil, nil
}
