package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func startInferenceServer(t *testing.T, impl pb.InferenceServer) pb.InferenceClient {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	pb.RegisterInferenceServer(server, impl)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return pb.NewInferenceClient(conn)
}

func infer(t *testing.T, client pb.InferenceClient, task string, payload []byte, mime string) (*pb.InferResponse, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	stream, err := client.Infer(ctx)
	if err != nil {
		t.Fatalf("open infer stream: %v", err)
	}
	if err := stream.Send(&pb.InferRequest{
		CorrelationId: "test",
		Task:          task,
		Payload:       payload,
		PayloadMime:   mime,
	}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("close send: %v", err)
	}

	response := &pb.InferResponse{}
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return response, nil
		}
		if err != nil {
			return nil, err
		}
		response.Result = append(response.Result, chunk.GetResult()...)
		if chunk.GetResultMime() != "" {
			response.ResultMime = chunk.GetResultMime()
		}
		if chunk.GetIsFinal() {
			response.IsFinal = true
		}
	}
}

func decodeEmbedding(t *testing.T, raw []byte) types.EmbeddingV1 {
	t.Helper()
	embedding := types.EmbeddingV1{}
	if err := json.Unmarshal(raw, &embedding); err != nil {
		t.Fatalf("decode embedding: %v", err)
	}
	return embedding
}

// compactJSON normalizes whitespace: fixture files are stored indented for
// review, so replayed JSON is compared semantically, not byte-for-byte.
func compactJSON(t *testing.T, raw []byte) string {
	t.Helper()
	var buffer bytes.Buffer
	if err := json.Compact(&buffer, raw); err != nil {
		t.Fatalf("compact %s: %v", raw, err)
	}
	return buffer.String()
}

func TestFixtureStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := openFixtureDir(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	payload := []byte("raccoon at night")
	record := newFixtureRecord(&pb.InferRequest{
		Task:        types.TaskSemanticTextEmbed,
		Payload:     payload,
		PayloadMime: "text/plain",
	}, "application/json;schema=embedding_v1", []byte(`{"vector":[0.5],"dim":1,"model_id":"real"}`), nil)
	if record.PayloadText != "raccoon at night" {
		t.Fatalf("text payload should be inlined for review, got %q", record.PayloadText)
	}
	if err := store.put(record); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := store.setCapabilities([]*pb.Capability{builtinCapability()}, "test-upstream:50051"); err != nil {
		t.Fatalf("set capabilities: %v", err)
	}

	reloaded, err := openFixtureDir(dir)
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	got, ok := reloaded.lookup(types.TaskSemanticTextEmbed, payloadDigest(payload))
	if !ok {
		t.Fatal("recorded fixture should survive a reload")
	}
	result, err := got.resultBytes()
	if err != nil {
		t.Fatalf("result bytes: %v", err)
	}
	if compactJSON(t, result) != `{"vector":[0.5],"dim":1,"model_id":"real"}` {
		t.Fatalf("unexpected replayed result %s", result)
	}
	caps := reloaded.capabilities()
	if len(caps) != 1 || caps[0].GetServiceName() != types.ServiceSigLIP {
		t.Fatalf("capability set should survive a reload, got %v", caps)
	}
	if reloaded.manifest.RecordedFrom != "test-upstream:50051" {
		t.Fatalf("manifest should record provenance, got %q", reloaded.manifest.RecordedFrom)
	}
}

