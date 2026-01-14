package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"go.uber.org/zap"
)

// LumenService interface defines the contract for Lumen AI operations
type LumenService interface {
	ClipTextEmbed(ctx context.Context, text []byte) (*types.EmbeddingV1, error)
	ClipImageEmbed(ctx context.Context, imageData []byte) (*types.EmbeddingV1, error)
	BioClipClassify(ctx context.Context, imageData []byte, topK int) ([]types.Label, error)
	FaceDetectEmbed(ctx context.Context, imageData []byte) (*types.FaceV1, error)
	OCR(ctx context.Context, imageData []byte) (*types.OCRV1, error)
	VLMCaption(ctx context.Context, imageData []byte) (string, error)
	VLMCaptionWithPrompt(ctx context.Context, imageData []byte, prompt string) (string, error)
	VLMCaptionWithMetadata(ctx context.Context, imageData []byte, prompt string) (*types.TextGenerationV1, error)
	GetAvailableModels(ctx context.Context) ([]*client.NodeInfo, error)
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

// ClipTextEmbed generates CLIP embeddings for the given image data using the specified model.
func (s *lumenService) ClipTextEmbed(ctx context.Context, text []byte) (*types.EmbeddingV1, error) {
	embedReq, err := types.NewEmbeddingRequest(text)
	if err != nil {
		return nil, fmt.Errorf("failed to create text embedding request: %w", err)
	}

	req := types.NewInferRequest("clip_text_embed").
		ForEmbedding(embedReq, "clip_text_embed").
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

	s.logger.Info("Generated CLIP embedding",
		zap.String("model", embedResp.ModelID),
		zap.Int("dimensions", len(embedResp.Vector)))

	return embedResp, nil
}

func (s *lumenService) ClipImageEmbed(ctx context.Context, imageData []byte) (*types.EmbeddingV1, error) {
	embedReq, err := types.NewEmbeddingRequest(imageData)
	if err != nil {
		return nil, fmt.Errorf("failed to create image embedding request: %w", err)
	}

	req := types.NewInferRequest("clip_image_embed").
		ForEmbedding(embedReq, "clip_image_embed").
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

	s.logger.Info("Generated CLIP image embedding",
		zap.String("model", embedResp.ModelID),
		zap.Int("dimensions", len(embedResp.Vector)))

	return embedResp, nil
}

func (s *lumenService) BioClipClassify(ctx context.Context, imageData []byte, topK int) ([]types.Label, error) {
	classifyReq, err := types.NewClassificationRequest(imageData)
	if err != nil {
		return nil, fmt.Errorf("failed to create classification request: %w", err)
	}

	req := types.NewInferRequest("bioclip_classify").
		ForClassification(classifyReq, "bioclip_classify").
		Build()
	resp, err := s.lumenClient.InferWithRetry(ctx, req,
		client.WithMaxWaitTime(10*time.Second),
		client.WithMaxRetries(3))
	if err != nil {
		return nil, fmt.Errorf("failed to infer classification: %w", err)
	}

	classifyResp, err := types.ParseInferResponse(resp).AsClassificationResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse classification response: %w", err)
	}

	topLabels := classifyResp.TopK(topK)
	s.logger.Info("Generated BioCLIP classification",
		zap.String("model", classifyResp.ModelID),
		zap.Int("top_labels", len(topLabels)))

	return topLabels, nil
}

// FaceDetectEmbed Face detection and embedding, returns FaceV1 response
func (s *lumenService) FaceDetectEmbed(ctx context.Context, imageData []byte) (*types.FaceV1, error) {
	faceReq, err := types.NewFaceRecognitionRequest(imageData,
		types.WithMaxFaces(10),                      // Allow multiple faces
		types.WithDetectionConfidenceThreshold(0.7), // Minimum confidence
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create face request: %w", err)
	}

	req := types.NewInferRequest("face_detect_and_embed").
		ForFaceDetection(faceReq, "face_detect_and_embed").
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

	s.logger.Info("Face detection completed",
		zap.String("model", faceResp.ModelID),
		zap.Int("face_count", faceResp.Count),
		zap.Int("detected_faces", len(faceResp.Faces)))

	return faceResp, nil
}

func (s *lumenService) OCR(ctx context.Context, imageData []byte) (*types.OCRV1, error) {
	ocrReq, err := types.NewOCRRequest(imageData)
	if err != nil {
		return nil, fmt.Errorf("failed to create OCR request: %w", err)
	}
	req := types.NewInferRequest("ocr").
		ForOCR(ocrReq, "ocr").
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

	s.logger.Info("Generated OCR results",
		zap.String("model", ocrResp.ModelID),
		zap.Int("items", len(ocrResp.Items)))

	return ocrResp, nil
}

func (s *lumenService) VLMCaption(ctx context.Context, imageData []byte) (string, error) {
	return s.VLMCaptionWithPrompt(ctx, imageData, "<image>Describe this image in detail.")
}

func (s *lumenService) VLMCaptionWithPrompt(ctx context.Context, imageData []byte, prompt string) (string, error) {
	captionReq, err := types.NewImageTextGenerationRequest(imageData,
		types.WithPrompt(prompt),
		types.WithMaxTokens(512),
		types.WithTemperature(0.7),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create caption request: %w", err)
	}
	req := types.NewInferRequest("vlm_generate").
		ForImageTextGeneration(captionReq, "vlm_generate").
		Build()
	resp, err := s.lumenClient.InferWithRetry(ctx, req,
		client.WithMaxWaitTime(10*time.Second),
		client.WithMaxRetries(3))
	if err != nil {
		return "", fmt.Errorf("failed to infer caption: %w", err)
	}
	captionResp, err := types.ParseInferResponse(resp).AsTextGenerationResponse()
	if err != nil {
		return "", fmt.Errorf("failed to parse caption response: %w", err)
	}

	s.logger.Info("Generated VLM caption",
		zap.String("model", captionResp.ModelID),
		zap.Int("tokens", captionResp.GeneratedTokens),
		zap.String("finished_reason", captionResp.FinishReason))

	return captionResp.Text, nil
}

// VLMCaptionWithMetadata generates VLM caption with detailed metadata
func (s *lumenService) VLMCaptionWithMetadata(ctx context.Context, imageData []byte, prompt string) (*types.TextGenerationV1, error) {
	captionReq, err := types.NewImageTextGenerationRequest(imageData,
		types.WithPrompt(prompt),
		types.WithMaxTokens(512),
		types.WithTemperature(0.7),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create caption request: %w", err)
	}
	req := types.NewInferRequest("vlm_generate").
		ForImageTextGeneration(captionReq, "vlm_generate").
		Build()
	resp, err := s.lumenClient.InferWithRetry(ctx, req,
		client.WithMaxWaitTime(10*time.Second),
		client.WithMaxRetries(3))
	if err != nil {
		return nil, fmt.Errorf("failed to infer caption: %w", err)
	}
	captionResp, err := types.ParseInferResponse(resp).AsTextGenerationResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse caption response: %w", err)
	}

	s.logger.Info("Generated VLM caption",
		zap.String("model", captionResp.ModelID),
		zap.Int("tokens", captionResp.GeneratedTokens),
		zap.String("finished_reason", captionResp.FinishReason))

	return captionResp, nil
}

// GetAvailableModels Get available models from discovered servers
func (s *lumenService) GetAvailableModels(ctx context.Context) ([]*client.NodeInfo, error) {
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
