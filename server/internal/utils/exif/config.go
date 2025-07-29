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
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Timeout:       30 * time.Second,
		BufferSize:    8192,
		MaxFileSize:   100 * 1024 * 1024, // 100MB
		WorkerCount:   4,
		RetryAttempts: 3,
		EnableCaching: true,
		CacheSize:     1000,
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
			"Model",
			"LensModel",
			"ExposureTime",
			"FNumber",
			"ISO",
			"GPSLatitude",
			"GPSLongitude",
			"ImageDescription",
			"UserComment",
			"XPComment",
		},
		VideoTags: []string{
			"CompressorID",
			"VideoCodec",
			"AudioCodec",
			"VideoBitrate",
			"AudioBitrate",
			"VideoFrameRate",
			"FrameRate",
			"CreateDate",
			"DateTimeOriginal",
			"Model",
			"Make",
			"GPSLatitude",
			"GPSLongitude",
			"Description",
			"Comment",
			"Title",
		},
		AudioTags: []string{
			"AudioCodec",
			"FileTypeExtension",
			"AudioBitrate",
			"SampleRate",
			"AudioChannels",
			"Channels",
			"Artist",
			"Album",
			"Title",
			"Genre",
			"Year",
			"Date",
			"Comment",
			"Description",
		},
	}
}
