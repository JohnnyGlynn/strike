// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.4.0
// - protoc             v5.29.3
// source: message.proto

package message

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
	Strike_Signup_FullMethodName       = "/message.Strike/Signup"
	Strike_BeginChat_FullMethodName    = "/message.Strike/BeginChat"
	Strike_ConfirmChat_FullMethodName  = "/message.Strike/ConfirmChat"
	Strike_KeyHandshake_FullMethodName = "/message.Strike/KeyHandshake"
	Strike_Login_FullMethodName        = "/message.Strike/Login"
	Strike_SendMessages_FullMethodName = "/message.Strike/SendMessages"
	Strike_UserStatus_FullMethodName   = "/message.Strike/UserStatus"
	Strike_GetMessages_FullMethodName  = "/message.Strike/GetMessages"
)

// StrikeClient is the client API for Strike service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type StrikeClient interface {
	Signup(ctx context.Context, in *ClientInit, opts ...grpc.CallOption) (*ServerResponse, error)
	BeginChat(ctx context.Context, in *BeginChatRequest, opts ...grpc.CallOption) (*BeginChatResponse, error)
	ConfirmChat(ctx context.Context, in *ConfirmChatRequest, opts ...grpc.CallOption) (*ServerResponse, error)
	KeyHandshake(ctx context.Context, in *ClientInit, opts ...grpc.CallOption) (*Stamp, error)
	Login(ctx context.Context, in *ClientLogin, opts ...grpc.CallOption) (*Stamp, error)
	SendMessages(ctx context.Context, in *Envelope, opts ...grpc.CallOption) (*Stamp, error)
	UserStatus(ctx context.Context, in *StatusRequest, opts ...grpc.CallOption) (Strike_UserStatusClient, error)
	GetMessages(ctx context.Context, in *Username, opts ...grpc.CallOption) (Strike_GetMessagesClient, error)
}

type strikeClient struct {
	cc grpc.ClientConnInterface
}

func NewStrikeClient(cc grpc.ClientConnInterface) StrikeClient {
	return &strikeClient{cc}
}

func (c *strikeClient) Signup(ctx context.Context, in *ClientInit, opts ...grpc.CallOption) (*ServerResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ServerResponse)
	err := c.cc.Invoke(ctx, Strike_Signup_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *strikeClient) BeginChat(ctx context.Context, in *BeginChatRequest, opts ...grpc.CallOption) (*BeginChatResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(BeginChatResponse)
	err := c.cc.Invoke(ctx, Strike_BeginChat_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *strikeClient) ConfirmChat(ctx context.Context, in *ConfirmChatRequest, opts ...grpc.CallOption) (*ServerResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ServerResponse)
	err := c.cc.Invoke(ctx, Strike_ConfirmChat_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *strikeClient) KeyHandshake(ctx context.Context, in *ClientInit, opts ...grpc.CallOption) (*Stamp, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Stamp)
	err := c.cc.Invoke(ctx, Strike_KeyHandshake_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *strikeClient) Login(ctx context.Context, in *ClientLogin, opts ...grpc.CallOption) (*Stamp, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Stamp)
	err := c.cc.Invoke(ctx, Strike_Login_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *strikeClient) SendMessages(ctx context.Context, in *Envelope, opts ...grpc.CallOption) (*Stamp, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Stamp)
	err := c.cc.Invoke(ctx, Strike_SendMessages_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *strikeClient) UserStatus(ctx context.Context, in *StatusRequest, opts ...grpc.CallOption) (Strike_UserStatusClient, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &Strike_ServiceDesc.Streams[0], Strike_UserStatus_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &strikeUserStatusClient{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Strike_UserStatusClient interface {
	Recv() (*StatusUpdate, error)
	grpc.ClientStream
}

type strikeUserStatusClient struct {
	grpc.ClientStream
}

func (x *strikeUserStatusClient) Recv() (*StatusUpdate, error) {
	m := new(StatusUpdate)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *strikeClient) GetMessages(ctx context.Context, in *Username, opts ...grpc.CallOption) (Strike_GetMessagesClient, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &Strike_ServiceDesc.Streams[1], Strike_GetMessages_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &strikeGetMessagesClient{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Strike_GetMessagesClient interface {
	Recv() (*Envelope, error)
	grpc.ClientStream
}

type strikeGetMessagesClient struct {
	grpc.ClientStream
}

func (x *strikeGetMessagesClient) Recv() (*Envelope, error) {
	m := new(Envelope)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// StrikeServer is the server API for Strike service.
// All implementations must embed UnimplementedStrikeServer
// for forward compatibility
type StrikeServer interface {
	Signup(context.Context, *ClientInit) (*ServerResponse, error)
	BeginChat(context.Context, *BeginChatRequest) (*BeginChatResponse, error)
	ConfirmChat(context.Context, *ConfirmChatRequest) (*ServerResponse, error)
	KeyHandshake(context.Context, *ClientInit) (*Stamp, error)
	Login(context.Context, *ClientLogin) (*Stamp, error)
	SendMessages(context.Context, *Envelope) (*Stamp, error)
	UserStatus(*StatusRequest, Strike_UserStatusServer) error
	GetMessages(*Username, Strike_GetMessagesServer) error
	mustEmbedUnimplementedStrikeServer()
}

// UnimplementedStrikeServer must be embedded to have forward compatible implementations.
type UnimplementedStrikeServer struct {
}

func (UnimplementedStrikeServer) Signup(context.Context, *ClientInit) (*ServerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Signup not implemented")
}
func (UnimplementedStrikeServer) BeginChat(context.Context, *BeginChatRequest) (*BeginChatResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method BeginChat not implemented")
}
func (UnimplementedStrikeServer) ConfirmChat(context.Context, *ConfirmChatRequest) (*ServerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ConfirmChat not implemented")
}
func (UnimplementedStrikeServer) KeyHandshake(context.Context, *ClientInit) (*Stamp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method KeyHandshake not implemented")
}
func (UnimplementedStrikeServer) Login(context.Context, *ClientLogin) (*Stamp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Login not implemented")
}
func (UnimplementedStrikeServer) SendMessages(context.Context, *Envelope) (*Stamp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SendMessages not implemented")
}
func (UnimplementedStrikeServer) UserStatus(*StatusRequest, Strike_UserStatusServer) error {
	return status.Errorf(codes.Unimplemented, "method UserStatus not implemented")
}
func (UnimplementedStrikeServer) GetMessages(*Username, Strike_GetMessagesServer) error {
	return status.Errorf(codes.Unimplemented, "method GetMessages not implemented")
}
func (UnimplementedStrikeServer) mustEmbedUnimplementedStrikeServer() {}

// UnsafeStrikeServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to StrikeServer will
// result in compilation errors.
type UnsafeStrikeServer interface {
	mustEmbedUnimplementedStrikeServer()
}

func RegisterStrikeServer(s grpc.ServiceRegistrar, srv StrikeServer) {
	s.RegisterService(&Strike_ServiceDesc, srv)
}

func _Strike_Signup_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ClientInit)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StrikeServer).Signup(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Strike_Signup_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StrikeServer).Signup(ctx, req.(*ClientInit))
	}
	return interceptor(ctx, in, info, handler)
}

func _Strike_BeginChat_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BeginChatRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StrikeServer).BeginChat(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Strike_BeginChat_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StrikeServer).BeginChat(ctx, req.(*BeginChatRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Strike_ConfirmChat_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ConfirmChatRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StrikeServer).ConfirmChat(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Strike_ConfirmChat_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StrikeServer).ConfirmChat(ctx, req.(*ConfirmChatRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Strike_KeyHandshake_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ClientInit)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StrikeServer).KeyHandshake(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Strike_KeyHandshake_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StrikeServer).KeyHandshake(ctx, req.(*ClientInit))
	}
	return interceptor(ctx, in, info, handler)
}

