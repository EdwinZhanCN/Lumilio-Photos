package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// AssetType represents the type of asset
// @Description Type of digital asset
type AssetType string

const (
	AssetTypePhoto    AssetType = "PHOTO"
	AssetTypeVideo    AssetType = "VIDEO"
	AssetTypeAudio    AssetType = "AUDIO"
	AssetTypeDocument AssetType = "DOCUMENT"
)

// Valid returns true if the AssetType is valid
func (at AssetType) Valid() bool {
	switch at {
	case AssetTypePhoto, AssetTypeVideo, AssetTypeAudio, AssetTypeDocument:
		return true
	}
	return false
}

// SpecificMetadata is a JSON field that stores type-specific metadata
// @Description Type-specific metadata stored as JSON
type SpecificMetadata map[string]interface{}

// Value implements the driver.Valuer interface for database storage
func (sm SpecificMetadata) Value() (driver.Value, error) {
	if sm == nil {
		return nil, nil
	}
	return json.Marshal(sm)
}

// Scan implements the sql.Scanner interface for database retrieval
func (sm *SpecificMetadata) Scan(value interface{}) error {
	if value == nil {
		*sm = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("cannot scan non-[]byte value into SpecificMetadata")
	}

	return json.Unmarshal(bytes, sm)
}

// Asset represents any type of digital asset in the system
// @Description Digital asset (photo, video, audio, document) with metadata and relationships
type Asset struct {
	AssetID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"asset_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	OwnerID          *int       `gorm:"index" json:"owner_id,omitempty" example:"123"`
	Type             AssetType  `gorm:"type:varchar(20);not null;index" json:"type" example:"PHOTO"`
	OriginalFilename string     `gorm:"type:varchar(255);not null" json:"original_filename" example:"vacation_photo.jpg"`
	StoragePath      string     `gorm:"type:varchar(512);not null" json:"storage_path" example:"2024/01/abc123.jpg"`
	MimeType         string     `gorm:"type:varchar(50);not null" json:"mime_type" example:"image/jpeg"`
	FileSize         int64      `gorm:"not null" json:"file_size" example:"1048576"`
	Hash             string     `gorm:"type:varchar(64);index" json:"hash,omitempty" example:"abcd1234567890efgh"`
	Width            *int       `json:"width,omitempty" example:"1920"`
	Height           *int       `json:"height,omitempty" example:"1080"`
	Duration         *float64   `json:"duration,omitempty" example:"120.5"` // For video/audio assets in seconds
	UploadTime       time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"upload_time" example:"2024-01-15T10:30:00Z"`
	IsDeleted        bool       `gorm:"default:false" json:"is_deleted" example:"false"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty" example:"2024-01-16T10:30:00Z"`

	// JSON field for type-specific metadata
	SpecificMetadata SpecificMetadata `gorm:"type:jsonb" json:"specific_metadata,omitempty"`

	// Relationships
	Thumbnails []Thumbnail `gorm:"foreignKey:AssetID" json:"thumbnails,omitempty"`
	Tags       []Tag       `gorm:"many2many:asset_tags;foreignKey:AssetID;joinForeignKey:AssetID;References:TagID;joinReferences:TagID" json:"tags,omitempty"`
	Albums     []Album     `gorm:"many2many:album_assets;foreignKey:AssetID;joinForeignKey:AssetID;References:AlbumID;joinReferences:AlbumID" json:"albums,omitempty"`
}

// TableName specifies the table name for the Asset model
func (Asset) TableName() string {
	return "assets"
}

// IsPhoto returns true if the asset is a photo
func (a *Asset) IsPhoto() bool {
	return a.Type == AssetTypePhoto
}

// IsVideo returns true if the asset is a video
func (a *Asset) IsVideo() bool {
	return a.Type == AssetTypeVideo
}

// IsAudio returns true if the asset is an audio file
func (a *Asset) IsAudio() bool {
	return a.Type == AssetTypeAudio
}

