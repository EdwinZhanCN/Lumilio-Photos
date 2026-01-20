package exif

import "time"

// Config holds configuration for the EXIF extractor
type Config struct {
	// Timeout for exiftool command execution
	Timeout time.Duration

	// BufferSize for streaming operations
	BufferSize int

	// MaxFileSize maximum allowed file size for processing
	MaxFileSize int64

	// WorkerCount number of concurrent workers
	WorkerCount int

	// RetryAttempts number of retry attempts for failed operations
	RetryAttempts int

	// EnableCaching whether to enable metadata caching
	EnableCaching bool

	// CacheSize maximum number of entries in cache
	CacheSize int

	// FastMode use exiftool fast mode (skip scanning entire file)
	FastMode bool
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Timeout:       30 * time.Second,
		BufferSize:    8192,
		MaxFileSize:   2 * 1024 * 1024 * 1024, // 2GB
		WorkerCount:   8,
		RetryAttempts: 2,
		EnableCaching: false, // Disable caching for large files to save memory
		CacheSize:     100,   // Reduced cache size for large file processing
		FastMode:      false, // Default to full scan for accuracy
	}
}

// TagConfig defines which EXIF tags to extract for each asset type
type TagConfig struct {
	PhotoTags []string
	VideoTags []string
	AudioTags []string
}

// DefaultTagConfig returns default tag configuration
func DefaultTagConfig() *TagConfig {
	return &TagConfig{
		PhotoTags: []string{
			"DateTimeOriginal",
			"CreateDate",
			"DateTime",
			"Model",
			"CameraModelName",
			"UniqueCameraModel",
			"LensModel",
			"LensID",
			"LensInfo",
			"LensType",
			"Lens",
			"ExposureTime",
			"ShutterSpeedValue",
			"ShutterSpeed",
			"FNumber",
			"Aperture",
			"ApertureValue",
			"ISO",
			"ISOSpeedRatings",
			"RecommendedExposureIndex",
			"FocalLength",
			"FocalLengthIn35mmFilm",
			"FocalLengthIn35mmFormat",
			"GPSLatitude",
			"GPSLongitude",
			"ImageDescription",
			"UserComment",
			"XPComment",
			"Caption",
			"Description",
			"Comment",
			"ImageWidth",
			"ImageHeight",
			"Orientation",
			"ExposureCompensation",
			"ExposureBiasValue",
			"ImageSize",
			"FileType",
			"FileTypeExtension",
			"MIMEType",
		},
		VideoTags: []string{
			"VideoCodec",
			"CompressorID",
			"AudioCodec",
			"VideoFormat",
			"VideoBitrate",
			"Bitrate",
			"AudioBitrate",
			"OverallBitrate",
			"VideoFrameRate",
			"FrameRate",
			"NominalFrameRate",
			"CreateDate",
			"DateTimeOriginal",
			"MediaCreateDate",
			"TrackCreateDate",
			"Model",
			"Make",
			"CameraModelName",
			"RecorderModel",
			"GPSLatitude",
			"GPSLongitude",
			"Description",
			"Comment",
			"Title",
			"Synopsis",
			"FileType",
			"FileTypeExtension",
			"MIMEType",
		},
		AudioTags: []string{
			"AudioCodec",
			"AudioFormat",
			"FileTypeExtension",
			"AudioEncoding",
			"AudioBitrate",
			"Bitrate",
			"NominalBitrate",
			"SampleRate",
			"AudioSampleRate",
			"SamplingRate",
			"AudioChannels",
			"Channels",
			"ChannelCount",
			"Artist",
			"AlbumArtist",
			"Performer",
			"Author",
			"Album",
			"AlbumTitle",
			"Title",
			"SongTitle",
			"TrackTitle",
			"Genre",
			"ContentType",
			"Year",
			"Date",
			"ReleaseDate",
			"RecordingDate",
			"Comment",
			"Description",
			"Lyrics",
			"Synopsis",
			"FileType",
			"MIMEType",
		},
	}
}
