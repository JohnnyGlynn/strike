package client

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/JohnnyGlynn/strike/internal/auth"
	"github.com/JohnnyGlynn/strike/internal/config"
	"github.com/JohnnyGlynn/strike/internal/db"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
	"github.com/google/uuid"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type ClientInfo struct {
	Config      *config.ClientConfig
	Pbclient    pb.StrikeClient
	Keys        map[string][]byte
	Username    string
	Cache       ClientCache
	Pstatements *db.ClientDB
}

type ClientCache struct {
	Invites    map[string]*pb.BeginChatRequest
	Chats      map[string]*pb.Chat
	ActiveChat string
}

func ConnectPayloadStream(ctx context.Context, c ClientInfo) error {
	// Pass your own username to register your stream
	stream, err := c.Pbclient.PayloadStream(ctx, &pb.Username{Username: c.Username})
	if err != nil {
		log.Fatalf("MessageStream Failed: %v", err)
		return err
	}

	// Start our demultiplexer and baseline processor functions
	// TODO: Pass more in less
	demux := NewDemultiplexer(c)

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

func SendMessage(c ClientInfo, target string, message string) {
	envelope := pb.Envelope{
		SenderPublicKey: c.Keys["SigningPublicKey"],
		SentAt:          timestamppb.Now(),
		FromUser:        c.Username,
		ToUser:          target,
		Chat:            c.Cache.Chats[c.Cache.ActiveChat], // TODO: Ensure nothing can be set if ActiveChat == ""
		Message:         message,
	}

	payloadEnvelope := pb.StreamPayload{
		Target:  target,
		Payload: &pb.StreamPayload_Envelope{Envelope: &envelope},
	}

	_, err := c.Pbclient.SendPayload(context.Background(), &payloadEnvelope)
	if err != nil {
		log.Fatalf("Error sending message: %v", err)
	}
}

func ConfirmChat(ctx context.Context, c ClientInfo, chatRequest *pb.BeginChatRequest, inviteState bool) error {
	confirmation := pb.ConfirmChatRequest{
		InviteId:  chatRequest.InviteId,
		ChatName:  chatRequest.Chat.Name,
		Initiator: chatRequest.Initiator,
		Confirmer: chatRequest.Target,
		State:     inviteState,
		Chat:      chatRequest.Chat,
	}

	payload := pb.StreamPayload{
		Target:  chatRequest.Initiator,
		Payload: &pb.StreamPayload_ChatConfirm{ChatConfirm: &confirmation},
	}

	resp, err := c.Pbclient.SendPayload(ctx, &payload)
	if err != nil {
		return fmt.Errorf("failed to confirm chat: %v", err)
	}

	delete(c.Cache.Invites, chatRequest.InviteId)
	// Cache a chat when you acceot an invite
	if inviteState {
		c.Cache.Chats[chatRequest.Chat.Id] = chatRequest.Chat
	}
	fmt.Printf("Chat invite acknowledged: %+v\n", resp)

	return nil
}

func ClientSignup(c pb.StrikeClient, username string, password string, curve25519key []byte, ed25519key []byte) error {
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
		Username:            username,
		PasswordHash:        passwordHash,
		Salt:                salt,
		EncryptionPublicKey: curve25519key,
		SigningPublicKey:    ed25519key,
	}

	serverRes, err := c.Signup(ctx, &initUser)
	if err != nil {
		log.Fatalf("signup failed: %v", err)
		return err
	}

	fmt.Printf("Server Response: %+v\n", serverRes)
	return nil
}

func Login(c pb.StrikeClient, username string, password string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	saltMine := pb.Username{
		Username: username,
	}

	salt, err := c.SaltMine(ctx, &saltMine)
	if err != nil {
		log.Fatalf("Salt retrieval failed: %v", err)
		return err
	}

	passwordHash, err := auth.HashPassword(password, salt.Salt)
	if err != nil {
		return fmt.Errorf("password input error: %v", err)
	}

	loginUP := pb.LoginRequest{
		Username:     username,
		PasswordHash: passwordHash,
	}

	userStatus := pb.StatusRequest{
		Username: username,
	}

	loginReq, err := c.Login(ctx, &loginUP)
	if err != nil {
		log.Fatalf("login failed: %v", err)
		return err
	}

	fmt.Printf("%v:%s\n", loginReq.Success, loginReq.Message)

	// TODO: handle this elsewhere?
	stream, err := c.UserStatus(ctx, &userStatus)
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

		fmt.Printf("%s Status: %s\n", username, connectionStream.Message)
	}
}

