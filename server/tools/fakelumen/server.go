package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	modeReplay = "replay"
	modeRecord = "record"

	// builtinModelID identifies the deterministic no-fixture fallback. It is
	// also the legacy behavior of this tool, so an E2E stack without recorded
	// fixtures behaves exactly as before.
	builtinModelID = "lumilio-e2e-deterministic-v1"

	// forwardChunkBytes keeps forwarded upstream messages under the default
	// gRPC 4 MiB message ceiling.
	forwardChunkBytes = 1 << 20
)

// metrics counts per-task requests, fixture hits/misses, and recordings.
// The two legacy keys (semantic_image / semantic_text) remain top-level in
// the JSON so existing Playwright assertions keep working unchanged.
type metrics struct {
	mu       sync.Mutex
	requests map[string]uint64
	hits     map[string]uint64
	misses   map[string]uint64
	recorded map[string]uint64
}

func newMetrics() *metrics {
	return &metrics{
		requests: map[string]uint64{},
		hits:     map[string]uint64{},
		misses:   map[string]uint64{},
		recorded: map[string]uint64{},
	}
}

func bump(m map[string]uint64, task string) { m[task]++ }

func snapshotCounts(m map[string]uint64) map[string]uint64 {
	out := make(map[string]uint64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (m *metrics) snapshot(mode string, fixturesLoaded int) map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return map[string]any{
		"semantic_image":  m.requests[types.TaskSemanticImageEmbed],
		"semantic_text":   m.requests[types.TaskSemanticTextEmbed],
		"mode":            mode,
		"fixtures_loaded": fixturesLoaded,
		"requests":        snapshotCounts(m.requests),
		"fixture_hits":    snapshotCounts(m.hits),
		"fixture_misses":  snapshotCounts(m.misses),
		"recorded":        snapshotCounts(m.recorded),
	}
}

func metricsHandler(server *inferenceServer) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(server.metrics.snapshot(server.mode, server.store.size()))
	})
	return mux
}

type inferenceServer struct {
	pb.UnimplementedInferenceServer
	mode         string
	strict       bool
	store        *fixtureStore
	metrics      *metrics
	upstream     pb.InferenceClient // record mode only
	upstreamAddr string
}

func newReplayServer(store *fixtureStore, strict bool) *inferenceServer {
	return &inferenceServer{mode: modeReplay, strict: strict, store: store, metrics: newMetrics()}
}

func newRecordServer(store *fixtureStore, upstream pb.InferenceClient, upstreamAddr string) *inferenceServer {
	return &inferenceServer{
		mode: modeRecord, store: store, metrics: newMetrics(),
		upstream: upstream, upstreamAddr: upstreamAddr,
	}
}

// builtinCapability is the deterministic default advertised when no recorded
// capability set exists: SigLIP semantic embedding only, matching the
// historical constant-vector fixture.
func builtinCapability() *pb.Capability {
	return &pb.Capability{
		ServiceName:     types.ServiceSigLIP,
		ModelIds:        []string{builtinModelID},
		Runtime:         "e2e-deterministic",
		MaxConcurrency:  8,
		Precisions:      []string{"fp32"},
		ProtocolVersion: "1.0.0",
		Tasks: []*pb.IOTask{
			{
				Name:        types.TaskSemanticTextEmbed,
				InputMimes:  []string{"text/plain"},
				OutputMimes: []string{"application/json;schema=embedding_v1"},
			},
			{
				Name:                    types.TaskSemanticImageEmbed,
				InputMimes:              []string{"image/jpeg", "image/png", "image/webp", types.DefaultTensorMIME},
				OutputMimes:             []string{"application/json;schema=embedding_v1"},
				TensorPreprocessId:      types.PreprocessSigLIP2BasePatch16_224Image,
				TensorBatchingSupported: true,
			},
		},
	}
}

func (s *inferenceServer) advertisedCapabilities() []*pb.Capability {
	if recorded := s.store.capabilities(); len(recorded) > 0 {
		return recorded
	}
	return []*pb.Capability{builtinCapability()}
}

func (s *inferenceServer) GetCapabilities(ctx context.Context, _ *emptypb.Empty) (*pb.Capability, error) {
	if s.mode == modeRecord {
		capability, err := s.upstream.GetCapabilities(ctx, &emptypb.Empty{})
		if err != nil {
			return nil, err
		}
		return capability, nil
	}
	return s.advertisedCapabilities()[0], nil
}

