package client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func AutoChat(c pb.StrikeClient, username string, pubkey []byte) error {

	newChat := pb.Chat{
		Name:    "endpoint0",
		Message: "Hello from client0",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//TODO: This has to go
	go func() {
		for {
			now := time.Now()
			timestamp := timestamppb.New(now)

			newEnvelope := pb.Envelope{
				SenderPublicKey: pubkey,
				SentAt:          timestamp,
				Chat:            &newChat,
			}

			stamp, err := c.SendMessages(ctx, &newEnvelope)
			if err != nil {
				log.Fatalf("SendMessages Failed: %v", err)
			}

			fmt.Println("Stamp: ", stamp.KeyUsed)

			time.Sleep(1 * time.Minute)
		}
	}()

	//Pass your own username to register your stream
	stream, err := c.GetMessages(ctx, &pb.Username{Username: username})
	if err != nil {
		log.Fatalf("GetMessages Failed: %v", err)
		return err
	}

	for {
		messageStream, err := stream.Recv()
		if err == io.EOF {
			log.Printf("Recieve Failed EOF: %v", err)
			log.Printf("Awaiting Messages")
			continue
		}
		if err != nil {
			log.Fatalf("Recieve Failed: %v", err)
			return err
		}
		fmt.Printf("Chat Name: %s\n", messageStream.Chat.Name)
		fmt.Printf("Message: %s\n", messageStream.Chat.Message)
	}

}

// TODO: Take this out to the main client function and we can have an easier time manipulating.
func ConnectMessageStream(ctx context.Context, c pb.StrikeClient, username string) error {

	//Pass your own username to register your stream
	stream, err := c.GetMessages(ctx, &pb.Username{Username: username})
	if err != nil {
		log.Fatalf("GetMessages Failed: %v", err)
		return err
	}

	fmt.Println("Message Stream Connected: Listening...")

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			log.Println("Stream closed by server.")
			break
		}
		if err != nil {
			log.Fatalf("Error receiving message: %v", err)
		}

		fmt.Printf("[%s] [%s] [From:%s] : %s\n", msg.SentAt.AsTime(), msg.Chat.Name, msg.FromUser, msg.Chat.Message)
	}

	//TODO: Modify the type here, pass either a Envelope or a System envelope
	// if messageStream.Chat.Name == "SERVER-CHAT_REQUEST" {
	// 	log.Printf("\nWE CAN SEE A CHAT REQUEST\n")
	// 	//TODO: Chaining these is a mess
	// 	ConfirmChat(ctx, c, username, messageStream)

	// }
	return nil
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

func ClientSignup(c pb.StrikeClient, username string, curve25519key []byte, ed25519key []byte) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	initClient := pb.ClientInit{
		Username:      username,
		EncryptionKey: curve25519key,
		SigningKey:    ed25519key,
	}

	serverRes, err := c.Signup(ctx, &initClient)
	if err != nil {
		log.Fatalf("signup failed: %v", err)
		return err
	}

	fmt.Printf("Server Response: %+v\n", serverRes)
	return nil
}

func Login(c pb.StrikeClient, username string) error {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	getOnline := pb.StatusRequest{
		Username: username,
	}

	stream, err := c.UserStatus(ctx, &getOnline)
	if err != nil {
		log.Fatalf("Login Failed: %v", err)
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
			cancel() //TODO: Handle a cancel serverside
			fmt.Println("Exiting msgshell...")
			return
		}

		userAndMessage := strings.SplitN(input, ":", 2) //Check for : then splint into target and message
		if len(userAndMessage) != 2 {
			fmt.Println("Invalid format. Use recipient:message")
			continue
		}

		target, message := userAndMessage[0], userAndMessage[1]
		fmt.Printf("[YOU]: %s\n", message)
		SendMessage(c, username, publicKey, target, message, "The Foreign Policy of the Bulgarian Police Force")
	}
}
