package exif

import (
	"fmt"
	"server/internal/db/dbtypes"
	"strconv"
	"strings"
)

// parsePhotoMetadata parses raw EXIF data into PhotoSpecificMetadata
func parsePhotoMetadata(rawData map[string]string) *dbtypes.PhotoSpecificMetadata {

	metadata := &dbtypes.PhotoSpecificMetadata{
		Rating: 0,     // Set default rating to 0
		Like:   false, // Set default like status to false
	}

	// Parse TakenTime from various datetime fields with expanded fallback options
	for _, field := range []string{"DateTimeOriginal", "CreateDate", "DateTime", "ModifyDate", "FileModifyDate", "DateTimeDigitized", "SubSecDateTimeOriginal", "GPSDateTime"} {
		if dateStr, exists := rawData[field]; exists && dateStr != "" {
			if parsedTime, err := parseDateTime(dateStr); err == nil {
				metadata.TakenTime = &parsedTime
				break
			}
		}
	}

	// Parse CameraModel
	for _, field := range []string{"Model", "CameraModelName", "UniqueCameraModel"} {
		if model, exists := rawData[field]; exists {
			normalized := normalizeString(model)
			if normalized != "" {
				metadata.CameraModel = normalized
				break
			}
		}
	}

	// Parse LensModel
	for _, field := range []string{"LensModel", "LensInfo", "LensType", "Lens"} {
		if lens, exists := rawData[field]; exists {
			normalized := normalizeString(lens)
			if normalized != "" {
				metadata.LensModel = normalized
				break
			}
		}
	}

	// Parse ExposureTime
	for _, field := range []string{"ExposureTime", "ShutterSpeedValue", "ShutterSpeed"} {
		if exposure, exists := rawData[field]; exists {
			normalized := normalizeString(exposure)
			if normalized != "" {
				metadata.ExposureTime = normalized
				break
			}
		}
	}

	// Parse FNumber
	for _, field := range []string{"FNumber", "Aperture", "ApertureValue"} {
		if fNum, exists := rawData[field]; exists {
			if val, err := strconv.ParseFloat(fNum, 32); err == nil {
				metadata.FNumber = float32(val)
				break
			}
		}
	}

	// Parse ISO
	for _, field := range []string{"ISO", "ISOSpeedRatings", "RecommendedExposureIndex"} {
		if iso, exists := rawData[field]; exists {
			if val, err := strconv.Atoi(iso); err == nil {
				metadata.IsoSpeed = val
				break
			}
		}
	}

	// Parse FocalLength
	for _, field := range []string{"FocalLength", "FocalLengthIn35mmFilm", "FocalLengthIn35mmFormat"} {
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

	// Parse Description from various fields
	for _, field := range []string{"ImageDescription", "UserComment", "XPComment", "Caption", "Description"} {
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
					// Keep Resolution as an integer number of megapixels (rounded)
					pixels := w * h
					mpInt := (pixels + 500_000) / 1_000_000
					metadata.Resolution = fmt.Sprintf("%dMP", mpInt)
					// Also set Dimensions if not already set from ImageSize
					if metadata.Dimensions == "" {
						metadata.Dimensions = fmt.Sprintf("%dx%d", w, h)
					}
				}
			}
		}
	}

	// Parse Exposure bias
	for _, field := range []string{"ExposureCompensation"} {
		if ebStr, exists := rawData[field]; exists {
			if val, err := strconv.ParseFloat(ebStr, 32); err == nil {
				metadata.Exposure = float32(val)
				break
			}
		}
	}

	// Parse Dimensions from ImageSize or fallback to width x height
	if sizeStr, exists := rawData["ImageSize"]; exists {
		d := normalizeString(sizeStr)
		if d != "" {
			// Normalize common separators (e.g., "4032x3024" or "4032 x 3024")
			d = strings.ReplaceAll(d, " ", "")
			metadata.Dimensions = d
		}
	}

	return metadata
}