func (s *inferenceServer) StreamCapabilities(_ *emptypb.Empty, stream grpc.ServerStreamingServer[pb.Capability]) error {
	if s.mode == modeRecord {
		upstreamStream, err := s.upstream.StreamCapabilities(stream.Context(), &emptypb.Empty{})
		if err != nil {
			return err
		}
		var caps []*pb.Capability
		for {
			capability, err := upstreamStream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return err
			}
			caps = append(caps, capability)
			if err := stream.Send(capability); err != nil {
				return err
			}
		}
		if len(caps) > 0 {
			if err := s.store.setCapabilities(caps, s.upstreamAddr); err != nil {
				return status.Errorf(codes.Internal, "persist capabilities: %v", err)
			}
		}
		return nil
	}
	for _, capability := range s.advertisedCapabilities() {
		if err := stream.Send(capability); err != nil {
			return err
		}
	}
	return nil
}

// Health is local in replay mode. In record mode it proxies the upstream hub,
// so the Compose healthcheck holds the stack until the recording chain is
// actually usable.
func (s *inferenceServer) Health(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	if s.mode == modeRecord {
		return s.upstream.Health(ctx, &emptypb.Empty{})
	}
	return &emptypb.Empty{}, nil
}

func (s *inferenceServer) Infer(stream grpc.BidiStreamingServer[pb.InferRequest, pb.InferResponse]) error {
	request, err := receiveFullRequest(stream)
	if err != nil {
		return err
	}
	if request == nil {
		return status.Error(codes.InvalidArgument, "inference request is empty")
	}

	s.metrics.mu.Lock()
	bump(s.metrics.requests, request.GetTask())
	s.metrics.mu.Unlock()

	if s.mode == modeRecord {
		return s.recordAndRespond(stream, request)
	}
	return s.replay(stream, request)
}

func (s *inferenceServer) replay(stream grpc.BidiStreamingServer[pb.InferRequest, pb.InferResponse], request *pb.InferRequest) error {
	task := request.GetTask()
	if record, ok := s.store.lookup(task, payloadDigest(request.GetPayload())); ok {
		result, err := record.resultBytes()
		if err != nil {
			return status.Errorf(codes.Internal, "decode fixture: %v", err)
		}
		s.metrics.mu.Lock()
		bump(s.metrics.hits, task)
		s.metrics.mu.Unlock()
		return stream.Send(&pb.InferResponse{
			CorrelationId: request.GetCorrelationId(),
			IsFinal:       true,
			Result:        result,
			ResultMime:    record.ResultMime,
			Meta:          record.ResponseMeta,
		})
	}

	s.metrics.mu.Lock()
	bump(s.metrics.misses, task)
	s.metrics.mu.Unlock()
	if s.strict {
		return status.Errorf(codes.NotFound, "no fixture for task %q payload sha256=%s; re-record fixtures", task, payloadDigest(request.GetPayload()))
	}
	result, resultMime, err := builtinResponse(task)
	if err != nil {
		return err
	}
	return stream.Send(&pb.InferResponse{
		CorrelationId: request.GetCorrelationId(),
		IsFinal:       true,
		Result:        result,
		ResultMime:    resultMime,
	})
}

// builtinResponse is the deterministic no-fixture fallback per task. The two
// semantic tasks return the historical constant embedding; the other Photos
// tasks return valid empty results so a partially recorded fixture set
// degrades visibly (miss counters) instead of failing pipelines.
func builtinResponse(task string) (result []byte, resultMime string, err error) {
	switch task {
	case types.TaskSemanticTextEmbed, types.TaskSemanticImageEmbed:
		vector := make([]float32, 768)
		vector[0] = 1
		result, err = json.Marshal(types.EmbeddingV1{Vector: vector, Dim: len(vector), ModelID: builtinModelID})
		return result, "application/json;schema=embedding_v1", err
	case types.TaskBioCLIPClassify:
		result, err = json.Marshal(types.LabelsV1{Labels: []types.Label{}, ModelID: builtinModelID})
		return result, "application/json;schema=labels_v1", err
	case types.TaskFaceRecognition:
		result, err = json.Marshal(types.FaceV1{Faces: []types.Face{}, Count: 0, ModelID: builtinModelID})
		return result, "application/json;schema=face_v1", err
	case types.TaskOCR:
		result, err = json.Marshal(types.OCRV1{Items: []types.OCRItem{}, Count: 0, ModelID: builtinModelID})
		return result, "application/json;schema=ocr_v1", err
	default:
		return nil, "", status.Errorf(codes.Unimplemented, "unsupported task %q", task)
	}
}

