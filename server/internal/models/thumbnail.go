package models

import (
	"time"

	"github.com/google/uuid"
)

// Thumbnail represents a thumbnail for any asset type
// @Description Thumbnail image for assets (photos, videos, etc.)
type Thumbnail struct {
	ThumbnailID int       `gorm:"primaryKey;autoIncrement" json:"thumbnail_id" example:"1"`
	AssetID     uuid.UUID `gorm:"type:uuid;not null;index" json:"asset_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Size        string    `gorm:"type:varchar(20);check:size IN ('small', 'medium', 'large')" json:"size" example:"medium" enums:"small,medium,large"`
	StoragePath string    `gorm:"type:varchar(512);not null" json:"storage_path" example:"thumbnails/2024/01/thumb_abc123.jpg"`
	MimeType    string    `gorm:"type:varchar(50);not null" json:"mime_type" example:"image/jpeg"` // For video thumbnails, this might be image/jpeg
	CreatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at" example:"2024-01-15T10:30:00Z"`
}

// TableName specifies the table name for the Thumbnail model
func (Thumbnail) TableName() string {
	return "thumbnails"
}
