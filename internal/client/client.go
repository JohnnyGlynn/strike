package client

import (
	"context"
	"fmt"
	"io"
	"log"
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

			time.Sleep(5 * time.Second)
		}
	}()

	stream, err := c.GetMessages(ctx, &newChat)
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
