package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	// Adjust this import path to where your generated code lives.
	// For example, if your package is "server/proto", change to:
	//   server/proto
	"server/proto"
)

// MLService is a thin convenience wrapper over the generated gRPC client.
// It provides helpers for all tasks exposed by the Unified ML Service:
//
// - clip_classify
// - bioclip_classify
// - smart_classify
// - clip_embed
// - bioclip_embed
// - clip_image_embed
// - bioclip_image_embed
type MLService struct {
	conn *grpc.ClientConn
	rpc  proto.InferenceClient
}

// NewMLService creates a client using the new gRPC dialing API.
// Example (insecure/local):
//
//	c, err := mlclient.New("localhost:50051", grpc.WithInsecure()) // or insecure.NewCredentials()
//
// For TLS, pass grpc.WithTransportCredentials(credentials).
func NewMLService(address string, opts ...grpc.DialOption) (*MLService, error) {
	conn, err := grpc.NewClient(address, opts...)
	if err != nil {
		return nil, err
	}
	return NewFromConn(conn), nil
}

// NewFromConn wraps an existing ClientConn.
func NewFromConn(conn *grpc.ClientConn) *MLService {
	return &MLService{
		conn: conn,
		rpc:  proto.NewInferenceClient(conn),
	}
}

// Close closes the underlying connection.
func (c *MLService) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Health pings the server.
func (c *MLService) Health(ctx context.Context) error {
	_, err := c.rpc.Health(ctx, &emptypb.Empty{})
	return err
}

// Capability is returned by GetCapabilities/StreamCapabilities.
type Capability = proto.Capability

// GetCapabilities fetches the (single) capability snapshot.
func (c *MLService) GetCapabilities(ctx context.Context) (*Capability, error) {
	return c.rpc.GetCapabilities(ctx, &emptypb.Empty{})
}

// StreamCapabilities streams capability updates (recommended).
// The provided handler is called for every received Capability until the stream ends or context is done.
func (c *MLService) StreamCapabilities(ctx context.Context, handler func(*Capability) error) error {
	stream, err := c.rpc.StreamCapabilities(ctx, &emptypb.Empty{})
	if err != nil {
		return err
	}
	for {
		cap, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if handler != nil {
			if herr := handler(cap); herr != nil {
				return herr
			}
		}
	}
}

// JSON result types the server returns for embeddings and labels.

type EmbeddingResult struct {
	Vector  []float64 `json:"vector"`
	Dim     int       `json:"dim"`
	ModelID string    `json:"model_id"`
}

type Label struct {
	Label string  `json:"label"`
	Score float64 `json:"score"`
}

type LabelsResult struct {
	Labels  []Label `json:"labels"`
	ModelID string  `json:"model_id"`
}

// --------------- Public helpers for each task ---------------

// ClipEmbed text -> vector
func (c *MLService) ClipEmbed(ctx context.Context, text string) (*EmbeddingResult, error) {
	return c.doEmbed(ctx, "clip_embed", []byte(text), "text/plain;charset=utf-8", nil)
}

// BioClipEmbed text -> vector
func (c *MLService) BioClipEmbed(ctx context.Context, text string) (*EmbeddingResult, error) {
	return c.doEmbed(ctx, "bioclip_embed", []byte(text), "text/plain;charset=utf-8", nil)
}

// ClipImageEmbed image bytes -> vector
// payloadMime can be "", in which case "image/jpeg" is used by default.
func (c *MLService) ClipImageEmbed(ctx context.Context, image []byte, payloadMime string) (*EmbeddingResult, error) {
	if payloadMime == "" {
		payloadMime = "image/jpeg"
	}
	return c.doEmbed(ctx, "clip_image_embed", image, payloadMime, nil)
}

// BioClipImageEmbed image bytes -> vector
func (c *MLService) BioClipImageEmbed(ctx context.Context, image []byte, payloadMime string) (*EmbeddingResult, error) {
	if payloadMime == "" {
		payloadMime = "image/jpeg"
	}
	return c.doEmbed(ctx, "bioclip_image_embed", image, payloadMime, nil)
}

// ClipClassify image bytes -> labels
func (c *MLService) ClipClassify(ctx context.Context, image []byte, topk int, payloadMime string) (*LabelsResult, error) {
	if payloadMime == "" {
		payloadMime = "image/jpeg"
	}
	meta := map[string]string{"topk": strconv.Itoa(topk)}
	return c.doLabels(ctx, "clip_classify", image, payloadMime, meta)
}

