package models

import (
	"time"

	"github.com/google/uuid"
)

// Album represents a collection of assets organized by users
// @Description Album for organizing and grouping related assets
type Album struct {
	AlbumID      int        `gorm:"primaryKey;autoIncrement" json:"album_id" example:"1"`
	UserID       int        `gorm:"not null;index" json:"user_id" example:"123"`
	AlbumName    string     `gorm:"type:varchar(100);not null" json:"album_name" example:"Vacation 2024"`
	CreatedAt    time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"created_at" example:"2024-01-15T10:30:00Z"`
	UpdatedAt    time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at" example:"2024-01-16T10:30:00Z"`
	Description  string     `gorm:"type:text" json:"description" example:"Photos and videos from our summer vacation"`
	CoverAssetID *uuid.UUID `gorm:"type:uuid" json:"cover_asset_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	CoverAsset   Asset      `gorm:"foreignKey:CoverAssetID" json:"cover_asset,omitempty"`

	Assets []Asset `gorm:"many2many:album_assets;foreignKey:AlbumID;joinForeignKey:AlbumID;References:AssetID;joinReferences:AssetID" json:"assets,omitempty"`
}

// AlbumAsset represents the many-to-many relationship between albums and assets
// @Description Association between an album and an asset with positioning information
type AlbumAsset struct {
	AlbumID   int       `gorm:"primaryKey" json:"album_id" example:"1"`
	AssetID   uuid.UUID `gorm:"type:uuid;primaryKey" json:"asset_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Position  int       `gorm:"default:0" json:"position" example:"0"`
	AddedTime time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"added_time" example:"2024-01-15T10:30:00Z"`
}