func TestReplayHitMissAndStrict(t *testing.T) {
	dir := t.TempDir()
	store, err := openFixtureDir(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	recordedPayload := []byte("known query")
	realResult := []byte(`{"vector":[0.25,0.75],"dim":2,"model_id":"recorded-model"}`)
	if err := store.put(newFixtureRecord(&pb.InferRequest{
		Task:        types.TaskSemanticTextEmbed,
		Payload:     recordedPayload,
		PayloadMime: "text/plain",
	}, "application/json;schema=embedding_v1", realResult, nil)); err != nil {
		t.Fatalf("put: %v", err)
	}

	replay := newReplayServer(store, false)
	client := startInferenceServer(t, replay)

	hit, err := infer(t, client, types.TaskSemanticTextEmbed, recordedPayload, "text/plain")
	if err != nil {
		t.Fatalf("hit infer: %v", err)
	}
	if compactJSON(t, hit.GetResult()) != string(realResult) {
		t.Fatalf("hit should replay recorded bytes, got %s", hit.GetResult())
	}

	miss, err := infer(t, client, types.TaskSemanticTextEmbed, []byte("never recorded"), "text/plain")
	if err != nil {
		t.Fatalf("miss infer: %v", err)
	}
	embedding := decodeEmbedding(t, miss.GetResult())
	if embedding.ModelID != builtinModelID || embedding.Dim != 768 || embedding.Vector[0] != 1 {
		t.Fatalf("miss should fall back to the builtin constant embedding, got %+v", embedding)
	}

	snapshot := replay.metrics.snapshot(replay.mode, store.size())
	hits := snapshot["fixture_hits"].(map[string]uint64)
	misses := snapshot["fixture_misses"].(map[string]uint64)
	if hits[types.TaskSemanticTextEmbed] != 1 || misses[types.TaskSemanticTextEmbed] != 1 {
		t.Fatalf("expected one hit and one miss, got hits=%v misses=%v", hits, misses)
	}
	if snapshot["semantic_text"].(uint64) != 2 {
		t.Fatalf("legacy semantic_text counter should count every request, got %v", snapshot["semantic_text"])
	}

	strict := newReplayServer(store, true)
	strictClient := startInferenceServer(t, strict)
	_, err = infer(t, strictClient, types.TaskSemanticTextEmbed, []byte("never recorded"), "text/plain")
	if status.Code(err) != codes.NotFound {
		t.Fatalf("strict miss should be NotFound, got %v", err)
	}
}

func TestReplayFallbackShapesForOtherTasks(t *testing.T) {
	store, err := loadFixtureStore(os.DirFS(t.TempDir()))
	if err != nil {
		t.Fatalf("empty store: %v", err)
	}
	client := startInferenceServer(t, newReplayServer(store, false))

	face, err := infer(t, client, types.TaskFaceRecognition, []byte{1, 2, 3}, "image/webp")
	if err != nil {
		t.Fatalf("face fallback: %v", err)
	}
	faceResult := types.FaceV1{}
	if err := json.Unmarshal(face.GetResult(), &faceResult); err != nil {
		t.Fatalf("face fallback must be valid face_v1 JSON: %v", err)
	}
	if faceResult.Count != 0 || face.GetResultMime() != "application/json;schema=face_v1" {
		t.Fatalf("face fallback should be an empty result, got %+v (%s)", faceResult, face.GetResultMime())
	}

	ocr, err := infer(t, client, types.TaskOCR, []byte{1, 2, 3}, "image/webp")
	if err != nil {
		t.Fatalf("OCR fallback: %v", err)
	}
	ocrResult := types.OCRV1{}
	if err := json.Unmarshal(ocr.GetResult(), &ocrResult); err != nil {
		t.Fatalf("OCR fallback must be valid ocr_v1 JSON: %v", err)
	}
	if ocrResult.Count != 2 || len(ocrResult.Items) != 2 || ocrResult.Items[0].Text != "Lumilio OCR first line" {
		t.Fatalf("OCR fallback should contain ordered deterministic text, got %+v", ocrResult)
	}

	_, err = infer(t, client, "text_generation", []byte("prompt"), "text/plain")
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("unknown task should stay Unimplemented, got %v", err)
	}
}

func TestBuiltinCapabilityWithoutFixtures(t *testing.T) {
	store, err := loadFixtureStore(os.DirFS(t.TempDir()))
	if err != nil {
		t.Fatalf("empty store: %v", err)
	}
	client := startInferenceServer(t, newReplayServer(store, false))
	capability, err := client.GetCapabilities(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("get capabilities: %v", err)
	}
	if capability.GetServiceName() != types.ServiceSigLIP {
		t.Fatalf("builtin capability should advertise siglip, got %q", capability.GetServiceName())
	}

	stream, err := client.StreamCapabilities(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("stream capabilities: %v", err)
	}
	var services []string
	for {
		capability, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("receive capability: %v", err)
		}
		services = append(services, capability.GetServiceName())
	}
	if !slices.Contains(services, types.ServiceOCR) {
		t.Fatalf("builtin capabilities should advertise OCR, got %v", services)
	}
}

