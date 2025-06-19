package models

import "github.com/google/uuid"

// Tag represents a tag that can be associated with assets
// @Description Tag for categorizing and labeling assets
type Tag struct {
	TagID         int    `gorm:"primaryKey;autoIncrement" json:"tag_id" example:"1"`
	TagName       string `gorm:"type:varchar(50);uniqueIndex" json:"tag_name" example:"landscape"`
	Category      string `gorm:"type:varchar(50)" json:"category" example:"subject"`
	IsAIGenerated bool   `gorm:"default:true" json:"is_ai_generated" example:"true"`

	Assets []Asset `gorm:"many2many:asset_tags;" json:"assets,omitempty"`
}

// AssetTag represents the many-to-many relationship between assets and tags,
// with additional metadata like confidence and source.
// This model is used by GORM's AutoMigrate to create the join table with the extra fields.
// @Description Association between an asset and a tag with confidence and source information
type AssetTag struct {
	AssetID    uuid.UUID `gorm:"primaryKey;type:uuid" json:"asset_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	TagID      int       `gorm:"primaryKey" json:"tag_id" example:"1"`
	Confidence float32   `gorm:"type:numeric(4,3);not null;check:confidence >= 0 AND confidence <= 1" json:"confidence" example:"0.95"`
	Source     string    `gorm:"type:varchar(20);not null;default:'system';check:source IN ('system', 'user', 'ai')" json:"source" example:"ai" enums:"system,user,ai"`
}

// TableName explicitly sets the table name for the AssetTag model.
func (AssetTag) TableName() string {
	return "asset_tags"
}
