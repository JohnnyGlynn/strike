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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/JohnnyGlynn/strike/internal/server/types"
	"github.com/JohnnyGlynn/strike/internal/shared"
	common_pb "github.com/JohnnyGlynn/strike/msgdef/common"
	fedpb "github.com/JohnnyGlynn/strike/msgdef/federation"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
)

type StrikeServer struct {
	pb.UnimplementedStrikeServer

	ID   uuid.UUID
	Name string

	PeerMgr *PeerManager

	DBpool      *pgxpool.Pool
	PStatements *ServerDB

	Connected       map[uuid.UUID]*common_pb.UserInfo
	PayloadStreams  map[uuid.UUID]pb.Strike_PayloadStreamServer
	PayloadChannels map[uuid.UUID]chan *pb.StreamPayload

	Pending        map[uuid.UUID]*types.PendingMsg
	mu             sync.Mutex
	RemotePresence map[uuid.UUID]string
}

func (s *StrikeServer) mapInit() {
	if s.Connected == nil {
		s.Connected = make(map[uuid.UUID]*common_pb.UserInfo)
	}
	if s.PayloadStreams == nil {
		s.PayloadStreams = make(map[uuid.UUID]pb.Strike_PayloadStreamServer)
	}
	if s.PayloadChannels == nil {
		s.PayloadChannels = make(map[uuid.UUID]chan *pb.StreamPayload)
	}
	if s.Pending == nil {
		s.Pending = make(map[uuid.UUID]*types.PendingMsg)
	}
	if s.RemotePresence == nil {
		s.RemotePresence = make(map[uuid.UUID]string)
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

	payloadBytes, err := proto.Marshal(payload)
	if err != nil {
		return &pb.ServerResponse{Success: false, Message: "failed to marshal payload"}, fmt.Errorf("send payload: marshal: %v", err)
	}

	s.mu.Lock()
	s.mapInit()
	pmsg := &types.PendingMsg{
		MessageID:    messageID,
		From:         parsedSender,
		To:           parsedTarget,
		SenderDomain: payload.SenderDomain,
		TargetDomain: payload.TargetDomain,
		Created:      time.Now(),
		Payload:      payloadBytes,
		Attempts:     3,
	}

	s.Pending[messageID] = pmsg
	s.mu.Unlock()

	go s.attemptDelivery(context.TODO(), messageID)

	return &pb.ServerResponse{Success: true, Message: fmt.Sprintf("relay-OK: %s", messageID.String())}, nil

}

func (s *StrikeServer) attemptDelivery(
	ctx context.Context,
	msgID uuid.UUID,
) {

	s.mu.Lock()
	pmsg, ok := s.Pending[msgID]
	if !ok {
		s.mu.Unlock()
		return
	}

	// Route by domain: empty or matching our name means local
	isLocal := pmsg.TargetDomain == "" || pmsg.TargetDomain == s.Name

	if isLocal {
		ch, connected := s.PayloadChannels[pmsg.To]
		s.mu.Unlock()

		if connected {
			delivered, err := s.localDelivery(ctx, ch, pmsg, 5*time.Second)
			if err == nil && delivered {
				s.mu.Lock()
				delete(s.Pending, msgID)
				s.mu.Unlock()
				return
			}
		}
	} else {
		s.mu.Unlock()
	}

	// Fall back to federation (domain-based lookup)
	delivered, err := s.fedDelivery(ctx, pmsg)
	if err != nil || !delivered {
		s.mu.Lock()
		pmsg.Attempts--
		if pmsg.Attempts <= 0 {
			delete(s.Pending, msgID)
		}
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	delete(s.Pending, msgID)
	s.mu.Unlock()
}
func (s *StrikeServer) EnqueueFederated(ctx context.Context, rp *fedpb.RelayPayload) error {

	from, err := uuid.Parse(rp.Sender.UInfo.UserId)
	if err != nil {
		return fmt.Errorf("invalid sender id")
	}

	to, err := uuid.Parse(rp.Recipient.UInfo.UserId)
	if err != nil {
		return fmt.Errorf("invalid recipient id")
	}

	msgID := uuid.New()

	s.mu.Lock()
	s.mapInit()
	s.Pending[msgID] = &types.PendingMsg{
		MessageID:    msgID,
		From:         from,
		To:           to,
		SenderDomain: rp.Sender.Domain,
		TargetDomain: rp.Recipient.Domain,
		Payload:      rp.PayloadData,
		Created:      time.Now(),
		Attempts:     3,
	}
	s.mu.Unlock()

	go s.attemptDelivery(ctx, msgID)
	return nil
}

func (s *StrikeServer) lookupRemoteUser(user uuid.UUID) (string, bool) {
	// TODO: Replace with actual presence tracking / mapping.
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.RemotePresence == nil {
		s.RemotePresence = make(map[uuid.UUID]string)
	}

	peerID, ok := s.RemotePresence[user]
	return peerID, ok
}

func (s *StrikeServer) UpdateRemotePresence(user uuid.UUID, peerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.RemotePresence == nil {
		s.RemotePresence = make(map[uuid.UUID]string)
	}
	s.RemotePresence[user] = peerID
}

func (s *StrikeServer) fedDelivery(
	ctx context.Context,
	pmsg *types.PendingMsg,
) (bool, error) {

	relay := &fedpb.RelayPayload{
		EnvelopeId:   uuid.NewString(),
		OriginServer: s.ID.String(),
		Sender: &common_pb.UserAddress{
			Domain: pmsg.SenderDomain,
			UInfo:  &common_pb.UserInfo{UserId: pmsg.From.String()},
		},
		Recipient: &common_pb.UserAddress{
			Domain: pmsg.TargetDomain,
			UInfo:  &common_pb.UserInfo{UserId: pmsg.To.String()},
		},
		PayloadData: pmsg.Payload,
		SentAt:      timestamppb.Now(),
	}

	// Try domain-based routing first
	if pmsg.TargetDomain != "" {
		client, ok := s.PeerMgr.ClientByName(pmsg.TargetDomain)
		if ok {
			_, err := client.Relay(ctx, relay)
			if err != nil {
				return false, err
			}
			return true, nil
		}
	}

	// Fall back to RemotePresence lookup (legacy)
	peerID, ok := s.lookupRemoteUser(pmsg.To)
	if !ok {
		return false, nil
	}

	client, ok := s.PeerMgr.Client(peerID)
	if !ok {
		return false, fmt.Errorf("peer %s not connected", peerID)
	}

	_, err := client.Relay(ctx, relay)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (s *StrikeServer) localDelivery(ctx context.Context, ch chan<- *pb.StreamPayload, pmsg *types.PendingMsg, timeout time.Duration) (bool, error) {
	out := &pb.StreamPayload{}
	if err := proto.Unmarshal(pmsg.Payload, out); err != nil {
		return false, fmt.Errorf("unmarshal payload: %v", err)
	}

	select {
	case ch <- out:
		return true, nil
	case <-time.After(timeout):
		return false, fmt.Errorf("delivery timed out")
	case <-ctx.Done():
		return false, nil
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

	parsedId, err := uuid.Parse(req.UserId)
	if err != nil {
		return fmt.Errorf("failed to parse user ID: %v", err)
	}

	s.mu.Lock()
	s.mapInit()
	s.Connected[parsedId] = &common_pb.UserInfo{
		Username:            req.Username,
		UserId:              req.UserId,
		EncryptionPublicKey: req.EncryptionPublicKey,
		SigningPublicKey:    req.SigningPublicKey,
	}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.Connected, parsedId)
		s.mu.Unlock()
		log.Printf("%s is now offline.\n", req.Username)
	}()

	log.Printf("%s is online.\n", req.Username)

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-time.After(2 * time.Minute):
			err := stream.Send(&pb.StatusUpdate{
				Message:   "Still alive",
				UpdatedAt: timestamppb.Now(),
			})
			if err != nil {
				log.Printf("Failed to send status update: %v\n", err)
				return err
			}
		}
	}
}

