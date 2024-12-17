package strike_server

import (
	"bytes"
	"context"
	"encoding/binary"
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

func (s *strikeServer) Login(ctx context.Context, clientLogin *pb.ClientLogin) (*pb.Stamp, error) {
	return &pb.Stamp{}, nil
}

func (s *strikeServer) KeyHandshake(ctx context.Context, clientinit *pb.ClientInit) (*pb.Stamp, error) {
	//TODO: Create a "user" in the DB
	fmt.Printf("New User signup: %s\n", clientinit.Uname)
	fmt.Printf("%v's Public Key: %v\n", clientinit.Uname, clientinit.PublicKey)

	//TODO: Change message type to support keys not ints
	var keyAsInt int32
	binary.Read(bytes.NewReader(clientinit.PublicKey), binary.BigEndian, &keyAsInt)

	return &pb.Stamp{KeyUsed: keyAsInt}, nil
}

func InitServer() *strikeServer {
	s := strikeServer{}
	return &s
}
