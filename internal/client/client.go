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
	"sync"

	"github.com/JohnnyGlynn/strike/internal/auth"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
	"github.com/google/uuid"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type ClientCache struct {
	Invites    map[string]*pb.BeginChatRequest
	Chats      map[string]*pb.Chat
	ActiveChat string
}

// Client Reciever
var newCache ClientCache

func init() {
	if newCache.Invites == nil {
		newCache.Invites = make(map[string]*pb.BeginChatRequest)
	}

	if newCache.Chats == nil {
		newCache.Chats = make(map[string]*pb.Chat)
	}
}

func ConnectMessageStream(ctx context.Context, c pb.StrikeClient, username string, inputLock *sync.Mutex) error {
	// Pass your own username to register your stream
	stream, err := c.MessageStream(ctx, &pb.Username{Username: username})
	if err != nil {
		log.Fatalf("MessageStream Failed: %v", err)
		return err
	}

	// Start our demultiplexer and baseline processor functions
	demux := NewDemultiplexer(newCache.Chats, newCache.Invites)

	// Start Monitoring
	demux.StartMonitoring(newCache.Chats, newCache.Invites)

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

func SendMessage(c pb.StrikeClient, username string, publicKey []byte, target string, message string) {
	envelope := pb.Envelope{
		SenderPublicKey: publicKey,
		SentAt:          timestamppb.Now(),
		FromUser:        username,
		ToUser:          target,
		Chat:            newCache.Chats[newCache.ActiveChat], // TODO: Ensure nothing can be set if ActiveChat == ""
		Message:         message,
	}

	payloadEnvelope := pb.MessageStreamPayload{
		Target:  target,
		Payload: &pb.MessageStreamPayload_Envelope{Envelope: &envelope},
	}

	_, err := c.SendMessages(context.Background(), &payloadEnvelope)
	if err != nil {
		log.Fatalf("Error sending message: %v", err)
	}
}

func ConfirmChat(ctx context.Context, c pb.StrikeClient, chatRequest *pb.BeginChatRequest, inviteState bool) error {
	confirmation := pb.ConfirmChatRequest{
		InviteId:  chatRequest.InviteId,
		ChatName:  chatRequest.Chat.Name,
		Confirmer: chatRequest.Target,
		State:     inviteState,
		Chat:      chatRequest.Chat,
	}

	payload := pb.MessageStreamPayload{
		Target:  chatRequest.Initiator,
		Payload: &pb.MessageStreamPayload_ChatConfirm{ChatConfirm: &confirmation},
	}

	resp, err := c.SendMessages(ctx, &payload)
	if err != nil {
		return fmt.Errorf("failed to confirm chat: %v", err)
	}

	delete(newCache.Invites, chatRequest.InviteId)
	// Cache a chat when you acceot an invite
	if inviteState {
		newCache.Chats[chatRequest.Chat.Id] = chatRequest.Chat
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

	payloadChatRequest := pb.MessageStreamPayload{
		Target:  chatTarget,
		Payload: &pb.MessageStreamPayload_ChatRequest{ChatRequest: beginChat},
	}

	beginChatResponse, err := c.SendMessages(ctx, &payloadChatRequest)
	if err != nil {
		log.Fatalf("Begin Chat failed: %v", err)
		return err
	}

	fmt.Printf("Chat Request sent: %+v", beginChatResponse)

	return nil
}

// TODO: No longer fit for purpose - Terminal UI library time
func MessagingShell(c pb.StrikeClient, username string, publicKey []byte) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var inputLock sync.Mutex

	// Get messages
	go func() {
		err := ConnectMessageStream(ctx, c, username, &inputLock)
		if err != nil {
			log.Fatalf("failed to connect message stream: %v\n", err)
		}
	}()

	inputReader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter chatTarget:message to send a message (e.g., '%v:HelloWorld')\n", username)

	for {
		inputLock.Lock()
		// Prompt for input
		fmt.Print("msgshell> ")
		input, err := inputReader.ReadString('\n')
		if err != nil {
			log.Printf("Error reading input: %v\n", err)
			continue
		}
		inputLock.Unlock()

		input = strings.TrimSpace(input)

		isCommand := strings.HasPrefix(input, "/")

		if isCommand {
			switch input {
			case "/beginchat":
				shellBeginChat(c, username, inputReader)
			case "/chats":
				shellChat(inputReader)
			case "/invites":
				shellInvites(ctx, c)
			case "/help":
				// TODO: Line to long
				fmt.Print("---Available Commands---\n,",
					"/beginchat: Invite a User to a Chat\n",
					"/chats: List joined chats and set one to active\n",
					"/invites: See and respond to Chat Invites\n",
					"/exit: ...\n",
				)
			case "/exit":
				// TODO: Handle a cancel serverside
				cancel()
				fmt.Println("Exiting msgshell...")
				return
			default:
				fmt.Printf("Unknown command: %s\n", input)
			}
		} else {
			if input == "" {
				continue
			}

			userAndMessage := strings.SplitN(input, ":", 2) // Check for : then splint into target and message
			if len(userAndMessage) != 2 && input != "" {
				fmt.Println("Invalid format. Use recipient:message")
				continue
			}

			target, message := userAndMessage[0], userAndMessage[1]

			// TODO: Stopgap handle this elsewhere
			if newCache.ActiveChat == "" {
				fmt.Println("No chat has been selected. Use /chats to enable a chat first")
				continue
			}

			SendMessage(c, username, publicKey, target, message)
			// Print what was sent to the shell for full chat history
			fmt.Printf("[YOU]: %s\n", message)
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

func shellInvites(ctx context.Context, c pb.StrikeClient) {
	if len(newCache.Invites) == 0 {
		fmt.Println("No pending invites :^[")
	} else {
		fmt.Println("Pending Invites")

		for inviteID, chatRequest := range newCache.Invites {
			fmt.Printf("%v: %s [FROM: %s]\n", inviteID, chatRequest.Chat.Name, chatRequest.Initiator)
			fmt.Printf("y[Accept]/n[Decline]")

			fmt.Print("> ")
			inputReader := bufio.NewReader(os.Stdin)

			input, err := inputReader.ReadString('\n')
			if err != nil {
				log.Printf("Error reading input: %v\n", err)
				continue
			}

			input = strings.TrimSpace(input)
			accpeted := strings.ToLower(input) == "y"

			if accpeted {
				err = ConfirmChat(ctx, c, chatRequest, true)
				if err != nil {
					log.Fatalf("Failed to decline invite: %v", err)
				}
			} else {
				err = ConfirmChat(ctx, c, chatRequest, false)
				if err != nil {
					log.Fatalf("Failed to decline invite: %v", err)
				}
			}
		}
	}
}

func shellChat(inputReader *bufio.Reader) {
	if len(newCache.Chats) == 0 {
		fmt.Println("You haven't joined any Chats")
	} else {
		fmt.Println("Available Chats:")

		chatList := make([]string, 0, len(newCache.Chats))
		index := 1

		for chatID, chat := range newCache.Chats {
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
		newCache.ActiveChat = selectedChatID
		fmt.Printf("Active chat: %s\n", newCache.Chats[selectedChatID].Name)
	}
}

func shellBeginChat(c pb.StrikeClient, username string, inputReader *bufio.Reader) {
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

	err = BeginChat(c, username, inviteUser, chatName)
	// TODO: Not fatal?
	if err != nil {
		log.Fatalf("error beginning chat: %v", err)
	}
}
