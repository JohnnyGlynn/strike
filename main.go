package main

import (
	"context"
	"fmt"
	"log"
	"net"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"

	grpc "google.golang.org/grpc"
)

type strikeServer struct {
	pb.UnimplementedStrikeServer
	Env []*pb.Envelope
	// mu sync.Mutex
}

func (s *strikeServer) GetMessages(chat *pb.Chat, stream pb.Strike_GetMessagesServer) error {
	for _, envelope := range s.Env {
		if envelope.Chat.Name == "endpoint0" {
			if err := stream.Send(envelope); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *strikeServer) SendMessages(ctx context.Context, envelope *pb.Envelope) (*pb.Stamp, error) {
	fmt.Printf("Received message: %s\n", envelope)
	return &pb.Stamp{KeyUsed: envelope.SenderPublicKey}, nil
}

func main() {
	fmt.Println("Strike Server")

	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption

	srvr := grpc.NewServer(opts...)
	//s := &pb.StrikeServer{}
	pb.RegisterStrikeServer(srvr, newServer())

	srvr.Serve(lis)

	err = srvr.Serve(lis)
	if err != nil {
		fmt.Printf("Error")
	}

}

func newServer() *strikeServer {
	s := strikeServer{}
	return &s
}
