package server

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/JohnnyGlynn/strike/internal/db"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
)

type StrikeServer struct {
	pb.UnimplementedStrikeServer
	Env         []*pb.Envelope
	DBpool      *pgxpool.Pool
	PStatements *db.PreparedStatements

	OnlineUsers     map[string]pb.Strike_UserStatusServer
	MessageStreams  map[string]pb.Strike_GetMessagesServer
	MessageChannels map[string]chan *pb.Envelope
	mu              sync.Mutex
}

func (s *StrikeServer) GetMessages(username *pb.Username, stream pb.Strike_GetMessagesServer) error {
	log.Printf("Stream Established: %v online \n", username.Username)

	// TODO: cleaner map initilization
	if s.MessageStreams == nil {
		s.MessageStreams = make(map[string]pb.Strike_GetMessagesServer)
	}

	// Register the users message steam
	s.mu.Lock()
	s.MessageStreams[username.Username] = stream
	s.mu.Unlock()

	//create a channel for each connected client
	messageChannel := make(chan *pb.Envelope, 100)

	// Register the users message channel
	s.mu.Lock()
	s.MessageChannels[username.Username] = messageChannel
	s.mu.Unlock()

	//Defer our cleanup of stream map and message channel
	defer func() {
		s.mu.Lock()
		//TODO: removed streams and just use channels
		delete(s.MessageStreams, username.Username)
		delete(s.MessageChannels, username.Username)
		close(messageChannel) // Safely close the channel
		s.mu.Unlock()
		fmt.Printf("Client %s disconnected.\n", username.Username)
	}()

	//Goroutine to send messages from channel
	//Only exits when the channel is closed thanks to the for/range
	go func() {
		for msg := range messageChannel {
			if err := stream.Send(msg); err != nil {
				fmt.Printf("Failed to send message to %s: %v\n", username.Username, err)
				return
			}
		}
	}()

	for {
		select {
		case <-stream.Context().Done():
			// TODO: Graceful Disconnect/Shutdown
			fmt.Println("Client disconnected.")
			return nil
		case <-time.After(1 * time.Second):
			keepAlive := pb.Envelope{
				SenderPublicKey: []byte{}, //Send server key
				FromUser:        "SERVER",
				ToUser:          username.Username,
				SentAt:          timestamppb.Now(),
				Chat: &pb.Chat{
					Name:    "SERVER INFO",
					Message: "Ping: Keep alive",
				}}
			if err := stream.Send(&keepAlive); err != nil {
				fmt.Printf("Failed to Keep Alive: %v\n", err)
				return err
			}
		}
	}
}

func (s *StrikeServer) SendMessages(ctx context.Context, envelope *pb.Envelope) (*pb.Stamp, error) {
	//TODO: Work out the most effecient Syncing for mutexs, this is a lot of locking and unlocking
	s.mu.Lock()
	channel, ok := s.MessageChannels[envelope.ToUser]
	s.mu.Unlock()

	if !ok {
		fmt.Printf("%s is not able to recieve messages.\n", envelope.ToUser)
		//TODO: time to get rid of stamps
		return &pb.Stamp{}, fmt.Errorf("no message channel available for: %s", envelope.ToUser)
	}

	//Push Message into Channel TODO: handle full channel case
	select {
	case channel <- envelope:
		fmt.Printf("Message Sent: To %s, From %s\n", envelope.ToUser, envelope.FromUser)
		return &pb.Stamp{KeyUsed: envelope.SenderPublicKey}, nil
	default:
		fmt.Printf("Message not sent. %s's channel is full.\n", envelope.ToUser)
		return &pb.Stamp{}, fmt.Errorf("%s's channel is full", envelope.ToUser)
	}
}

// func (s *StrikeServer) Login(ctx context.Context, clientLogin *pb.ClientLogin) (*pb.Stamp, error) {
// 	fmt.Println("Logging in...")

// 	var exists bool

// 	err := s.DBpool.QueryRow(ctx, s.PStatements.LoginUser, clientLogin.Username, clientLogin.PublicKey).Scan(&exists)
// 	if err != nil {
// 		if pgerr, ok := err.(*pgconn.PgError); ok && pgerr.Code == "no-data-found" {
// 			log.Fatalf("Unable to login: %v", err)
// 			return nil, nil
// 		}
// 		log.Fatalf("An Error occured while logging in: %v", err)
// 		return nil, nil
// 	}

