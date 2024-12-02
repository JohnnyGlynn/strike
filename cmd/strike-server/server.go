package strike_server

import (
	"context"
	"fmt"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"
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

func InitServer() *strikeServer {
	s := strikeServer{}
	return &s
}
