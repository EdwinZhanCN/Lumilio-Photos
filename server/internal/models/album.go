package models

import (
	"github.com/google/uuid"
	"time"
)

type Album struct {
	AlbumID      int        `gorm:"primaryKey;autoIncrement"`
	UserID       int        `gorm:"not null;index"`
	AlbumName    string     `gorm:"type:varchar(100);not null"`
	CreatedAt    time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt    time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
	Description  string     `gorm:"type:text"`
	CoverPhotoID *uuid.UUID `gorm:"type:uuid"`
	CoverPhoto   Photo      `gorm:"foreignKey:CoverPhotoID"`

	Photos []Photo `gorm:"many2many:album_photos;foreignKey:AlbumID;joinForeignKey:AlbumID;References:PhotoID;joinReferences:PhotoID"`
}

type AlbumPhoto struct {
	AlbumID   int       `gorm:"primaryKey"`
	PhotoID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Position  int       `gorm:"default:0"`
	AddedTime time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}
