package service

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	"github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
	"go.uber.org/zap"

	"server/internal/utils/imagesource"
)

// LumenService interface defines the contract for Lumen AI operations
type LumenService interface {
	ClipTextEmbed(ctx context.Context, text []byte) (*types.EmbeddingV1, error)
	ClipTextEmbedFast(ctx context.Context, text []byte) (*types.EmbeddingV1, error)
	ClipImageEmbed(ctx context.Context, imageData *imagesource.MLImage) (*types.EmbeddingV1, error)
	BioClipClassify(ctx context.Context, imageData *imagesource.MLImage, topK int) ([]types.Label, error)
	FaceDetectEmbed(ctx context.Context, imageData *imagesource.MLImage) (*types.FaceV1, error)
	OCR(ctx context.Context, imageData *imagesource.MLImage) (*types.OCRV1, error)
	VLMCaption(ctx context.Context, imageData *imagesource.MLImage) (string, error)
	VLMCaptionWithPrompt(ctx context.Context, imageData *imagesource.MLImage, prompt string) (string, error)
	VLMCaptionWithMetadata(ctx context.Context, imageData *imagesource.MLImage, prompt string) (*types.TextGenerationV1, error)
	GetAvailableModels(ctx context.Context) ([]*discovery.NodeInfo, error)
	WarmupTasks(ctx context.Context, tasks []string) map[string]bool
	IsTaskAvailable(taskName string) bool
	Start(ctx context.Context) error
	Close() error
}

const rgbTensorPayloadMime = "application/x.rgb+uint8"

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
	return s.clipTextEmbedWithRetry(ctx, text, 5*time.Second, 3)
}

func (s *lumenService) ClipTextEmbedFast(ctx context.Context, text []byte) (*types.EmbeddingV1, error) {
	return s.clipTextEmbedWithRetry(ctx, text, 750*time.Millisecond, 0)
}

func (s *lumenService) clipTextEmbedWithRetry(ctx context.Context, text []byte, maxWait time.Duration, maxRetries int) (*types.EmbeddingV1, error) {
	embedReq, err := types.NewEmbeddingRequest(text)
	if err != nil {
		return nil, fmt.Errorf("failed to create text embedding request: %w", err)
	}

	req := types.NewInferRequest("clip_text_embed").
		ForEmbedding(embedReq, "clip_text_embed").
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

func (s *lumenService) ClipImageEmbed(ctx context.Context, imageData *imagesource.MLImage) (*types.EmbeddingV1, error) {
	req, err := newRGBTensorInferRequest("clip_image_embed", imageData)
	if err != nil {
		return nil, fmt.Errorf("failed to create image embedding request: %w", err)
	}

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
	req, err := newRGBTensorInferRequest(taskName, imageData)
	if err != nil {
		return nil, fmt.Errorf("failed to create classification request: %w", err)
	}

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

// FaceDetectEmbed Face detection and embedding, returns FaceV1 response
func (s *lumenService) FaceDetectEmbed(ctx context.Context, imageData *imagesource.MLImage) (*types.FaceV1, error) {
	req, err := newRGBTensorInferRequest("face_detect_and_embed", imageData)
	if err != nil {
		return nil, fmt.Errorf("failed to create face request: %w", err)
	}
	req.Meta["max_faces"] = "10"
	req.Meta["detection_confidence_threshold"] = "0.700"

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
	req, err := newRGBTensorInferRequest("ocr", imageData)
	if err != nil {
		return nil, fmt.Errorf("failed to create OCR request: %w", err)
	}

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

func (s *lumenService) VLMCaption(ctx context.Context, imageData *imagesource.MLImage) (string, error) {
	return s.VLMCaptionWithPrompt(ctx, imageData, "<image>Describe this image in detail.")
}

func (s *lumenService) VLMCaptionWithPrompt(ctx context.Context, imageData *imagesource.MLImage, prompt string) (string, error) {
	req, err := newVLMCaptionTensorInferRequest(imageData, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to create caption request: %w", err)
	}
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

	s.logger.Debug("Generated VLM caption",
		zap.String("model", captionResp.ModelID),
		zap.Int("tokens", captionResp.GeneratedTokens),
		zap.String("finished_reason", captionResp.FinishReason))

	return captionResp.Text, nil
}

// VLMCaptionWithMetadata generates VLM caption with detailed metadata
func (s *lumenService) VLMCaptionWithMetadata(ctx context.Context, imageData *imagesource.MLImage, prompt string) (*types.TextGenerationV1, error) {
	req, err := newVLMCaptionTensorInferRequest(imageData, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to create caption request: %w", err)
	}
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

	s.logger.Debug("Generated VLM caption",
		zap.String("model", captionResp.ModelID),
		zap.Int("tokens", captionResp.GeneratedTokens),
		zap.String("finished_reason", captionResp.FinishReason))

	return captionResp, nil
}

func newVLMCaptionTensorInferRequest(imageData *imagesource.MLImage, prompt string) (*pb.InferRequest, error) {
	req, err := newRGBTensorInferRequest("vlm_generate", imageData)
	if err != nil {
		return nil, err
	}
	req.Meta["prompt"] = prompt
	req.Meta["max_new_tokens"] = "512"
	req.Meta["temperature"] = "0.7"
	req.Meta["top_p"] = "1.0"
	req.Meta["repetition_penalty"] = "1.0"
	req.Meta["do_sample"] = "false"
	req.Meta["add_generation_prompt"] = "true"
	return req, nil
}

func newRGBTensorInferRequest(task string, imageData *imagesource.MLImage) (*pb.InferRequest, error) {
	if imageData == nil {
		return nil, fmt.Errorf("nil ml image")
	}
	if imageData.Width <= 0 || imageData.Height <= 0 || imageData.Channels != 3 {
		return nil, fmt.Errorf("invalid tensor shape: %dx%dx%d", imageData.Width, imageData.Height, imageData.Channels)
	}
	if len(imageData.Data) != imageData.Width*imageData.Height*imageData.Channels {
		return nil, fmt.Errorf("tensor data length %d does not match shape %dx%dx%d", len(imageData.Data), imageData.Width, imageData.Height, imageData.Channels)
	}

	layout := imageData.Layout
	if layout == "" {
		layout = "HWC"
	}
	dtype := imageData.DType
	if dtype == "" {
		dtype = "uint8"
	}
	colorSpace := imageData.ColorSpace
	if colorSpace == "" {
		colorSpace = "RGB"
	}

	req := types.NewInferRequest(task).
		WithMeta("input_format", "rgb_tensor").
		WithMeta("tensor_dtype", dtype).
		WithMeta("tensor_layout", layout).
		WithMeta("tensor_color_space", colorSpace).
		WithMeta("tensor_width", strconv.Itoa(imageData.Width)).
		WithMeta("tensor_height", strconv.Itoa(imageData.Height)).
		WithMeta("tensor_channels", strconv.Itoa(imageData.Channels)).
		Build()
	req.Payload = imageData.Data
	req.PayloadMime = rgbTensorPayloadMime
	return req, nil
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
