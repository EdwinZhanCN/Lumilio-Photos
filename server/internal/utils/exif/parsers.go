package exif

import (
	"server/internal/db/dbtypes"
	"strconv"
)

// parsePhotoMetadata parses raw EXIF data into PhotoSpecificMetadata
func parsePhotoMetadata(rawData map[string]string) *dbtypes.PhotoSpecificMetadata {
	metadata := &dbtypes.PhotoSpecificMetadata{}

	// Parse TakenTime from various datetime fields
	for _, field := range []string{"DateTimeOriginal", "CreateDate", "DateTime"} {
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
			if val, err := strconv.ParseFloat(focalLength, 32); err == nil {
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

	// Parse RecordedTime
	for _, field := range []string{"CreateDate", "DateTimeOriginal", "MediaCreateDate", "TrackCreateDate"} {
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
