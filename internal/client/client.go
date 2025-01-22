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

	"github.com/JohnnyGlynn/strike/internal/auth"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// TODO: Take this out to the main client function and we can have an easier time manipulating.
func ConnectMessageStream(ctx context.Context, c pb.StrikeClient, username string) error {

	//Pass your own username to register your stream
	stream, err := c.GetMessages(ctx, &pb.Username{Username: username})
	if err != nil {
		log.Fatalf("GetMessages Failed: %v", err)
		return err
	}

	//Interfering with output/input in shell
	// fmt.Println("Message Stream Connected: Listening...")

	for {
		select {
		case <-ctx.Done():
			//Graceful exit
			log.Println("Message stream context canceled. Exiting...")
			return nil
		default:
			msg, err := stream.Recv()
			if err == io.EOF {
				log.Println("Stream closed by server.")
				return nil
			}
			if err != nil {
				log.Printf("Error receiving message: %v", err)
				return err
			}

			fmt.Printf("[%s] [%s] [From:%s] : %s\n", msg.SentAt.AsTime(), msg.Chat.Name, msg.FromUser, msg.Chat.Message)
		}
	}

	//TODO: Modify the type here, pass either a Envelope or a System envelope
	// if messageStream.Chat.Name == "SERVER-CHAT_REQUEST" {
	// 	log.Printf("\nWE CAN SEE A CHAT REQUEST\n")
	// 	//TODO: Chaining these is a mess
	// 	ConfirmChat(ctx, c, username, messageStream)

	// }
}

func SendMessage(c pb.StrikeClient, username string, publicKey []byte, target string, message string, chatName string) {

	envelope := &pb.Envelope{
		SenderPublicKey: publicKey,
		SentAt:          timestamppb.Now(),
		FromUser:        username,
		ToUser:          target,
		Chat: &pb.Chat{
			Name:    chatName,
			Message: message,
		},
	}

	// resp, err := c.SendMessages(context.Background(), envelope)
	_, err := c.SendMessages(context.Background(), envelope)
	if err != nil {
		log.Fatalf("Error sending message: %v", err)
	}

	// fmt.Printf("Stamp: %v", resp.KeyUsed)

	//Implement Send status into SendMessages
	// if success{
	// 	fmt.Println("Message sent successfully.")
	// } else {
	// 	fmt.Printf("Failed to send message: %s\n", error)
	// }

}

func ConfirmChat(ctx context.Context, c pb.StrikeClient, username string, chatContent *pb.Envelope) error {

	ConfirmChatResp, err := c.ConfirmChat(ctx, &pb.ConfirmChatRequest{ChatId: chatContent.Chat.Message, Confirmer: username})
	if err != nil {
		log.Fatalf("GetMessages Failed: %v", err)
		return err
	}

	fmt.Printf("Chat Confirmed: %+v", ConfirmChatResp)

	return nil
}

func ClientSignup(c pb.StrikeClient, username string, password string, curve25519key []byte, ed25519key []byte) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	salt := make([]byte, 16)
	//add salt
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

	//TODO: handle this elsewhere?
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

func BeginChat(c pb.StrikeClient, username string, chatTarget string) error {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	beginChat := pb.BeginChatRequest{
		Initiator: username,
		Target:    chatTarget,
		ChatName:  "General",
	}

	beginChatResponse, err := c.BeginChat(ctx, &beginChat)
	if err != nil {
		log.Fatalf("Begin Chat failed Failed: %v", err)
		return err
	}

	fmt.Printf("Chat Created: %+v", beginChatResponse)

	return nil
}

func MessagingShell(c pb.StrikeClient, username string, publicKey []byte) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//Get messages
	go func() {
		ConnectMessageStream(ctx, c, username)
	}()

	inputReader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter chatTarget:message to send a message (e.g., '%v:HelloWorld')\n", username)

	for {
		// Prompt for input
		fmt.Print("msgshell> ")
		input, err := inputReader.ReadString('\n')
		if err != nil {
			log.Printf("Error reading input: %v\n", err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "/exit" {
			//TODO: Handle a cancel serverside
			cancel()
			fmt.Println("Exiting msgshell...")
			return
		}

		userAndMessage := strings.SplitN(input, ":", 2) //Check for : then splint into target and message
		if len(userAndMessage) != 2 {
			fmt.Println("Invalid format. Use recipient:message")
			continue
		}

		target, message := userAndMessage[0], userAndMessage[1]

		//Print what was sent to the shell for full chat history
		fmt.Printf("[YOU]: %s\n", message)
		SendMessage(c, username, publicKey, target, message, "The Foreign Policy of the Bulgarian Police Force")
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
