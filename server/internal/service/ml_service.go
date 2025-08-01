package service

import (
	"context"
	"errors"

	pb "server/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// Error constants
var (
	ErrCLIPServiceUnavailable = errors.New("CLIP service not available")
	ErrUnknownService         = errors.New("Unknown service")
	ErrInternal               = errors.New("Internal server error, please check PML service error log")
	ErrNotAnimalLike          = errors.New("Not an animal‑like image (BioAtlas skipped)")
)

// handleGRPCError handles gRPC errors and returns the appropriate error message.
func handleGRPCError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	switch st.Code() {
	case codes.FailedPrecondition:
		return ErrCLIPServiceUnavailable
	case codes.Unimplemented:
		return ErrUnknownService
	case codes.Internal:
		return ErrInternal
	default:
		return err
	}
}

type MLService interface {
	ProcessImageForCLIP(req *pb.ImageProcessRequest) (*pb.ImageProcessResponse, error)
	GetTextEmbeddingForCLIP(req *pb.TextEmbeddingRequest) (*pb.TextEmbeddingResponse, error)
	Predict(req *pb.PredictRequest) (*pb.PredictResponse, error)
	BatchPredict(req *pb.BatchPredictRequest) (*pb.BatchPredictResponse, error)
	HealthCheck(req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error)
	GetSpeciesForBioAtlas(req *pb.ImageProcessRequest) (*pb.BioAtlasResponse, error)
}

type mlService struct {
	conn   *grpc.ClientConn
	client pb.PredictionServiceClient
}

// NewMLClient TODO: Change WithInsecure into WithTransportCredential()
func NewMLClient(addr string) (MLService, error) {
	// 	creds, err := credentials.NewClientTLSFromFile("ca.pem", "")
	// if err != nil {
	//     log.Fatalf("Failed to load TLS credentials: %v", err)
	// }
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	client := pb.NewPredictionServiceClient(conn)
	return &mlService{conn: conn, client: client}, nil
}

func (m *mlService) ProcessImageForCLIP(req *pb.ImageProcessRequest) (*pb.ImageProcessResponse, error) {
	resp, err := m.client.ProcessImageForCLIP(context.Background(), req)
	if err != nil {
		return nil, handleGRPCError(err)
	}
	return resp, nil
}

func (m *mlService) GetSpeciesForBioAtlas(req *pb.ImageProcessRequest) (*pb.BioAtlasResponse, error) {
	resp, err := m.client.GetSpeciesForBioAtlas(context.Background(), req)
	if err != nil {
		return nil, handleGRPCError(err)
	}

	switch resp.GetStatus() {
	case pb.BioAtlasResponse_OK:
		return resp, nil
	case pb.BioAtlasResponse_NOT_ANIMAL:
		// No species found – still return a valid (empty) response, no error
		return resp, nil
	case pb.BioAtlasResponse_MODEL_ERROR:
		return nil, ErrInternal
	default:
		// Any unrecognised status – treat as internal for safety
		return nil, ErrInternal
	}
}

func (m *mlService) GetTextEmbeddingForCLIP(req *pb.TextEmbeddingRequest) (*pb.TextEmbeddingResponse, error) {
	resp, err := m.client.GetTextEmbeddingForCLIP(context.Background(), req)
	if err != nil {
		return nil, handleGRPCError(err)
	}
	return resp, nil
}

func (m *mlService) Predict(req *pb.PredictRequest) (*pb.PredictResponse, error) {
	resp, err := m.client.Predict(context.Background(), req)
	if err != nil {
		return nil, handleGRPCError(err)
	}
	return resp, nil
}

func (m *mlService) BatchPredict(req *pb.BatchPredictRequest) (*pb.BatchPredictResponse, error) {
	resp, err := m.client.BatchPredict(context.Background(), req)
	if err != nil {
		return nil, handleGRPCError(err)
	}
	return resp, nil
}

func (m *mlService) HealthCheck(req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	resp, err := m.client.HealthCheck(context.Background(), req)
	if err != nil {
		return nil, handleGRPCError(err)
	}
	return resp, nil
}
