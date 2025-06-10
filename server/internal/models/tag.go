package models

import "github.com/google/uuid"

type Tag struct {
	TagID         int    `gorm:"primaryKey;autoIncrement"`
	TagName       string `gorm:"type:varchar(50);uniqueIndex"`
	Category      string `gorm:"type:varchar(50)"`
	IsAIGenerated bool   `gorm:"default:true"`

	Assets []Asset `gorm:"many2many:asset_tags;foreignKey:TagID;joinForeignKey:TagID;References:AssetID;joinReferences:AssetID"`
}

type AssetTag struct {
	AssetID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	TagID      int       `gorm:"primaryKey"`
	Confidence float32   `gorm:"type:numeric(4,3);check:confidence >= 0 AND confidence <= 1"`
	Source     string    `gorm:"type:varchar(20);default:'system';check:source IN ('system', 'user', 'ai')"`
}
