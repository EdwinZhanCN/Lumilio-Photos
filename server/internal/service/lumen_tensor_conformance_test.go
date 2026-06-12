package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"server/internal/utils/imagesource"
)

// Opt-in end-to-end conformance check for the Photos tensor fast path.
//
// It validates that the vips-based ML preprocessing (imagesource → MLImage.Data
// → SDK decoded-input tensor path) produces embeddings consistent with
// Hub-owned raw preprocessing of the same source bytes. This is the guard
// against preprocessing drift across Photos, Lumen-SDK, and Lumen-Hub.
//
// Requires a running Lumen-Hub with the SigLIP model loaded:
//
//	LUMILIO_LUMEN_CONFORMANCE_ADDR=127.0.0.1:50051 \
//	LUMILIO_LUMEN_CONFORMANCE_IMAGES=/path/to/jpeg/dir \
//	go test ./internal/service -run TestLumenTensorPathConformance -v
const (
	lumenConformanceAddrEnv   = "LUMILIO_LUMEN_CONFORMANCE_ADDR"
	lumenConformanceImagesEnv = "LUMILIO_LUMEN_CONFORMANCE_IMAGES"
)

func TestLumenTensorPathConformance(t *testing.T) {
	addr := os.Getenv(lumenConformanceAddrEnv)
	if addr == "" {
		t.Skipf("set %s to run the Lumen tensor conformance test", lumenConformanceAddrEnv)
	}
	imageDir := os.Getenv(lumenConformanceImagesEnv)
	if imageDir == "" {
		t.Skipf("set %s to a directory of test images", lumenConformanceImagesEnv)
	}

	paths := conformanceImages(t, imageDir, 12)
	if len(paths) == 0 {
		t.Fatalf("no images found in %s", imageDir)
	}

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial hub: %v", err)
	}
	defer conn.Close()
	client := pb.NewInferenceClient(conn)
	ctx := context.Background()

	preprocessID, serviceName := conformanceContract(ctx, t, client)
	preprocessor, ok := types.DefaultTensorPreprocessorRegistry().Lookup(preprocessID)
	if !ok {
		t.Fatalf("SDK does not know preprocess id %q", preprocessID)
	}

	worstDownscale := 1.0
	worstUpscale := 1.0
	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		upscaled := isUpscaledSource(t, source)

		mlImage, err := imagesource.ProcessMLImageTensorBytes(source, imagesource.PurposeSemantic)
		if err != nil {
			t.Fatalf("vips preprocess %s: %v", path, err)
		}
		if mlImage.Width != 224 || mlImage.Height != 224 {
			t.Fatalf("vips output for %s is %dx%d, want 224x224", path, mlImage.Width, mlImage.Height)
		}

		// Decoded-only input: fail loudly if the vips pixels are rejected
		// instead of silently falling back to the encoded path.
		tensor, err := preprocessor.Preprocess(ctx, types.ImageInput{
			Data:       mlImage.Data,
			Width:      mlImage.Width,
			Height:     mlImage.Height,
			Channels:   mlImage.Channels,
			Layout:     mlImage.Layout,
			DType:      mlImage.DType,
			ColorSpace: mlImage.ColorSpace,
		})
		if err != nil {
			t.Fatalf("SDK tensor preprocess (decoded input) %s: %v", path, err)
		}

		tensorReq := types.NewInferRequest(types.TaskSemanticImageEmbed).
			ForTensorInput(tensor.Payload, tensor.PayloadMIME, tensor.Descriptor).
			WithService(serviceName).
			Build()
		rawReq := types.NewInferRequest(types.TaskSemanticImageEmbed).
			ForSemanticImageEmbed(mlImage.EncodedSource, mimeForConformancePath(path)).
			WithService(serviceName).
			Build()

		tensorVec := conformanceEmbedding(ctx, t, client, tensorReq, path)
		rawVec := conformanceEmbedding(ctx, t, client, rawReq, path)
		cosine := cosineSimilarityF32(tensorVec, rawVec)
		t.Logf("cosine=%.6f upscaled=%v image=%s", cosine, upscaled, filepath.Base(path))
		if upscaled {
			worstUpscale = math.Min(worstUpscale, cosine)
		} else {
			worstDownscale = math.Min(worstDownscale, cosine)
		}
	}

	// The two pipelines use different resize implementations (libvips vs the
	// Hub image crate), so bit-exact equality is not expected. 0.995 catches
	// semantic drift such as crop-vs-squash or kernel mismatches. Sources
	// smaller than the model input (rare degenerate photos) go through
	// upscaling, where libvips and the image crate use different half-pixel
	// conventions; they get a relaxed gate.
	if worstDownscale < 0.995 {
		t.Fatalf("tensor/raw embedding drift: worst downscale cosine %.6f < 0.995", worstDownscale)
	}
	if worstUpscale < 0.97 {
		t.Fatalf("tensor/raw embedding drift: worst upscale cosine %.6f < 0.97", worstUpscale)
	}
}

// isUpscaledSource reports whether the source image is smaller than the
// SigLIP input in either dimension, i.e. preprocessing will upscale it.
func isUpscaledSource(t *testing.T, source []byte) bool {
	t.Helper()
	config, _, err := image.DecodeConfig(bytes.NewReader(source))
	if err != nil {
		t.Fatalf("decode config: %v", err)
	}
	return config.Width < 224 || config.Height < 224
}

func conformanceImages(t *testing.T, dir string, limit int) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read image dir: %v", err)
	}
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".jpg", ".jpeg", ".png", ".webp":
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(paths)
	if len(paths) > limit {
		// Spread the sample across the directory instead of taking a prefix.
		step := len(paths) / limit
		var sampled []string
		for i := 0; i < len(paths) && len(sampled) < limit; i += step {
			sampled = append(sampled, paths[i])
		}
		paths = sampled
	}
	return paths
}

func conformanceContract(ctx context.Context, t *testing.T, client pb.InferenceClient) (preprocessID, serviceName string) {
	t.Helper()
	stream, err := client.StreamCapabilities(ctx, nil)
	if err != nil {
		t.Fatalf("stream capabilities: %v", err)
	}
	for {
		capability, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("recv capability: %v", err)
		}
		for _, task := range capability.GetTasks() {
			if task.GetName() == types.TaskSemanticImageEmbed && task.GetTensorPreprocessId() != "" {
				return task.GetTensorPreprocessId(), capability.GetServiceName()
			}
		}
	}
	t.Fatal("hub does not advertise a tensor path for semantic_image_embed")
	return "", ""
}

func conformanceEmbedding(ctx context.Context, t *testing.T, client pb.InferenceClient, req *pb.InferRequest, path string) []float32 {
	t.Helper()
	stream, err := client.Infer(ctx)
	if err != nil {
		t.Fatalf("infer %s: %v", path, err)
	}
	if err := stream.Send(req); err != nil {
		t.Fatalf("send %s: %v", path, err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("close send %s: %v", path, err)
	}
	var final *pb.InferResponse
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("recv %s: %v", path, err)
		}
		if respErr := resp.GetError(); respErr != nil && respErr.GetMessage() != "" {
			t.Fatalf("hub error for %s: %s", path, respErr.GetMessage())
		}
		final = resp
		if resp.GetIsFinal() {
			break
		}
	}
	if final == nil {
		t.Fatalf("empty response for %s", path)
	}
	embedding, err := types.ParseInferResponse(final).AsEmbeddingResponse()
	if err != nil {
		t.Fatalf("parse embedding for %s: %v", path, err)
	}
	return embedding.Vector
}

func mimeForConformancePath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	default:
		return "image/webp"
	}
}

func cosineSimilarityF32(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
