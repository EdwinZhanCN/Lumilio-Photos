// Command fakelumen provides the deterministic external inference boundary used
// by the isolated browser E2E stack. It implements the public Lumen gRPC
// contract, advertises only semantic image/text embedding, and returns one
// stable 768-dimensional vector for every request. Product code still exercises
// discovery, gRPC streaming, image preprocessing, queues, PostgreSQL vector
// storage, retrieval, and best-frame selection; only model inference is faked.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	defaultAddress = ":50051"
	modelID        = "lumilio-e2e-deterministic-v1"
)

type inferenceServer struct {
	pb.UnimplementedInferenceServer
	result []byte
}

func newInferenceServer() (*inferenceServer, error) {
	vector := make([]float32, 768)
	vector[0] = 1
	result, err := json.Marshal(types.EmbeddingV1{
		Vector:  vector,
		Dim:     len(vector),
		ModelID: modelID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal deterministic embedding: %w", err)
	}
	return &inferenceServer{result: result}, nil
}

func semanticCapability() *pb.Capability {
	return &pb.Capability{
		ServiceName:     types.ServiceSigLIP,
		ModelIds:        []string{modelID},
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

func (s *inferenceServer) GetCapabilities(context.Context, *emptypb.Empty) (*pb.Capability, error) {
	return semanticCapability(), nil
}

func (s *inferenceServer) StreamCapabilities(_ *emptypb.Empty, stream grpc.ServerStreamingServer[pb.Capability]) error {
	return stream.Send(semanticCapability())
}

func (s *inferenceServer) Health(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *inferenceServer) Infer(stream grpc.BidiStreamingServer[pb.InferRequest, pb.InferResponse]) error {
	var request *pb.InferRequest
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
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

	if request == nil {
		return status.Error(codes.InvalidArgument, "inference request is empty")
	}
	switch request.GetTask() {
	case types.TaskSemanticImageEmbed, types.TaskSemanticTextEmbed:
	default:
		return status.Errorf(codes.Unimplemented, "unsupported E2E task %q", request.GetTask())
	}

	return stream.Send(&pb.InferResponse{
		CorrelationId: request.GetCorrelationId(),
		IsFinal:       true,
		Result:        s.result,
		ResultMime:    "application/json;schema=embedding_v1",
	})
}

func cloneStrings(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func runHealthCheck(endpoint string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = pb.NewInferenceClient(conn).Health(ctx, &emptypb.Empty{})
	return err
}

func run(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	defer listener.Close()

	service, err := newInferenceServer()
	if err != nil {
		return err
	}
	server := grpc.NewServer()
	pb.RegisterInferenceServer(server, service)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		server.GracefulStop()
	}()

	log.Printf("deterministic Lumen E2E fixture listening on %s", listener.Addr())
	if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return err
	}
	return nil
}

func main() {
	address := flag.String("listen", defaultAddress, "gRPC listen address")
	healthCheck := flag.String("health-check", "", "probe an existing fixture endpoint")
	flag.Parse()

	var err error
	if *healthCheck != "" {
		err = runHealthCheck(*healthCheck)
	} else {
		err = run(*address)
	}
	if err != nil {
		log.Fatal(err)
	}
}