// recordingUpstream is the stand-in real hub: distinctive embeddings derived
// from the payload, chunked responses, and a two-service capability stream.
type recordingUpstream struct {
	pb.UnimplementedInferenceServer
}

func (u *recordingUpstream) Infer(stream grpc.BidiStreamingServer[pb.InferRequest, pb.InferResponse]) error {
	request, err := receiveFullRequest(stream)
	if err != nil {
		return err
	}
	result, err := json.Marshal(types.EmbeddingV1{
		Vector:  []float32{float32(len(request.GetPayload())), 42},
		Dim:     2,
		ModelID: "upstream-real-model",
	})
	if err != nil {
		return err
	}
	// Two chunks on purpose: the recorder must reassemble streamed results.
	half := len(result) / 2
	if err := stream.Send(&pb.InferResponse{
		CorrelationId: request.GetCorrelationId(),
		Result:        result[:half],
		Seq:           0,
	}); err != nil {
		return err
	}
	return stream.Send(&pb.InferResponse{
		CorrelationId: request.GetCorrelationId(),
		IsFinal:       true,
		Result:        result[half:],
		ResultMime:    "application/json;schema=embedding_v1",
		Seq:           1,
	})
}

func (u *recordingUpstream) GetCapabilities(context.Context, *emptypb.Empty) (*pb.Capability, error) {
	return builtinCapability(), nil
}

func (u *recordingUpstream) StreamCapabilities(_ *emptypb.Empty, stream grpc.ServerStreamingServer[pb.Capability]) error {
	if err := stream.Send(builtinCapability()); err != nil {
		return err
	}
	return stream.Send(&pb.Capability{
		ServiceName: types.ServiceFace,
		ModelIds:    []string{"upstream-face-model"},
		Tasks: []*pb.IOTask{{
			Name:        types.TaskFaceRecognition,
			InputMimes:  []string{"image/webp"},
			OutputMimes: []string{"application/json;schema=face_v1"},
		}},
	})
}

func (u *recordingUpstream) Health(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func TestRecordProxyPersistsAndReplays(t *testing.T) {
	upstreamClient := startInferenceServer(t, &recordingUpstream{})

	dir := t.TempDir()
	store, err := openFixtureDir(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	recorder := newRecordServer(store, upstreamClient, "upstream-test")
	recordClient := startInferenceServer(t, recorder)

	payload := []byte("golden retriever on a beach")
	recorded, err := infer(t, recordClient, types.TaskSemanticTextEmbed, payload, "text/plain")
	if err != nil {
		t.Fatalf("record infer: %v", err)
	}
	embedding := decodeEmbedding(t, recorded.GetResult())
	if embedding.ModelID != "upstream-real-model" {
		t.Fatalf("record mode must stream the real upstream answer, got %+v", embedding)
	}

	fixturePath := filepath.Join(dir, recordsDirname, types.TaskSemanticTextEmbed, payloadDigest(payload)+".json")
	if _, err := os.Stat(fixturePath); err != nil {
		t.Fatalf("fixture file should be written: %v", err)
	}

	stream, err := recordClient.StreamCapabilities(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("stream capabilities: %v", err)
	}
	services := []string{}
	for {
		capability, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("recv capability: %v", err)
		}
		services = append(services, capability.GetServiceName())
	}
	if len(services) != 2 {
		t.Fatalf("record mode should proxy both upstream capabilities, got %v", services)
	}

	replayStore, err := openFixtureDir(dir)
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	replayClient := startInferenceServer(t, newReplayServer(replayStore, true))
	replayed, err := infer(t, replayClient, types.TaskSemanticTextEmbed, payload, "text/plain")
	if err != nil {
		t.Fatalf("strict replay of a recorded payload must hit: %v", err)
	}
	if compactJSON(t, replayed.GetResult()) != compactJSON(t, recorded.GetResult()) {
		t.Fatalf("replayed result differs from recorded result:\n%s\n%s", replayed.GetResult(), recorded.GetResult())
	}
	replayedCaps := replayStore.capabilities()
	if len(replayedCaps) != 2 || replayedCaps[1].GetServiceName() != types.ServiceFace {
		t.Fatalf("replay should advertise the recorded capability set, got %v", replayedCaps)
	}
}
