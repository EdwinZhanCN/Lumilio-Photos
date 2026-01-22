package service

import (
	"context"
	"server/internal/db/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type AlbumService interface {
	CreateNewAlbum(ctx context.Context, params repo.CreateAlbumParams) (repo.Album, error)
	DeleteAlbum(ctx context.Context, id int32) error
	GetAlbumByID(ctx context.Context, id int32) (repo.Album, error)
	GetAlbumsByUser(ctx context.Context, params repo.GetAlbumsByUserParams) ([]repo.Album, error)
	UpdateAlbum(ctx context.Context, params repo.UpdateAlbumParams) (repo.Album, error)
	GetAlbumAssets(ctx context.Context, albumID int32) ([]repo.GetAlbumAssetsRow, error)
	GetAlbumAssetCount(ctx context.Context, albumID int32) (int64, error)
	AddAssetToAlbum(ctx context.Context, params repo.AddAssetToAlbumParams) error
	RemoveAssetFromAlbum(ctx context.Context, params repo.RemoveAssetFromAlbumParams) error
	UpdateAssetPositionInAlbum(ctx context.Context, params repo.UpdateAssetPositionInAlbumParams) error
	GetAssetAlbums(ctx context.Context, assetID pgtype.UUID) ([]repo.GetAssetAlbumsRow, error)
	FilterAlbumAssets(ctx context.Context, params repo.FilterAlbumAssetsParams) ([]repo.Asset, error)
}

type albumService struct {
	queries *repo.Queries
}

// Request/Response types
type NewAlbumRequest struct {
	UserID      int32   `json:"user_id" binding:"required"`
	AlbumName   string  `json:"album_name" binding:"required"`
	Description *string `json:"description,omitempty"`
	// Accept UUID as string and validate format.
	CoverAssetID string `json:"cover_asset_id" binding:"required,uuid4"`
}

func (r NewAlbumRequest) CoverAssetAsPG() (pgtype.UUID, error) {
	u, err := uuid.Parse(r.CoverAssetID)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: u, Valid: true}, nil
}

func NewAlbumService(q *repo.Queries) AlbumService {
	return &albumService{
		queries: q,
	}
}

// CreateNewAlbum creates a new album, userID, name not null
func (s *albumService) CreateNewAlbum(ctx context.Context, params repo.CreateAlbumParams) (repo.Album, error) {
	album, err := s.queries.CreateAlbum(ctx, params)
	return album, err
}

func (s *albumService) DeleteAlbum(ctx context.Context, id int32) error {
	return s.queries.DeleteAlbum(ctx, id)
}

// GetAlbumByID retrieves a specific album by ID
func (s *albumService) GetAlbumByID(ctx context.Context, id int32) (repo.Album, error) {
	return s.queries.GetAlbumByID(ctx, id)
}

// GetAlbumsByUser retrieves albums for a specific user with pagination
func (s *albumService) GetAlbumsByUser(ctx context.Context, params repo.GetAlbumsByUserParams) ([]repo.Album, error) {
	return s.queries.GetAlbumsByUser(ctx, params)
}

// UpdateAlbum updates an existing album
func (s *albumService) UpdateAlbum(ctx context.Context, params repo.UpdateAlbumParams) (repo.Album, error) {
	return s.queries.UpdateAlbum(ctx, params)
}

// GetAlbumAssets retrieves all assets in an album
func (s *albumService) GetAlbumAssets(ctx context.Context, albumID int32) ([]repo.GetAlbumAssetsRow, error) {
	return s.queries.GetAlbumAssets(ctx, albumID)
}

// GetAlbumAssetCount returns the number of assets in an album
func (s *albumService) GetAlbumAssetCount(ctx context.Context, albumID int32) (int64, error) {
	return s.queries.GetAlbumAssetCount(ctx, albumID)
}

// AddAssetToAlbum adds an asset to an album
func (s *albumService) AddAssetToAlbum(ctx context.Context, params repo.AddAssetToAlbumParams) error {
	return s.queries.AddAssetToAlbum(ctx, params)
}

// RemoveAssetFromAlbum removes an asset from an album
func (s *albumService) RemoveAssetFromAlbum(ctx context.Context, params repo.RemoveAssetFromAlbumParams) error {
	return s.queries.RemoveAssetFromAlbum(ctx, params)
}

// UpdateAssetPositionInAlbum updates the position of an asset within an album
func (s *albumService) UpdateAssetPositionInAlbum(ctx context.Context, params repo.UpdateAssetPositionInAlbumParams) error {
	return s.queries.UpdateAssetPositionInAlbum(ctx, params)
}

// GetAssetAlbums retrieves all albums that contain a specific asset
func (s *albumService) GetAssetAlbums(ctx context.Context, assetID pgtype.UUID) ([]repo.GetAssetAlbumsRow, error) {
	return s.queries.GetAssetAlbums(ctx, assetID)
}

// FilterAlbumAssets filters assets within a specific album
func (s *albumService) FilterAlbumAssets(ctx context.Context, params repo.FilterAlbumAssetsParams) ([]repo.Asset, error) {
	return s.queries.FilterAlbumAssets(ctx, params)
}
