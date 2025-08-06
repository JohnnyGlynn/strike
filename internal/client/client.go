package client

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/JohnnyGlynn/strike/internal/client/crypto"
	"github.com/JohnnyGlynn/strike/internal/client/network"
	"github.com/JohnnyGlynn/strike/internal/client/types"
	"github.com/JohnnyGlynn/strike/internal/shared"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
	"github.com/google/uuid"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func ConnectPayloadStream(ctx context.Context, c *types.Client) error {
	// Pass your own username to register your stream
	stream, err := c.PBC.PayloadStream(ctx, &pb.UserInfo{
		Username:            c.Identity.Username,
		UserId:              c.Identity.ID.String(),
		EncryptionPublicKey: c.Identity.Keys["EncryptionPublicKey"],
		SigningPublicKey:    c.Identity.Keys["SigningPublicKey"],
	})
	if err != nil {
		log.Printf("MessageStream Failed: %v", err)
		return err
	}

	// Start our demultiplexer and baseline processor functions
	demux := network.NewDemultiplexer(c)
	defer demux.Shutdown()

	// Start Monitoring

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

func Signup(c *types.Client, password string, curve25519key []byte, ed25519key []byte) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	salt, err := shared.GenerateSalt(16)
	if err != nil {
		return fmt.Errorf("salt generator: %v", err)
	}

	passwordHash, err := shared.HashPassword(password, salt)
	if err != nil {
		return fmt.Errorf("password input error: %v", err)
	}

	initUser := pb.InitUser{
		Username:            c.Identity.Username,
		UserId:              c.Identity.ID.String(),
		PasswordHash:        passwordHash,
		Salt:                &pb.Salt{Salt: salt},
		EncryptionPublicKey: curve25519key,
		SigningPublicKey:    ed25519key,
	}

	serverRes, err := c.PBC.Signup(ctx, &initUser)
	if err != nil {
		log.Printf("signup failed: %v\n", err)
		return err
	}

	// Save users own details to local client db
	_, err = c.DB.SaveID.ExecContext(ctx, c.Identity.ID.String(), c.Identity.Username, curve25519key, ed25519key)
	if err != nil {
		return fmt.Errorf("failed adding to address book: %v", err)
	}

	fmt.Printf("Server Response: %v\n", serverRes.Success)
	return nil
}

func Login(c *types.Client, password string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	localIdentity := false

	salt, err := c.PBC.SaltMine(ctx, &pb.UserInfo{Username: c.Identity.Username})
	if err != nil {
		log.Printf("Salt retrieval failed: %v\n", err)
		return err
	}

	passwordHash, err := shared.HashPassword(password, salt.Salt)
	if err != nil {
		return fmt.Errorf("password input error: %v", err)
	}

	loginResp, err := c.PBC.Login(ctx, &pb.LoginVerify{
		Username:     c.Identity.Username,
		PasswordHash: passwordHash,
	})
	if err != nil {
		log.Printf("login error: %v\n", err)
		return err
	}
	if !loginResp.Success {
		return fmt.Errorf("login failed: %v", loginResp.Message)
	}

	var userID uuid.UUID

	row := c.DB.GetUID.QueryRowContext(context.TODO(), c.Identity.Username)
	err = row.Scan(&userID)
	if err == nil {
		c.Identity.ID = userID
		localIdentity = true
	} else if err != sql.ErrNoRows {
		return err
	}

	if !localIdentity {

		dbsync, err := c.PBC.UserRequest(context.TODO(), &pb.UserInfo{Username: c.Identity.Username})
		if err != nil {
			log.Printf("error syncing: %v\n", err)
			return err
		}

		c.Identity.ID = uuid.MustParse(dbsync.UserId)

		_, err = c.DB.SaveID.ExecContext(ctx, c.Identity.ID.String(), c.Identity.Username, c.Identity.Keys["EncryptionPublicKey"], c.Identity.Keys["SigningPublicKey"])
		if err != nil {
			return fmt.Errorf("failed to rebuild identity: %v", err)
		}
	}

	fmt.Printf("%v:%s\n", loginResp.Success, loginResp.Message)
	return nil

}

func SendMessage(c *types.Client, message string) error {
	sealedMessage, err := crypto.Encrypt(c, []byte(message))
	if err != nil {
		log.Println("Couldnt encrypt message")
		return err
	}

	encenv := pb.EncryptedEnvelope{
		SenderPublicKey:  c.Identity.Keys["SigningPublicKey"],
		SentAt:           timestamppb.Now(),
		FromUser:         c.Identity.ID.String(),
		ToUser:           c.State.Cache.CurrentChat.User.Id.String(),
		EncryptedMessage: sealedMessage,
	}

	payloadEnvelope := pb.StreamPayload{
		Target:  c.State.Cache.CurrentChat.User.Id.String(),
		Sender:  c.Identity.ID.String(),
		Payload: &pb.StreamPayload_Encenv{Encenv: &encenv},
		Info:    "Encrypted Payload",
	}

	_, err = c.PBC.SendPayload(context.Background(), &payloadEnvelope)
	if err != nil {
		log.Println("Error sending payload")
		return err
	}

	_, err = c.DB.SaveMessage.ExecContext(context.TODO(), uuid.New().String(), c.State.Cache.CurrentChat.User.Id.String(), "outbound", sealedMessage, time.Now().UnixMilli())
	if err != nil {
		log.Println("Error saving message")
		return err
	}

	return nil
}

