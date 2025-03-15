package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"path/filepath"
	"server/internal/models"
	"server/internal/repository"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Error constants
var (
	ErrInvalidFileType = errors.New("invalid file type: only image files are allowed")
	ErrFileTooLarge    = errors.New("file too large: maximum file size exceeded")
)

// CloudStorage defines the interface for storage services
type CloudStorage interface {
	Upload(ctx context.Context, file io.Reader) (string, error)
	Delete(ctx context.Context, path string) error
	Get(ctx context.Context, path string) (io.ReadCloser, error)
}

// PhotoService defines the interface for photo-related operations
type PhotoService interface {
	UploadPhoto(ctx context.Context, file io.Reader, filename string, fileSize int64) (*models.Photo, error)
	GetPhoto(ctx context.Context, id uuid.UUID) (*models.Photo, error)
	DeletePhoto(ctx context.Context, id uuid.UUID) error
	UpdatePhotoMetadata(ctx context.Context, id uuid.UUID, metadata *models.PhotoMetadata) error
	AddPhotoToAlbum(ctx context.Context, photoID uuid.UUID, albumID int) error
	RemovePhotoFromAlbum(ctx context.Context, photoID uuid.UUID, albumID int) error
	AddTagToPhoto(ctx context.Context, photoID uuid.UUID, tagID int, confidence float32, source string) error
	RemoveTagFromPhoto(ctx context.Context, photoID uuid.UUID, tagID int) error
	CreateThumbnail(ctx context.Context, photoID uuid.UUID, size string, thumbnailPath string) (*models.Thumbnail, error)
}

type photoService struct {
	repo    repository.PhotoRepository
	storage CloudStorage
}

// NewPhotoService creates a new instance of PhotoService
func NewPhotoService(r repository.PhotoRepository, s CloudStorage) PhotoService {
	return &photoService{repo: r, storage: s}
}

// UploadPhoto handles the photo upload process
func (s *photoService) UploadPhoto(ctx context.Context, file io.Reader, filename string, fileSize int64) (*models.Photo, error) {
	log.Println("Uploading photo")

	// 1. Validate file type
	if !isValidImageType(filename) {
		return nil, ErrInvalidFileType
	}

	// 2. Upload to cloud storage
	storagePath, err := s.storage.Upload(ctx, file)
	if err != nil {
		return nil, fmt.Errorf("storage upload failed: %w", err)
	}

	// 3. Create database record
	photo := &models.Photo{
		PhotoID:          uuid.New(),
		OriginalFilename: filename,
		StoragePath:      storagePath,
		MimeType:         getContentType(filename),
		FileSize:         fileSize,
		UploadTime:       time.Now(),
		IsDeleted:        false,
		// Initialize empty relationships
		Metadata:   models.PhotoMetadata{},
		Thumbnails: []models.Thumbnail{},
		Tags:       []models.Tag{},
		Albums:     []models.Album{},
	}

	// Create initial metadata record
	photo.Metadata = models.PhotoMetadata{
		PhotoID: photo.PhotoID,
		// Other metadata fields will be populated later
	}

	if err := s.repo.CreatePhoto(ctx, photo); err != nil {
		// Compensating transaction: delete the uploaded file
		go s.storage.Delete(ctx, storagePath)
		return nil, fmt.Errorf("failed to create record: %w", err)
	}

	return photo, nil
}

// GetPhoto retrieves a photo by its ID
func (s *photoService) GetPhoto(ctx context.Context, id uuid.UUID) (*models.Photo, error) {
	return s.repo.GetByID(ctx, id)
}

// DeletePhoto marks a photo as deleted
func (s *photoService) DeletePhoto(ctx context.Context, id uuid.UUID) error {
	photo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find photo: %w", err)
	}

	// Mark as deleted instead of physically removing
	now := time.Now()
	photo.IsDeleted = true
	photo.DeletedAt = &now

	return s.repo.UpdatePhoto(ctx, photo)
}

// UpdatePhotoMetadata updates the metadata for a photo
func (s *photoService) UpdatePhotoMetadata(ctx context.Context, id uuid.UUID, metadata *models.PhotoMetadata) error {
	log.Println("Updating photo metadata")
	photo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find photo: %w", err)
	}

	// Ensure the PhotoID matches
	if metadata.PhotoID != photo.PhotoID {
		return errors.New("photo ID mismatch")
	}

	// print metadata
	log.Printf("Updating metadata for photo ID: %s %d", photo.PhotoID, metadata.IsoSpeed)

	// 直接更新元数据记录，而不是通过photo对象
	return s.repo.UpdatePhotoMetadata(ctx, metadata)
}

// AddPhotoToAlbum adds a photo to an album
func (s *photoService) AddPhotoToAlbum(ctx context.Context, photoID uuid.UUID, albumID int) error {
	return s.repo.AddPhotoToAlbum(ctx, photoID, albumID)
}

// RemovePhotoFromAlbum removes a photo from an album
func (s *photoService) RemovePhotoFromAlbum(ctx context.Context, photoID uuid.UUID, albumID int) error {
	return s.repo.RemovePhotoFromAlbum(ctx, photoID, albumID)
}

// AddTagToPhoto adds a tag to a photo
func (s *photoService) AddTagToPhoto(ctx context.Context, photoID uuid.UUID, tagID int, confidence float32, source string) error {
	return s.repo.AddTagToPhoto(ctx, photoID, tagID, confidence, source)
}

// RemoveTagFromPhoto removes a tag from a photo
func (s *photoService) RemoveTagFromPhoto(ctx context.Context, photoID uuid.UUID, tagID int) error {
	return s.repo.RemoveTagFromPhoto(ctx, photoID, tagID)
}

// CreateThumbnail creates a new thumbnail for a photo
func (s *photoService) CreateThumbnail(ctx context.Context, photoID uuid.UUID, size string, thumbnailPath string) (*models.Thumbnail, error) {
	thumbnail := &models.Thumbnail{
		PhotoID:     photoID,
		Size:        size,
		StoragePath: thumbnailPath,
		CreatedAt:   time.Now(),
	}

	if err := s.repo.CreateThumbnail(ctx, thumbnail); err != nil {
		return nil, fmt.Errorf("failed to create thumbnail record: %w", err)
	}

	return thumbnail, nil
}

// isValidImageType checks if the file is a valid image type
func isValidImageType(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	validExtensions := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
		".heic": true,
	}
	return validExtensions[ext]
}

// getContentType determines the content type based on file extension
func getContentType(filename string) string {
	ext := filepath.Ext(filename)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		return "application/octet-stream"
	}
	return mimeType
}
