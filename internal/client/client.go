package client

import (
	"bufio"
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/JohnnyGlynn/strike/internal/client/auth"
	"github.com/JohnnyGlynn/strike/internal/client/crypto"
	"github.com/JohnnyGlynn/strike/internal/client/network"
	"github.com/JohnnyGlynn/strike/internal/client/types"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
	"github.com/google/uuid"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func ConnectPayloadStream(ctx context.Context, c *types.ClientInfo) error {
	// Pass your own username to register your stream
	stream, err := c.Pbclient.PayloadStream(ctx, &pb.UserInfo{
		Username:            c.Username,
		UserId:              c.UserID.String(),
		EncryptionPublicKey: c.Keys["EncryptionPublicKey"],
		SigningPublicKey:    c.Keys["SigningPublicKey"],
	})
	if err != nil {
		log.Printf("MessageStream Failed: %v", err)
		return err
	}

	// Start our demultiplexer and baseline processor functions
	demux := network.NewDemultiplexer(c)

	// Start Monitoring
	demux.StartMonitoring(c)

	for {
		select {
		case <-ctx.Done():
			// Graceful exit
			log.Println("Message stream context canceled. Exiting...")
			return nil
		default:
			msg, err := stream.Recv()
			if err == io.EOF {
				log.Println("Stream closed by server.")
				return nil
			} else if err != nil {
				log.Printf("Error receiving message: %v", err)
				return err
			}

			demux.Dispatcher(msg)
		}
	}
}

func SendMessage(c *types.ClientInfo, target uuid.UUID, message string) {
	sealedMessage, err := crypto.Encrypt(c, []byte(message))
	if err != nil {
		log.Fatal("Couldnt encrypt message")
	}

	encenv := pb.EncryptedEnvelope{
		SenderPublicKey:  c.Keys["SigningPublicKey"],
		SentAt:           timestamppb.Now(),
		FromUser:         c.UserID.String(),
		ToUser:           target.String(),
		Chat:             c.Cache.Chats[uuid.MustParse(c.Cache.ActiveChat.Chat.Id)],
		EncryptedMessage: sealedMessage,
	}

	payloadEnvelope := pb.StreamPayload{
		Target:  target.String(),
		Sender:  c.UserID.String(),
		Payload: &pb.StreamPayload_Encenv{Encenv: &encenv},
		Info:    "Encrypted Payload",
	}

	_, err = c.Pbclient.SendPayload(context.Background(), &payloadEnvelope)
	if err != nil {
		log.Fatalf("Error sending message: %v", err)
	}

	_, err = c.Pstatements.SaveMessage.ExecContext(context.TODO(), uuid.New().String(), c.Cache.Chats[uuid.MustParse(c.Cache.ActiveChat.Chat.Id)], c.UserID.String(), target.String(), "outbound", sealedMessage, time.Now().UnixMilli())
	if err != nil {
		log.Fatalf("Failed to save message")
	}

}

func ConfirmChat(ctx context.Context, c *types.ClientInfo, chatRequest *pb.BeginChatRequest, inviteState bool) error {
	confirmation := pb.ConfirmChatRequest{
		InviteId:  chatRequest.InviteId,
		Initiator: chatRequest.Initiator,
		Confirmer: c.UserID.String(),
		State:     inviteState,
		Chat:      chatRequest.Chat,
	}

	payload := pb.StreamPayload{
		Target:  chatRequest.Initiator,
		Sender:  c.UserID.String(),
		Payload: &pb.StreamPayload_ChatConfirm{ChatConfirm: &confirmation},
		Info:    "Chat Confirmation payload",
	}

	resp, err := c.Pbclient.SendPayload(ctx, &payload)
	if err != nil {
		return fmt.Errorf("failed to confirm chat: %v", err)
	}

	delete(c.Cache.Invites, uuid.MustParse(chatRequest.InviteId))
	// Cache a chat when you acceot an invite

	if inviteState {
		c.Cache.Chats[uuid.MustParse(chatRequest.Chat.Id)] = chatRequest.Chat
	}

	fmt.Printf("Chat invite acknowledged: %+v\n", resp)

	return nil
}

func FriendRequest(ctx context.Context, c *types.ClientInfo, target string) error {

	req := pb.FriendRequest{
		InviteId: uuid.New().String(),
		Target:   target,
		UserInfo: &pb.UserInfo{
			Username:            c.Username,
			UserId:              c.UserID.String(),
			EncryptionPublicKey: c.Keys["EncryptionPublicKey"],
			SigningPublicKey:    c.Keys["SigningPublicKey"],
		},
	}

	payload := pb.StreamPayload{
		Target:  target,
		Sender:  c.UserID.String(),
		Payload: &pb.StreamPayload_FriendRequest{FriendRequest: &req},
		Info:    "Friend Request payload",
	}

	resp, err := c.Pbclient.SendPayload(ctx, &payload)
	if err != nil {
		return fmt.Errorf("failed to confirm chat: %v", err)
	}

	//Add to cache on send for placeholder?
	c.Cache.FriendRequests[uuid.MustParse(req.InviteId)] = &pb.FriendRequest{Target: target}

	fmt.Printf("Friend request sent: %+v\n", resp)

	return nil
}

func FriendResponse(ctx context.Context, c *types.ClientInfo, friendReq *pb.FriendRequest, state bool) error {

	res := pb.FriendResponse{
		InviteId: friendReq.InviteId,
		Target:   friendReq.UserInfo.UserId,
		UserInfo: &pb.UserInfo{
			Username:            c.Username,
			UserId:              c.UserID.String(),
			EncryptionPublicKey: c.Keys["EncryptionPublicKey"],
			SigningPublicKey:    c.Keys["SigningPublicKey"],
		},
		State: state,
	}

	payload := pb.StreamPayload{
		Target:  friendReq.UserInfo.UserId,
		Sender:  c.UserID.String(),
		Payload: &pb.StreamPayload_FriendResponse{FriendResponse: &res},
		Info:    "Friend Response payload",
	}

	resp, err := c.Pbclient.SendPayload(ctx, &payload)
	if err != nil {
		return fmt.Errorf("failed to confirm chat: %v", err)
	}

	delete(c.Cache.FriendRequests, uuid.MustParse(friendReq.InviteId))

	if state {
		_, err = c.Pstatements.SaveUserDetails.ExecContext(ctx, friendReq.UserInfo.UserId, friendReq.UserInfo.Username, friendReq.UserInfo.EncryptionPublicKey, friendReq.UserInfo.SigningPublicKey)
		if err != nil {
			return fmt.Errorf("failed adding to address book: %v", err)
		}
	}

	fmt.Printf("Friend request acknowledged: %+v\n", resp)

	return nil
}

func ClientSignup(c *types.ClientInfo, password string, curve25519key []byte, ed25519key []byte) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	salt := make([]byte, 16)
	// add salt
	_, err := rand.Read(salt)
	if err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	passwordHash, err := auth.HashPassword(password, salt)
	if err != nil {
		return fmt.Errorf("password input error: %v", err)
	}

	initUser := pb.InitUser{
		Username:            c.Username,
		UserId:              c.UserID.String(),
		PasswordHash:        passwordHash,
		Salt:                &pb.Salt{Salt: salt},
		EncryptionPublicKey: curve25519key,
		SigningPublicKey:    ed25519key,
	}

	serverRes, err := c.Pbclient.Signup(ctx, &initUser)
	if err != nil {
		log.Fatalf("signup failed: %v", err)
		return err
	}

	// Save users own details to local client db
	_, err = c.Pstatements.SaveUserDetails.ExecContext(ctx, c.UserID.String(), c.Username, curve25519key, ed25519key)
	if err != nil {
		return fmt.Errorf("failed adding to address book: %v", err)
	}

	fmt.Printf("Server Response: %v\n", serverRes.Success)
	return nil
}

