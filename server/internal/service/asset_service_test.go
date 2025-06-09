package service

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"server/internal/models"
)

// MockCloudStorage is a mock implementation of CloudStorage for testing
type MockCloudStorage struct {
	mock.Mock
}

func (m *MockCloudStorage) Upload(ctx context.Context, file io.Reader) (string, error) {
	args := m.Called(ctx, file)
	return args.String(0), args.Error(1)
}

func (m *MockCloudStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	args := m.Called(ctx, path)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockCloudStorage) Delete(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

// MockAssetRepository is a mock implementation of AssetRepository for testing
type MockAssetRepository struct {
	mock.Mock
}

func (m *MockAssetRepository) CreateAsset(ctx context.Context, asset *models.Asset) error {
	args := m.Called(ctx, asset)
	return args.Error(0)
}

func (m *MockAssetRepository) GetByID(ctx context.Context, assetID uuid.UUID) (*models.Asset, error) {
	args := m.Called(ctx, assetID)
	return args.Get(0).(*models.Asset), args.Error(1)
}

func (m *MockAssetRepository) GetByType(ctx context.Context, assetType models.AssetType, limit, offset int) ([]*models.Asset, error) {
	args := m.Called(ctx, assetType, limit, offset)
	return args.Get(0).([]*models.Asset), args.Error(1)
}

func (m *MockAssetRepository) GetByOwner(ctx context.Context, ownerID int, limit, offset int) ([]*models.Asset, error) {
	args := m.Called(ctx, ownerID, limit, offset)
	return args.Get(0).([]*models.Asset), args.Error(1)
}

func (m *MockAssetRepository) UpdateAsset(ctx context.Context, asset *models.Asset) error {
	args := m.Called(ctx, asset)
	return args.Error(0)
}

func (m *MockAssetRepository) DeleteAsset(ctx context.Context, assetID uuid.UUID) error {
	args := m.Called(ctx, assetID)
	return args.Error(0)
}

func (m *MockAssetRepository) SearchAssets(ctx context.Context, query string, assetType *models.AssetType, limit, offset int) ([]*models.Asset, error) {
	args := m.Called(ctx, query, assetType, limit, offset)
	return args.Get(0).([]*models.Asset), args.Error(1)
}

func (m *MockAssetRepository) GetAssetsByHash(ctx context.Context, hash string) ([]*models.Asset, error) {
	args := m.Called(ctx, hash)
	return args.Get(0).([]*models.Asset), args.Error(1)
}

func (m *MockAssetRepository) AddTagToAsset(ctx context.Context, assetID uuid.UUID, tagID int, confidence float32, source string) error {
	args := m.Called(ctx, assetID, tagID, confidence, source)
	return args.Error(0)
}

func (m *MockAssetRepository) RemoveTagFromAsset(ctx context.Context, assetID uuid.UUID, tagID int) error {
	args := m.Called(ctx, assetID, tagID)
	return args.Error(0)
}

func (m *MockAssetRepository) AddAssetToAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error {
	args := m.Called(ctx, assetID, albumID)
	return args.Error(0)
}

func (m *MockAssetRepository) RemoveAssetFromAlbum(ctx context.Context, assetID uuid.UUID, albumID int) error {
	args := m.Called(ctx, assetID, albumID)
	return args.Error(0)
}

func (m *MockAssetRepository) CreateThumbnail(ctx context.Context, thumbnail *models.Thumbnail) error {
	args := m.Called(ctx, thumbnail)
	return args.Error(0)
}

func (m *MockAssetRepository) UpdateAssetMetadata(ctx context.Context, assetID uuid.UUID, metadata models.SpecificMetadata) error {
	args := m.Called(ctx, assetID, metadata)
	return args.Error(0)
}

func TestAssetService_UploadAsset(t *testing.T) {
	mockRepo := new(MockAssetRepository)
	mockStorage := new(MockCloudStorage)
	service := NewAssetService(mockRepo, mockStorage)

	ctx := context.Background()
	filename := "test.jpg"
	fileSize := int64(1024)
	ownerID := 1
	fileContent := "test file content"
	file := strings.NewReader(fileContent)

	// Mock storage upload
	mockStorage.On("Upload", ctx, mock.AnythingOfType("*strings.Reader")).Return("/storage/path/test.jpg", nil)

	// Mock repository methods
	mockRepo.On("GetAssetsByHash", ctx, mock.AnythingOfType("string")).Return([]*models.Asset{}, nil)
	mockRepo.On("CreateAsset", ctx, mock.AnythingOfType("*models.Asset")).Return(nil)

	asset, err := service.UploadAsset(ctx, file, filename, fileSize, &ownerID)

	assert.NoError(t, err)
	assert.NotNil(t, asset)
	assert.Equal(t, models.AssetTypePhoto, asset.Type)
	assert.Equal(t, filename, asset.OriginalFilename)
	assert.Equal(t, "/storage/path/test.jpg", asset.StoragePath)
	assert.Equal(t, "image/jpeg", asset.MimeType)
	assert.Equal(t, fileSize, asset.FileSize)
	assert.Equal(t, &ownerID, asset.OwnerID)

	mockRepo.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestAssetService_GetAssetsByOwner(t *testing.T) {
	mockRepo := new(MockAssetRepository)
	mockStorage := new(MockCloudStorage)
	service := NewAssetService(mockRepo, mockStorage)

	ctx := context.Background()
	ownerID := 1
	limit := 10
	offset := 0

	expectedAssets := []*models.Asset{
		{
			AssetID:          uuid.New(),
			OwnerID:          &ownerID,
			Type:             models.AssetTypePhoto,
			OriginalFilename: "test1.jpg",
			StoragePath:      "/path/to/test1.jpg",
			MimeType:         "image/jpeg",
			FileSize:         1024,
			UploadTime:       time.Now(),
		},
		{
			AssetID:          uuid.New(),
			OwnerID:          &ownerID,
			Type:             models.AssetTypePhoto,
			OriginalFilename: "test2.jpg",
			StoragePath:      "/path/to/test2.jpg",
			MimeType:         "image/jpeg",
			FileSize:         2048,
			UploadTime:       time.Now(),
		},
	}

	mockRepo.On("GetByOwner", ctx, ownerID, limit, offset).Return(expectedAssets, nil)

	assets, err := service.GetAssetsByOwner(ctx, ownerID, limit, offset)

	assert.NoError(t, err)
	assert.Equal(t, expectedAssets, assets)
	assert.Len(t, assets, 2)

	mockRepo.AssertExpectations(t)
}
