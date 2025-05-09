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
	"sync"
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
	BatchUploadPhotos(ctx context.Context, files []io.Reader, filenames []string, fileSizes []int64) ([]*models.Photo, []error)
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
		go func() {
			err := s.storage.Delete(ctx, storagePath)
			if err != nil {
				log.Printf("Failed to delete uploaded file: %v", err)
			} else {
				log.Println("Successfully deleted uploaded file")
			}
		}()
		return nil, fmt.Errorf("failed to create record: %w", err)
	}

	return photo, nil
}

// BatchUploadPhotos 新增批量上传方法
func (s *photoService) BatchUploadPhotos(ctx context.Context, files []io.Reader, filenames []string, fileSizes []int64) ([]*models.Photo, []error) {
	// 参数校验
	if len(files) != len(filenames) || len(files) != len(fileSizes) {
		return nil, []error{errors.New("参数长度不一致")}
	}

	// 并发控制（根据实际情况调整数值）
	maxConcurrency := 5
	sem := make(chan struct{}, maxConcurrency)

	var wg sync.WaitGroup
	resultChan := make(chan *models.Photo, len(files))
	errorChan := make(chan uploadError, len(files))

	// 启动worker
	for i := range files {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// 处理单个上传
			photo, err := s.UploadPhoto(ctx, files[index], filenames[index], fileSizes[index])
			if err != nil {
				errorChan <- uploadError{Index: index, Err: err}
				return
			}
			resultChan <- photo
		}(i)
	}

	// 结果收集
	go func() {
		wg.Wait()
		close(resultChan)
		close(errorChan)
	}()

	// 处理结果
	var photos []*models.Photo
	for photo := range resultChan {
		photos = append(photos, photo)
	}

	// 处理错误（保持原始顺序）
	errs := make([]error, len(files))
	for e := range errorChan {
		errs[e.Index] = e.Err
	}

	return photos, errs
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

// 错误结构体保持索引信息
type uploadError struct {
	Index int
	Err   error
}
