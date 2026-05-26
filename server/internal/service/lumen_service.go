package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"go.uber.org/zap"

	"server/internal/utils/imagesource"
)

// LumenService interface defines the contract for Lumen AI operations
type LumenService interface {
	SemanticTextEmbed(ctx context.Context, text []byte) (*types.EmbeddingV1, error)
	SemanticTextEmbedFast(ctx context.Context, text []byte) (*types.EmbeddingV1, error)
	SemanticImageEmbed(ctx context.Context, imageData *imagesource.MLImage) (*types.EmbeddingV1, error)
	BioClipClassify(ctx context.Context, imageData *imagesource.MLImage, topK int) ([]types.Label, error)
	FaceRecognition(ctx context.Context, imageData *imagesource.MLImage) (*types.FaceV1, error)
	OCR(ctx context.Context, imageData *imagesource.MLImage) (*types.OCRV1, error)
	GetAvailableModels(ctx context.Context) ([]*discovery.NodeInfo, error)
	WarmupTasks(ctx context.Context, tasks []string) map[string]bool
	IsTaskAvailable(taskName string) bool
	Start(ctx context.Context) error
	Close() error
}

type lumenService struct {
	lumenClient *client.LumenClient
	logger      *zap.Logger

	mu         sync.RWMutex
	availCache map[string]taskAvailability
	cacheTTL   time.Duration
}

type taskAvailability struct {
	available bool
	checkedAt time.Time
}

func NewLumenService(cfg *config.Config, logger *zap.Logger) (LumenService, error) {
	lumenClient, err := client.NewLumenClient(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create lumen client: %w", err)
	}

	return &lumenService{
		lumenClient: lumenClient,
		logger:      logger,
		availCache:  make(map[string]taskAvailability),
		cacheTTL:    15 * time.Second,
	}, nil
}

// Start the service
func (s *lumenService) Start(ctx context.Context) error {
	return s.lumenClient.Start(ctx)
}

// Close Stop the service
func (s *lumenService) Close() error {
	return s.lumenClient.Close()
}

// SemanticTextEmbed generates CLIP embeddings for the given image data using the specified model.
func (s *lumenService) SemanticTextEmbed(ctx context.Context, text []byte) (*types.EmbeddingV1, error) {
	return s.semanticTextEmbedWithRetry(ctx, text, 5*time.Second, 3)
}

func (s *lumenService) SemanticTextEmbedFast(ctx context.Context, text []byte) (*types.EmbeddingV1, error) {
	return s.semanticTextEmbedWithRetry(ctx, text, 750*time.Millisecond, 0)
}

func (s *lumenService) semanticTextEmbedWithRetry(ctx context.Context, text []byte, maxWait time.Duration, maxRetries int) (*types.EmbeddingV1, error) {
	req := types.NewInferRequest("semantic_text_embed").
		ForSemanticTextEmbed(string(text), types.ServiceSigLIP).
		Build()

	resp, err := s.lumenClient.InferWithRetry(ctx, req,
		client.WithMaxWaitTime(maxWait),
		client.WithMaxRetries(maxRetries))
	if err != nil {
		return nil, fmt.Errorf("failed to infer embedding: %w", err)
	}

	embedResp, err := types.ParseInferResponse(resp).AsEmbeddingResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedding response: %w", err)
	}

	s.logger.Debug("Generated CLIP embedding",
		zap.String("model", embedResp.ModelID),
		zap.Int("dimensions", len(embedResp.Vector)))

	return embedResp, nil
}

func (s *lumenService) SemanticImageEmbed(ctx context.Context, imageData *imagesource.MLImage) (*types.EmbeddingV1, error) {
	req := types.NewInferRequest("semantic_image_embed").
		ForSemanticImageEmbed(imageData.EncodedSource, "image/webp", types.ServiceSigLIP).
		Build()

	resp, err := s.lumenClient.InferWithRetry(ctx, req,
		client.WithMaxWaitTime(5*time.Second),
		client.WithMaxRetries(3))
	if err != nil {
		return nil, fmt.Errorf("failed to infer embedding: %w", err)
	}

	embedResp, err := types.ParseInferResponse(resp).AsEmbeddingResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedding response: %w", err)
	}

	s.logger.Debug("Generated CLIP image embedding",
		zap.String("model", embedResp.ModelID),
		zap.Int("dimensions", len(embedResp.Vector)))

	return embedResp, nil
}