func BeginChat(c pb.StrikeClient, username string, chatTarget string, chatName string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	newInvite := uuid.New().String()

	beginChat := &pb.BeginChatRequest{
		InviteId:  newInvite,
		Initiator: username,
		Target:    chatTarget,
		ChatName:  chatName,
		Chat: &pb.Chat{
			Id:    uuid.New().String(),
			Name:  chatName,
			State: pb.Chat_INIT,
		},
	}

	payloadChatRequest := pb.StreamPayload{
		Target:  chatTarget,
		Payload: &pb.StreamPayload_ChatRequest{ChatRequest: beginChat},
	}

	beginChatResponse, err := c.SendPayload(ctx, &payloadChatRequest)
	if err != nil {
		log.Fatalf("Begin Chat failed: %v", err)
		return err
	}

	fmt.Printf("Chat Request sent: %+v", beginChatResponse)

	return nil
}

// TODO: No longer fit for purpose - Terminal UI library time
func MessagingShell(c ClientInfo) {
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
	fmt.Println("---MsgShell---")
	fmt.Println("/help for available commands")
	fmt.Printf("Enter chatTarget:message to send a message (e.g., '%v:HelloWorld') - Chat selection required\n", c.Username)

	commands := map[string]func(){
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
		if c.Cache.ActiveChat == "" {
			fmt.Print("[NO-CHAT]msgshell> ")
		} else {
			fmt.Printf("[CHAT:%s]> ", c.Cache.ActiveChat)
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

func shellInvites(ctx context.Context, c ClientInfo) {
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

func shellChat(inputReader *bufio.Reader, c ClientInfo) {
	if len(c.Cache.ActiveChat) == 0 {
		fmt.Println("You haven't joined any Chats")
		return
	}

	fmt.Println("Available Chats:")

	chatList := make([]string, 0, len(c.Cache.Chats))
	index := 1

	for chatID, chat := range c.Cache.Chats {
		fmt.Printf("%d: %s [STATE: %v]\n", index, chat.Name, chat.State.String())
		chatList = append(chatList, chatID)
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

	selectedChatID := chatList[selectedIndex-1]

	if c.Cache.ActiveChat == selectedChatID {
		fmt.Printf("%s already active", selectedChatID)
		return
	}

	c.Cache.ActiveChat = selectedChatID
	fmt.Printf("Active chat: %s\n", c.Cache.Chats[selectedChatID].Name)
}

func shellBeginChat(c ClientInfo, inputReader *bufio.Reader) {
	fmt.Print("Invite User> ")
	inviteUser, err := inputReader.ReadString('\n')
	if err != nil {
		log.Printf("Error reading invite input user: %v\n", err)
		return
	}
	inviteUser = strings.TrimSpace(inviteUser)

	fmt.Print("Chat Name> ")
	chatName, err := inputReader.ReadString('\n')
	if err != nil {
		log.Printf("Error reading invite input chat name: %v\n", err)
		return
	}
	chatName = strings.TrimSpace(chatName)

	err = BeginChat(c.Pbclient, c.Username, inviteUser, chatName)
	if err != nil {
		log.Printf("error beginning chat: %v", err)
	}
}

func shellSendMessage(input string, c ClientInfo) error {
	if input == "" {
		return nil
	}

	userAndMessage := strings.SplitN(input, ":", 2) // Check for : then splint into target and message
	if len(userAndMessage) != 2 {
		return fmt.Errorf("Invalid format. Use recipient:message")
	}

	// TODO: Stopgap handle this elsewhere
	if c.Cache.ActiveChat == "" {
		return fmt.Errorf("No chat has been selected. Use /chats to enable a chat first")
	}

	target, message := userAndMessage[0], userAndMessage[1]

	SendMessage(c, target, message)
	fmt.Printf("[YOU]: %s\n", message)

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