func _Strike_Login_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ClientLogin)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StrikeServer).Login(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Strike_Login_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StrikeServer).Login(ctx, req.(*ClientLogin))
	}
	return interceptor(ctx, in, info, handler)
}

func _Strike_SendMessages_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Envelope)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StrikeServer).SendMessages(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Strike_SendMessages_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StrikeServer).SendMessages(ctx, req.(*Envelope))
	}
	return interceptor(ctx, in, info, handler)
}

func _Strike_UserStatus_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(StatusRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(StrikeServer).UserStatus(m, &strikeUserStatusServer{ServerStream: stream})
}

type Strike_UserStatusServer interface {
	Send(*StatusUpdate) error
	grpc.ServerStream
}

type strikeUserStatusServer struct {
	grpc.ServerStream
}

func (x *strikeUserStatusServer) Send(m *StatusUpdate) error {
	return x.ServerStream.SendMsg(m)
}

func _Strike_GetMessages_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(Username)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(StrikeServer).GetMessages(m, &strikeGetMessagesServer{ServerStream: stream})
}

type Strike_GetMessagesServer interface {
	Send(*Envelope) error
	grpc.ServerStream
}

type strikeGetMessagesServer struct {
	grpc.ServerStream
}

func (x *strikeGetMessagesServer) Send(m *Envelope) error {
	return x.ServerStream.SendMsg(m)
}

// Strike_ServiceDesc is the grpc.ServiceDesc for Strike service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Strike_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "message.Strike",
	HandlerType: (*StrikeServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Signup",
			Handler:    _Strike_Signup_Handler,
		},
		{
			MethodName: "BeginChat",
			Handler:    _Strike_BeginChat_Handler,
		},
		{
			MethodName: "ConfirmChat",
			Handler:    _Strike_ConfirmChat_Handler,
		},
		{
			MethodName: "KeyHandshake",
			Handler:    _Strike_KeyHandshake_Handler,
		},
		{
			MethodName: "Login",
			Handler:    _Strike_Login_Handler,
		},
		{
			MethodName: "SendMessages",
			Handler:    _Strike_SendMessages_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "UserStatus",
			Handler:       _Strike_UserStatus_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "GetMessages",
			Handler:       _Strike_GetMessages_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "message.proto",
}