func (s *StrikeServer) UserRequest(ctx context.Context, addr *common_pb.UserAddress) (*common_pb.UserInfo, error) {
	// If domain is set and doesn't match us, forward to the peer
	if addr.Domain != "" && addr.Domain != s.Name {
		return s.federatedUserLookup(ctx, addr.Username, addr.Domain)
	}

	return s.localUserLookup(ctx, addr.Username)
}

func (s *StrikeServer) localUserLookup(ctx context.Context, username string) (*common_pb.UserInfo, error) {
	var userid uuid.UUID
	var encryptionPubKey, signingPubKey []byte

	err := s.DBpool.QueryRow(ctx, s.PStatements.User.GetUser, username).Scan(&userid)
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

	return &common_pb.UserInfo{UserId: userid.String(), Username: username, EncryptionPublicKey: encryptionPubKey, SigningPublicKey: signingPubKey}, nil
}

func (s *StrikeServer) federatedUserLookup(ctx context.Context, username string, domain string) (*common_pb.UserInfo, error) {
	client, ok := s.PeerMgr.ClientByName(domain)
	if !ok {
		return nil, fmt.Errorf("unknown domain: %s", domain)
	}

	resp, err := client.UserLookup(ctx, &fedpb.UserLookupReq{Username: username})
	if err != nil {
		return nil, fmt.Errorf("federated lookup failed: %v", err)
	}

	if !resp.Found {
		return nil, nil
	}

	return resp.UserInfo, nil
}

