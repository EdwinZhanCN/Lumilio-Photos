package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/edwinzhancn/lumen-sdk/pkg/client"
	lumenconfig "github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/edwinzhancn/lumen-sdk/pkg/discovery"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
	"go.uber.org/zap"

	"server/config"
	"server/internal/utils/imagesource"
)

// PoolStats mirrors client.PoolStats so callers don't need the SDK import.
type PoolStats = client.PoolStats

// LumenService is the contract for ML inference operations.
//
// It has no IsTaskAvailable, no WarmupTasks, no caching, and no retry.
// The connection pool is the single source of truth: if no healthy gRPC
// connection exists, Infer returns an error immediately.
// Retry is handled at the caller layer (River exponential backoff).
type LumenService interface {
	SemanticTextEmbed(ctx context.Context, text []byte) (*types.EmbeddingV1, error)
	SemanticTextEmbedFast(ctx context.Context, text []byte) (*types.EmbeddingV1, error)
	SemanticImageEmbed(ctx context.Context, imageData *imagesource.MLImage) (*types.EmbeddingV1, error)
	BioClipClassify(ctx context.Context, imageData *imagesource.MLImage, topK int) ([]types.Label, error)
	FaceRecognition(ctx context.Context, imageData *imagesource.MLImage) (*types.FaceV1, error)
	OCR(ctx context.Context, imageData *imagesource.MLImage) (*types.OCRV1, error)

	Start(ctx context.Context) error
	Close() error

	// PoolStats returns connection pool statistics for monitoring.
	PoolStats() PoolStats
	GetNodes() []*discovery.NodeInfo
	IsTaskAvailable(taskName string) bool
}

type lumenService struct {
	lumenClient *client.LumenClient
	logger      *zap.Logger
}

// NewLumenServiceFromAppConfig builds the LumenService from the app-level
// [lumen] configuration. A disabled integration (discovery off, or no backend
// configured) yields a no-op service whose inference methods return
// ErrLumenDisabled, so the server boots and media management degrades
// gracefully without external ML.
//
// The app-owned fields (enabled/mDNS/hub URL) always come from cfg, which the
// app config loader has already resolved with its TOML+env precedence.
// SDK-only tuning knobs (deployment ID, timeouts, chunking) remain
// env-configurable through the SDK's LUMEN_* variables.
func NewLumenServiceFromAppConfig(cfg config.LumenConfig, logger *zap.Logger) (LumenService, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	if !cfg.Enabled() {
		if cfg.DiscoveryEnabled {
			logger.Warn("lumen discovery is enabled but no backend is configured; ML features are disabled",
				zap.String("hint", "set [lumen] discovery_mdns_enabled = true or discovery_hub_url"))
		} else {
			logger.Info("lumen ML integration disabled by config; media management runs without external ML")
		}
		return NewDisabledLumenService(), nil
	}

	sdkCfg, err := buildLumenSDKConfig(cfg)
	if err != nil {
		return nil, err
	}
	return NewLumenService(sdkCfg, logger)
}

// buildLumenSDKConfig maps the app-level Lumen config onto the SDK config:
// SDK defaults, then LUMEN_* env for the SDK-only knobs, then the app-owned
// discovery fields on top.
func buildLumenSDKConfig(cfg config.LumenConfig) (*lumenconfig.Config, error) {
	sdkCfg := lumenconfig.DefaultConfig()
	if err := sdkCfg.LoadFromEnv(); err != nil {
		return nil, fmt.Errorf("load lumen env overrides: %w", err)
	}
	sdkCfg.Discovery.Enabled = cfg.DiscoveryEnabled
	sdkCfg.Discovery.MDNSEnabled = cfg.DiscoveryMDNSEnabled
	sdkCfg.Discovery.HubURL = strings.TrimSpace(cfg.DiscoveryHubURL)
	sdkCfg.Discovery.StaticNodes = cfg.StaticNodes()
	if err := sdkCfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid lumen sdk config: %w", err)
	}
	return sdkCfg, nil
}

func NewLumenService(cfg *lumenconfig.Config, logger *zap.Logger) (LumenService, error) {
	c, err := client.NewLumenClient(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("create lumen client: %w", err)
	}
	return &lumenService{
		lumenClient: c,
		logger:      logger,
	}, nil
}