func (s *inferenceServer) recordAndRespond(stream grpc.BidiStreamingServer[pb.InferRequest, pb.InferResponse], request *pb.InferRequest) error {
	resultMime, result, responseMeta, err := s.forwardUpstream(stream.Context(), request)
	if err != nil {
		return err
	}
	record := newFixtureRecord(request, resultMime, result, responseMeta)
	if err := s.store.put(record); err != nil {
		return status.Errorf(codes.Internal, "write fixture: %v", err)
	}
	s.metrics.mu.Lock()
	bump(s.metrics.recorded, request.GetTask())
	s.metrics.mu.Unlock()
	return stream.Send(&pb.InferResponse{
		CorrelationId: request.GetCorrelationId(),
		IsFinal:       true,
		Result:        result,
		ResultMime:    resultMime,
		Meta:          responseMeta,
	})
}

// forwardUpstream replays the assembled request against the real hub in
// bounded chunks and reassembles the streamed response.
func (s *inferenceServer) forwardUpstream(ctx context.Context, request *pb.InferRequest) (resultMime string, result []byte, responseMeta map[string]string, err error) {
	upstreamStream, err := s.upstream.Infer(ctx)
	if err != nil {
		return "", nil, nil, status.Errorf(codes.Unavailable, "upstream infer: %v", err)
	}
	payload := request.GetPayload()
	total := uint64((len(payload) + forwardChunkBytes - 1) / forwardChunkBytes)
	if total == 0 {
		total = 1
	}
	for seq := uint64(0); seq < total; seq++ {
		start := int(seq) * forwardChunkBytes
		end := min(start+forwardChunkBytes, len(payload))
		chunk := &pb.InferRequest{
			CorrelationId: request.GetCorrelationId(),
			Task:          request.GetTask(),
			PayloadMime:   request.GetPayloadMime(),
			Meta:          request.GetMeta(),
			Payload:       payload[start:end],
			Seq:           seq,
			Total:         total,
			Offset:        uint64(start),
		}
		if err := upstreamStream.Send(chunk); err != nil {
			return "", nil, nil, status.Errorf(codes.Unavailable, "upstream send: %v", err)
		}
	}
	if err := upstreamStream.CloseSend(); err != nil {
		return "", nil, nil, status.Errorf(codes.Unavailable, "upstream close-send: %v", err)
	}

	for {
		chunk, err := upstreamStream.Recv()
		if errors.Is(err, io.EOF) {
			return "", nil, nil, status.Error(codes.Unavailable, "upstream closed before a final response")
		}
		if err != nil {
			return "", nil, nil, err
		}
		if upstreamErr := chunk.GetError(); upstreamErr != nil {
			return "", nil, nil, status.Errorf(codes.Internal, "upstream error %s: %s", upstreamErr.GetCode(), upstreamErr.GetMessage())
		}
		result = append(result, chunk.GetResult()...)
		if chunk.GetResultMime() != "" {
			resultMime = chunk.GetResultMime()
		}
		if chunk.GetMeta() != nil {
			responseMeta = chunk.GetMeta()
		}
		if chunk.GetIsFinal() {
			return resultMime, result, responseMeta, nil
		}
	}
}

func receiveFullRequest(stream grpc.BidiStreamingServer[pb.InferRequest, pb.InferResponse]) (*pb.InferRequest, error) {
	var request *pb.InferRequest
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return request, nil
		}
		if err != nil {
			return nil, err
		}
		if request == nil {
			request = &pb.InferRequest{
				CorrelationId: chunk.GetCorrelationId(),
				Task:          chunk.GetTask(),
				PayloadMime:   chunk.GetPayloadMime(),
				Meta:          cloneStrings(chunk.GetMeta()),
			}
		}
		request.Payload = append(request.Payload, chunk.GetPayload()...)
	}
}

func cloneStrings(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
