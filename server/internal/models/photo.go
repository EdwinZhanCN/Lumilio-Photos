package models

import (
	"github.com/google/uuid"
	"time"
)

func (Photo) TableName() string {
	return "photos"
}

type Photo struct {
	PhotoID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OriginalFilename string    `gorm:"type:varchar(255);not null"`
	StoragePath      string    `gorm:"type:varchar(512);not null"`
	MimeType         string    `gorm:"type:varchar(50);not null"`
	FileSize         int64     `gorm:"not null"`
	UploadTime       time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	Width            int
	Height           int
	IsDeleted        bool `gorm:"default:false"`
	DeletedAt        *time.Time

	Metadata   PhotoMetadata `gorm:"foreignKey:PhotoID"`
	Thumbnails []Thumbnail   `gorm:"foreignKey:PhotoID"`
	Tags       []Tag         `gorm:"many2many:photo_tags;foreignKey:PhotoID;joinForeignKey:PhotoID;References:TagID;joinReferences:TagID"`
	Albums     []Album       `gorm:"many2many:album_photos;foreignKey:PhotoID;joinForeignKey:PhotoID;References:AlbumID;joinReferences:AlbumID"`
}

func (Thumbnail) TableName() string {
	return "thumbnails"
}

type Thumbnail struct {
	ThumbnailID int       `gorm:"primaryKey;autoIncrement"`
	PhotoID     uuid.UUID `gorm:"type:uuid;not null;index"`
	Size        string    `gorm:"type:varchar(20);check:size IN ('small', 'medium', 'large')"`
	StoragePath string    `gorm:"type:varchar(512);not null"`
	CreatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}

func (PhotoMetadata) TableName() string {
	return "photo_metadata"
}

type PhotoMetadata struct {
	MetadataID   int       `gorm:"primaryKey;autoIncrement"`
	PhotoID      uuid.UUID `gorm:"type:uuid;unique;not null"`
	TakenTime    *time.Time
	CameraModel  string  `gorm:"type:varchar(100)"`
	LensModel    string  `gorm:"type:varchar(100)"`
	ExposureTime string  `gorm:"type:varchar(20)"`
	FNumber      float32 `gorm:"type:numeric(3,1)"`
	IsoSpeed     int
	GPSLatitude  float64 `gorm:"type:numeric(9,6)"`
	GPSLongitude float64 `gorm:"type:numeric(9,6)"`
	Description  string  `gorm:"type:text"`
}