// ErrLumenDisabled is returned by the disabled LumenService. Callers already
// treat any inference error as "ML unavailable", so it flows through the same
// degradation paths as a missing node.
var ErrLumenDisabled = errors.New("lumen ML integration is disabled")

// disabledLumenService keeps the server bootable when the Lumen integration is
// disabled by configuration: inference fails with ErrLumenDisabled, no task is
// ever available, and the pool reports zero nodes.
type disabledLumenService struct{}

// NewDisabledLumenService returns the no-op LumenService used when the Lumen
// integration is disabled by configuration.
func NewDisabledLumenService() LumenService { return disabledLumenService{} }

func (disabledLumenService) SemanticTextEmbed(context.Context, []byte) (*types.EmbeddingV1, error) {
	return nil, ErrLumenDisabled
}

func (disabledLumenService) SemanticTextEmbedFast(context.Context, []byte) (*types.EmbeddingV1, error) {
	return nil, ErrLumenDisabled
}

func (disabledLumenService) SemanticImageEmbed(context.Context, *imagesource.MLImage) (*types.EmbeddingV1, error) {
	return nil, ErrLumenDisabled
}

func (disabledLumenService) BioClipClassify(context.Context, *imagesource.MLImage, int) ([]types.Label, error) {
	return nil, ErrLumenDisabled
}

func (disabledLumenService) FaceRecognition(context.Context, *imagesource.MLImage) (*types.FaceV1, error) {
	return nil, ErrLumenDisabled
}

func (disabledLumenService) OCR(context.Context, *imagesource.MLImage) (*types.OCRV1, error) {
	return nil, ErrLumenDisabled
}

func (disabledLumenService) Start(context.Context) error { return nil }

func (disabledLumenService) Close() error { return nil }

func (disabledLumenService) PoolStats() PoolStats { return PoolStats{} }

func (disabledLumenService) GetNodes() []*discovery.NodeInfo { return nil }

func (disabledLumenService) IsTaskAvailable(string) bool { return false }

func (s *lumenService) Start(ctx context.Context) error { return s.lumenClient.Start(ctx) }

func (s *lumenService) Close() error { return s.lumenClient.Close() }

func (s *lumenService) PoolStats() PoolStats { return s.lumenClient.PoolStats() }

func (s *lumenService) GetNodes() []*discovery.NodeInfo { return s.lumenClient.GetNodes() }

func (s *lumenService) IsTaskAvailable(taskName string) bool {
	for _, n := range s.lumenClient.GetNodes() {
		if !n.IsActive() {
			continue
		}
		for _, t := range n.Tasks {
			if t.Name == taskName {
				return true
			}
		}
	}
	return false
}

func (s *lumenService) tensorImageRequest(ctx context.Context, taskName string, imageData *imagesource.MLImage) (*pb.InferRequest, bool) {
	if imageData == nil {
		return nil, false
	}
	contract, serviceName, ok := s.lumenClient.FindTaskContract(taskName)
	if !ok || !contract.HasTensorPath() {
		return nil, false
	}
	preprocessID := contract.TensorPreprocessID()
	preprocessor, ok := types.DefaultTensorPreprocessorRegistry().Lookup(preprocessID)
	if !ok {
		return nil, false
	}
	tensor, err := preprocessor.Preprocess(ctx, types.ImageInput{
		Encoded:     imageData.EncodedSource,
		PayloadMIME: "image/webp",
		Data:        imageData.Data,
		Width:       imageData.Width,
		Height:      imageData.Height,
		Channels:    imageData.Channels,
		Layout:      imageData.Layout,
		DType:       imageData.DType,
		ColorSpace:  imageData.ColorSpace,
	})
	if err != nil {
		s.logger.Debug("tensor preprocessor unavailable; falling back to raw Lumen path",
			zap.String("task", taskName),
			zap.String("preprocess_id", preprocessID),
			zap.Error(err),
		)
		return nil, false
	}
	return types.NewInferRequest(taskName).
		ForTensorInput(tensor.Payload, tensor.PayloadMIME, tensor.Descriptor).
		WithService(serviceName).
		Build(), true
}

// ---- Inference methods ----