// GetPhotoMetadata returns photo-specific metadata if the asset is a photo
func (a *Asset) GetPhotoMetadata() (*PhotoSpecificMetadata, error) {
	if !a.IsPhoto() {
		return nil, errors.New("asset is not a photo")
	}

	var metadata PhotoSpecificMetadata

	data, err := json.Marshal(a.SpecificMetadata)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &metadata)
	if err != nil {
		return nil, err
	}

	return &metadata, nil
}

// SetPhotoMetadata sets photo-specific metadata
func (a *Asset) SetPhotoMetadata(metadata *PhotoSpecificMetadata) error {
	if !a.IsPhoto() {
		return errors.New("asset is not a photo")
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &a.SpecificMetadata)
}

// GetVideoMetadata returns video-specific metadata if the asset is a video
func (a *Asset) GetVideoMetadata() (*VideoSpecificMetadata, error) {
	if !a.IsVideo() {
		return nil, errors.New("asset is not a video")
	}

	var metadata VideoSpecificMetadata
	data, err := json.Marshal(a.SpecificMetadata)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &metadata)
	return &metadata, err
}

// PhotoSpecificMetadata represents metadata specific to photo assets
// @Description EXIF and photo-specific metadata
type PhotoSpecificMetadata struct {
	TakenTime    *time.Time `json:"taken_time,omitempty" example:"2024-01-15T10:30:00Z"`
	CameraModel  string     `json:"camera_model,omitempty" example:"Canon EOS R5"`
	LensModel    string     `json:"lens_model,omitempty" example:"RF 24-70mm F2.8 L IS USM"`
	ExposureTime string     `json:"exposure_time,omitempty" example:"1/250"`
	FNumber      float32    `json:"f_number,omitempty" example:"2.8"`
	IsoSpeed     int        `json:"iso_speed,omitempty" example:"400"`
	GPSLatitude  float64    `json:"gps_latitude,omitempty" example:"37.7749"`
	GPSLongitude float64    `json:"gps_longitude,omitempty" example:"-122.4194"`
	Description  string     `json:"description,omitempty" example:"Beautiful sunset at Golden Gate Bridge"`
}

// VideoSpecificMetadata represents metadata specific to video assets
// @Description Video-specific metadata including codec and recording information
type VideoSpecificMetadata struct {
	Codec        string     `json:"codec,omitempty" example:"H.264"`
	Bitrate      int        `json:"bitrate,omitempty" example:"8000000"`
	FrameRate    float64    `json:"frame_rate,omitempty" example:"29.97"`
	RecordedTime *time.Time `json:"recorded_time,omitempty" example:"2024-01-15T10:30:00Z"`
	CameraModel  string     `json:"camera_model,omitempty" example:"Sony FX3"`
	GPSLatitude  float64    `json:"gps_latitude,omitempty" example:"37.7749"`
	GPSLongitude float64    `json:"gps_longitude,omitempty" example:"-122.4194"`
	Description  string     `json:"description,omitempty" example:"Time-lapse video of city skyline"`
}

// AudioSpecificMetadata represents metadata specific to audio assets
// @Description Audio-specific metadata including codec and music information
type AudioSpecificMetadata struct {
	Codec       string `json:"codec,omitempty" example:"MP3"`
	Bitrate     int    `json:"bitrate,omitempty" example:"320000"`
	SampleRate  int    `json:"sample_rate,omitempty" example:"44100"`
	Channels    int    `json:"channels,omitempty" example:"2"`
	Artist      string `json:"artist,omitempty" example:"The Beatles"`
	Album       string `json:"album,omitempty" example:"Abbey Road"`
	Title       string `json:"title,omitempty" example:"Come Together"`
	Genre       string `json:"genre,omitempty" example:"Rock"`
	Year        int    `json:"year,omitempty" example:"1969"`
	Description string `json:"description,omitempty" example:"Classic rock song from Abbey Road album"`
}
