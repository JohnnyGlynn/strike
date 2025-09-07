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

	"github.com/JohnnyGlynn/strike/internal/server/types"
	"github.com/JohnnyGlynn/strike/internal/shared"
	common_pb "github.com/JohnnyGlynn/strike/msgdef/common"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
)

type StrikeServer struct {
	pb.UnimplementedStrikeServer

	DBpool      *pgxpool.Pool
	PStatements *ServerDB
	Name        string
	ID          uuid.UUID

	// TODO: Package the stream better
	Connected       map[uuid.UUID]*common_pb.UserInfo
	PayloadStreams  map[uuid.UUID]pb.Strike_PayloadStreamServer
	PayloadChannels map[uuid.UUID]chan *pb.StreamPayload
	Pending         map[uuid.UUID]*types.PendingMsg //TODO: Memory constraint

	mu sync.Mutex

	//Federation
	Federation *FederationOrchestrator
}

func (s *StrikeServer) mapInit() {
	if s.Connected == nil {
		s.Connected = make(map[uuid.UUID]*common_pb.UserInfo)
	}
	if s.PayloadChannels == nil {
		s.PayloadChannels = make(map[uuid.UUID]chan *pb.StreamPayload)
	}
	if s.PayloadStreams == nil {
		s.PayloadStreams = make(map[uuid.UUID]pb.Strike_PayloadStreamServer)
	}
	if s.Pending == nil {
		s.Pending = make(map[uuid.UUID]*types.PendingMsg)
	}
}

func (s *StrikeServer) SendPayload(ctx context.Context, payload *pb.StreamPayload) (*pb.ServerResponse, error) {

	if payload == nil {
		return &pb.ServerResponse{Success: false, Message: "payload empty"}, fmt.Errorf("send payload: empty payload")
	}

	if payload.Target == "" {
		return &pb.ServerResponse{Success: false, Message: "missing target"}, fmt.Errorf("send payload: missing target")
	}

	parsedTarget, err := uuid.Parse(payload.Target)
	if err != nil {
		return &pb.ServerResponse{Success: false, Message: "invalid target id"}, fmt.Errorf("send payload: invalid target")
	}

	parsedSender, err := uuid.Parse(payload.Sender)
	if err != nil {
		return &pb.ServerResponse{Success: false, Message: "invalid sender id"}, fmt.Errorf("send payload: sender target")
	}

	//TODO: Handle some federated origin tracking here?

	messageID := uuid.New()

	s.mu.Lock()
	s.mapInit()
	pmsg := &types.PendingMsg{
		MessageID: messageID,
		From:      parsedSender,
		To:        parsedTarget,
		Payload:   payload,
		Created:   time.Now(),
		Attempts:  3,
	}

	s.Pending[messageID] = pmsg
	s.mu.Unlock()

	go s.attemptDelivery(messageID)

	return &pb.ServerResponse{Success: true, Message: fmt.Sprintf("relay-OK: %s", messageID.String())}, nil

}

func (s *StrikeServer) RoutePayload(pmsg *types.PendingMsg) (bool, error) {
	return false, nil
}

func (s *StrikeServer) attemptDelivery(messageID uuid.UUID) {
	const maxAttempts = 5

	for {
		s.mu.Lock()
		pmsg, ok := s.Pending[messageID]
		s.mu.Unlock()
		if !ok {
			//not Pending
			return
		}

		s.mu.Lock()
		ch, ok2 := s.PayloadChannels[pmsg.To]
		s.mu.Unlock()

		if ok2 && ch != nil {
			select {
			case ch <- pmsg.Payload: //proto.Clone?
				//TODO: Delivery receipt?
				fmt.Printf("Delivered: sent to local channel for %s", pmsg.To)
				return
			}
			//TODO: Timeout case
		} else {
			//handle federated delivery

			//check for my pending messages destination domain.
			// s.Federation.peers[pmsg.To]
			//begin acquiring a client, then sending a grpc message to Federation RoutePayload rpc
			//Unlock Fedreration and server?

		}
		s.mu.Lock()
		pmsg.Attempts++
		att := pmsg.Attempts
		s.mu.Unlock()

		if att >= maxAttempts {
			// failure to deliver case
		}

		//sleep
	}

}

func (s *StrikeServer) SaltMine(ctx context.Context, userInfo *common_pb.UserInfo) (*pb.Salt, error) {
	var salt []byte

	// TODO: ERROR this fails after server has been running long
	err := s.DBpool.QueryRow(ctx, s.PStatements.User.SaltMine, userInfo.Username).Scan(&salt)
	if err != nil {
		if pgerr, ok := err.(*pgconn.PgError); ok && pgerr.Code == "no-data-found" {
			fmt.Printf("Unable mine salt: %v", err)
			return nil, nil
		}
		fmt.Printf("An Error occured while mining salt: %v", err)
		return nil, nil
	}

	return &pb.Salt{Salt: salt}, nil
}