// parseVideoMetadata parses raw EXIF data into VideoSpecificMetadata
func parseVideoMetadata(rawData map[string]string) *dbtypes.VideoSpecificMetadata {
	metadata := &dbtypes.VideoSpecificMetadata{}

	// Parse Codec from various fields
	for _, field := range []string{"VideoCodec", "CompressorID", "AudioCodec", "VideoFormat"} {
		if codec, exists := rawData[field]; exists {
			normalized := normalizeString(codec)
			if normalized != "" {
				metadata.Codec = normalized
				break
			}
		}
	}

	// Parse Bitrate (prefer video bitrate over audio)
	for _, field := range []string{"VideoBitrate", "Bitrate", "AudioBitrate", "OverallBitrate"} {
		if bitrate, exists := rawData[field]; exists {
			if val, err := parseBitrate(bitrate); err == nil {
				metadata.Bitrate = val
				break
			}
		}
	}

	// Parse FrameRate
	for _, field := range []string{"VideoFrameRate", "FrameRate", "NominalFrameRate"} {
		if frameRate, exists := rawData[field]; exists {
			if val, err := parseFrameRate(frameRate); err == nil {
				metadata.FrameRate = val
				break
			}
		}
	}

	// Parse RecordedTime with expanded fallback options
	for _, field := range []string{"CreateDate", "DateTimeOriginal", "MediaCreateDate", "TrackCreateDate", "ModifyDate", "FileModifyDate", "DateTimeDigitized"} {
		if dateStr, exists := rawData[field]; exists && dateStr != "" {
			if parsedTime, err := parseDateTime(dateStr); err == nil {
				metadata.RecordedTime = &parsedTime
				break
			}
		}
	}

	// Parse CameraModel
	for _, field := range []string{"Model", "Make", "CameraModelName", "RecorderModel"} {
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

	// Parse Description
	for _, field := range []string{"Description", "Comment", "Title", "Synopsis"} {
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

	// Parse Codec
	for _, field := range []string{"AudioCodec", "AudioFormat", "FileTypeExtension", "AudioEncoding"} {
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

	// Parse Bitrate
	for _, field := range []string{"AudioBitrate", "Bitrate", "NominalBitrate"} {
		if bitrate, exists := rawData[field]; exists {
			if val, err := parseBitrate(bitrate); err == nil {
				metadata.Bitrate = val
				break
			}
		}
	}

	// Parse SampleRate
	for _, field := range []string{"SampleRate", "AudioSampleRate", "SamplingRate"} {
		if sampleRate, exists := rawData[field]; exists {
			if val, err := parseSampleRate(sampleRate); err == nil {
				metadata.SampleRate = val
				break
			}
		}
	}

	// Parse Channels
	for _, field := range []string{"AudioChannels", "Channels", "ChannelCount"} {
		if channels, exists := rawData[field]; exists {
			if val, err := strconv.Atoi(channels); err == nil {
				metadata.Channels = val
				break
			}
		}
	}

	// Parse Artist
	for _, field := range []string{"Artist", "AlbumArtist", "Performer", "Author"} {
		if artist, exists := rawData[field]; exists {
			normalized := normalizeString(artist)
			if normalized != "" {
				metadata.Artist = normalized
				break
			}
		}
	}

	// Parse Album
	for _, field := range []string{"Album", "AlbumTitle"} {
		if album, exists := rawData[field]; exists {
			normalized := normalizeString(album)
			if normalized != "" {
				metadata.Album = normalized
				break
			}
		}
	}

	// Parse Title
	for _, field := range []string{"Title", "SongTitle", "TrackTitle"} {
		if title, exists := rawData[field]; exists {
			normalized := normalizeString(title)
			if normalized != "" {
				metadata.Title = normalized
				break
			}
		}
	}

	// Parse Genre
	for _, field := range []string{"Genre", "ContentType"} {
		if genre, exists := rawData[field]; exists {
			normalized := normalizeString(genre)
			if normalized != "" {
				metadata.Genre = normalized
				break
			}
		}
	}

	// Parse Year
	for _, field := range []string{"Year", "Date", "ReleaseDate", "RecordingDate"} {
		if yearStr, exists := rawData[field]; exists {
			if year, err := parseYear(yearStr); err == nil {
				metadata.Year = year
				break
			}
		}
	}

	// Parse Description
	for _, field := range []string{"Comment", "Description", "Lyrics", "Synopsis"} {
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