func (s *lumenService) SemanticTextEmbed(ctx context.Context, text []byte) (*types.EmbeddingV1, error) {
	// The service is set to be empty
	req := types.NewInferRequest("semantic_text_embed").
		ForSemanticTextEmbed(string(text)).
		Build()

	resp, err := s.lumenClient.Infer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("semantic text embed: %w", err)
	}
	embedResp, err := types.ParseInferResponse(resp).AsEmbeddingResponse()
	if err != nil {
		return nil, fmt.Errorf("parse embedding response: %w", err)
	}
	s.logger.Debug("semantic text embedding", zap.String("model", embedResp.ModelID), zap.Int("dim", len(embedResp.Vector)))
	return embedResp, nil
}

func (s *lumenService) SemanticTextEmbedFast(ctx context.Context, text []byte) (*types.EmbeddingV1, error) {
	return s.SemanticTextEmbed(ctx, text) // same path; caller controls timeout via ctx
}

func (s *lumenService) SemanticImageEmbed(ctx context.Context, imageData *imagesource.MLImage) (*types.EmbeddingV1, error) {
	req, ok := s.tensorImageRequest(ctx, types.TaskSemanticImageEmbed, imageData)
	if !ok {
		req = types.NewInferRequest(types.TaskSemanticImageEmbed).
			ForSemanticImageEmbed(imageData.EncodedSource, "image/webp").
			Build()
	}

	resp, err := s.lumenClient.Infer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("semantic image embed: %w", err)
	}
	embedResp, err := types.ParseInferResponse(resp).AsEmbeddingResponse()
	if err != nil {
		return nil, fmt.Errorf("parse embedding response: %w", err)
	}
	s.logger.Debug("CLIP image embedding", zap.String("model", embedResp.ModelID), zap.Int("dim", len(embedResp.Vector)))
	return embedResp, nil
}

func (s *lumenService) BioClipClassify(ctx context.Context, imageData *imagesource.MLImage, topK int) ([]types.Label, error) {
	req, ok := s.tensorImageRequest(ctx, types.TaskBioCLIPClassify, imageData)
	if ok {
		if topK > 0 {
			req.Meta[types.MetaTopK] = strconv.Itoa(topK)
		}
	} else {
		req = types.NewInferRequest(types.TaskBioCLIPClassify).
			ForBioCLIPClassify(imageData.EncodedSource, "image/webp", topK).
			Build()
	}

	resp, err := s.lumenClient.Infer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("bioclip classify: %w", err)
	}
	classifyResp, err := types.ParseInferResponse(resp).AsClassificationResponse()
	if err != nil {
		return nil, fmt.Errorf("parse classification response: %w", err)
	}
	topLabels := classifyResp.TopK(topK)
	s.logger.Debug("bioclip classification", zap.String("model", classifyResp.ModelID), zap.Int("labels", len(topLabels)))
	return topLabels, nil
}

func (s *lumenService) FaceRecognition(ctx context.Context, imageData *imagesource.MLImage) (*types.FaceV1, error) {
	req := types.NewInferRequest(types.TaskFaceRecognition).
		ForFaceRecognitionRaw(imageData.EncodedSource, "image/webp").
		Build()

	resp, err := s.lumenClient.Infer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("face recognition: %w", err)
	}
	faceResp, err := types.ParseInferResponse(resp).AsFaceResponse()
	if err != nil {
		return nil, fmt.Errorf("parse face response: %w", err)
	}
	s.logger.Debug("face detection", zap.String("model", faceResp.ModelID), zap.Int("faces", len(faceResp.Faces)))
	return faceResp, nil
}

func (s *lumenService) OCR(ctx context.Context, imageData *imagesource.MLImage) (*types.OCRV1, error) {
	req := types.NewInferRequest(types.TaskOCR).
		ForOCRRaw(imageData.EncodedSource, "image/webp").
		Build()

	resp, err := s.lumenClient.Infer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ocr: %w", err)
	}
	ocrResp, err := types.ParseInferResponse(resp).AsOCRResponse()
	if err != nil {
		return nil, fmt.Errorf("parse OCR response: %w", err)
	}
	s.logger.Debug("ocr", zap.String("model", ocrResp.ModelID), zap.Int("items", len(ocrResp.Items)))
	return ocrResp, nil
}
