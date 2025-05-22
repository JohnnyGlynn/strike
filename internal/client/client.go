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
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/JohnnyGlynn/strike/internal/auth"
	"github.com/JohnnyGlynn/strike/internal/network"
	"github.com/JohnnyGlynn/strike/internal/types"
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
	sealedMessage, err := network.Encrypt(c, []byte(message))
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

func MessagingShell(c *types.ClientInfo) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get messages
	// TODO: Pass a single client with everything we need
	go func() {
		err := ConnectPayloadStream(ctx, c)
		if err != nil {
			log.Fatalf("failed to connect message stream: %v\n", err)
		}
	}()

	inputReader := bufio.NewReader(os.Stdin)
	fmt.Println("/help for available commands")
	fmt.Printf("Enter chatTarget:message to send a message (e.g., '%v:HelloWorld') - Chat selection required\n", c.Username)

	commands := map[string]func(){
		"/addfriend": func() { shellAddFriend(inputReader, c) },
		"/beginchat": func() { shellBeginChat(c, inputReader) },
		"/chats":     func() { shellChat(inputReader, c) },
		"/invites":   func() { shellInvites(ctx, c) },
		"/help":      printHelp,
		"/exit": func() {
			cancel()
			fmt.Println("Exiting msgshell...")
			os.Exit(0) // Ensures we exit cleanly
		},
	}

	for {
		// Prompt for input
		if c.Cache.ActiveChat.Chat == nil {
			fmt.Print("[NO-CHAT]msgshell> ")
		} else {
			fmt.Printf("[CHAT:%s]\n[%s]>", c.Cache.ActiveChat.Chat.Name[:20], c.Username)
		}

		input, err := inputReader.ReadString('\n')
		if err != nil {
			log.Printf("Error reading input: %v\n", err)
			continue
		}

		input = strings.TrimSpace(input)

		if strings.HasPrefix(input, "/") {
			if cmd, ok := commands[input]; ok {
				cmd()
			} else {
				fmt.Printf("Unknown command: %s\n", input)
			}
			continue
		}

		if err := shellSendMessage(input, c); err != nil {
			fmt.Println(err)
			continue
		}
	}
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

func shellInvites(ctx context.Context, c *types.ClientInfo) {
	if len(c.Cache.Invites) == 0 {
		fmt.Println("No pending invites :^[")
		return
	}

	fmt.Println("Pending Invites")
	inputReader := bufio.NewReader(os.Stdin)

	for inviteID, chatRequest := range c.Cache.Invites {
		fmt.Printf("%v: %s [FROM: %s]\n", inviteID, chatRequest.Chat.Name, chatRequest.Initiator)
		fmt.Printf("y[Accept]/n[Decline]")

		input, err := inputReader.ReadString('\n')
		if err != nil {
			log.Printf("Error reading input: %v\n", err)
			continue
		}

		input = strings.TrimSpace(strings.ToLower(input))
		accepted := input == "y"

		if err := ConfirmChat(ctx, c, chatRequest, accepted); err != nil {
			log.Printf("Failed to decline invite: %v", err)
		}
	}
}

func GetActiveUsers(c *types.ClientInfo, uinfo *pb.UserInfo) *pb.UsersInfo {
	activeUsers, err := c.Pbclient.OnlineUsers(context.TODO(), uinfo)
	if err != nil {
		log.Printf("error beginning chat: %v", err)
	}

	return activeUsers
}

