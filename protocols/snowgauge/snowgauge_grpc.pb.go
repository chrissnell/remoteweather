// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.4.0
// - protoc             v5.27.1
// source: protocols/snowgauge/snowgauge.proto

package snowgauge

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.62.0 or later.
const _ = grpc.SupportPackageIsVersion8

const (
	SnowGaugeService_StreamReading_FullMethodName = "/snowgauge.SnowGaugeService/StreamReading"
)

// SnowGaugeServiceClient is the client API for SnowGaugeService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// Define the gRPC service
type SnowGaugeServiceClient interface {
	StreamReading(ctx context.Context, in *StreamRequest, opts ...grpc.CallOption) (SnowGaugeService_StreamReadingClient, error)
}

type snowGaugeServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewSnowGaugeServiceClient(cc grpc.ClientConnInterface) SnowGaugeServiceClient {
	return &snowGaugeServiceClient{cc}
}

func (c *snowGaugeServiceClient) StreamReading(ctx context.Context, in *StreamRequest, opts ...grpc.CallOption) (SnowGaugeService_StreamReadingClient, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &SnowGaugeService_ServiceDesc.Streams[0], SnowGaugeService_StreamReading_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &snowGaugeServiceStreamReadingClient{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type SnowGaugeService_StreamReadingClient interface {
	Recv() (*Reading, error)
	grpc.ClientStream
}

type snowGaugeServiceStreamReadingClient struct {
	grpc.ClientStream
}

func (x *snowGaugeServiceStreamReadingClient) Recv() (*Reading, error) {
	m := new(Reading)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// SnowGaugeServiceServer is the server API for SnowGaugeService service.
// All implementations must embed UnimplementedSnowGaugeServiceServer
// for forward compatibility
//
// Define the gRPC service
type SnowGaugeServiceServer interface {
	StreamReading(*StreamRequest, SnowGaugeService_StreamReadingServer) error
	mustEmbedUnimplementedSnowGaugeServiceServer()
}

// UnimplementedSnowGaugeServiceServer must be embedded to have forward compatible implementations.
type UnimplementedSnowGaugeServiceServer struct {
}

func (UnimplementedSnowGaugeServiceServer) StreamReading(*StreamRequest, SnowGaugeService_StreamReadingServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamReading not implemented")
}
func (UnimplementedSnowGaugeServiceServer) mustEmbedUnimplementedSnowGaugeServiceServer() {}

// UnsafeSnowGaugeServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SnowGaugeServiceServer will
// result in compilation errors.
type UnsafeSnowGaugeServiceServer interface {
	mustEmbedUnimplementedSnowGaugeServiceServer()
}

func RegisterSnowGaugeServiceServer(s grpc.ServiceRegistrar, srv SnowGaugeServiceServer) {
	s.RegisterService(&SnowGaugeService_ServiceDesc, srv)
}

func _SnowGaugeService_StreamReading_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(StreamRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(SnowGaugeServiceServer).StreamReading(m, &snowGaugeServiceStreamReadingServer{ServerStream: stream})
}

type SnowGaugeService_StreamReadingServer interface {
	Send(*Reading) error
	grpc.ServerStream
}

type snowGaugeServiceStreamReadingServer struct {
	grpc.ServerStream
}

func (x *snowGaugeServiceStreamReadingServer) Send(m *Reading) error {
	return x.ServerStream.SendMsg(m)
}

// SnowGaugeService_ServiceDesc is the grpc.ServiceDesc for SnowGaugeService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var SnowGaugeService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "snowgauge.SnowGaugeService",
	HandlerType: (*SnowGaugeServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "StreamReading",
			Handler:       _SnowGaugeService_StreamReading_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "protocols/snowgauge/snowgauge.proto",
}