func (s *StrikeServer) Login(ctx context.Context, clientLogin *pb.LoginVerify) (*pb.ServerResponse, error) {
	var storedHash string

	err := s.DBpool.QueryRow(ctx, s.PStatements.User.LoginUser, clientLogin.Username).Scan(&storedHash)
	if err != nil {
		if pgerr, ok := err.(*pgconn.PgError); ok && pgerr.Code == "no-data-found" {
			fmt.Printf("Unable to verify user: %v", err)
			return nil, nil
		}
		fmt.Printf("An Error occured while verifying user: %v", err)
		return nil, nil
	}

	// verify our password is right
	// TODO: Check efficiency here, i.e. argon2 using 128mb ram
	passMatch, err := shared.VerifyPassword(clientLogin.PasswordHash, storedHash)
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
	_, err := s.DBpool.Exec(ctx, s.PStatements.User.CreateUser, uuid.MustParse(userInit.UserId), userInit.Username, userInit.PasswordHash, userInit.Salt.Salt)
	if err != nil {
		return &pb.ServerResponse{Success: false, Message: "failed to register user"}, err
	}

	// keys: uuid, encryption, signing
	_, err = s.DBpool.Exec(ctx, s.PStatements.Keys.CreatePublicKeys, uuid.MustParse(userInit.UserId), userInit.EncryptionPublicKey, userInit.SigningPublicKey)
	if err != nil {
		return &pb.ServerResponse{Success: false, Message: "failed to register user keys"}, err
	}

	return &pb.ServerResponse{
		Success: true,
		Message: "Signup successful",
	}, nil
}

func (s *StrikeServer) StatusStream(req *common_pb.UserInfo, stream pb.Strike_StatusStreamServer) error {

	// TODO: Parse function
	parsedId, err := uuid.Parse(req.UserId)
	if err != nil {
		return fmt.Errorf("failed to parse user ID: %v", err)
	}
	// Register the user as online
	s.mu.Lock()
	s.Connected[parsedId] = &common_pb.UserInfo{
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

func (s *StrikeServer) UserRequest(ctx context.Context, userInfo *common_pb.UserInfo) (*common_pb.UserInfo, error) {
	var userid uuid.UUID
	var encryptionPubKey, signingPubKey []byte

	err := s.DBpool.QueryRow(ctx, s.PStatements.User.GetUser, userInfo.Username).Scan(&userid)
	if err != nil {
		if pgerr, ok := err.(*pgconn.PgError); ok && pgerr.Code == "no-data-found" {
			fmt.Printf("Unable get username: %v", err)
			return nil, nil
		}
		fmt.Printf("Error acquiring username: %v", err)
		return nil, nil
	}

	row := s.DBpool.QueryRow(ctx, s.PStatements.Keys.GetPublicKeys, userid)
	if err := row.Scan(&encryptionPubKey, &signingPubKey); err != nil {
		fmt.Println("Failed to get keys")
		return nil, err
	}

	return &common_pb.UserInfo{UserId: userid.String(), Username: userInfo.Username, EncryptionPublicKey: encryptionPubKey, SigningPublicKey: signingPubKey}, nil
}

func (s *StrikeServer) OnlineUsers(ctx context.Context, userInfo *common_pb.UserInfo) (*common_pb.Users, error) {
	//TODO: Log user making request userInfo
	log.Printf("%s (%s) requested active user list\n", userInfo.Username, userInfo.UserId)

	//TODO: revisit
	s.mu.Lock()
	users := make([]*common_pb.UserInfo, 0, len(s.Connected))
	for _, v := range s.Connected {
		users = append(users, &common_pb.UserInfo{
			UserId:              v.UserId,
			Username:            v.Username,
			EncryptionPublicKey: v.EncryptionPublicKey,
			SigningPublicKey:    v.SigningPublicKey,
		})

	}
	s.mu.Unlock()

	return &common_pb.Users{Users: users}, nil
}

func (s *StrikeServer) PollServer(ctx context.Context, userInfo *common_pb.UserInfo) (*pb.ServerInfo, error) {
	//TODO: Wait groups?
	s.mu.Lock()
	users := make([]*common_pb.UserInfo, 0, len(s.Connected))
	for _, v := range s.Connected {
		users = append(users, &common_pb.UserInfo{
			UserId:              v.UserId,
			Username:            v.Username,
			EncryptionPublicKey: v.EncryptionPublicKey,
			SigningPublicKey:    v.SigningPublicKey,
		})

	}
	s.mu.Unlock()

	return &pb.ServerInfo{
		ServerId:   s.ID.String(),
		ServerName: s.Name,
		Users:      users,
	}, nil
}

func (s *StrikeServer) PayloadStream(user *common_pb.UserInfo, stream pb.Strike_PayloadStreamServer) error {
	log.Printf("Stream Established: %v online \n", user.Username)

	parsedId, err := uuid.Parse(user.UserId)
	if err != nil {
		return fmt.Errorf("failed to parse user id: %v", err)
	}
	// Register the users message steam
	s.mu.Lock()
	s.PayloadStreams[parsedId] = stream
	s.mu.Unlock()

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
