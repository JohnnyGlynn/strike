// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.29.0
// source: router.proto

package pb

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
	RouterService_Register_FullMethodName        = "/router.RouterService/Register"
	RouterService_GetPublicKey_FullMethodName    = "/router.RouterService/GetPublicKey"
	RouterService_SendMessage_FullMethodName     = "/router.RouterService/SendMessage"
	RouterService_ReceiveMessages_FullMethodName = "/router.RouterService/ReceiveMessages"
	RouterService_Heartbeat_FullMethodName       = "/router.RouterService/Heartbeat"
)

// RouterServiceClient is the client API for RouterService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type RouterServiceClient interface {
	Register(ctx context.Context, in *RegisterRequest, opts ...grpc.CallOption) (*RegisterResponse, error)
	GetPublicKey(ctx context.Context, in *PublicKeyRequest, opts ...grpc.CallOption) (*PublicKeyResponse, error)
	SendMessage(ctx context.Context, in *Message, opts ...grpc.CallOption) (*SendResponse, error)
	ReceiveMessages(ctx context.Context, in *ReceiveRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[Message], error)
	Heartbeat(ctx context.Context, in *HeartbeatRequest, opts ...grpc.CallOption) (*HeartbeatResponse, error)
}

type routerServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewRouterServiceClient(cc grpc.ClientConnInterface) RouterServiceClient {
	return &routerServiceClient{cc}
}

func (c *routerServiceClient) Register(ctx context.Context, in *RegisterRequest, opts ...grpc.CallOption) (*RegisterResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(RegisterResponse)
	err := c.cc.Invoke(ctx, RouterService_Register_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *routerServiceClient) GetPublicKey(ctx context.Context, in *PublicKeyRequest, opts ...grpc.CallOption) (*PublicKeyResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(PublicKeyResponse)
	err := c.cc.Invoke(ctx, RouterService_GetPublicKey_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *routerServiceClient) SendMessage(ctx context.Context, in *Message, opts ...grpc.CallOption) (*SendResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SendResponse)
	err := c.cc.Invoke(ctx, RouterService_SendMessage_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *routerServiceClient) ReceiveMessages(ctx context.Context, in *ReceiveRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[Message], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &RouterService_ServiceDesc.Streams[0], RouterService_ReceiveMessages_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[ReceiveRequest, Message]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type RouterService_ReceiveMessagesClient = grpc.ServerStreamingClient[Message]

func (c *routerServiceClient) Heartbeat(ctx context.Context, in *HeartbeatRequest, opts ...grpc.CallOption) (*HeartbeatResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(HeartbeatResponse)
	err := c.cc.Invoke(ctx, RouterService_Heartbeat_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RouterServiceServer is the server API for RouterService service.
// All implementations must embed UnimplementedRouterServiceServer
// for forward compatibility.
type RouterServiceServer interface {
	Register(context.Context, *RegisterRequest) (*RegisterResponse, error)
	GetPublicKey(context.Context, *PublicKeyRequest) (*PublicKeyResponse, error)
	SendMessage(context.Context, *Message) (*SendResponse, error)
	ReceiveMessages(*ReceiveRequest, grpc.ServerStreamingServer[Message]) error
	Heartbeat(context.Context, *HeartbeatRequest) (*HeartbeatResponse, error)
	mustEmbedUnimplementedRouterServiceServer()
}

// UnimplementedRouterServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedRouterServiceServer struct{}

func (UnimplementedRouterServiceServer) Register(context.Context, *RegisterRequest) (*RegisterResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Register not implemented")
}
func (UnimplementedRouterServiceServer) GetPublicKey(context.Context, *PublicKeyRequest) (*PublicKeyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetPublicKey not implemented")
}
func (UnimplementedRouterServiceServer) SendMessage(context.Context, *Message) (*SendResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SendMessage not implemented")
}
func (UnimplementedRouterServiceServer) ReceiveMessages(*ReceiveRequest, grpc.ServerStreamingServer[Message]) error {
	return status.Errorf(codes.Unimplemented, "method ReceiveMessages not implemented")
}
func (UnimplementedRouterServiceServer) Heartbeat(context.Context, *HeartbeatRequest) (*HeartbeatResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Heartbeat not implemented")
}
func (UnimplementedRouterServiceServer) mustEmbedUnimplementedRouterServiceServer() {}
func (UnimplementedRouterServiceServer) testEmbeddedByValue()                       {}

// UnsafeRouterServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to RouterServiceServer will
// result in compilation errors.
type UnsafeRouterServiceServer interface {
	mustEmbedUnimplementedRouterServiceServer()
}

func RegisterRouterServiceServer(s grpc.ServiceRegistrar, srv RouterServiceServer) {
	// If the following call pancis, it indicates UnimplementedRouterServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&RouterService_ServiceDesc, srv)
}

func _RouterService_Register_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RegisterRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RouterServiceServer).Register(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: RouterService_Register_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RouterServiceServer).Register(ctx, req.(*RegisterRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _RouterService_GetPublicKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PublicKeyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RouterServiceServer).GetPublicKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: RouterService_GetPublicKey_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RouterServiceServer).GetPublicKey(ctx, req.(*PublicKeyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _RouterService_SendMessage_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Message)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RouterServiceServer).SendMessage(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: RouterService_SendMessage_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RouterServiceServer).SendMessage(ctx, req.(*Message))
	}
	return interceptor(ctx, in, info, handler)
}

func _RouterService_ReceiveMessages_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(ReceiveRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(RouterServiceServer).ReceiveMessages(m, &grpc.GenericServerStream[ReceiveRequest, Message]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type RouterService_ReceiveMessagesServer = grpc.ServerStreamingServer[Message]

func _RouterService_Heartbeat_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HeartbeatRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RouterServiceServer).Heartbeat(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: RouterService_Heartbeat_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RouterServiceServer).Heartbeat(ctx, req.(*HeartbeatRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// RouterService_ServiceDesc is the grpc.ServiceDesc for RouterService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var RouterService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "router.RouterService",
	HandlerType: (*RouterServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Register",
			Handler:    _RouterService_Register_Handler,
		},
		{
			MethodName: "GetPublicKey",
			Handler:    _RouterService_GetPublicKey_Handler,
		},
		{
			MethodName: "SendMessage",
			Handler:    _RouterService_SendMessage_Handler,
		},
		{
			MethodName: "Heartbeat",
			Handler:    _RouterService_Heartbeat_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "ReceiveMessages",
			Handler:       _RouterService_ReceiveMessages_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "router.proto",
}
