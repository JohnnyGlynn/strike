package server

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/JohnnyGlynn/strike/internal/auth"
	"github.com/JohnnyGlynn/strike/internal/db"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
)

type StrikeServer struct {
	pb.UnimplementedStrikeServer
	Env         []*pb.Envelope
	DBpool      *pgxpool.Pool
	PStatements *db.ServerDB

	// TODO: Package the stream better
	OnlineUsers     map[uuid.UUID]pb.Strike_UserStatusServer
	PayloadStreams  map[uuid.UUID]pb.Strike_PayloadStreamServer
	PayloadChannels map[uuid.UUID]chan *pb.StreamPayload
	mu              sync.Mutex
}

func (s *StrikeServer) SendPayload(ctx context.Context, payload *pb.StreamPayload) (*pb.ServerResponse, error) {
	// TODO: Work out the most effecient Syncing for mutexs, this is a lot of locking and unlocking

	parsedId, err := uuid.Parse(payload.TargetId)
	if err != nil {
		return nil, fmt.Errorf("failed to parse target uuid: %v", err)
	}
	s.mu.Lock()
	channel, ok := s.PayloadChannels[parsedId]
	s.mu.Unlock()

	if !ok {
		fmt.Printf("%s is not able to recieve messages.\n", payload.Target)
		return &pb.ServerResponse{Success: false}, fmt.Errorf("no message channel available for: %s", payload.Target)
	}

	// Push Message into Channel TODO: handle full channel case
	select {
	case channel <- payload:
		log.Printf("Payload Sent to: %s\n", payload.Target)
		return &pb.ServerResponse{Success: true, Message: "PAYLOAD OK"}, nil
	default:
		return &pb.ServerResponse{Success: false}, fmt.Errorf("%s's channel is full", payload.Target)
	}
}

func (s *StrikeServer) SaltMine(ctx context.Context, userInfo *pb.UserInfo) (*pb.Salt, error) {
	var salt []byte

	// TODO: ERROR this fails after server has been running long
	err := s.DBpool.QueryRow(ctx, s.PStatements.SaltMine, userInfo.UserId).Scan(&salt)
	if err != nil {
		if pgerr, ok := err.(*pgconn.PgError); ok && pgerr.Code == "no-data-found" {
			log.Fatalf("Unable mine salt: %v", err)
			return nil, nil
		}
		log.Fatalf("An Error occured while mining salt: %v", err)
		return nil, nil
	}

	return &pb.Salt{Salt: salt}, nil
}

func (s *StrikeServer) Login(ctx context.Context, clientLogin *pb.LoginRequest) (*pb.ServerResponse, error) {
	var storedHash string

	err := s.DBpool.QueryRow(ctx, s.PStatements.LoginUser, clientLogin.UserId).Scan(&storedHash)
	if err != nil {
		if pgerr, ok := err.(*pgconn.PgError); ok && pgerr.Code == "no-data-found" {
			log.Fatalf("Unable to login: %v", err)
			return nil, nil
		}
		log.Fatalf("An Error occured while logging in: %v", err)
		return nil, nil
	}

	// verify our password is right
	// TODO: Check efficiency here, i.e. argon2 using 128mb ram
	passMatch, err := auth.VerifyPassword(clientLogin.PasswordHash, storedHash)
	if err != nil {
		return &pb.ServerResponse{Success: false, Message: "an error occured"}, err
	}

	if passMatch {
		fmt.Printf("%s login successful\n", clientLogin.UserId)
		return &pb.ServerResponse{Success: passMatch, Message: "login successful"}, nil
	} else if !passMatch {
		fmt.Printf("failed login attempt for: %s\n", clientLogin.UserId)
		return &pb.ServerResponse{Success: passMatch, Message: "login unsuccessful"}, nil
	}

	// TODO: Make this unreachable?
	return &pb.ServerResponse{Success: false, Message: "How is this not unreachable???"}, nil
}

func (s *StrikeServer) Signup(ctx context.Context, userInit *pb.InitUser) (*pb.ServerResponse, error) {
	fmt.Printf("New User signup: %s\n", userInit.Username)

	// user: uuid, username, password_hash, salt
	_, err := s.DBpool.Exec(ctx, s.PStatements.CreateUser, uuid.MustParse(userInit.UserId), userInit.Username, userInit.PasswordHash, userInit.Salt.Salt)
	if err != nil {
		return &pb.ServerResponse{Success: false, Message: "failed to register user"}, err
	}

	// keys: uuid, encryption, signing
	_, err = s.DBpool.Exec(ctx, s.PStatements.CreatePublicKeys, uuid.MustParse(userInit.UserId), userInit.EncryptionPublicKey, userInit.SigningPublicKey)
	if err != nil {
		return &pb.ServerResponse{Success: false, Message: "failed to register user keys"}, err
	}

	return &pb.ServerResponse{
		Success: true,
		Message: "Signup successful",
	}, nil
}

