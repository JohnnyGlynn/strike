package client

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/JohnnyGlynn/strike/internal/auth"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
	"github.com/google/uuid"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type ClientCache struct {
	Invites map[string]*pb.BeginChatRequest
	Chats   map[string]*pb.Chat
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

// TODO: Take this out to the main client function and we can have an easier time manipulating.
func ConnectMessageStream(ctx context.Context, c pb.StrikeClient, username string, inputLock *sync.Mutex) error {
	// Pass your own username to register your stream
	stream, err := c.MessageStream(ctx, &pb.Username{Username: username})
	if err != nil {
		log.Fatalf("MessageStream Failed: %v", err)
		return err
	}

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

			switch payload := msg.Payload.(type) {
			case *pb.MessageStreamPayload_Envelope:
				fmt.Printf("[%s] [%s] [From:%s] : %s\n", payload.Envelope.SentAt.AsTime(), payload.Envelope.Chat.Name, payload.Envelope.FromUser, payload.Envelope.Message)
			case *pb.MessageStreamPayload_ChatRequest:
				chatRequest := payload.ChatRequest

				fmt.Printf("Chat Invite recieved from:%v Chat Name: %v\n", chatRequest.Initiator, chatRequest.ChatName)

				// Recieve an invite, cache it
				newCache.Invites[chatRequest.InviteId] = chatRequest

			case *pb.MessageStreamPayload_ChatConfirm:
				chatConfirm := payload.ChatConfirm

				// Cache a chat when your invitation is accepted
				if chatConfirm.State {
					fmt.Printf("Invitation %v for:%s, With: %s, Status: Accepted\n", chatConfirm.InviteId, chatConfirm.ChatName, chatConfirm.Confirmer)
					newCache.Chats[chatConfirm.Chat.Id] = chatConfirm.Chat
				} else {
					fmt.Printf("Invitation %v for:%s, With: %s, Status: Declined\n", chatConfirm.InviteId, chatConfirm.ChatName, chatConfirm.Confirmer)
				}
			}
		}
	}
}

func SendMessage(c pb.StrikeClient, username string, publicKey []byte, target string, message string, chatName string) {
	envelope := pb.Envelope{
		SenderPublicKey: publicKey,
		SentAt:          timestamppb.Now(),
		FromUser:        username,
		ToUser:          target,
		Chat: &pb.Chat{
			Id:   uuid.New().String(), // TODO: Hook in actual chats here
			Name: chatName,
		},
		Message: message,
	}

	payloadEnvelope := pb.MessageStreamPayload{
		Target:  target,
		Payload: &pb.MessageStreamPayload_Envelope{Envelope: &envelope},
	}

	// resp, err := c.SendMessages(context.Background(), envelope)
	_, err := c.SendMessages(context.Background(), &payloadEnvelope)
	if err != nil {
		log.Fatalf("Error sending message: %v", err)
	}
}

func ConfirmChat(ctx context.Context, c pb.StrikeClient, chatRequest *pb.BeginChatRequest, inviteState bool) error {
	confirmation := pb.ConfirmChatRequest{
		InviteId:  chatRequest.InviteId,
		ChatName:  chatRequest.ChatName,
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
	fmt.Printf("Chat invite acknowledged: %+v", resp)

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
		ChatName:  "General",
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
				fmt.Print("Invite User> ")
				inviteUser, err := inputReader.ReadString('\n')
				if err != nil {
					log.Printf("Error reading invite input user: %v\n", err)
					continue
				}
				inviteUser = strings.TrimSpace(inviteUser)

				fmt.Print("Chat Name> ")
				chatName, err := inputReader.ReadString('\n')
				if err != nil {
					log.Printf("Error reading invite input chat name: %v\n", err)
					continue
				}
				chatName = strings.TrimSpace(chatName)

				err = BeginChat(c, username, inviteUser, chatName)
				// TODO: Not fatal?
				if err != nil {
					log.Fatalf("error beginning chat: %v", err)
				}
			case "/chats":
				if len(newCache.Chats) == 0 {
					fmt.Println("You haven't joined any Chats")
				} else {
					fmt.Println("Available Chats")
					for chatId, chat := range newCache.Chats {
						fmt.Printf("%v: %s\n", chatId, chat.Name)
					}
				}

			case "/invites":
				if len(newCache.Invites) == 0 {
					fmt.Println("No pending invites :^[")
				} else {
					fmt.Println("Pending Invites")
					for inviteID, chatRequest := range newCache.Invites {
						fmt.Printf("%v: %s [FROM: %s]\n", inviteID, chatRequest.ChatName, chatRequest.Initiator)
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
			case "/exit":
				// TODO: Handle a cancel serverside
				cancel()
				fmt.Println("Exiting msgshell...")
				return
			default:
				fmt.Printf("Unknown command: %s\n", input)
			}
		} else {
			userAndMessage := strings.SplitN(input, ":", 2) // Check for : then splint into target and message
			if len(userAndMessage) != 2 {
				fmt.Println("Invalid format. Use recipient:message")
				continue
			}

			target, message := userAndMessage[0], userAndMessage[1]

			// Print what was sent to the shell for full chat history
			fmt.Printf("[YOU]: %s\n", message)
			SendMessage(c, username, publicKey, target, message, "The Foreign Policy of the Bulgarian Police Force")
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