func (s *StrikeServer) OnlineUsers(ctx context.Context, userInfo *common_pb.UserInfo) (*common_pb.Users, error) {
	log.Printf("%s (%s) requested active user list\n", userInfo.Username, userInfo.UserId)

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
	s.mu.Lock()
	defer s.mu.Unlock()

	users := make([]*common_pb.UserInfo, 0, len(s.Connected))
	for _, v := range s.Connected {
		users = append(users, &common_pb.UserInfo{
			UserId:              v.UserId,
			Username:            v.Username,
			EncryptionPublicKey: v.EncryptionPublicKey,
			SigningPublicKey:    v.SigningPublicKey,
		})
	}

	return &pb.ServerInfo{
		ServerId:   s.ID.String(),
		ServerName: s.Name,
		Users:      users,
		Domain:     s.Name,
	}, nil
}

func (s *StrikeServer) PayloadStream(user *common_pb.UserInfo, stream pb.Strike_PayloadStreamServer) error {
	log.Printf("Stream Established: %v online\n", user.Username)

	parsedId, err := uuid.Parse(user.UserId)
	if err != nil {
		return fmt.Errorf("failed to parse user id: %v", err)
	}

	s.mu.Lock()
	s.mapInit()
	s.PayloadStreams[parsedId] = stream
	s.mu.Unlock()

	payloadChannel := make(chan *pb.StreamPayload, 100)

	s.mu.Lock()
	s.PayloadChannels[parsedId] = payloadChannel
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.PayloadStreams, parsedId)
		delete(s.PayloadChannels, parsedId)
		close(payloadChannel)
		s.mu.Unlock()
		log.Printf("Client %s disconnected.\n", user.Username)
	}()

	go func() {
		for msg := range payloadChannel {
			if err := stream.Send(msg); err != nil {
				log.Printf("Failed to send message to %s: %v\n", user.Username, err)
				return
			}
		}
	}()

	for {
		select {
		case <-stream.Context().Done():
			log.Println("Client disconnected.")
			return nil
		case <-time.After(1 * time.Minute):
			// TODO: Heart Beat
		}
	}
}
