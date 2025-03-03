package service

import (
	"context"
	"fmt"
	"io"
	"server/internal/models"
	"server/internal/repository"
)

type PhotoService interface {
	UploadPhoto(ctx context.Context, file io.Reader, filename string) (*models.Photo, error)
}

type photoService struct {
	repo    repository.PhotoRepository
	storage CloudStorage // 依赖存储服务
}

func NewPhotoService(r repository.PhotoRepository, s CloudStorage) PhotoService {
	return &photoService{repo: r, storage: s}
}

func (s *photoService) UploadPhoto(ctx context.Context, file io.Reader, filename string) (*models.Photo, error) {
	// 1. 校验文件类型
	if !isValidImageType(filename) {
		return nil, ErrInvalidFileType
	}

	// 2. 上传到云存储
	storagePath, err := s.storage.Upload(ctx, file)
	if err != nil {
		return nil, fmt.Errorf("storage upload failed: %w", err)
	}

	// 3. 创建数据库记录
	photo := &models.Photo{
		OriginalFilename: filename,
		StoragePath:      storagePath,
		FileSize:         getFileSize(file),
	}

	if err := s.repo.CreatePhoto(ctx, photo); err != nil {
		// 事务补偿：删除已上传的文件
		go s.storage.Delete(ctx, storagePath)
		return nil, fmt.Errorf("failed to create record: %w", err)
	}

	return photo, nil
}
