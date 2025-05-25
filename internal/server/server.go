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
	// "github.com/JohnnyGlynn/strike/internal/db"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
)

type StrikeServer struct {
	pb.UnimplementedStrikeServer
	Env         []*pb.EncryptedEnvelope
	DBpool      *pgxpool.Pool
	PStatements *ServerDB

	// TODO: Package the stream better
	Connected       map[uuid.UUID]*pb.UserInfo
	PayloadStreams  map[uuid.UUID]pb.Strike_PayloadStreamServer
	PayloadChannels map[uuid.UUID]chan *pb.StreamPayload
	mu              sync.Mutex
}

func (s *StrikeServer) SendPayload(ctx context.Context, payload *pb.StreamPayload) (*pb.ServerResponse, error) {
	// TODO: Work out the most effecient Syncing for mutexs, this is a lot of locking and unlocking

	fmt.Printf("payload from SendPayload: %v", payload.Payload)
	fmt.Printf("uuid from SendPayload: %v", payload.Target)

	parsedId := uuid.MustParse(payload.Target)

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
		log.Printf("Payload sent to: %s From: %v\n", payload.Target, payload.Sender)
		return &pb.ServerResponse{Success: true, Message: "PAYLOAD OK"}, nil
	default:
		return &pb.ServerResponse{Success: false}, fmt.Errorf("%s's channel is full", payload.Target)
	}
}

func (s *StrikeServer) SaltMine(ctx context.Context, userInfo *pb.UserInfo) (*pb.Salt, error) {
	var salt []byte

	// TODO: ERROR this fails after server has been running long
	err := s.DBpool.QueryRow(ctx, s.PStatements.SaltMine, userInfo.Username).Scan(&salt)
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

func (s *StrikeServer) Login(ctx context.Context, clientLogin *pb.LoginVerify) (*pb.ServerResponse, error) {
	var storedHash string

	err := s.DBpool.QueryRow(ctx, s.PStatements.LoginUser, clientLogin.Username).Scan(&storedHash)
	if err != nil {
		if pgerr, ok := err.(*pgconn.PgError); ok && pgerr.Code == "no-data-found" {
			log.Fatalf("Unable to verify user: %v", err)
			return nil, nil
		}
		log.Fatalf("An Error occured while verifying user: %v", err)
		return nil, nil
	}

	// verify our password is right
	// TODO: Check efficiency here, i.e. argon2 using 128mb ram
	passMatch, err := auth.VerifyPassword(clientLogin.PasswordHash, storedHash)
	if err != nil {
		return &pb.ServerResponse{Success: false, Message: "an error occured"}, err
	}

	if passMatch {
		return &pb.ServerResponse{Success: passMatch, Message: "User verification successful"}, nil
	} else {
		return &pb.ServerResponse{Success: passMatch, Message: "Unable to verify user"}, nil
	}
}

func (s *StrikeServer) Signup(ctx context.Context, userInit *pb.InitUser) (*pb.ServerResponse, error) {
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

func (s *StrikeServer) StatusStream(req *pb.UserInfo, stream pb.Strike_StatusStreamServer) error {
	// TODO: cleaner map initilization
	if s.Connected == nil {
		s.Connected = make(map[uuid.UUID]*pb.UserInfo)
	}

	// TODO: Parse function
	parsedId, err := uuid.Parse(req.UserId)
	if err != nil {
		return fmt.Errorf("failed to parse user ID: %v", err)
	}
	// Register the user as online
	s.mu.Lock()
	s.Connected[parsedId] = &pb.UserInfo{
		Username:            req.Username,
		UserId:              req.UserId,
		EncryptionPublicKey: req.EncryptionPublicKey,
		SigningPublicKey:    req.SigningPublicKey,
	}
	s.mu.Unlock()

	// Defer so regardless of how we exit (gracefully or an error), the user is removed from OnlineUsers
	defer func() {
		s.mu.Lock()
		delete(s.Connected, parsedId)
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
	var userid uuid.UUID
	var encryptionPubKey, signingPubKey []byte

	err := s.DBpool.QueryRow(ctx, s.PStatements.GetUser, userInfo.Username).Scan(&userid)
	if err != nil {
		if pgerr, ok := err.(*pgconn.PgError); ok && pgerr.Code == "no-data-found" {
			log.Fatalf("Unable get username: %v", err)
			return nil, nil
		}
		log.Fatalf("Error acquiring username: %v", err)
		return nil, nil
	}

	row := s.DBpool.QueryRow(ctx, s.PStatements.GetPublicKeys, userid)
	if err := row.Scan(&encryptionPubKey, &signingPubKey); err != nil {
		fmt.Println("Failed to get keys")
		return nil, err
	}

	return &pb.UserInfo{UserId: userid.String(), Username: userInfo.Username, EncryptionPublicKey: encryptionPubKey, SigningPublicKey: signingPubKey}, nil
}

func (s *StrikeServer) OnlineUsers(ctx context.Context, userInfo *pb.UserInfo) (*pb.UsersInfo, error) {
	//TODO: Log user making request userInfo
	log.Printf("%s (%s) requested active user list\n", userInfo.Username, userInfo.UserId)

	s.mu.Lock()
	users := make([]*pb.UserInfo, 0, len(s.Connected))
	for _, v := range s.Connected {
		users = append(users, &pb.UserInfo{
			UserId:              v.UserId,
			Username:            v.Username,
			EncryptionPublicKey: v.EncryptionPublicKey,
			SigningPublicKey:    v.SigningPublicKey,
		})

	}
	s.mu.Unlock()

	return &pb.UsersInfo{Users: users}, nil
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
