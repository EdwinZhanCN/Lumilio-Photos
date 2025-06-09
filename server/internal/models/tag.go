package models

import (
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type Tag struct {
	TagID         int             `gorm:"primaryKey;autoIncrement"`
	TagName       string          `gorm:"type:varchar(50);uniqueIndex"`
	Category      string          `gorm:"type:varchar(50)"`
	IsAIGenerated bool            `gorm:"default:true"`
	Embedding     pq.Float32Array `gorm:"type:vector(384)"` // Embedding vector for semantic search

	Photos []Photo `gorm:"many2many:photo_tags;foreignKey:TagID;joinForeignKey:TagID;References:PhotoID;joinReferences:PhotoID"`
}

type PhotoTag struct {
	PhotoID    uuid.UUID `gorm:"type:uuid;primaryKey"` // PhotoID that binding with a specific photo
	TagID      int       `gorm:"primaryKey"`           // TagID that binding with a specific tag
	Confidence float32   `gorm:"type:numeric(4,3);check:confidence >= 0 AND confidence <= 1"`
	// System usually means AI generated
	Source    string          `gorm:"type:varchar(20);default:'system';check:source IN ('system', 'user')"`
	Embedding pq.Float32Array `gorm:"type:vector(384)"` // Embedding for context-specific tag representation
}
