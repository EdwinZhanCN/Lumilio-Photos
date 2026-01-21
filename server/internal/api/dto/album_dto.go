package dto

import (
	"time"

	"server/internal/db/repo"

	"github.com/google/uuid"
)

// CreateAlbumRequestDTO represents the request structure for creating an album
type CreateAlbumRequestDTO struct {
	AlbumName    string  `json:"album_name" binding:"required"`
	Description  *string `json:"description"`
	CoverAssetID *string `json:"cover_asset_id" binding:"omitempty,uuid4"`
}

// UpdateAlbumRequestDTO represents the request structure for updating an album
type UpdateAlbumRequestDTO struct {
	AlbumName    *string `json:"album_name"`
	Description  *string `json:"description"`
	CoverAssetID *string `json:"cover_asset_id" binding:"omitempty,uuid4"`
}

// AlbumDTO represents an album
type AlbumDTO struct {
	AlbumID      int32     `json:"album_id"`
	UserID       int32     `json:"user_id"`
	AlbumName    string    `json:"album_name"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Description  *string   `json:"description"`
	CoverAssetID *string   `json:"cover_asset_id"`
}

// ToAlbumDTO converts a repo.Album to AlbumDTO
func ToAlbumDTO(a repo.Album) AlbumDTO {
	var createdAt time.Time
	if a.CreatedAt.Valid {
		createdAt = a.CreatedAt.Time
	}
	var updatedAt time.Time
	if a.UpdatedAt.Valid {
		updatedAt = a.UpdatedAt.Time
	}
	var coverID *string
	if a.CoverAssetID.Valid {
		s := uuid.UUID(a.CoverAssetID.Bytes).String()
		coverID = &s
	}

	return AlbumDTO{
		AlbumID:      a.AlbumID,
		UserID:       a.UserID,
		AlbumName:    a.AlbumName,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
		Description:  a.Description,
		CoverAssetID: coverID,
	}
}

// GetAlbumResponseDTO represents the response structure for getting an album
type GetAlbumResponseDTO struct {
	AlbumDTO
	AssetCount int64 `json:"asset_count"`
}

// ListAlbumsResponseDTO represents the response structure for listing albums
type ListAlbumsResponseDTO struct {
	Albums []GetAlbumResponseDTO `json:"albums"`
	Total  int                   `json:"total"`
	Limit  int                   `json:"limit"`
	Offset int                   `json:"offset"`
}

// AddAssetToAlbumRequestDTO represents the request structure for adding an asset to an album
type AddAssetToAlbumRequestDTO struct {
	Position *int32 `json:"position"`
}

// UpdateAssetPositionRequestDTO represents the request structure for updating an asset's position in an album
type UpdateAssetPositionRequestDTO struct {
	Position *int32 `json:"position" binding:"required"`
}
