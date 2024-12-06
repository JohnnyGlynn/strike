package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	pb "messenger/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RouterServer struct {
	pb.UnimplementedRouterServiceServer
	mu            sync.Mutex
	clients       map[string]chan *pb.Message // Maps usernames to message channels
	publicKeys    map[string]string           // Maps usernames to public keys
	activeClients map[string]time.Time        // Maps usernames to their last heartbeat
}

func NewRouterServer() *RouterServer {
	server := &RouterServer{
		clients:       make(map[string]chan *pb.Message),
		publicKeys:    make(map[string]string),
		activeClients: make(map[string]time.Time),
	}

	// Start the cleanup goroutine
	go server.cleanupInactiveClients()

	return server
}

// Register registers a client with the server.
func (s *RouterServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.clients[req.Username]; exists {
		return nil, status.Errorf(codes.AlreadyExists, "User already registered")
	}

	// Store user channel and public key
	s.clients[req.Username] = make(chan *pb.Message, 10)
	s.publicKeys[req.Username] = req.PublicKey
	s.activeClients[req.Username] = time.Now()
	return &pb.RegisterResponse{Status: "Registered successfully"}, nil
}

func (s *RouterServer) GetPublicKey(ctx context.Context, req *pb.PublicKeyRequest) (*pb.PublicKeyResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	publicKey, exists := s.publicKeys[req.Username]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "User not found")
	}
	return &pb.PublicKeyResponse{PublicKey: publicKey}, nil
}

// SendMessage routes a message to the recipient if online.
func (s *RouterServer) SendMessage(ctx context.Context, req *pb.Message) (*pb.SendResponse, error) {
	s.mu.Lock()
	recipientChan, online := s.clients[req.Recipient]
	s.mu.Unlock()

	if !online {
		return nil, status.Errorf(codes.Unavailable, "Recipient is not online")
	}

	// Deliver the encrypted message to the recipient
	recipientChan <- req
	return &pb.SendResponse{Status: "Message delivered"}, nil
}

// ReceiveMessages streams messages to a connected client.
func (s *RouterServer) ReceiveMessages(req *pb.ReceiveRequest, stream pb.RouterService_ReceiveMessagesServer) error {
	s.mu.Lock()
	messageChannel, exists := s.clients[req.Username]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("user %s is not registered", req.Username)
	}
	s.mu.Unlock()

	log.Printf("User %s is now receiving messages", req.Username)

	// Stream messages to the client
	for msg := range messageChannel {
		if err := stream.Send(msg); err != nil {
			log.Printf("Failed to send message to %s: %v", req.Username, err)
			break
		}
	}

	log.Printf("User %s disconnected from message stream", req.Username)
	return nil
}

func (s *RouterServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	username := req.GetUsername()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.activeClients[username] = time.Now()
	log.Printf("Heartbeat received from %s", username)

	return &pb.HeartbeatResponse{Status: "alive"}, nil
}

func (s *RouterServer) cleanupInactiveClients() {
	for {
		time.Sleep(30 * time.Second) // Adjust interval as needed

		s.mu.Lock()
		for username, lastActive := range s.activeClients {
			if time.Since(lastActive) > 1*time.Minute { // If inactive for more than 2 minutes
				log.Printf("Client %s is inactive, cleaning up", username)

				// Clean up the client from all maps
				delete(s.activeClients, username)
				delete(s.clients, username)
				delete(s.publicKeys, username)
			}
		}
		s.mu.Unlock()
	}
}

func main() {
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterRouterServiceServer(grpcServer, NewRouterServer())
	log.Println("Server is running on port 50051")

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
