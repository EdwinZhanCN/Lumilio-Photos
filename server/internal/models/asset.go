package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// AssetType represents the type of asset
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
type Asset struct {
	AssetID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"asset_id"`
	OwnerID          *int       `gorm:"index" json:"owner_id,omitempty"`
	Type             AssetType  `gorm:"type:varchar(20);not null;index" json:"type"`
	OriginalFilename string     `gorm:"type:varchar(255);not null" json:"original_filename"`
	StoragePath      string     `gorm:"type:varchar(512);not null" json:"storage_path"`
	MimeType         string     `gorm:"type:varchar(50);not null" json:"mime_type"`
	FileSize         int64      `gorm:"not null" json:"file_size"`
	Hash             string     `gorm:"type:varchar(64);index" json:"hash,omitempty"`
	Width            *int       `json:"width,omitempty"`
	Height           *int       `json:"height,omitempty"`
	Duration         *float64   `json:"duration,omitempty"` // For video/audio assets in seconds
	UploadTime       time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"upload_time"`
	IsDeleted        bool       `gorm:"default:false" json:"is_deleted"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`

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
type PhotoSpecificMetadata struct {
	TakenTime    *time.Time `json:"taken_time,omitempty"`
	CameraModel  string     `json:"camera_model,omitempty"`
	LensModel    string     `json:"lens_model,omitempty"`
	ExposureTime string     `json:"exposure_time,omitempty"`
	FNumber      float32    `json:"f_number,omitempty"`
	IsoSpeed     int        `json:"iso_speed,omitempty"`
	GPSLatitude  float64    `json:"gps_latitude,omitempty"`
	GPSLongitude float64    `json:"gps_longitude,omitempty"`
	Description  string     `json:"description,omitempty"`
}

// VideoSpecificMetadata represents metadata specific to video assets
type VideoSpecificMetadata struct {
	Codec        string     `json:"codec,omitempty"`
	Bitrate      int        `json:"bitrate,omitempty"`
	FrameRate    float64    `json:"frame_rate,omitempty"`
	RecordedTime *time.Time `json:"recorded_time,omitempty"`
	CameraModel  string     `json:"camera_model,omitempty"`
	GPSLatitude  float64    `json:"gps_latitude,omitempty"`
	GPSLongitude float64    `json:"gps_longitude,omitempty"`
	Description  string     `json:"description,omitempty"`
}

// AudioSpecificMetadata represents metadata specific to audio assets
type AudioSpecificMetadata struct {
	Codec       string `json:"codec,omitempty"`
	Bitrate     int    `json:"bitrate,omitempty"`
	SampleRate  int    `json:"sample_rate,omitempty"`
	Channels    int    `json:"channels,omitempty"`
	Artist      string `json:"artist,omitempty"`
	Album       string `json:"album,omitempty"`
	Title       string `json:"title,omitempty"`
	Genre       string `json:"genre,omitempty"`
	Year        int    `json:"year,omitempty"`
	Description string `json:"description,omitempty"`
}