func (s *StrikeServer) UserStatus(req *pb.UserInfo, stream pb.Strike_UserStatusServer) error {
	// TODO: cleaner map initilization
	if s.OnlineUsers == nil {
		s.OnlineUsers = make(map[uuid.UUID]pb.Strike_UserStatusServer)
	}

	// TODO: Parse function
	parsedId, err := uuid.Parse(req.UserId)
	if err != nil {
		return fmt.Errorf("failed to parse user ID: %v", err)
	}
	// Register the user as online
	s.mu.Lock()
	s.OnlineUsers[parsedId] = stream
	s.mu.Unlock()

	// Defer so regardless of how we exit (gracefully or an error), the user is removed from OnlineUsers
	defer func() {
		s.mu.Lock()
		delete(s.OnlineUsers, parsedId)
		s.mu.Unlock()
		fmt.Printf("%s is now offline.\n", req.Username)
	}()

	fmt.Printf("%s is online.\n", req.Username)

	for {
		select {
		case <-stream.Context().Done():
			// TODO: Add cleanup post client disconnect?
			return nil
		case <-time.After(2 * time.Minute):
			// TODO: Countdown until disconnect
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

func (s *StrikeServer) UserRequest(ctx context.Context, userInfo *pb.UserInfo) (*pb.UserInfo, error) {
	var reqUserId uuid.UUID
	var encryptionPubKey, signingPubKey []byte

	// TODO: db pool expiring
	err := s.DBpool.QueryRow(ctx, s.PStatements.GetUserId, userInfo.Username).Scan(&reqUserId)
	if err != nil {
		if pgerr, ok := err.(*pgconn.PgError); ok && pgerr.Code == "no-data-found" {
			log.Fatalf("Unable mine salt: %v", err)
			return nil, nil
		}
		log.Fatalf("An Error occured while mining salt: %v", err)
		return nil, nil
	}

	row := s.DBpool.QueryRow(ctx, s.PStatements.GetUserKeys, &reqUserId)
	if err := row.Scan(&encryptionPubKey, &signingPubKey); err != nil {
		return nil, err
	}

	return &pb.UserInfo{UserId: reqUserId.String(), Username: userInfo.Username, EncryptionPublicKey: encryptionPubKey, SigningPublicKey: signingPubKey}, nil
}

func (s *StrikeServer) PayloadStream(user *pb.UserInfo, stream pb.Strike_PayloadStreamServer) error {
	log.Printf("Stream Established: %v online \n", user.Username)

	// TODO: cleaner map initilization
	if s.PayloadStreams == nil {
		s.PayloadStreams = make(map[uuid.UUID]pb.Strike_PayloadStreamServer)
	}

	parsedId, err := uuid.Parse(user.UserId)
	if err != nil {
		return fmt.Errorf("failed to parse user id: %v", err)
	}
	// Register the users message steam
	s.mu.Lock()
	s.PayloadStreams[parsedId] = stream
	s.mu.Unlock()

	if s.PayloadChannels == nil {
		s.PayloadChannels = make(map[uuid.UUID]chan *pb.StreamPayload)
	}

	// create a channel for each connected client
	payloadChannel := make(chan *pb.StreamPayload, 100)

	// Register the users message channel
	s.mu.Lock()
	s.PayloadChannels[parsedId] = payloadChannel
	s.mu.Unlock()

	// Defer our cleanup of stream map and message channel
	defer func() {
		s.mu.Lock()
		delete(s.PayloadStreams, parsedId)
		delete(s.PayloadChannels, parsedId)
		close(payloadChannel) // Safely close the channel
		s.mu.Unlock()
		fmt.Printf("Client %s disconnected.\n", user.Username)
	}()

	// Goroutine to send messages from channel
	// Only exits when the channel is closed thanks to the for/range
	go func() {
		for msg := range payloadChannel {
			if err := stream.Send(msg); err != nil {
				fmt.Printf("Failed to send message to %s: %v\n", user.Username, err)
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
		case <-time.After(1 * time.Minute):
			// TODO: Heart Beat
		}
	}
}
