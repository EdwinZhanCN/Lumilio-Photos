// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.29.3
// source: ml_service.proto

package proto

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	PredictionService_ProcessImageForCLIP_FullMethodName        = "/prediction.PredictionService/ProcessImageForCLIP"
	PredictionService_GetTextEmbeddingForCLIP_FullMethodName    = "/prediction.PredictionService/GetTextEmbeddingForCLIP"
	PredictionService_ProcessImageForBioCLIP_FullMethodName     = "/prediction.PredictionService/ProcessImageForBioCLIP"
	PredictionService_GetTextEmbeddingForBioCLIP_FullMethodName = "/prediction.PredictionService/GetTextEmbeddingForBioCLIP"
	PredictionService_GetSpeciesForBioAtlas_FullMethodName      = "/prediction.PredictionService/GetSpeciesForBioAtlas"
	PredictionService_Predict_FullMethodName                    = "/prediction.PredictionService/Predict"
	PredictionService_BatchPredict_FullMethodName               = "/prediction.PredictionService/BatchPredict"
	PredictionService_HealthCheck_FullMethodName                = "/prediction.PredictionService/HealthCheck"
)

// PredictionServiceClient is the client API for PredictionService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// PredictionService: 通用机器学习预测服务接口
type PredictionServiceClient interface {
	// General CLIP proto
	ProcessImageForCLIP(ctx context.Context, in *ImageProcessRequest, opts ...grpc.CallOption) (*ImageProcessResponse, error)
	GetTextEmbeddingForCLIP(ctx context.Context, in *TextEmbeddingRequest, opts ...grpc.CallOption) (*TextEmbeddingResponse, error)
	// BioCLIP proto
	ProcessImageForBioCLIP(ctx context.Context, in *ImageProcessRequest, opts ...grpc.CallOption) (*ImageProcessResponse, error)
	GetTextEmbeddingForBioCLIP(ctx context.Context, in *TextEmbeddingRequest, opts ...grpc.CallOption) (*TextEmbeddingResponse, error)
	// Bio Atlas
	GetSpeciesForBioAtlas(ctx context.Context, in *ImageProcessRequest, opts ...grpc.CallOption) (*BioAtlasResponse, error)
	// Predict: 通用预测接口，可用于未来其他模型的推理（如分类、回归等）
	// 注意：这里的PredictRequest/Response可能需要根据具体模型调整
	Predict(ctx context.Context, in *PredictRequest, opts ...grpc.CallOption) (*PredictResponse, error)
	// BatchPredict: 批量通用预测接口
	BatchPredict(ctx context.Context, in *BatchPredictRequest, opts ...grpc.CallOption) (*BatchPredictResponse, error)
	// HealthCheck: 模型状态检查，支持指定模型名称
	HealthCheck(ctx context.Context, in *HealthCheckRequest, opts ...grpc.CallOption) (*HealthCheckResponse, error)
}

type predictionServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewPredictionServiceClient(cc grpc.ClientConnInterface) PredictionServiceClient {
	return &predictionServiceClient{cc}
}