func Login(c *types.ClientInfo, password string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Retrieve UUID
	var userID uuid.UUID
	row := c.Pstatements.GetUserId.QueryRowContext(context.TODO(), c.Username)
	err := row.Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			dbsync, err := c.Pbclient.UserRequest(context.TODO(), &pb.UserInfo{Username: c.Username})
			if err != nil {
				log.Printf("error syncing: %v", err)
			}

			_, err = c.Pstatements.SaveUserDetails.ExecContext(ctx, c.UserID.String(), c.Username, c.Keys["EncryptionPublicKey"], c.Keys["SigningPublicKey"])
			if err != nil {
				return fmt.Errorf("failed adding to address book: %v", err)
			}

			userID = uuid.MustParse(dbsync.UserId)
		} else {
			log.Fatalf("an error occured while logging in: %v", err)
		}
	}

	c.UserID = userID

	userInfo := pb.UserInfo{
		Username:            c.Username,
		UserId:              c.UserID.String(),
		EncryptionPublicKey: c.Keys["EncryptionPublicKey"],
		SigningPublicKey:    c.Keys["SigningPublicKey"],
	}

	salt, err := c.Pbclient.SaltMine(ctx, &userInfo)
	if err != nil {
		log.Fatalf("Salt retrieval failed: %v", err)
		return err
	}

	passwordHash, err := auth.HashPassword(password, salt.Salt)
	if err != nil {
		return fmt.Errorf("password input error: %v", err)
	}

	loginUP := pb.LoginVerify{
		Username:     c.Username,
		PasswordHash: passwordHash,
	}

	loginReq, err := c.Pbclient.Login(ctx, &loginUP)
	if err != nil {
		log.Fatalf("login failed: %v", err)
		return err
	}

	fmt.Printf("%v:%s\n", loginReq.Success, loginReq.Message)
	return nil

}