// 	fmt.Println("Login Successful...")

// 	return &pb.Stamp{KeyUsed: clientLogin.PublicKey}, nil
// }

func (s *StrikeServer) Signup(ctx context.Context, clientInit *pb.ClientInit) (*pb.ServerResponse, error) {
	fmt.Printf("New User signup: %s\n", clientInit.Username)

	_, err := s.DBpool.Exec(ctx, s.PStatements.CreateUser, clientInit.Username, clientInit.EncryptionKey, clientInit.SigningKey)
	if err != nil {
		return &pb.ServerResponse{Success: false, Message: "failed to register user"}, err
	}

	return &pb.ServerResponse{
		Success: true,
		Message: "Signup successful",
	}, nil
}

func (s *StrikeServer) UserStatus(req *pb.StatusRequest, stream pb.Strike_UserStatusServer) error {

	username := req.Username

	// TODO: cleaner map initilization
	if s.OnlineUsers == nil {
		s.OnlineUsers = make(map[string]pb.Strike_UserStatusServer)
	}

	// Register the user as online
	s.mu.Lock()
	s.OnlineUsers[username] = stream
	s.mu.Unlock()

	// Defer so regardless of how we exit (gracefully or an error), the user is removed from OnlineUsers
	defer func() {
		s.mu.Lock()
		delete(s.OnlineUsers, username)
		s.mu.Unlock()
		fmt.Printf("%s is now offline.\n", username)
	}()

	fmt.Printf("%s is online.\n", username)

	for {
		select {
		case <-stream.Context().Done():
			// TODO: Add cleanup post client disconnect?
			return nil
		case <-time.After(2 * time.Minute):
			//TODO: Countdown until disconnect
			err := stream.Send(&pb.StatusUpdate{
				Message:   "Still alive",
				UpdatedAt: timestamppb.Now(),
			})
			if err != nil {
				fmt.Printf("Failed to send status update: %v\n", err)
				return err
			}
		}
	}
}

func (s *StrikeServer) BeginChat(ctx context.Context, req *pb.BeginChatRequest) (*pb.BeginChatResponse, error) {

	fmt.Printf("Begining Chat: %s\n", req.ChatName)
	_, exists := s.MessageStreams[req.Target]
	if !exists {
		return nil, fmt.Errorf("%v not found", req.Target)
	}

	// ReadWrite mutex
	s.mu.Lock()
	targetMessageStream := s.MessageStreams[req.Target]
	s.mu.Unlock()

	// creationEvent := fmt.Sprintf("SYSTEM NOTIFICATION - CHAT CREATION: %s (Init: %s, Trgt: %s)", req.ChatName, req.Target, req.Initiator)
	// initiationMessage := fmt.Sprintf("ATTN %s: %s wants to begin a chat! y/n?", req.Target, req.Initiator)

	err := targetMessageStream.Send(&pb.Envelope{
		SenderPublicKey: []byte{},
		SentAt:          timestamppb.Now(),
		Chat: &pb.Chat{
			Name:    "SERVER-CHAT_REQUEST",
			Message: req.ChatName,
		},
	})
	if err != nil {
		fmt.Printf("Failed to send message on %s's stream: %v\n", req.Target, err)
		return nil, err
	}

	//TODO: Pass initiator keys, then db query for target keys here

	return &pb.BeginChatResponse{
		Success:          true,
		ChatId:           uuid.New().String(),
		ChatName:         req.ChatName,
		TargetPublicKey:  []byte{},
		TargetSigningKey: []byte{},
	}, nil

}

func (s *StrikeServer) ConfirmChat(ctx context.Context, req *pb.ConfirmChatRequest) (*pb.ServerResponse, error) {

	fmt.Printf("Confirming Chat: %s\n", req.ChatId)

	//TODO: Pass initiator keys, then db query for target keys here

	Confirmed := fmt.Sprintf("%v has accepted chat request, entering Chat", req.Confirmer)

	return &pb.ServerResponse{
		Success: true,
		Message: Confirmed,
	}, nil

}