func FriendRequest(ctx context.Context, c *types.Client, target *pb.UserInfo) error {

	req := pb.FriendRequest{
		Target: target.UserId,
		UserInfo: &pb.UserInfo{
			Username:            c.Identity.Username,
			UserId:              c.Identity.ID.String(),
			EncryptionPublicKey: c.Identity.Keys["EncryptionPublicKey"],
			SigningPublicKey:    c.Identity.Keys["SigningPublicKey"],
		},
	}

	payload := pb.StreamPayload{
		Target:  target.UserId,
		Sender:  c.Identity.ID.String(),
		Payload: &pb.StreamPayload_FriendRequest{FriendRequest: &req},
		Info:    "Friend Request payload",
	}

	resp, err := c.PBC.SendPayload(ctx, &payload)
	if err != nil {
		return fmt.Errorf("failed to confirm chat: %v", err)
	}

	_, err = c.DB.SaveFriendRequest.ExecContext(context.TODO(), target.UserId, target.Username, nil, nil, "outbound")
	if err != nil {
		fmt.Printf("failed to save Friend Request")
		return err
	}

	fmt.Printf("Friend request sent: %+v\n", resp)

	return nil
}

func FriendResponse(ctx context.Context, c *types.Client, friendReq *pb.FriendRequest, state bool) error {

	res := pb.FriendResponse{
		Target: friendReq.UserInfo.UserId,
		UserInfo: &pb.UserInfo{
			Username:            c.Identity.Username,
			UserId:              c.Identity.ID.String(),
			EncryptionPublicKey: c.Identity.Keys["EncryptionPublicKey"],
			SigningPublicKey:    c.Identity.Keys["SigningPublicKey"],
		},
		State: state,
	}

	payload := pb.StreamPayload{
		Target:  friendReq.UserInfo.UserId,
		Sender:  c.Identity.ID.String(),
		Payload: &pb.StreamPayload_FriendResponse{FriendResponse: &res},
		Info:    "Friend Response payload",
	}

	resp, err := c.PBC.SendPayload(ctx, &payload)
	if err != nil {
		return fmt.Errorf("failed to confirm chat: %v", err)
	}

	if state {
		_, err = c.DB.SaveUserDetails.ExecContext(ctx, friendReq.UserInfo.UserId, friendReq.UserInfo.Username, friendReq.UserInfo.EncryptionPublicKey, friendReq.UserInfo.SigningPublicKey)
		if err != nil {
			return fmt.Errorf("failed adding to address book: %v", err)
		}
	}

	_, err = c.DB.DeleteFriendRequest.ExecContext(context.TODO(), friendReq.UserInfo.UserId)
	if err != nil {
		fmt.Printf("failed deleting friend request: %v", err)
		return err
	}

	fmt.Printf("Friend request acknowledged: %+v\n", resp)

	return nil
}

func RegisterStatus(c *types.Client) error {

	//TODO:Messy
	userInfo := pb.UserInfo{
		Username:            c.Identity.Username,
		UserId:              c.Identity.ID.String(),
		EncryptionPublicKey: c.Identity.Keys["EncryptionPublicKey"],
		SigningPublicKey:    c.Identity.Keys["SigningPublicKey"],
	}

	stream, err := c.PBC.StatusStream(context.TODO(), &userInfo)
	if err != nil {
		log.Printf("status failure: %v\n", err)
		return err
	}

	for {
		connectionStream, err := stream.Recv()
		if err != nil {
			log.Printf("Failed to connect to Status stream: %v\n", err)
			return err
		}

		fmt.Printf("%s Status: %s\n", c.Identity.Username, connectionStream.Message)
	}

}

func GetActiveUsers(c *types.Client, uinfo *pb.UserInfo) (*pb.Users, error) {
	activeUsers, err := c.PBC.OnlineUsers(context.TODO(), uinfo)
	if err != nil {
		log.Printf("error getting active users: %v\n", err)
		return nil, err
	}

	return activeUsers, nil
}

func PollServer(c *types.Client) (*pb.ServerInfo, error) {
	sInfo, err := c.PBC.PollServer(context.TODO(), &pb.UserInfo{
		Username:            c.Identity.Username,
		UserId:              c.Identity.ID.String(),
		EncryptionPublicKey: c.Identity.Keys["EncryptionPublicKey"],
		SigningPublicKey:    c.Identity.Keys["SigningPublicKeyu"],
	})
	if err != nil {
		log.Printf("error polling server: %v\n", err)
		return nil, err
	}

	return sInfo, nil
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