func shellChat(inputReader *bufio.Reader, c *types.ClientInfo) {
	if len(c.Cache.Chats) == 0 {
		if err := loadChats(c); err != nil {
			log.Printf("Error loading chats: %v", err)
			return
		}
		if len(c.Cache.Chats) == 0 {
			fmt.Println("No chats available")
			return
		}
	}

	fmt.Println("Available Chats:")

	chatList := make([]*pb.Chat, 0, len(c.Cache.Chats))
	index := 1

	for _, chat := range c.Cache.Chats {
		fmt.Printf("%d: %s [STATE: %v]\n", index, chat.Name, chat.State.String())
		chatList = append(chatList, chat)
		index++
	}

	fmt.Print("Enter the chat number to set active (Enter to cancel): ")
	selectedIndexString, err := inputReader.ReadString('\n')
	if err != nil {
		log.Printf("Error reading input: %v\n", err)
		return
	}

	selectedIndexString = strings.TrimSpace(selectedIndexString)
	if selectedIndexString == "" {
		fmt.Println("No chat selected.")
		return
	}

	selectedIndex, err := strconv.Atoi(selectedIndexString)
	if err != nil || selectedIndex < 1 || selectedIndex > len(chatList) {
		fmt.Println("Invalid selection. Please enter a valid chat number.")
		return
	}

	selectedChat := chatList[selectedIndex-1]

	if c.Cache.ActiveChat.Chat == selectedChat {
		fmt.Printf("%s already active", selectedChat.Name)
		return
	}

	c.Cache.ActiveChat.Chat = selectedChat
	fmt.Printf("Active chat: %s\n", c.Cache.ActiveChat.Chat.Name)

	participants := c.Cache.ActiveChat.Chat.Participants

	for k, v := range participants {
		if v == c.UserID.String() {
			participants = slices.Delete(participants, k, k+1)
			break
		}
	}

	if len(participants) == 0 {
		log.Print("No other participants in the chat")
		return
	}

	uinfo := &pb.UserInfo{
		UserId: participants[0],
	}

	target, err := c.Pbclient.UserRequest(context.TODO(), uinfo)
	if err != nil {
		log.Printf("error beginning chat: %v", err)
	}

	sharedSecret, err := network.ComputeSharedSecret(c.Keys["EncryptionPrivateKey"], target.EncryptionPublicKey)
	if err != nil {
		// TODO: Error return
		log.Print("failed to compute shared secret")
		return
	}

	c.Cache.ActiveChat.SharedSecret = sharedSecret

	err = network.DeriveKeys(c, sharedSecret)
	if err != nil {
		log.Fatalf("Failed to derive keys")
	}

}

func shellAddFriend(inputReader *bufio.Reader, c *types.ClientInfo) {
	fmt.Println("Online Users:")

	au := GetActiveUsers(c, &pb.UserInfo{
		Username:            c.Username,
		UserId:              c.UserID.String(),
		EncryptionPublicKey: c.Keys["EncryptionPublicKey"],
		SigningPublicKey:    c.Keys["SigningPublicKey"],
	})

	userList := make([]*pb.UserInfo, 0, len(au.Users))
	index := 1

	for _, user := range au.Users {
		fmt.Printf("%d: %s\n", index, user.Username)
		userList = append(userList, user)
		index++
	}

	fmt.Print("Enter the number of the user you want to invite (Enter to cancel): ")
	selectedIndexString, err := inputReader.ReadString('\n')
	if err != nil {
		log.Printf("Error reading input: %v\n", err)
		return
	}

	selectedIndexString = strings.TrimSpace(selectedIndexString)
	if selectedIndexString == "" {
		fmt.Println("No user selected.")
		return
	}

	selectedIndex, err := strconv.Atoi(selectedIndexString)
	if err != nil || selectedIndex < 1 || selectedIndex > len(userList) {
		fmt.Println("Invalid selection. Please enter a valid user number.")
		return
	}

	selectedUser := userList[selectedIndex-1]

	_, err = c.Pstatements.SaveUserDetails.ExecContext(context.TODO(), selectedUser.UserId, selectedUser.Username, selectedUser.EncryptionPublicKey, selectedUser.SigningPublicKey)
	if err != nil {
		fmt.Printf("User to be saved: %v\n", selectedUser)
		log.Fatalf("failed adding to address book: %v", err)
	}

}

