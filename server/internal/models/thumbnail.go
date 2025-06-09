package models

import (
	"time"

	"github.com/google/uuid"
)

// Thumbnail represents a thumbnail for any asset type
type Thumbnail struct {
	ThumbnailID int       `gorm:"primaryKey;autoIncrement"`
	AssetID     uuid.UUID `gorm:"type:uuid;not null;index"`
	Size        string    `gorm:"type:varchar(20);check:size IN ('small', 'medium', 'large')"`
	StoragePath string    `gorm:"type:varchar(512);not null"`
	MimeType    string    `gorm:"type:varchar(50);not null"` // For video thumbnails, this might be image/jpeg
	CreatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}

// TableName specifies the table name for the Thumbnail model
func (Thumbnail) TableName() string {
	return "thumbnails"
}
