package classification

import (
	"context"
	"fmt"
	"io"
	"log"
	"server/internal/service"
	"strings"

	"github.com/google/uuid"
)

// ClassificationService handles image classification operations
type ClassificationService struct {
	classifier     *ImageClassifier
	photoService   service.PhotoService
	modelPath      string
	classIndexPath string
	topN           int
}

// NewClassificationService creates a new classification service
func NewClassificationService(photoService service.PhotoService, modelPath, classIndexPath string, topN int) (*ClassificationService, error) {
	// Create the classifier
	classifier, err := NewImageClassifier(modelPath, classIndexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create image classifier: %w", err)
	}

	return &ClassificationService{
		classifier:     classifier,
		photoService:   photoService,
		modelPath:      modelPath,
		classIndexPath: classIndexPath,
		topN:           topN,
	}, nil
}

// ClassifyImage classifies an image and stores the results as tags
func (s *ClassificationService) ClassifyImage(ctx context.Context, photoID uuid.UUID, imageReader io.Reader) error {
	// Classify the image
	results, err := s.classifier.ClassifyFromReader(ctx, imageReader, s.topN)
	if err != nil {
		return fmt.Errorf("failed to classify image: %w", err)
	}

	// Store the results as tags
	for _, result := range results {
		// Create or get the tag
		tagName := formatTagName(result.ClassName)

		// Add the tag to the photo with confidence score
		if err := s.photoService.AddTagToPhoto(ctx, photoID, 0, result.Confidence, "ai"); err != nil {
			log.Printf("Failed to add tag '%s' to photo %s: %v", tagName, photoID, err)
			continue
		}

		log.Printf("Added classification tag '%s' with confidence %.2f to photo %s", tagName, result.Confidence, photoID)
	}

	return nil
}

// ClassifyImageFromStorage classifies an image from storage
func (s *ClassificationService) ClassifyImageFromStorage(ctx context.Context, photoID uuid.UUID, storagePath string, storage service.CloudStorage) error {
	// Get the image from storage
	file, err := storage.Get(ctx, storagePath)
	if err != nil {
		return fmt.Errorf("failed to get image from storage: %w", err)
	}
	defer file.Close()

	// Classify the image
	return s.ClassifyImage(ctx, photoID, file)
}

// formatTagName formats a class name into a tag name
func formatTagName(className string) string {
	// Remove any text in parentheses
	if idx := strings.Index(className, "("); idx != -1 {
		className = strings.TrimSpace(className[:idx])
	}

	// Convert to lowercase and replace spaces with underscores
	return strings.ToLower(strings.ReplaceAll(className, " ", "_"))
}

// Close releases resources used by the classification service
func (s *ClassificationService) Close() {
	if s.classifier != nil {
		s.classifier.Close()
	}
}
