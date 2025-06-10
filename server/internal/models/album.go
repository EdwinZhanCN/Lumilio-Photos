package models

import (
	"time"

	"github.com/google/uuid"
)

type Album struct {
	AlbumID      int        `gorm:"primaryKey;autoIncrement"`
	UserID       int        `gorm:"not null;index"`
	AlbumName    string     `gorm:"type:varchar(100);not null"`
	CreatedAt    time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt    time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
	Description  string     `gorm:"type:text"`
	CoverAssetID *uuid.UUID `gorm:"type:uuid"`
	CoverAsset   Asset      `gorm:"foreignKey:CoverAssetID"`

	Assets []Asset `gorm:"many2many:album_assets;foreignKey:AlbumID;joinForeignKey:AlbumID;References:AssetID;joinReferences:AssetID"`
}

type AlbumAsset struct {
	AlbumID   int       `gorm:"primaryKey"`
	AssetID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Position  int       `gorm:"default:0"`
	AddedTime time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}