func (s *lumenService) BioClipClassify(ctx context.Context, imageData *imagesource.MLImage, topK int) ([]types.Label, error) {
	return s.classifyImage(ctx, imageData, "bioclip_classify", topK, 10*time.Second)
}

func (s *lumenService) classifyImage(ctx context.Context, imageData *imagesource.MLImage, taskName string, topK int, maxWait time.Duration) ([]types.Label, error) {
	req := types.NewInferRequest(taskName).
		ForBioCLIPClassify(imageData.EncodedSource, "image/webp", topK).
		Build()

	resp, err := s.lumenClient.InferWithRetry(ctx, req,
		client.WithMaxWaitTime(maxWait),
		client.WithMaxRetries(3))
	if err != nil {
		return nil, fmt.Errorf("failed to infer classification: %w", err)
	}

	classifyResp, err := types.ParseInferResponse(resp).AsClassificationResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse classification response: %w", err)
	}

	topLabels := classifyResp.TopK(topK)
	s.logger.Debug("Generated image classification",
		zap.String("task", taskName),
		zap.String("model", classifyResp.ModelID),
		zap.Int("top_labels", len(topLabels)))

	return topLabels, nil
}

// FaceRecognition Face detection and embedding, returns FaceV1 response
func (s *lumenService) FaceRecognition(ctx context.Context, imageData *imagesource.MLImage) (*types.FaceV1, error) {
	req := types.NewInferRequest("face").
		ForFaceRecognitionRaw(imageData.EncodedSource, "image/webp").
		Build()

	resp, err := s.lumenClient.InferWithRetry(ctx, req,
		client.WithMaxWaitTime(10*time.Second), // Longer timeout for multiple faces
		client.WithMaxRetries(3))
	if err != nil {
		return nil, fmt.Errorf("failed to infer face embedding: %w", err)
	}

	faceResp, err := types.ParseInferResponse(resp).AsFaceResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse face response: %w", err)
	}

	s.logger.Debug("Face detection completed",
		zap.String("model", faceResp.ModelID),
		zap.Int("face_count", faceResp.Count),
		zap.Int("detected_faces", len(faceResp.Faces)))

	return faceResp, nil
}

func (s *lumenService) OCR(ctx context.Context, imageData *imagesource.MLImage) (*types.OCRV1, error) {
	req := types.NewInferRequest("ocr").
		ForOCRRaw(imageData.EncodedSource, "image/webp").
		Build()

	resp, err := s.lumenClient.InferWithRetry(ctx, req,
		client.WithMaxWaitTime(10*time.Second),
		client.WithMaxRetries(3))
	if err != nil {
		return nil, fmt.Errorf("failed to infer OCR: %w", err)
	}
	ocrResp, err := types.ParseInferResponse(resp).AsOCRResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse OCR response: %w", err)
	}

	s.logger.Debug("Generated OCR results",
		zap.String("model", ocrResp.ModelID),
		zap.Int("items", len(ocrResp.Items)))

	return ocrResp, nil
}

// GetAvailableModels Get available models from discovered servers
func (s *lumenService) GetAvailableModels(ctx context.Context) ([]*discovery.NodeInfo, error) {
	nodes := s.lumenClient.GetNodes()
	return nodes, nil
}

func (s *lumenService) WarmupTasks(ctx context.Context, tasks []string) map[string]bool {
	results := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		results[task] = s.updateTaskAvailability(task)
	}
	return results
}

func (s *lumenService) IsTaskAvailable(taskName string) bool {
	s.mu.RLock()
	entry, ok := s.availCache[taskName]
	ttl := s.cacheTTL
	s.mu.RUnlock()

	if ok && time.Since(entry.checkedAt) < ttl {
		return entry.available
	}
	return s.updateTaskAvailability(taskName)
}

func (s *lumenService) updateTaskAvailability(taskName string) bool {
	available := s.lumenClient.IsTaskAvailable(taskName)
	s.mu.Lock()
	s.availCache[taskName] = taskAvailability{
		available: available,
		checkedAt: time.Now(),
	}
	s.mu.Unlock()
	return available
}