// BioClipClassify image bytes -> species labels
func (c *MLService) BioClipClassify(ctx context.Context, image []byte, topk int, payloadMime string) (*LabelsResult, error) {
	if payloadMime == "" {
		payloadMime = "image/jpeg"
	}
	meta := map[string]string{"topk": strconv.Itoa(topk)}
	return c.doLabels(ctx, "bioclip_classify", image, payloadMime, meta)
}

// SmartClassify image bytes -> if scene is animal-like, uses BioCLIP; else scene label from CLIP.
// Returns labels + response meta (includes "source": scene_classification|bioclip_classification).
func (c *MLService) SmartClassify(ctx context.Context, image []byte, topk int, payloadMime string) (*LabelsResult, map[string]string, error) {
	if payloadMime == "" {
		payloadMime = "image/jpeg"
	}
	meta := map[string]string{"topk": strconv.Itoa(topk)}
	resp, raw, err := c.singleInfer(ctx, "smart_classify", image, payloadMime, meta)
	if err != nil {
		return nil, nil, err
	}
	if resp.Error != nil && resp.Error.Code != proto.ErrorCode_ERROR_CODE_UNSPECIFIED {
		return nil, resp.Meta, fmt.Errorf("server error: %s", resp.Error.Message)
	}
	var out LabelsResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, resp.Meta, err
	}
	return &out, resp.Meta, nil
}

// --------------- Internal helpers ---------------

const chunkSize = 512 * 1024 // 512KB chunks for large payloads

func (c *MLService) doEmbed(ctx context.Context, task string, payload []byte, payloadMime string, meta map[string]string) (*EmbeddingResult, error) {
	resp, raw, err := c.singleInfer(ctx, task, payload, payloadMime, meta)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil && resp.Error.Code != proto.ErrorCode_ERROR_CODE_UNSPECIFIED {
		return nil, fmt.Errorf("server error: %s", resp.Error.Message)
	}
	var out EmbeddingResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *MLService) doLabels(ctx context.Context, task string, payload []byte, payloadMime string, meta map[string]string) (*LabelsResult, error) {
	resp, raw, err := c.singleInfer(ctx, task, payload, payloadMime, meta)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil && resp.Error.Code != proto.ErrorCode_ERROR_CODE_UNSPECIFIED {
		return nil, fmt.Errorf("server error: %s", resp.Error.Message)
	}
	var out LabelsResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// singleInfer sends exactly one logical request (with optional chunking) and waits for the final response.
// It returns the final InferResponse and the raw result bytes.
func (c *MLService) singleInfer(ctx context.Context, task string, payload []byte, payloadMime string, meta map[string]string) (*proto.InferResponse, []byte, error) {
	stream, err := c.rpc.Infer(ctx)
	if err != nil {
		return nil, nil, err
	}
	cid := fmt.Sprintf("go-%d", time.Now().UnixNano())

	// Prepare chunking
	total := len(payload)/chunkSize + 1
	if len(payload)%chunkSize == 0 {
		total = len(payload) / chunkSize
	}
	if total == 0 {
		total = 1
	}

	offset := 0
	for i := 0; i < total; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(payload) {
			end = len(payload)
		}
		chunk := payload[start:end]

		req := &proto.InferRequest{
			CorrelationId: cid,
			Task:          task,
			Payload:       chunk,
			Meta:          nil,
			PayloadMime:   payloadMime,
			Seq:           uint64(i),
			Total:         uint64(total),
			Offset:        uint64(offset),
		}
		if meta != nil && i == 0 {
			// Send meta once; server does not require meta per chunk.
			req.Meta = meta
		}

		if err := stream.SendMsg(req); err != nil {
			_ = stream.CloseSend()
			return nil, nil, err
		}
		offset += len(chunk)
	}

	// Close sending side and read responses
	if err := stream.CloseSend(); err != nil {
		return nil, nil, err
	}

	var last *proto.InferResponse
	for {
		resp, rerr := stream.Recv()
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			return nil, nil, rerr
		}
		last = resp
		if resp.IsFinal {
			break
		}
	}

	if last == nil {
		return nil, nil, errors.New("no response received")
	}
	return last, last.GetResult(), nil
}