func RegisterStatus(c *types.ClientInfo) error {

	//TODO:Messy
	userInfo := pb.UserInfo{
		Username:            c.Username,
		UserId:              c.UserID.String(),
		EncryptionPublicKey: c.Keys["EncryptionPublicKey"],
		SigningPublicKey:    c.Keys["SigningPublicKey"],
	}

	stream, err := c.Pbclient.StatusStream(context.TODO(), &userInfo)
	if err != nil {
		log.Fatalf("status failure: %v", err)
		return err
	}

	for {
		connectionStream, err := stream.Recv()
		if err != nil {
			log.Fatalf("Failed to connect to Status stream: %v", err)
			return err
		}

		fmt.Printf("%s Status: %s\n", c.Username, connectionStream.Message)
	}

}

func BeginChat(c *types.ClientInfo, target uuid.UUID, chatName string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	newInvite := uuid.New().String()

	participants := []string{c.UserID.String(), target.String()}

	beginChat := &pb.BeginChatRequest{
		InviteId:  newInvite,
		Initiator: c.UserID.String(),
		Target:    target.String(),
		Chat: &pb.Chat{
			Id:           uuid.New().String(),
			Name:         chatName,
			State:        pb.Chat_INIT,
			Participants: participants,
		},
	}

	payloadChatRequest := pb.StreamPayload{
		Target:  target.String(),
		Sender:  c.UserID.String(),
		Payload: &pb.StreamPayload_ChatRequest{ChatRequest: beginChat},
		Info:    "Begin Chat payload",
	}

	beginChatResponse, err := c.Pbclient.SendPayload(ctx, &payloadChatRequest)
	if err != nil {
		log.Fatalf("Begin Chat failed: %v", err)
		return err
	}

	fmt.Printf("Chat Request sent: %v", beginChatResponse)

	return nil
}

func GetActiveUsers(c *types.ClientInfo, uinfo *pb.UserInfo) *pb.Users {
	activeUsers, err := c.Pbclient.OnlineUsers(context.TODO(), uinfo)
	if err != nil {
		log.Printf("error beginning chat: %v", err)
	}

	return activeUsers
}

// TODO: Handle all input like this?
func LoginInput(prompt string, reader *bufio.Reader) (string, error) {
	fmt.Print(prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("error reading input: %w", err)
	}
	return strings.TrimSpace(input), nil
}