func (c *predictionServiceClient) ProcessImageForCLIP(ctx context.Context, in *ImageProcessRequest, opts ...grpc.CallOption) (*ImageProcessResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ImageProcessResponse)
	err := c.cc.Invoke(ctx, PredictionService_ProcessImageForCLIP_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *predictionServiceClient) GetTextEmbeddingForCLIP(ctx context.Context, in *TextEmbeddingRequest, opts ...grpc.CallOption) (*TextEmbeddingResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(TextEmbeddingResponse)
	err := c.cc.Invoke(ctx, PredictionService_GetTextEmbeddingForCLIP_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *predictionServiceClient) ProcessImageForBioCLIP(ctx context.Context, in *ImageProcessRequest, opts ...grpc.CallOption) (*ImageProcessResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ImageProcessResponse)
	err := c.cc.Invoke(ctx, PredictionService_ProcessImageForBioCLIP_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *predictionServiceClient) GetTextEmbeddingForBioCLIP(ctx context.Context, in *TextEmbeddingRequest, opts ...grpc.CallOption) (*TextEmbeddingResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(TextEmbeddingResponse)
	err := c.cc.Invoke(ctx, PredictionService_GetTextEmbeddingForBioCLIP_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *predictionServiceClient) GetSpeciesForBioAtlas(ctx context.Context, in *ImageProcessRequest, opts ...grpc.CallOption) (*BioAtlasResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(BioAtlasResponse)
	err := c.cc.Invoke(ctx, PredictionService_GetSpeciesForBioAtlas_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *predictionServiceClient) Predict(ctx context.Context, in *PredictRequest, opts ...grpc.CallOption) (*PredictResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(PredictResponse)
	err := c.cc.Invoke(ctx, PredictionService_Predict_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *predictionServiceClient) BatchPredict(ctx context.Context, in *BatchPredictRequest, opts ...grpc.CallOption) (*BatchPredictResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(BatchPredictResponse)
	err := c.cc.Invoke(ctx, PredictionService_BatchPredict_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *predictionServiceClient) HealthCheck(ctx context.Context, in *HealthCheckRequest, opts ...grpc.CallOption) (*HealthCheckResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(HealthCheckResponse)
	err := c.cc.Invoke(ctx, PredictionService_HealthCheck_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PredictionServiceServer is the server API for PredictionService service.
// All implementations must embed UnimplementedPredictionServiceServer
// for forward compatibility.
//
// PredictionService: 通用机器学习预测服务接口
type PredictionServiceServer interface {
	// General CLIP proto
	ProcessImageForCLIP(context.Context, *ImageProcessRequest) (*ImageProcessResponse, error)
	GetTextEmbeddingForCLIP(context.Context, *TextEmbeddingRequest) (*TextEmbeddingResponse, error)
	// BioCLIP proto
	ProcessImageForBioCLIP(context.Context, *ImageProcessRequest) (*ImageProcessResponse, error)
	GetTextEmbeddingForBioCLIP(context.Context, *TextEmbeddingRequest) (*TextEmbeddingResponse, error)
	// Bio Atlas
	GetSpeciesForBioAtlas(context.Context, *ImageProcessRequest) (*BioAtlasResponse, error)
	// Predict: 通用预测接口，可用于未来其他模型的推理（如分类、回归等）
	// 注意：这里的PredictRequest/Response可能需要根据具体模型调整
	Predict(context.Context, *PredictRequest) (*PredictResponse, error)
	// BatchPredict: 批量通用预测接口
	BatchPredict(context.Context, *BatchPredictRequest) (*BatchPredictResponse, error)
	// HealthCheck: 模型状态检查，支持指定模型名称
	HealthCheck(context.Context, *HealthCheckRequest) (*HealthCheckResponse, error)
	mustEmbedUnimplementedPredictionServiceServer()
}

// UnimplementedPredictionServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedPredictionServiceServer struct{}

func (UnimplementedPredictionServiceServer) ProcessImageForCLIP(context.Context, *ImageProcessRequest) (*ImageProcessResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ProcessImageForCLIP not implemented")
}
func (UnimplementedPredictionServiceServer) GetTextEmbeddingForCLIP(context.Context, *TextEmbeddingRequest) (*TextEmbeddingResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTextEmbeddingForCLIP not implemented")
}
func (UnimplementedPredictionServiceServer) ProcessImageForBioCLIP(context.Context, *ImageProcessRequest) (*ImageProcessResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ProcessImageForBioCLIP not implemented")
}
func (UnimplementedPredictionServiceServer) GetTextEmbeddingForBioCLIP(context.Context, *TextEmbeddingRequest) (*TextEmbeddingResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTextEmbeddingForBioCLIP not implemented")
}
func (UnimplementedPredictionServiceServer) GetSpeciesForBioAtlas(context.Context, *ImageProcessRequest) (*BioAtlasResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetSpeciesForBioAtlas not implemented")
}
func (UnimplementedPredictionServiceServer) Predict(context.Context, *PredictRequest) (*PredictResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Predict not implemented")
}
func (UnimplementedPredictionServiceServer) BatchPredict(context.Context, *BatchPredictRequest) (*BatchPredictResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method BatchPredict not implemented")
}
func (UnimplementedPredictionServiceServer) HealthCheck(context.Context, *HealthCheckRequest) (*HealthCheckResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method HealthCheck not implemented")
}
func (UnimplementedPredictionServiceServer) mustEmbedUnimplementedPredictionServiceServer() {}
func (UnimplementedPredictionServiceServer) testEmbeddedByValue()                           {}

// UnsafePredictionServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to PredictionServiceServer will
// result in compilation errors.
type UnsafePredictionServiceServer interface {
	mustEmbedUnimplementedPredictionServiceServer()
}

func RegisterPredictionServiceServer(s grpc.ServiceRegistrar, srv PredictionServiceServer) {
	// If the following call pancis, it indicates UnimplementedPredictionServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&PredictionService_ServiceDesc, srv)
}

func _PredictionService_ProcessImageForCLIP_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ImageProcessRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PredictionServiceServer).ProcessImageForCLIP(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: PredictionService_ProcessImageForCLIP_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PredictionServiceServer).ProcessImageForCLIP(ctx, req.(*ImageProcessRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PredictionService_GetTextEmbeddingForCLIP_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TextEmbeddingRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PredictionServiceServer).GetTextEmbeddingForCLIP(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: PredictionService_GetTextEmbeddingForCLIP_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PredictionServiceServer).GetTextEmbeddingForCLIP(ctx, req.(*TextEmbeddingRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PredictionService_ProcessImageForBioCLIP_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ImageProcessRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PredictionServiceServer).ProcessImageForBioCLIP(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: PredictionService_ProcessImageForBioCLIP_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PredictionServiceServer).ProcessImageForBioCLIP(ctx, req.(*ImageProcessRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PredictionService_GetTextEmbeddingForBioCLIP_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TextEmbeddingRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PredictionServiceServer).GetTextEmbeddingForBioCLIP(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: PredictionService_GetTextEmbeddingForBioCLIP_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PredictionServiceServer).GetTextEmbeddingForBioCLIP(ctx, req.(*TextEmbeddingRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PredictionService_GetSpeciesForBioAtlas_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ImageProcessRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PredictionServiceServer).GetSpeciesForBioAtlas(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: PredictionService_GetSpeciesForBioAtlas_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PredictionServiceServer).GetSpeciesForBioAtlas(ctx, req.(*ImageProcessRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PredictionService_Predict_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PredictRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PredictionServiceServer).Predict(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: PredictionService_Predict_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PredictionServiceServer).Predict(ctx, req.(*PredictRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PredictionService_BatchPredict_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BatchPredictRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PredictionServiceServer).BatchPredict(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: PredictionService_BatchPredict_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PredictionServiceServer).BatchPredict(ctx, req.(*BatchPredictRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PredictionService_HealthCheck_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HealthCheckRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PredictionServiceServer).HealthCheck(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: PredictionService_HealthCheck_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PredictionServiceServer).HealthCheck(ctx, req.(*HealthCheckRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// PredictionService_ServiceDesc is the grpc.ServiceDesc for PredictionService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var PredictionService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "prediction.PredictionService",
	HandlerType: (*PredictionServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ProcessImageForCLIP",
			Handler:    _PredictionService_ProcessImageForCLIP_Handler,
		},
		{
			MethodName: "GetTextEmbeddingForCLIP",
			Handler:    _PredictionService_GetTextEmbeddingForCLIP_Handler,
		},
		{
			MethodName: "ProcessImageForBioCLIP",
			Handler:    _PredictionService_ProcessImageForBioCLIP_Handler,
		},
		{
			MethodName: "GetTextEmbeddingForBioCLIP",
			Handler:    _PredictionService_GetTextEmbeddingForBioCLIP_Handler,
		},
		{
			MethodName: "GetSpeciesForBioAtlas",
			Handler:    _PredictionService_GetSpeciesForBioAtlas_Handler,
		},
		{
			MethodName: "Predict",
			Handler:    _PredictionService_Predict_Handler,
		},
		{
			MethodName: "BatchPredict",
			Handler:    _PredictionService_BatchPredict_Handler,
		},
		{
			MethodName: "HealthCheck",
			Handler:    _PredictionService_HealthCheck_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "ml_service.proto",
}