func shellBeginChat(c *types.ClientInfo, inputReader *bufio.Reader) {
	fmt.Print("Invite User> ")
	inviteUser, err := inputReader.ReadString('\n')
	if err != nil {
		log.Printf("Error reading invite input user: %v\n", err)
		return
	}
	inviteUser = strings.TrimSpace(inviteUser)

	var targetUser *pb.UserInfo

	au := GetActiveUsers(c, &pb.UserInfo{
		Username:            c.Username,
		UserId:              c.UserID.String(),
		EncryptionPublicKey: c.Keys["EncryptionPublicKey"],
		SigningPublicKey:    c.Keys["SigningPublicKey"],
	})

	//TODO Add user function directly
	for _, value := range au.Users {
		if value.Username == inviteUser {
			targetUser = value
		}

	}

	fmt.Print("Chat Name> ")
	chatName, err := inputReader.ReadString('\n')
	if err != nil {
		log.Printf("Error reading invite input chat name: %v\n", err)
		return
	}
	chatName = strings.TrimSpace(chatName)

	err = BeginChat(c, uuid.MustParse(targetUser.UserId), chatName)
	if err != nil {
		log.Printf("error beginning chat: %v", err)
	}
}

func shellSendMessage(input string, c *types.ClientInfo) error {
	if input == "" {
		return nil
	}

	userAndMessage := strings.SplitN(input, ":", 2) // Check for : then splint into target and message
	if len(userAndMessage) != 2 {
		return fmt.Errorf("invalid format. Use recipient:message")
	}

	// TODO: Stopgap handle this elsewhere
	if c.Cache.ActiveChat.Chat == nil {
		return fmt.Errorf("no chat has been selected. use /chats to enable a chat first")
	}

	target, message := userAndMessage[0], userAndMessage[1]

	// TODO: Migrate messaging shell to active chat only, stop having to query uuid on every message
	var targetID uuid.UUID
	row := c.Pstatements.GetUserId.QueryRowContext(context.TODO(), target)
	err := row.Scan(&targetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Fatalf("DB error: %v", err)
		}
		log.Fatalf("an error occured: %v", err)
	}

	SendMessage(c, targetID, message)

	return nil
}

func printHelp() {
	// Is multiple println better?
	fmt.Print("---Available Commands---\n,",
		"/beginchat: Invite a User to a Chat\n",
		"/chats: List joined chats and set one to active\n",
		"/invites: See and respond to Chat Invites\n",
		"/exit: ...\n",
	)
}

func loadChats(c *types.ClientInfo) error {
	rows, err := c.Pstatements.GetChats.QueryContext(context.TODO())
	if err != nil {
		return fmt.Errorf("error querying chats: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			chat_id      uuid.UUID
			chat_name    string
			initiator    uuid.UUID
			participants []uuid.UUID
			stateStr     string
		)

		if err := rows.Scan(&chat_id, &chat_name, &initiator, &participants, &stateStr); err != nil {
			log.Printf("error scanning row: %v", err)
			return err
		}

		stateEnum, ok := pb.Chat_State_value[stateStr]
		if !ok {
			return fmt.Errorf("invalid chat state: %s", stateStr)
		}

		var participantsStrung []string
		for _, uID := range participants {
			participantsStrung = append(participantsStrung, uID.String())
		}

		chat := &pb.Chat{
			Id:           chat_id.String(),
			Name:         chat_name,
			State:        pb.Chat_State(stateEnum),
			Participants: participantsStrung,
		}

		c.Cache.Chats[chat_id] = chat
	}

	return nil
}

func loadMessages(c *types.ClientInfo) ([]types.MessageStruct, error) {
	rows, err := c.Pstatements.GetMessages.QueryContext(context.TODO(), c.Cache.ActiveChat)
	if err != nil {
		return nil, fmt.Errorf("error querying messages: %v", err)
	}
	defer rows.Close()

	var messages []types.MessageStruct

	for rows.Next() {
		var msg types.MessageStruct
		if err := rows.Scan(&msg.Id, &msg.ChatId, &msg.Sender, &msg.Content); err != nil {
			log.Printf("error scanning row: %v", err)
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, nil
}
